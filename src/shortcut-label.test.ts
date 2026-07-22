import assert from 'node:assert/strict';
import test from 'node:test';
import { shortcutLabel } from './ui';

test('shortcutLabel uses the native backspace symbol', () => {
  assert.equal(shortcutLabel('Command+Backspace'), '⌘⌫');
});
