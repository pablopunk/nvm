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
  assert.equal(shortcutLabel('Control+Alt+K', 'linux'), 'Control+Alt+K');
  assert.equal(shortcutLabel('Control+Alt+K', 'win32'), 'Control+Alt+K');
});

test('shortcutLabel names the meta key for Linux and Windows instead of Command', () => {
  assert.equal(shortcutLabel('Command+Y', 'linux'), 'Super+Y');
  assert.equal(
    shortcutLabel('Command+Shift+Enter', 'linux'),
    'Super+Shift+Enter',
  );
  assert.equal(shortcutLabel('Command+Backspace', 'win32'), 'Win+Backspace');
});

test('CommandOrControl shows the primary modifier for each platform', () => {
  assert.equal(shortcutLabel('CommandOrControl+Enter', 'darwin'), '⌘↵');
  assert.equal(
    shortcutLabel('CommandOrControl+Enter', 'linux'),
    'Control+Enter',
  );
  assert.equal(
    shortcutLabel('CommandOrControl+Enter', 'win32'),
    'Control+Enter',
  );
});
