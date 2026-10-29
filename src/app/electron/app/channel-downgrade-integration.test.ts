import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: true },
    shell: {},
    systemPreferences: { isTrustedAccessibilityClient: () => false },
  },
});
mock.module('electron-log/main', {
  defaultExport: { error() {}, info() {} },
});

const { createUpdateManager } = await import('./update-manager');
const { createStateMaintenanceService } = await import(
  './state-maintenance-service'
);
const { isStateSafeForInstall } = await import('./state-update-safety');
const { advanceStateCompatibility } = await import(
  '../state/state-compatibility'
);
const { createMaintenanceActivity } = await import(
  '../platform/maintenance-activity'
);

const CURRENT_VERSION = '2026.1030.5';

async function fixture(t: test.TestContext, targetEpoch: unknown) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-downgrade-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const roots = {
    privateRoot: path.join(root, 'private'),
    sharedRoot: path.join(root, 'shared'),
    backupParent: path.join(root, 'backups'),
  };
  await advanceStateCompatibility(roots.privateRoot, 1, roots.sharedRoot);
  await fs.writeFile(path.join(roots.privateRoot, 'state.json'), '{}');
  const activity = createMaintenanceActivity();
  const maintenance = createStateMaintenanceService({
    writers: [activity],
    flushPendingState: async () => {},
    roots: () => roots,
    epoch: 1,
    ownsSingleInstance: () => true,
    isUpdateInstalling: () => false,
  });
  const calls: string[] = [];
  const updateInfo = {
    version: '2026.1029.4',
    updateChannel: 'stable' as const,
    nevermindStateEpoch: targetEpoch,
  };
  const listeners = new Map<string, Array<(info: unknown) => void>>();
  const updater = {
    autoDownload: false,
    autoInstallOnAppQuit: false,
    allowDowngrade: false,
    on: (event: string, listener: (info: unknown) => void) =>
      listeners.set(event, [...(listeners.get(event) ?? []), listener]),
    checkForUpdates: async () => {
      calls.push('check');
      return { isUpdateAvailable: true, updateInfo };
    },
    downloadUpdate: async () => {
      calls.push('download');
      for (const listener of listeners.get('update-downloaded') ?? [])
        listener(updateInfo);
    },
    quitAndInstall: () => calls.push('install'),
  };
  const manager = createUpdateManager(updater, undefined, {
    getUpdateChannel: () => 'stable',
    getCurrentVersion: () => CURRENT_VERSION,
    canInstallUpdate: (info) =>
      isStateSafeForInstall(CURRENT_VERSION, info, [
        roots.privateRoot,
        roots.sharedRoot,
      ]),
  });
  manager.configure();
  manager.clearTimers();
  return { roots, maintenance, manager, calls, updateInfo };
}

test('switching to an older channel build requires a verified backup, then installs it', async (t) => {
  const { roots, maintenance, manager, calls, updateInfo } = await fixture(
    t,
    1,
  );

  await manager.checkForUpdates('manual', { download: true });
  assert.equal(manager.state.blockedDowngradeVersion, updateInfo.version);
  assert.equal(manager.state.availableInfo, null);

  const result = await maintenance.verifyAndTrust();
  assert.equal(result.phase, 'complete');
  const backup = (result as { backupPath: string }).backupPath;
  assert.equal(
    await fs.readFile(path.join(backup, 'private', 'state.json'), 'utf8'),
    '{}',
  );

  await manager.checkForUpdates('manual', { download: true });
  assert.equal(manager.state.availableInfo, updateInfo);
  assert.deepEqual(calls, ['check', 'check']);
  await manager.downloadAvailableUpdate();
  assert.equal(await manager.prepareInstall(), true);
  assert.equal(manager.quitAndInstall(), true);

  await advanceStateCompatibility(roots.privateRoot, 2, roots.sharedRoot);
  assert.equal(
    await isStateSafeForInstall(CURRENT_VERSION, updateInfo, [
      roots.privateRoot,
      roots.sharedRoot,
    ]),
    false,
  );
});

test('a verified backup does not authorize a target without matching state support', async (t) => {
  for (const epoch of [undefined, 2]) {
    const { maintenance, manager, updateInfo } = await fixture(t, epoch);
    assert.equal((await maintenance.verifyAndTrust()).phase, 'complete');
    await manager.checkForUpdates('manual', { download: true });
    assert.equal(manager.state.availableInfo, null);
    assert.equal(manager.state.blockedDowngradeVersion, updateInfo.version);
  }
});

test('state changed after verification revokes a downloaded older build', async (t) => {
  const { roots, maintenance, manager } = await fixture(t, 1);
  await maintenance.verifyAndTrust();
  await manager.checkForUpdates();
  await manager.downloadAvailableUpdate();
  await fs.rm(path.join(roots.sharedRoot, 'state-compatibility.json'));
  assert.equal(await manager.prepareInstall(), false);
});
