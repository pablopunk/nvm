import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: { app: { isPackaged: true } },
});
mock.module('electron-log/main', {
  defaultExport: { error() {}, info() {} },
});

const { selectPortableUpdateFile } = await import('./windows-portable-updater');
const { portableUpdateFailureMessage } = await import(
  './portable-update-outcome'
);

function file(name: string) {
  return {
    url: new URL(`https://example.test/download/v1/${name}`),
    info: { url: name, sha512: 'x', size: 1 },
  };
}

test('selects only the portable executable for the running architecture', () => {
  const portable = file('Nevermind-2026.1030.2-win-x64-portable.exe');
  assert.equal(
    selectPortableUpdateFile(
      [file('Nevermind-2026.1030.2-win-arm64-portable.exe'), portable],
      'x64',
    ),
    portable,
  );
  for (const files of [
    [file('Nevermind-2026.1030.2-win-x64-setup.exe')],
    [portable, portable],
    [],
  ])
    assert.throws(
      () => selectPortableUpdateFile(files, 'x64'),
      /exactly one -win-x64-portable\.exe/,
    );
});

test('helper outcomes become user-facing update errors', () => {
  assert.equal(
    portableUpdateFailureMessage({
      outcome: 'rolled-back',
      reason: 'startup-failed: the updated app exited with code 1',
      fromVersion: '1',
      toVersion: '2',
    }),
    'Nevermind 2 did not start, so version 1 was restored. the updated app exited with code 1',
  );
  assert.match(
    String(
      portableUpdateFailureMessage({
        outcome: 'failed',
        reason: 'integrity: mismatch',
        fromVersion: '1',
        toVersion: '2',
      }),
    ),
    /^Nevermind 2 was not installed\. mismatch$/,
  );
  assert.equal(
    portableUpdateFailureMessage({
      outcome: 'installed',
      reason: 'healthy',
      fromVersion: '1',
      toVersion: '2',
    }),
    null,
  );
});
