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

function createAutoUpdater(calls: string[]) {
  return {
    autoDownload: false,
    autoInstallOnAppQuit: false,
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
  manager.state.downloadedInfo = { version: '0.16.6' };

  assert.equal(manager.prepareInstall(), true);
  assert.equal(manager.state.status, 'installing');
  assert.equal(manager.state.installInFlight, true);

  assert.equal(manager.quitAndInstall(), true);
  assert.deepEqual(calls, ['install']);
});
