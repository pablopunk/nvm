import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: false },
    shell: {},
  },
});

const { formatShortcut, normalizeAccelerator } = await import(
  './shortcut-utils'
);

test('shortcut display uses native macOS symbols only on macOS', () => {
  assert.equal(formatShortcut('Command+Alt+K', 'darwin'), '⌘⌥K');
  assert.equal(formatShortcut('Control+Alt+K', 'win32'), 'Control+Alt+K');
  assert.equal(formatShortcut('Control+Alt+K', 'linux'), 'Control+Alt+K');
});

test('shortcut normalization remains deterministic across platform wording', () => {
  assert.equal(normalizeAccelerator(' ctrl + option + k '), 'Control+Alt+K');
  assert.equal(formatShortcut('ctrl+option+k', 'win32'), 'Control+Alt+K');
});
