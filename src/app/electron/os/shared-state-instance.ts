import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

type InstanceApp = {
  getPath: (name: 'userData') => string;
  setPath: (name: 'userData', value: string) => void;
  getName: () => string;
  setName: (name: string) => void;
  hasSingleInstanceLock: () => boolean;
  requestSingleInstanceLock: () => boolean;
  releaseSingleInstanceLock: () => void;
};

export async function acquireDefaultSharedStateInstance(
  app: InstanceApp,
  testMode: boolean,
  // biome-ignore lint/style/noProcessEnv: test mode isolates its shared root in CI.
  testSharedRoot = process.env.NVM_TEST_SHARED_STATE_ROOT,
) {
  let root = testMode
    ? (testSharedRoot ??
      path.join(app.getPath('userData'), 'test-instance-state'))
    : path.join(os.homedir(), '.config', 'nevermind');
  if (testMode && testSharedRoot) {
    const parent = await fs.realpath(
      path.dirname(path.resolve(testSharedRoot)),
    );
    const temp = await fs.realpath(os.tmpdir());
    const relative = path.relative(temp, parent);
    if (
      relative === '..' ||
      relative.startsWith(`..${path.sep}`) ||
      path.isAbsolute(relative)
    )
      throw new Error(
        'Test shared state must be beneath the temporary directory',
      );
    root = path.join(parent, path.basename(testSharedRoot));
    const entry = await fs.lstat(root).catch((error: NodeJS.ErrnoException) => {
      if (error.code === 'ENOENT') return null;
      throw error;
    });
    if (entry?.isSymbolicLink())
      throw new Error('Test shared state cannot be a symbolic link');
  }
  return acquireSharedStateInstance(app, root);
}

export async function acquireSharedStateInstance(
  app: InstanceApp,
  sharedRoot: string,
) {
  if (app.hasSingleInstanceLock())
    throw new Error('An instance lock is already held');
  await fs.mkdir(sharedRoot, { recursive: true });
  const canonicalRoot = await fs.realpath(sharedRoot);
  const identity = path.join(
    path.dirname(canonicalRoot),
    `.${path.basename(canonicalRoot)}-instance`,
  );
  await fs.mkdir(identity, { recursive: true, mode: 0o700 });
  const originalPath = app.getPath('userData');
  const originalName = app.getName();
  let acquired = false;
  let failed = false;
  let failure: unknown;
  try {
    app.setPath('userData', identity);
    // Electron includes the application name in the Windows singleton identity.
    app.setName('Nevermind shared state');
    acquired = app.requestSingleInstanceLock();
  } catch (error) {
    failed = true;
    failure = error;
  }
  try {
    try {
      app.setName(originalName);
    } finally {
      app.setPath('userData', originalPath);
    }
  } catch (error) {
    if (acquired) app.releaseSingleInstanceLock();
    throw error;
  }
  if (failed) throw failure;
  return acquired;
}
