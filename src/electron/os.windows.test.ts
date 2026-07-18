import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: false },
    shell: {},
  },
});

const os = await import('./os');
const windowsOnly = { skip: process.platform !== 'win32' };

test('enables app icon extraction on Windows', windowsOnly, () => {
  assert.equal(os.hasCapability('app-icons'), true);
  assert.equal(os.hasCapability('quick-look'), false);
});
