import {
  error as logError,
  info as logInfo,
  warn as logWarn,
} from '../platform/logger';
import {
  applyPortableConfigToRecord,
  portableConfigFromRecord,
} from '../state/portable-config-projection';
import type {
  PortableConfig,
  PortableSettingKinds,
} from '../state/portable-config-schema';
import { createPortableConfigStore } from '../state/portable-config-store';
import type { UserStateStore } from '../state/user-state-store';
import {
  type PortableConfigEffects,
  reconcilePortableConfigChange,
} from './portable-config-reconcile';

export const PORTABLE_CONFIG_POLL_MS = 1000;

export type PortableConfigServiceDeps = {
  filePath: () => string;
  settingKinds: PortableSettingKinds;
  userStateStore: Pick<UserStateStore, 'get'>;
  effects: PortableConfigEffects;
  pollMs?: number;
};

function describeError(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export function createPortableConfigService(deps: PortableConfigServiceDeps) {
  const { userStateStore, settingKinds, effects } = deps;
  let pollTimer: ReturnType<typeof setInterval> | undefined;

  const store = createPortableConfigStore({
    filePath: deps.filePath,
    settingKinds,
    onConflict: (paths) =>
      logWarn(
        'config.write.conflict',
        { paths },
        { source: 'host', scope: 'state' },
      ),
    onBlockedReasonChange: (reason) => {
      if (reason)
        logWarn(
          'config.blocked',
          { reason },
          { source: 'host', scope: 'state' },
        );
      else logInfo('config.unblocked', {}, { source: 'host', scope: 'state' });
    },
  });

  function currentPortableConfig() {
    return portableConfigFromRecord(userStateStore.get(), settingKinds);
  }

  function adoptConfig(next: PortableConfig) {
    const previous = currentPortableConfig();
    applyPortableConfigToRecord(userStateStore.get(), next);
    reconcilePortableConfigChange(previous, next, effects);
  }

  async function loadPortableConfig() {
    const snapshot = await store.load();
    applyPortableConfigToRecord(userStateStore.get(), snapshot.config);
  }

  async function persistPortableConfig() {
    const result = await store.persist(currentPortableConfig());
    if (!result.ok)
      logWarn(
        'config.write.rejected',
        { reason: result.reason },
        { source: 'host', scope: 'state' },
      );
    adoptConfig(result.config);
  }

  async function pollPortableConfig() {
    const result = await store.pollForExternalChange();
    if (result.adopted) adoptConfig(result.adopted);
  }

  function startPollingPortableConfig() {
    if (pollTimer) return;
    pollTimer = setInterval(() => {
      pollPortableConfig().catch((error) =>
        logError(
          'config.poll.failed',
          { error: describeError(error) },
          { source: 'host', scope: 'state' },
        ),
      );
    }, deps.pollMs ?? PORTABLE_CONFIG_POLL_MS);
    pollTimer.unref?.();
  }

  function stopPollingPortableConfig() {
    if (!pollTimer) return;
    clearInterval(pollTimer);
    pollTimer = undefined;
  }

  return {
    loadPortableConfig,
    persistPortableConfig,
    startPollingPortableConfig,
    stopPollingPortableConfig,
  };
}
