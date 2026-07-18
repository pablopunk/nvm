import fsSync from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { app, session } from 'electron';

export const isNvmTestMode = process.env.NVM_TEST_MODE === '1';

function testUserDataPath() {
  const value = process.env.NVM_TEST_USER_DATA_DIR;
  if (!value)
    throw new Error('NVM_TEST_USER_DATA_DIR is required in test mode');
  const resolved = path.resolve(value);
  const tempRoot = path.resolve(os.tmpdir());
  if (!resolved.startsWith(`${tempRoot}${path.sep}`))
    throw new Error(`Test user data must be beneath ${tempRoot}`);
  if (!fsSync.existsSync(resolved))
    throw new Error(`Test user data directory does not exist: ${resolved}`);
  return resolved;
}

export function configureNvmTestMode() {
  if (!isNvmTestMode) return null;
  const userDataDir = testUserDataPath();
  app.setPath('userData', userDataDir);
  return userDataDir;
}

export function installTestNetworkPolicy() {
  if (!isNvmTestMode) return;
  const artifactDir = process.env.NVM_TEST_ARTIFACT_DIR;
  if (!artifactDir)
    throw new Error('NVM_TEST_ARTIFACT_DIR is required in test mode');
  fsSync.mkdirSync(artifactDir, { recursive: true });
  const requests: string[] = [];
  fsSync.writeFileSync(path.join(artifactDir, 'network.json'), '[]\n');
  session.defaultSession.webRequest.onBeforeRequest((details, callback) => {
    if (details.url.startsWith('file:')) return callback({ cancel: false });
    requests.push(details.url);
    fsSync.writeFileSync(
      path.join(artifactDir, 'network.json'),
      `${JSON.stringify(requests, null, 2)}\n`,
    );
    callback({ cancel: true });
  });
}

export function recordTestWindowEvent(event: 'hidden' | 'shown') {
  if (!isNvmTestMode || !process.env.NVM_TEST_ARTIFACT_DIR) return;
  const file = path.join(
    process.env.NVM_TEST_ARTIFACT_DIR,
    'window-events.json',
  );
  let events: string[] = [];
  try {
    events = JSON.parse(fsSync.readFileSync(file, 'utf8'));
  } catch {}
  events.push(event);
  fsSync.writeFileSync(file, `${JSON.stringify(events, null, 2)}\n`);
}

export function recordPackagedStartupReady() {
  if (process.env.NVM_PACKAGED_STARTUP_SMOKE !== '1') return;
  const markerPath = process.env.NVM_PORTABLE_SMOKE_MARKER;
  if (!markerPath) throw new Error('NVM_PORTABLE_SMOKE_MARKER is required');
  const resolvedMarkerPath = path.resolve(markerPath);
  const temporaryMarkerPath = `${resolvedMarkerPath}.${process.pid}.tmp`;
  fsSync.mkdirSync(path.dirname(resolvedMarkerPath), { recursive: true });
  fsSync.writeFileSync(
    temporaryMarkerPath,
    `${JSON.stringify(
      {
        pid: process.pid,
        appIsPackaged: app.isPackaged,
        appVersion: app.getVersion(),
        processExecPath: process.execPath,
        portableExecutableFile: process.env.PORTABLE_EXECUTABLE_FILE || null,
        portableExecutableDir: process.env.PORTABLE_EXECUTABLE_DIR || null,
        readyAt: new Date().toISOString(),
      },
      null,
      2,
    )}\n`,
  );
  fsSync.renameSync(temporaryMarkerPath, resolvedMarkerPath);
}
