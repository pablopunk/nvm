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
  onChange?: () => void;
};

export function createStateMaintenanceService(deps: StateMaintenanceDeps) {
  const acquireLease = deps.acquireLease ?? acquireSharedStateLease;
  let status: StateVerificationStatus = deps.roots()
    ? { phase: 'idle', trusted: false }
    : { phase: 'unsupported' };
  let running: Promise<StateVerificationStatus> | null = null;

  function publish(next: StateVerificationStatus) {
    status = next;
    deps.onChange?.();
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

  async function verifyWithWritersPaused(roots: StateMaintenanceRoots) {
    const paused: PausableWriter[] = [];
    let lease: SharedStateLease | undefined;
    try {
      await pauseWriters(paused);
      await deps.flushPendingState();
      await fs.mkdir(roots.backupParent, { recursive: true, mode: 0o700 });
      lease = await acquireLease(roots.sharedRoot);
      const { backupPath } = await backupAndTrustState({
        ...roots,
        epoch: deps.epoch,
        lease,
        legacyInstancesClosed: true,
        verifyNoUncoordinatedWriters: async () =>
          deps.ownsSingleInstance() && !deps.isUpdateInstalling(),
      });
      return backupPath;
    } finally {
      try {
        await lease?.release();
      } finally {
        resumeWriters(paused);
      }
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

  return { status: () => status, refresh, verifyAndTrust };
}

export type StateMaintenanceService = ReturnType<
  typeof createStateMaintenanceService
>;
