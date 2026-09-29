import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', { namedExports: { app: { isPackaged: false } } });

const { createWindowsDesktopText } = await import('./windows-desktop-text');
const target = { bundleId: 'windows:12345:42', windowId: '12345', pid: 42 };

function desktop(code?: number) {
  const calls: string[][] = [];
  const access = createWindowsDesktopText({
    helperPath: () => '/test/helper.exe',
    run: async (_command, args) => {
      calls.push(args);
      if (code)
        throw Object.assign(new Error('helper failed'), {
          code,
          stderr: 'Permission denied',
        });
      return args[0] === 'target'
        ? '42\n12345'
        : args[0] === 'read'
          ? 'selected text'
          : '';
    },
  });
  return { access, calls };
}

test('reads, restores, copies, and pastes through the same window handle', async () => {
  const { access, calls } = desktop();
  assert.deepEqual(await access.target(), target);
  assert.equal(await access.read(target), 'selected text');
  assert.equal(await access.restore(target), true);
  assert.equal(await access.copy(target), true);
  await access.paste(target.bundleId);
  assert.deepEqual(calls, [
    ['target'],
    ['read', '42', '12345'],
    ['restore', '42', '12345'],
    ['copy', '42', '12345'],
    ['target'],
    ['paste', '42', '12345'],
  ]);
});

test('distinguishes an empty selection from denied and failed desktop access', async () => {
  assert.equal(await desktop(3).access.read(target), null);
  await assert.rejects(desktop(2).access.copy(target), /Permission denied/);
  assert.equal((await desktop(2).access.access()).state, 'denied');
  assert.equal((await desktop(5).access.access()).state, 'unavailable');
});
