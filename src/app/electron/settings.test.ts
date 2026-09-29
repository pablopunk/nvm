import assert from 'node:assert/strict';
import test from 'node:test';
import { settingValue } from './settings';

test('palette shortcut defaults to a Linux-friendly accelerator', () => {
  assert.equal(
    settingValue(undefined, 'paletteHotkey', 'linux'),
    'Control+Alt+K',
  );
  assert.equal(settingValue(undefined, 'paletteHotkey', 'darwin'), 'Alt+Space');
});

test('stored palette shortcut overrides the platform default', () => {
  assert.equal(
    settingValue({ paletteHotkey: 'Alt+Space' }, 'paletteHotkey', 'linux'),
    'Alt+Space',
  );
});
