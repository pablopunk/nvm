import assert from 'node:assert/strict';
import test from 'node:test';
import { shortcutLabel } from './ui';

test('shortcutLabel uses native key symbols on macOS', () => {
  assert.equal(shortcutLabel('Command+Backspace', 'darwin'), '⌘⌫');
  assert.equal(shortcutLabel('Alt+Space', 'darwin'), '⌥Space');
});

test('shortcutLabel uses platform key names on Linux and Windows', () => {
  assert.equal(shortcutLabel('Alt+Space', 'linux'), 'Alt+Space');
  assert.equal(shortcutLabel('Alt+Space', 'win32'), 'Alt+Space');
  assert.equal(shortcutLabel('Control+Alt+K', 'win32'), 'Control+Alt+K');
});
