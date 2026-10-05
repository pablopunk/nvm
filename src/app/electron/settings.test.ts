import assert from 'node:assert/strict';
import test from 'node:test';
import { settingValue } from './settings';

test('technical reports can be disabled without disabling local diagnostics', () => {
  assert.equal(settingValue(undefined, 'errorReporting'), true);
  assert.equal(
    settingValue({ errorReporting: false }, 'errorReporting'),
    false,
  );
});

test('palette shortcut defaults to no binding', () => {
  assert.equal(settingValue(undefined, 'paletteHotkey'), '');
});

test('stored palette shortcut overrides the empty default', () => {
  assert.equal(
    settingValue({ paletteHotkey: 'Control+Alt+K' }, 'paletteHotkey'),
    'Control+Alt+K',
  );
});
