import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

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

type FakeCancellationToken = { cancelled: boolean; cancel: () => void };

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

test('exposes installing state before the deferred updater restart', () => {
  const calls: string[] = [];
  const manager = createUpdateManager(createAutoUpdater(calls));
  manager.state.downloadedInfo = { version: '0.16.6', updateChannel: 'stable' };

  assert.equal(manager.prepareInstall(), true);
  assert.equal(manager.state.status, 'installing');
  assert.equal(manager.state.installInFlight, true);

  assert.equal(manager.quitAndInstall(), true);
  assert.deepEqual(calls, ['install']);
});

test('configuring the updater disables downgrades and installs the channel feed before checking', async () => {
  const calls: string[] = [];
  const updater = createEventedAutoUpdater(calls);
  const manager = createUpdateManager(updater, undefined, {
    prepareUpdateFeed: async () => {
      calls.push('feed');
    },
  });

  manager.configure();
  assert.equal(updater.allowDowngrade, false);
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

test('refuses to install a downloaded build from another channel', () => {
  const calls: string[] = [];
  const manager = createUpdateManager(createAutoUpdater(calls), undefined, {
    getUpdateChannel: () => 'stable',
  });
  manager.state.downloadedInfo = {
    version: '0.17.1-nightly.20261028.42',
    updateChannel: 'nightly',
  };

  assert.equal(manager.prepareInstall(), false);
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
