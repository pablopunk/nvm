import assert from 'node:assert/strict';
import test from 'node:test';
import { actionsShortcutHint } from './actions-shortcut-hint';

test('actions hint uses the command symbol on macOS', () => {
  assert.equal(actionsShortcutHint('darwin'), '⌘K');
});

test('actions hint names the control key on Linux and Windows', () => {
  assert.equal(actionsShortcutHint('linux'), 'Control+K');
  assert.equal(actionsShortcutHint('win32'), 'Control+K');
});
