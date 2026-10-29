import assert from 'node:assert/strict';
import test from 'node:test';
import {
  settingDefinition,
  settingValue,
  toggledSettingValue,
} from './settings';

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

test('update channel defaults to stable and keeps a stored nightly preference', () => {
  assert.equal(settingValue(undefined, 'updateChannel'), 'stable');
  assert.equal(
    settingValue({ updateChannel: 'nightly' }, 'updateChannel'),
    'nightly',
  );
});

test('update channel cycles through stable, nightly and dev and normalizes unknown values', () => {
  const definition = settingDefinition('updateChannel');
  assert.ok(definition);
  assert.equal(toggledSettingValue(definition, 'stable'), 'nightly');
  assert.equal(toggledSettingValue(definition, 'nightly'), 'dev');
  assert.equal(toggledSettingValue(definition, 'dev'), 'stable');
  assert.equal(toggledSettingValue(definition, 'beta'), 'nightly');
});
