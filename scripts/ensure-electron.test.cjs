'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');

const {
  electronInstallerEnvironment,
  ensureElectronAvailable,
  installElectronBinary,
} = require('./ensure-electron.cjs');

function createElectronFixture(t) {
  const electronPackageDirectory = fs.mkdtempSync(
    path.join(os.tmpdir(), 'nvm-electron-repair-'),
  );
  t.after(() => fs.rmSync(electronPackageDirectory, { recursive: true }));
  return electronPackageDirectory;
}

function installFakeElectron(electronPackageDirectory) {
  const executablePath = path.join(
    electronPackageDirectory,
    'dist',
    'electron',
  );
  fs.mkdirSync(path.dirname(executablePath), { recursive: true });
  fs.writeFileSync(executablePath, 'fake executable');
  fs.writeFileSync(path.join(electronPackageDirectory, 'path.txt'), 'electron');
  return executablePath;
}

function resolveFakeElectron(electronPackageDirectory) {
  const pathFile = path.join(electronPackageDirectory, 'path.txt');
  if (!fs.existsSync(pathFile)) return undefined;
  const executablePath = path.join(
    electronPackageDirectory,
    'dist',
    fs.readFileSync(pathFile, 'utf8').trim(),
  );
  return fs.existsSync(executablePath) ? executablePath : undefined;
}

test('keeps a healthy Electron payload without reinstalling', (t) => {
  const electronPackageDirectory = createElectronFixture(t);
  const executablePath = installFakeElectron(electronPackageDirectory);

  const actual = ensureElectronAvailable({
    electronPackageDirectory,
    resolveExecutable: () => resolveFakeElectron(electronPackageDirectory),
    install: () => assert.fail('healthy Electron must not be reinstalled'),
  });

  assert.equal(actual, executablePath);
});

test('cleans stale dist when only path.txt is missing before installing', (t) => {
  const electronPackageDirectory = createElectronFixture(t);
  const stalePayload = path.join(
    electronPackageDirectory,
    'dist',
    'stale-file',
  );
  fs.mkdirSync(path.dirname(stalePayload), { recursive: true });
  fs.writeFileSync(stalePayload, 'stale payload');
  let installs = 0;

  const executablePath = ensureElectronAvailable({
    electronPackageDirectory,
    resolveExecutable: () => resolveFakeElectron(electronPackageDirectory),
    install: () => {
      installs += 1;
      assert.equal(fs.existsSync(path.dirname(stalePayload)), false);
      installFakeElectron(electronPackageDirectory);
    },
  });

  assert.equal(installs, 1);
  assert.equal(fs.existsSync(executablePath), true);
});

test('repairs a completely skipped Electron payload', (t) => {
  const electronPackageDirectory = createElectronFixture(t);
  let installs = 0;

  const executablePath = ensureElectronAvailable({
    electronPackageDirectory,
    resolveExecutable: () => resolveFakeElectron(electronPackageDirectory),
    install: () => {
      installs += 1;
      installFakeElectron(electronPackageDirectory);
    },
  });

  assert.equal(installs, 1);
  assert.equal(fs.existsSync(executablePath), true);
});

test('fails when installing does not produce an executable', (t) => {
  const electronPackageDirectory = createElectronFixture(t);

  assert.throws(
    () =>
      ensureElectronAvailable({
        electronPackageDirectory,
        resolveExecutable: () => undefined,
        install: () => {},
      }),
    /without a usable executable/,
  );
});

test('runs Electron installer directly through Node with download enabled', (t) => {
  const electronPackageDirectory = createElectronFixture(t);
  let invocation;

  installElectronBinary({
    electronPackageDirectory,
    environment: {
      ELECTRON_SKIP_BINARY_DOWNLOAD: '1',
      FORCE_NO_CACHE: 'false',
      PATH: 'test-path',
    },
    nodeExecutable: 'C:\\Program Files\\nodejs\\node.exe',
    spawn: (command, commandArguments, options) => {
      invocation = { command, commandArguments, options };
      return { status: 0 };
    },
  });

  assert.deepEqual(invocation, {
    command: 'C:\\Program Files\\nodejs\\node.exe',
    commandArguments: [path.join(electronPackageDirectory, 'install.js')],
    options: {
      cwd: electronPackageDirectory,
      env: {
        PATH: 'test-path',
        force_no_cache: 'true',
      },
      stdio: 'inherit',
    },
  });
});

test('removes case-insensitive skip flags from the installer environment', () => {
  assert.deepEqual(
    electronInstallerEnvironment({
      electron_skip_binary_download: 'true',
      Force_No_Cache: 'false',
      PATH: 'test-path',
    }),
    {
      PATH: 'test-path',
      force_no_cache: 'true',
    },
  );
});
