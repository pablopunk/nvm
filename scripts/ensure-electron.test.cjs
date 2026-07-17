'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');

const { ensureElectronAvailable } = require('./ensure-electron.cjs');

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

test('keeps a healthy Electron payload without rebuilding', (t) => {
  const electronPackageDirectory = createElectronFixture(t);
  const executablePath = installFakeElectron(electronPackageDirectory);

  const actual = ensureElectronAvailable({
    electronPackageDirectory,
    resolveExecutable: () => resolveFakeElectron(electronPackageDirectory),
    rebuild: () => assert.fail('healthy Electron must not be rebuilt'),
  });

  assert.equal(actual, executablePath);
});

test('cleans stale dist when only path.txt is missing before rebuilding', (t) => {
  const electronPackageDirectory = createElectronFixture(t);
  const stalePayload = path.join(
    electronPackageDirectory,
    'dist',
    'stale-file',
  );
  fs.mkdirSync(path.dirname(stalePayload), { recursive: true });
  fs.writeFileSync(stalePayload, 'stale payload');
  let rebuilds = 0;

  const executablePath = ensureElectronAvailable({
    electronPackageDirectory,
    resolveExecutable: () => resolveFakeElectron(electronPackageDirectory),
    rebuild: () => {
      rebuilds += 1;
      assert.equal(fs.existsSync(path.dirname(stalePayload)), false);
      installFakeElectron(electronPackageDirectory);
    },
  });

  assert.equal(rebuilds, 1);
  assert.equal(fs.existsSync(executablePath), true);
});

test('repairs a completely skipped Electron payload', (t) => {
  const electronPackageDirectory = createElectronFixture(t);
  let rebuilds = 0;

  const executablePath = ensureElectronAvailable({
    electronPackageDirectory,
    resolveExecutable: () => resolveFakeElectron(electronPackageDirectory),
    rebuild: () => {
      rebuilds += 1;
      installFakeElectron(electronPackageDirectory);
    },
  });

  assert.equal(rebuilds, 1);
  assert.equal(fs.existsSync(executablePath), true);
});

test('fails when rebuilding does not produce an executable', (t) => {
  const electronPackageDirectory = createElectronFixture(t);

  assert.throws(
    () =>
      ensureElectronAvailable({
        electronPackageDirectory,
        resolveExecutable: () => undefined,
        rebuild: () => {},
      }),
    /without a usable executable/,
  );
});
