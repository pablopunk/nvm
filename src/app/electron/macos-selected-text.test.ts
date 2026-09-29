import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: { app: {}, systemPreferences: {} },
});
mock.module('electron-log/main', {
  defaultExport: { debug() {}, warn() {} },
});

const { createMacosSelectionAccess } = await import('./macos-selected-text');
const target = { pid: 42 };

function access(exitCode: number, trusted = true, stdout = '') {
  const calls: string[] = [];
  const selection = createMacosSelectionAccess({
    trusted: () => trusted,
    run: async (operation) => {
      calls.push(operation);
      return { exitCode, stdout, stderr: '' };
    },
  });
  return { selection, calls };
}

test('checks both the Electron process and the native helper before reporting access', async () => {
  const deniedHost = access(0, false);
  assert.equal(await deniedHost.selection.permissionState(), 'denied');
  assert.deepEqual(deniedHost.calls, []);
  const deniedHelper = access(2);
  assert.equal(await deniedHelper.selection.permissionState(), 'denied');
  assert.deepEqual(deniedHelper.calls, ['permissions']);
  assert.equal(await access(0).selection.permissionState(), 'allowed');
  assert.equal(await access(1).selection.permissionState(), 'unknown');
});

test('returns the text byte-for-byte and preserves the genuine no-selection result', async () => {
  assert.equal(
    await access(0, true, '  hello there\n').selection.read(target),
    '  hello there\n',
  );
  assert.equal(await access(3).selection.read(target), null);
  assert.equal(await access(3).selection.replace(target, 'replacement'), false);
});

for (const [exitCode, message] of [
  [2, /Accessibility access is blocked/],
  [4, /source app lost focus/],
  [1, /selected-text helper failed/],
] as const) {
  test(`preserves native failure ${exitCode} across read, copy, and replacement`, async () => {
    const { selection } = access(exitCode);
    await assert.rejects(selection.read(target), message);
    await assert.rejects(selection.copy(target), message);
    await assert.rejects(selection.replace(target, 'replacement'), message);
  });
}

test('does not run the helper when host access is denied', async () => {
  const { selection, calls } = access(0, false);
  await assert.rejects(selection.read(target), /Accessibility/);
  await assert.rejects(selection.copy(target), /Accessibility/);
  assert.deepEqual(calls, []);
});

test('does not interpret a no-selection exit as successful Copy', async () => {
  await assert.rejects(access(3).selection.copy(target), /helper failed/);
});

test('reports an unavailable permission probe as unknown', async () => {
  const selection = createMacosSelectionAccess({
    trusted() {
      throw new Error('System preferences unavailable');
    },
    async run() {
      throw new Error('The helper must not run');
    },
  });
  assert.equal(await selection.permissionState(), 'unknown');
});
