import fs from 'node:fs/promises';
import {
  acquireSharedStateLease,
  type SharedStateLease,
} from '../platform/shared-state-lease';
import {
  backupAndTrustState,
  stateRootsSupportDowngrade,
} from '../state/state-compatibility';

type PausableWriter = {
  quiesce: () => Promise<unknown>;
  resume: () => void;
};

export type StateMaintenanceRoots = {
  privateRoot: string;
  sharedRoot: string;
  backupParent: string;
};

export type StateVerificationStatus =
  | { phase: 'unsupported' }
  | { phase: 'idle'; trusted: boolean }
  | { phase: 'running' }
  | { phase: 'complete'; backupPath: string }
  | { phase: 'failed'; message: string; trusted: boolean };

export type StateMaintenanceDeps = {
  /** Interactive and background writers, ordered from outermost to innermost. */
  writers: readonly PausableWriter[];
  flushPendingState: () => Promise<void>;
  roots: () => StateMaintenanceRoots | null;
  epoch: number;
  ownsSingleInstance: () => boolean;
  isUpdateInstalling: () => boolean;
  acquireLease?: (sharedRoot: string) => Promise<SharedStateLease>;
};

export function createStateMaintenanceService(deps: StateMaintenanceDeps) {
  const acquireLease = deps.acquireLease ?? acquireSharedStateLease;
  let status: StateVerificationStatus = deps.roots()
    ? { phase: 'idle', trusted: false }
    : { phase: 'unsupported' };
  let running: Promise<StateVerificationStatus> | null = null;
  let leaseHeld: Promise<void> = Promise.resolve();
  let quitting = false;
  const listeners = new Set<() => void>();

  function publish(next: StateVerificationStatus) {
    status = next;
    for (const listener of listeners) listener();
  }

  async function currentTrust(roots: StateMaintenanceRoots) {
    return stateRootsSupportDowngrade(
      [roots.privateRoot, roots.sharedRoot],
      deps.epoch,
    );
  }

  async function refresh() {
    const roots = deps.roots();
    if (!roots) return publish({ phase: 'unsupported' });
    if (status.phase === 'running' || status.phase === 'complete') return;
    publish({ phase: 'idle', trusted: await currentTrust(roots) });
  }

  async function pauseWriters(paused: PausableWriter[]) {
    for (const writer of deps.writers) {
      paused.push(writer);
      await writer.quiesce();
    }
  }

  function resumeWriters(paused: readonly PausableWriter[]) {
    const failures: unknown[] = [];
    for (const writer of [...paused].reverse()) {
      try {
        writer.resume();
      } catch (error) {
        failures.push(error);
      }
    }
    if (failures.length) throw failures[0];
  }

  async function withLease<T>(
    sharedRoot: string,
    use: (lease: SharedStateLease) => Promise<T>,
  ) {
    let markReleased!: () => void;
    leaseHeld = new Promise<void>((resolve) => {
      markReleased = resolve;
    });
    let lease: SharedStateLease | undefined;
    try {
      if (quitting) throw new Error('Nevermind is quitting');
      lease = await acquireLease(sharedRoot);
      if (quitting) throw new Error('Nevermind is quitting');
      return await use(lease);
    } finally {
      try {
        await lease?.release();
      } finally {
        markReleased();
      }
    }
  }

  async function verifyWithWritersPaused(roots: StateMaintenanceRoots) {
    const paused: PausableWriter[] = [];
    try {
      await pauseWriters(paused);
      await deps.flushPendingState();
      await fs.mkdir(roots.backupParent, { recursive: true, mode: 0o700 });
      return await withLease(roots.sharedRoot, async (lease) => {
        const { backupPath } = await backupAndTrustState({
          ...roots,
          epoch: deps.epoch,
          lease,
          legacyInstancesClosed: true,
          verifyNoUncoordinatedWriters: async () =>
            deps.ownsSingleInstance() && !deps.isUpdateInstalling(),
        });
        return backupPath;
      });
    } finally {
      resumeWriters(paused);
    }
  }

  async function run(): Promise<StateVerificationStatus> {
    const roots = deps.roots();
    if (!roots) return status;
    publish({ phase: 'running' });
    try {
      const backupPath = await verifyWithWritersPaused(roots);
      publish({ phase: 'complete', backupPath });
    } catch (error) {
      publish({
        phase: 'failed',
        message: error instanceof Error ? error.message : String(error),
        trusted: await currentTrust(roots).catch(() => false),
      });
    }
    return status;
  }

  /**
   * Starts a backup and trust transaction. The caller attests that older
   * Nevermind instances are closed; the transaction itself cannot detect them.
   * Returns the shared in-flight run so a repeated request never overlaps.
   */
  function verifyAndTrust() {
    if (running) return running;
    running = run().finally(() => {
      running = null;
    });
    return running;
  }

  return {
    status: () => status,
    refresh,
    verifyAndTrust,
    /**
     * Stops new transactions from taking the shared-state lease and resolves
     * once any transaction already holding it has released it.
     */
    prepareForQuit() {
      quitting = true;
      return leaseHeld;
    },
    onChange(listener: () => void) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}

export type StateMaintenanceService = ReturnType<
  typeof createStateMaintenanceService
>;
