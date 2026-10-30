import assert from 'node:assert/strict';
import test, { mock } from 'node:test';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import type { StateVerificationStatus } from './state-maintenance-service';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: true },
    shell: {},
    systemPreferences: { isTrustedAccessibilityClient: () => false },
  },
});

mock.module('electron-log/main', {
  defaultExport: {
    error() {},
    info() {},
  },
});

const { createUpdateManager } = await import('./update-manager');
const { createUpdatesService } = await import('./updates-service');
const { isStateSafeForInstall } = await import('./state-update-safety');
const { advanceStateCompatibility } = await import(
  '../state/state-compatibility'
);

type FakeCancellationToken = { cancelled: boolean; cancel: () => void };

test('a selected older channel build is visible without treating it as a backend compatibility update', () => {
  const manager = createUpdateManager(createAutoUpdater([]));
  manager.state.availableInfo = {
    version: '2026.1029.4',
    updateChannel: 'stable',
  };
  const service = createUpdatesService({
    updateManager: manager,
    getAppVersion: () => '2026.1030.5',
    autoUpdatesUnavailableMessage: () => '',
    getActiveNevermindBaseUrl: () => null,
    getUpdateChannel: () => 'stable',
    markAppQuitting: () => {},
    requestQuit: async () => {},
    quitApp: () => {},
    patchOpenView: () => {},
    stateMaintenance: unsupportedStateMaintenance(),
  });
  assert.equal(service.updatesStateSnapshot().availableVersion, '2026.1029.4');
  assert.equal(service.updatePromptAction()?.id, 'updates:download');
  assert.notEqual(
    service.updateActionForCompatibilityPrompt()?.type,
    'downloadUpdate',
  );
});

test('installation rechecks both state roots and target epoch after download', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-install-guard-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const privateRoot = path.join(root, 'private');
  const sharedRoot = path.join(root, 'shared');
  await advanceStateCompatibility(privateRoot, 1, sharedRoot);
  for (const directory of [privateRoot, sharedRoot])
    await fs.writeFile(
      path.join(directory, 'state-compatibility.json'),
      JSON.stringify({ version: 1, highestEpoch: 1, trusted: true }),
    );
  const manager = createUpdateManager(createAutoUpdater([]), undefined, {
    getUpdateChannel: () => 'stable',
    canInstallUpdate: (info) =>
      isStateSafeForInstall('2026.1030.5', info, [privateRoot, sharedRoot]),
  });
  manager.state.downloadedInfo = {
    version: '2026.1029.4',
    updateChannel: 'stable',
    nevermindStateEpoch: 1,
  };
  await fs.rm(path.join(sharedRoot, 'state-compatibility.json'));
  assert.equal(await manager.prepareInstall(), false);
  assert.equal(manager.state.installInFlight, false);
});

test('asynchronous install check cannot authorize a stale channel', async () => {
  let channel: 'stable' | 'nightly' = 'stable';
  let finishCheck: (value: boolean) => void = () => {};
  const manager = createUpdateManager(createAutoUpdater([]), undefined, {
    getUpdateChannel: () => channel,
    canInstallUpdate: () =>
      new Promise<boolean>((resolve) => {
        finishCheck = resolve;
      }),
  });
  manager.state.downloadedInfo = { version: '0.16.6', updateChannel: 'stable' };
  const pending = manager.prepareInstall();
  channel = 'nightly';
  finishCheck(true);
  assert.equal(await pending, false);
  assert.equal(manager.state.installInFlight, false);
});

test('a failed asynchronous state check blocks installation without requesting quit', async () => {
  const calls: string[] = [];
  const manager = createUpdateManager(createAutoUpdater(calls), undefined, {
    canInstallUpdate: async () => {
      throw new Error('state unavailable');
    },
  });
  manager.state.downloadedInfo = { version: '0.16.6', updateChannel: 'stable' };
  assert.equal(await manager.prepareInstall(), false);
  assert.equal(manager.state.installInFlight, false);
  assert.equal(manager.quitAndInstall(), false);
  assert.deepEqual(calls, []);
});

function createFakeCancellationToken(): FakeCancellationToken {
  return {
    cancelled: false,
    cancel() {
      this.cancelled = true;
    },
  };
}

function createEventedAutoUpdater(calls: string[]) {
  const listeners = new Map<string, Array<(...args: any[]) => void>>();
  return {
    ...createAutoUpdater(calls),
    on(event: string, listener: (...args: any[]) => void) {
      listeners.set(event, [...(listeners.get(event) ?? []), listener]);
    },
    emit(event: string, ...args: unknown[]) {
      for (const listener of listeners.get(event) ?? []) listener(...args);
    },
  };
}

function createAutoUpdater(calls: string[]) {
  return {
    autoDownload: false,
    autoInstallOnAppQuit: false,
    allowDowngrade: false,
    checkForUpdates: async () => {
      calls.push('check');
      return {
        isUpdateAvailable: false,
        updateInfo: { version: '0.16.5' },
      };
    },
    downloadUpdate: async () => {
      calls.push('download');
    },
    quitAndInstall: () => calls.push('install'),
    on: () => {},
  };
}

test('does not turn a missing update into a download error', async () => {
  const calls: string[] = [];
  const manager = createUpdateManager(createAutoUpdater(calls));

  await manager.downloadAvailableUpdate();

  assert.deepEqual(calls, []);
  assert.equal(manager.state.status, 'idle');
  assert.equal(manager.state.errorMessage, '');
});

test('does not download when the update check reports no update', async () => {
  const calls: string[] = [];
  const manager = createUpdateManager(createAutoUpdater(calls));

  await manager.checkForUpdates('manual', { download: true });

  assert.deepEqual(calls, ['check']);
  assert.equal(manager.state.status, 'idle');
  assert.equal(manager.state.errorMessage, '');
});

test('a build without an updater target never checks for updates', async () => {
  const calls: string[] = [];
  const manager = createUpdateManager(createAutoUpdater(calls), undefined, {
    isUpdaterAvailable: () => false,
  });

  await manager.checkForUpdates('manual', { download: true });

  assert.deepEqual(calls, []);
  assert.equal(manager.canUseAutoUpdates(), false);
  assert.equal(manager.state.status, 'unsupported');
});

test('keeps genuine update check failures as errors', async () => {
  const calls: string[] = [];
  const manager = createUpdateManager({
    ...createAutoUpdater(calls),
    checkForUpdates: async () => {
      calls.push('check');
      throw new Error('network unavailable');
    },
  });

  await manager.checkForUpdates();

  assert.deepEqual(calls, ['check']);
  assert.equal(manager.state.status, 'error');
  assert.equal(manager.state.errorMessage, 'network unavailable');
  assert.equal(manager.state.checkInFlight, false);
});

test('exposes installing state before the deferred updater restart', async () => {
  const calls: string[] = [];
  const manager = createUpdateManager(createAutoUpdater(calls));
  manager.state.downloadedInfo = { version: '0.16.6', updateChannel: 'stable' };

  assert.equal(await manager.prepareInstall(), true);
  assert.equal(manager.state.status, 'installing');
  assert.equal(manager.state.installInFlight, true);

  assert.equal(manager.quitAndInstall(), true);
  assert.deepEqual(calls, ['install']);
});

test('configuring the updater lets the host evaluate older builds and installs the channel feed before checking', async () => {
  const calls: string[] = [];
  const updater = createEventedAutoUpdater(calls);
  const manager = createUpdateManager(updater, undefined, {
    prepareUpdateFeed: async () => {
      calls.push('feed');
    },
  });

  manager.configure();
  assert.equal(updater.allowDowngrade, true);
  await manager.checkForUpdates();

  assert.deepEqual(calls, ['feed', 'check']);
  manager.clearTimers();
});

test('ignores update results stamped for a channel the user has left', () => {
  const calls: string[] = [];
  const updater = createEventedAutoUpdater(calls);
  let channel: 'stable' | 'nightly' = 'nightly';
  const manager = createUpdateManager(updater, undefined, {
    getUpdateChannel: () => channel,
  });
  manager.configure();

  channel = 'stable';
  updater.emit('update-available', {
    version: '0.17.1-nightly.20261028.42',
    updateChannel: 'nightly',
  });
  updater.emit('update-downloaded', {
    version: '0.17.1-nightly.20261028.42',
    updateChannel: 'nightly',
  });

  assert.equal(manager.state.availableInfo, null);
  assert.equal(manager.state.downloadedInfo, null);
  assert.equal(manager.state.status, 'idle');
  manager.clearTimers();
});

test('accepts update results for the active channel', () => {
  const calls: string[] = [];
  const updater = createEventedAutoUpdater(calls);
  const manager = createUpdateManager(updater, undefined, {
    getUpdateChannel: () => 'stable',
  });
  manager.configure();

  updater.emit('update-available', {
    version: '0.16.6',
    updateChannel: 'stable',
  });
  updater.emit('update-not-available', {
    version: '0.16.6',
    updateChannel: 'stable',
  });

  assert.equal(manager.state.latestVersion, '0.16.6');
  assert.equal(manager.state.status, 'idle');
  manager.clearTimers();
});

test('does not download a check result stamped for another channel', async () => {
  const calls: string[] = [];
  const updater = {
    ...createAutoUpdater(calls),
    checkForUpdates: async () => {
      calls.push('check');
      return {
        isUpdateAvailable: true,
        updateInfo: {
          version: '0.17.1-nightly.20261028.42',
          updateChannel: 'nightly' as const,
        },
      };
    },
  };
  const manager = createUpdateManager(updater, undefined, {
    getUpdateChannel: () => 'stable',
  });

  await manager.checkForUpdates('manual', { download: true });

  assert.deepEqual(calls, ['check']);
});

test('refuses to install a downloaded build from another channel', async () => {
  const calls: string[] = [];
  const manager = createUpdateManager(createAutoUpdater(calls), undefined, {
    getUpdateChannel: () => 'stable',
  });
  manager.state.downloadedInfo = {
    version: '0.17.1-nightly.20261028.42',
    updateChannel: 'nightly',
  };

  assert.equal(await manager.prepareInstall(), false);
  assert.equal(manager.state.installInFlight, false);
});

test('changing channel cancels an in-flight download without reporting an error', async () => {
  const calls: string[] = [];
  let channel: 'stable' | 'nightly' = 'nightly';
  const tokens: FakeCancellationToken[] = [];
  const updater = createEventedAutoUpdater(calls);
  let abortDownload: (error: Error) => void = () => {};
  updater.downloadUpdate = () => {
    calls.push('download');
    return new Promise((_resolve, reject) => {
      abortDownload = reject;
    });
  };
  const manager = createUpdateManager(updater, undefined, {
    getUpdateChannel: () => channel,
    createCancellationToken: () => {
      const token = createFakeCancellationToken();
      tokens.push(token);
      return token;
    },
  });
  manager.configure();
  updater.emit('update-available', {
    version: '0.17.1-nightly.20261028.42',
    updateChannel: 'nightly',
  });

  const download = manager.downloadAvailableUpdate();
  assert.equal(manager.state.status, 'downloading');

  channel = 'stable';
  manager.handleUpdateChannelChanged();
  assert.equal(tokens[0].cancelled, true);
  assert.equal(manager.state.availableInfo, null);

  abortDownload(new Error('Download cancelled'));
  await download;

  assert.equal(manager.state.downloadInFlight, false);
  assert.equal(manager.state.errorMessage, '');
  assert.notEqual(manager.state.status, 'error');
  assert.equal(calls[0], 'download');
  manager.clearTimers();
});

function createDowngradeFixture(options: {
  permitted: () => boolean | Promise<boolean>;
  channel?: 'stable' | 'nightly';
}) {
  const calls: string[] = [];
  const updateInfo = {
    version: '2026.1029.4',
    updateChannel: options.channel ?? ('stable' as const),
    nevermindStateEpoch: 1,
  };
  const updater = {
    ...createEventedAutoUpdater(calls),
    checkForUpdates: async () => {
      calls.push('check');
      updater.emit('update-available', updateInfo);
      return { isUpdateAvailable: true, updateInfo };
    },
    downloadUpdate: async () => {
      calls.push('download');
      updater.emit('update-downloaded', updateInfo);
    },
  };
  const manager = createUpdateManager(updater, undefined, {
    getUpdateChannel: () => options.channel ?? 'stable',
    getCurrentVersion: () => '2026.1030.5',
    canInstallUpdate: async () => options.permitted(),
  });
  manager.configure();
  manager.clearTimers();
  return { calls, manager, updateInfo };
}

test('an older build is withheld until the host authorizes its state format', async () => {
  let permitted = false;
  const { calls, manager, updateInfo } = createDowngradeFixture({
    permitted: () => permitted,
  });

  await manager.checkForUpdates('manual', { download: true });
  assert.equal(manager.state.availableInfo, null);
  assert.equal(manager.state.blockedDowngradeVersion, updateInfo.version);
  assert.equal(manager.state.status, 'idle');
  await manager.downloadAvailableUpdate();
  assert.deepEqual(calls, ['check']);

  permitted = true;
  await manager.checkForUpdates('manual', { download: true });
  assert.equal(manager.state.blockedDowngradeVersion, null);
  assert.equal(manager.state.availableInfo, updateInfo);
  assert.equal(manager.state.status, 'available');
  assert.deepEqual(calls, ['check', 'check']);
});

test('an authorized older build is never downloaded automatically', async () => {
  const { calls, manager } = createDowngradeFixture({ permitted: () => true });
  await manager.checkForUpdates('poll', { download: true });
  assert.deepEqual(calls, ['check']);
  assert.equal(manager.state.downloadedInfo, null);
});

test('an explicit older-build download rechecks authorization and can be revoked', async () => {
  let permitted = true;
  const { calls, manager, updateInfo } = createDowngradeFixture({
    permitted: () => permitted,
  });
  await manager.checkForUpdates();
  permitted = false;
  await manager.downloadAvailableUpdate();
  assert.deepEqual(calls, ['check']);
  assert.equal(manager.state.availableInfo, null);
  assert.equal(manager.state.blockedDowngradeVersion, updateInfo.version);
  assert.equal(manager.state.downloadInFlight, false);
});

test('an older build downloads and installs only while authorization holds at each step', async () => {
  let permitted = true;
  const { calls, manager } = createDowngradeFixture({
    permitted: () => permitted,
  });
  await manager.checkForUpdates();
  await manager.downloadAvailableUpdate();
  assert.equal(manager.state.status, 'downloaded');
  permitted = false;
  assert.equal(await manager.prepareInstall(), false);
  assert.deepEqual(calls, ['check', 'download']);
  permitted = true;
  assert.equal(await manager.prepareInstall(), true);
});

test('a failed authorization check blocks an older build', async () => {
  const { manager, updateInfo } = createDowngradeFixture({
    permitted: async () => {
      throw new Error('unreadable markers');
    },
  });
  await manager.checkForUpdates();
  assert.equal(manager.state.availableInfo, null);
  assert.equal(manager.state.blockedDowngradeVersion, updateInfo.version);
});

test('an authorization that resolves after a channel change is discarded', async () => {
  let channel: 'stable' | 'nightly' = 'stable';
  let finish: (value: boolean) => void = () => {};
  const calls: string[] = [];
  const updateInfo = {
    version: '2026.1029.4',
    updateChannel: 'stable' as const,
    nevermindStateEpoch: 1,
  };
  const manager = createUpdateManager(
    {
      ...createEventedAutoUpdater(calls),
      checkForUpdates: async () => ({ isUpdateAvailable: true, updateInfo }),
    },
    undefined,
    {
      getUpdateChannel: () => channel,
      getCurrentVersion: () => '2026.1030.5',
      canInstallUpdate: () =>
        new Promise<boolean>((resolve) => (finish = resolve)),
    },
  );
  manager.configure();
  manager.clearTimers();
  const checking = manager.checkForUpdates();
  await new Promise<void>((resolve) => setImmediate(resolve));
  channel = 'nightly';
  finish(true);
  await checking;
  assert.equal(manager.state.availableInfo, null);
});

function unsupportedStateMaintenance() {
  return {
    status: () => ({ phase: 'unsupported' as const }),
    refresh: async () => {},
    onChange: () => () => true,
    verifyAndTrust: async () => ({ phase: 'unsupported' as const }),
  };
}

function updatesServiceWithState(
  status: StateVerificationStatus,
  verified: string[] = [],
) {
  const manager = createUpdateManager(createAutoUpdater([]));
  return createUpdatesService({
    updateManager: manager,
    getAppVersion: () => '2026.1030.5',
    autoUpdatesUnavailableMessage: () => '',
    getActiveNevermindBaseUrl: () => null,
    getUpdateChannel: () => 'stable',
    markAppQuitting: () => {},
    requestQuit: async () => {},
    quitApp: () => {},
    patchOpenView: () => {},
    stateMaintenance: {
      status: () => status,
      refresh: async () => {},
      onChange: () => () => true,
      verifyAndTrust: async () => {
        verified.push('verified');
        return status;
      },
    },
  });
}

function stateVerificationNativeAction(
  action: { type: string } | undefined,
): { kind: string; confirmation?: unknown } | undefined {
  return action && 'nativeAction' in action
    ? (action.nativeAction as { kind: string; confirmation?: unknown })
    : undefined;
}

test('the updates view offers a host-confirmed state backup only when supported', () => {
  assert.equal(
    updatesServiceWithState({ phase: 'unsupported' }).updateStatusView().items
      .length,
    1,
  );
  const view = updatesServiceWithState({
    phase: 'idle',
    trusted: false,
  }).updateStatusView();
  const item = view.items[1];
  assert.equal(item.id, 'update-state-verification');
  assert.equal(item.primaryAction?.type, 'nativeAction');
  assert.equal(
    stateVerificationNativeAction(item.primaryAction)?.kind,
    'state-verification',
  );
});

test('state backup starts only for the confirmation the host rendered', () => {
  const verified: string[] = [];
  const service = updatesServiceWithState(
    { phase: 'idle', trusted: false },
    verified,
  );
  const action = service.updateStatusView().items[1].primaryAction;
  service.verifyStateView({ confirmation: 'forged' });
  service.verifyStateView({});
  service.verifyStateView(undefined);
  assert.deepEqual(verified, []);
  service.verifyStateView(stateVerificationNativeAction(action));
  assert.deepEqual(verified, ['verified']);
});

test('the updates view reports running, completed, and failed state backups', () => {
  const titles = [
    [{ phase: 'running' }, 'Backing up and verifying state...'],
    [{ phase: 'complete', backupPath: '/b' }, 'State backed up and verified'],
    [
      { phase: 'failed', message: 'boom', trusted: false },
      'State backup failed',
    ],
    [{ phase: 'idle', trusted: true }, 'Older builds can be installed'],
  ] as const;
  for (const [status, title] of titles) {
    const view = updatesServiceWithState(status).updateStatusView();
    assert.equal(view.items[1].title, title);
    assert.equal(
      Boolean(view.items[1].primaryAction),
      status.phase !== 'running' && status.phase !== 'complete',
    );
  }
});

test('the updates view explains a withheld older build', () => {
  const manager = createUpdateManager(createAutoUpdater([]));
  manager.canUseAutoUpdates = () => true;
  manager.state.blockedDowngradeVersion = '2026.1029.4';
  const service = createUpdatesService({
    updateManager: manager,
    getAppVersion: () => '2026.1030.5',
    autoUpdatesUnavailableMessage: () => '',
    getActiveNevermindBaseUrl: () => null,
    getUpdateChannel: () => 'stable',
    markAppQuitting: () => {},
    requestQuit: async () => {},
    quitApp: () => {},
    patchOpenView: () => {},
    stateMaintenance: unsupportedStateMaintenance(),
  });
  const [status] = service.updateStatusView().items;
  assert.equal(
    status.title,
    'Nevermind 2026.1029.4 needs a verified state backup',
  );
});
