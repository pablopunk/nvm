import {
  error as logError,
  info as logInfo,
  warn as logWarn,
} from '../platform/logger';
import {
  applyPortableConfigToRecord,
  diffPortableConfig,
  portableConfigFromRecord,
} from '../state/portable-config-projection';
import {
  createDefaultPortableConfig,
  type PortableConfig,
  type PortableSettingKinds,
} from '../state/portable-config-schema';
import { createPortableConfigStore } from '../state/portable-config-store';
import {
  changedKeysOf,
  diskViewOf,
  isLocalChange,
  isPortableActionId,
  mergeDiskView,
} from '../state/portable-keys';
import type { UserStateStore } from '../state/user-state-store';

export type PortableCommitInput = {
  label: string;
  actionId?: string;
  actionIsGenerated?: boolean;
  mutate: (draft: PortableConfig) => string | void;
  beforeApply?: () => () => void;
  afterCommit?: () => void;
};

export type PortableCommitResult =
  | { ok: true }
  | { ok: false; message: string };

export type LiveFailure = { key: string; message: string };

export type PortableLive = {
  apply: (
    before: PortableConfig,
    after: PortableConfig,
    keys: string[],
  ) => LiveFailure[];
  applyShortcuts: (view: PortableConfig) => LiveFailure[];
};

export type PortableConfigServiceDeps = {
  filePath: () => string;
  settingKinds: PortableSettingKinds;
  userStateStore: Pick<UserStateStore, 'get'>;
  live: PortableLive;
  scheduleSaveState: () => void;
  afterConfigAdopted?: (keys: readonly string[]) => void;
  pollMs?: number;
};

export const PORTABLE_CONFIG_POLL_MS = 1000;

const LOG_META = { source: 'host', scope: 'state' } as const;

function sameIds(left: ReadonlySet<string>, right: ReadonlySet<string>) {
  return left.size === right.size && [...left].every((id) => right.has(id));
}

function localIdsAfterCommit(
  beforeLocal: ReadonlySet<string>,
  input: PortableCommitInput,
) {
  const localIds = new Set(beforeLocal);
  const actionId = input.actionId;
  if (actionId && (input.actionIsGenerated || !isPortableActionId(actionId)))
    localIds.add(actionId);
  return localIds;
}

function hasExplicitStartAtLogin(config: PortableConfig) {
  return 'startAtLogin' in config.settings;
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: The service keeps commit, poll, and load on one serialized queue so every change sees one consistent base.
export function createPortableConfigService(deps: PortableConfigServiceDeps) {
  const { userStateStore, settingKinds, live } = deps;
  let queue: Promise<unknown> = Promise.resolve();
  let pollTimer: ReturnType<typeof setInterval> | undefined;
  const inFlight = new Set<Promise<unknown>>();
  let paused = false;
  let draining = false;
  let resumePolling = false;

  const store = createPortableConfigStore({
    filePath: deps.filePath,
    settingKinds,
    onBlockedReasonChange: (reason) =>
      reason
        ? logWarn('config.blocked', { reason }, LOG_META)
        : logInfo('config.unblocked', {}, LOG_META),
  });

  function enqueue<T>(task: () => Promise<T>): Promise<T> {
    if (paused)
      return Promise.reject(
        new Error('Portable config is paused for maintenance'),
      );
    const run = queue.then(task);
    queue = run.catch(() => undefined);
    inFlight.add(run);
    const remove = () => inFlight.delete(run);
    void run.then(remove, remove);
    return run;
  }

  function recordView() {
    return portableConfigFromRecord(userStateStore.get(), settingKinds);
  }

  function recordLocalIds() {
    return new Set<string>(userStateStore.get().localActionIds ?? []);
  }

  function writeRecord(view: PortableConfig, localIds: ReadonlySet<string>) {
    const record = userStateStore.get();
    applyPortableConfigToRecord(record, view);
    record.localActionIds = [...localIds];
  }

  function reportFailures(failures: LiveFailure[]) {
    for (const failure of failures)
      logWarn('config.live.failed', failure, LOG_META);
  }

  function adoptView(
    previous: PortableConfig,
    next: PortableConfig,
    localIds: ReadonlySet<string>,
  ) {
    const keys = changedKeysOf(diffPortableConfig(previous, next));
    writeRecord(next, localIds);
    if (keys.length) reportFailures(live.apply(previous, next, keys));
    if (keys.length) deps.afterConfigAdopted?.(keys);
  }

  async function commitNow(
    input: PortableCommitInput,
  ): Promise<PortableCommitResult> {
    const before = recordView();
    const beforeLocal = recordLocalIds();
    const draft = structuredClone(before);
    const error = input.mutate(draft);
    if (typeof error === 'string') return { ok: false, message: error };
    const localIds = localIdsAfterCommit(beforeLocal, input);
    const changes = diffPortableConfig(before, draft);
    if (!changes.length && sameIds(localIds, beforeLocal)) return { ok: true };

    const keys = changedKeysOf(changes);
    const undoPreparation = input.beforeApply?.();
    const blocking = live
      .apply(before, draft, keys)
      .find((failure) => keys.includes(failure.key));
    if (blocking) {
      undoPreparation?.();
      live.apply(draft, before, keys);
      return { ok: false, message: blocking.message };
    }

    let next = draft;
    if (changes.some((change) => !isLocalChange(change, localIds))) {
      const persisted = await store.persist(diskViewOf(draft, localIds));
      if (persisted.ok === false) {
        undoPreparation?.();
        live.apply(draft, before, keys);
        adoptView(
          before,
          mergeDiskView(before, persisted.current, localIds),
          localIds,
        );
        logWarn(
          'config.write.rejected',
          { label: input.label, reason: persisted.reason },
          LOG_META,
        );
        return { ok: false, message: persisted.reason };
      }
      next = mergeDiskView(draft, persisted.config, localIds);
    }
    adoptView(draft, next, localIds);
    input.afterCommit?.();
    deps.scheduleSaveState();
    return { ok: true };
  }

  async function pollNow() {
    const result = await store.pollForExternalChange();
    if (!result.adopted) return;
    const previous = recordView();
    const localIds = recordLocalIds();
    adoptView(
      previous,
      mergeDiskView(previous, result.adopted, localIds),
      localIds,
    );
  }

  async function loadNow() {
    const snapshot = await store.load();
    const localIds = recordLocalIds();
    const current = mergeDiskView(recordView(), snapshot.config, localIds);
    writeRecord(current, localIds);
    if (hasExplicitStartAtLogin(snapshot.config))
      reportFailures(
        live.apply(createDefaultPortableConfig(), current, [
          'setting:startAtLogin',
        ]),
      );
  }

  function refreshShortcutBindings() {
    return enqueue(async () => {
      reportFailures(live.applyShortcuts(recordView()));
    });
  }

  function startPollingPortableConfig() {
    if (pollTimer || paused) return;
    pollTimer = setInterval(() => {
      enqueue(pollNow).catch((error) =>
        logError('config.poll.failed', { error: String(error) }, LOG_META),
      );
    }, deps.pollMs ?? PORTABLE_CONFIG_POLL_MS);
    pollTimer.unref?.();
  }

  function stopPollingPortableConfig() {
    if (!pollTimer) return;
    clearInterval(pollTimer);
    pollTimer = undefined;
  }

  async function quiesce() {
    if (paused) throw new Error('Portable config is already paused');
    paused = true;
    draining = true;
    resumePolling = Boolean(pollTimer);
    stopPollingPortableConfig();
    try {
      const results = await Promise.allSettled([...inFlight]);
      const failure = results.find((result) => result.status === 'rejected');
      if (failure?.status === 'rejected') throw failure.reason;
    } finally {
      draining = false;
    }
  }

  function resume() {
    if (draining) throw new Error('Portable config is still draining');
    paused = false;
    if (resumePolling) startPollingPortableConfig();
    resumePolling = false;
  }

  return {
    commit: (input: PortableCommitInput) => enqueue(() => commitNow(input)),
    loadPortableConfig: () => enqueue(loadNow),
    refreshShortcutBindings,
    startPollingPortableConfig,
    stopPollingPortableConfig,
    quiesce,
    resume,
  };
}
