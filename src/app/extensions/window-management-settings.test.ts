// biome-ignore-all lint/style/noMagicNumbers: normalization and parse tables are literal boundary values.
// biome-ignore-all lint/suspicious/noExplicitAny: storage fakes are untyped bridge objects.
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  ALMOST_MAXIMIZE_SETTING,
  DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
  extensionWindowManagementSettingsStore,
  GAP_SETTING,
  normalizeWindowManagementSettings,
  parseWholeNumberSettingInput,
  RESIZE_STEP_SETTING,
  WINDOW_MANAGEMENT_SETTINGS_KEY,
} from './window-management-settings';
import { memoryExtensionStorage } from './window-management-test-doubles';

test('defaults match the documented window management defaults', () => {
  assert.deepEqual(DEFAULT_WINDOW_MANAGEMENT_SETTINGS, {
    gap: 0,
    cycle: true,
    almostMaximizePercent: 90,
    reasonableSizePercent: 60,
    resizeStepPercent: 5,
  });
});

test('missing or non-object stored settings normalize to defaults', () => {
  for (const raw of [undefined, null, 'settings', 42, ['gap']]) {
    assert.deepEqual(
      normalizeWindowManagementSettings(raw),
      DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
    );
  }
});

const NORMALIZATION_CASES: Array<[string, unknown, unknown]> = [
  ['gap above the maximum clamps to 100', { gap: 150 }, 100],
  ['negative gap clamps to 0', { gap: -4 }, 0],
  ['fractional gap rounds to a whole pixel', { gap: 12.6 }, 13],
  ['non-numeric gap falls back to the default', { gap: 'wide' }, 0],
  ['non-finite gap falls back to the default', { gap: Number.NaN }, 0],
  ['numeric string gap falls back to the default', { gap: '12' }, 0],
  ['non-boolean cycle falls back to the default', { cycle: 'no' }, true],
  ['false cycle is kept', { cycle: false }, false],
  ['almost maximize below 50 clamps to 50', { almostMaximizePercent: 10 }, 50],
  [
    'almost maximize above 100 clamps to 100',
    { almostMaximizePercent: 120 },
    100,
  ],
  ['almost maximize in range is kept', { almostMaximizePercent: 75 }, 75],
  ['reasonable size below 30 clamps to 30', { reasonableSizePercent: 5 }, 30],
  [
    'reasonable size missing falls back to 60',
    { reasonableSizePercent: undefined },
    60,
  ],
  ['resize step of 0 clamps to 1', { resizeStepPercent: 0 }, 1],
  ['resize step above 25 clamps to 25', { resizeStepPercent: 40 }, 25],
  ['non-numeric resize step falls back to 5', { resizeStepPercent: 'big' }, 5],
];

for (const [label, raw, expected] of NORMALIZATION_CASES) {
  test(`normalization: ${label}`, () => {
    const key = Object.keys(raw as object)[0] as string;
    const normalized = normalizeWindowManagementSettings(raw);
    assert.deepEqual(normalized[key as keyof typeof normalized], expected);
  });
}

const PARSE_CASES: Array<[string, unknown, boolean]> = [
  ['zero is a valid gap', '0', true],
  ['upper bound is valid', '100', true],
  ['surrounding spaces are ignored', '  7 ', true],
  ['above the gap maximum is rejected', '101', false],
  ['fractions are rejected', '12.5', false],
  ['negative values are rejected', '-1', false],
  ['empty input is rejected', '', false],
  ['text is rejected', 'wide', false],
];

for (const [label, input, accepted] of PARSE_CASES) {
  test(`parse: ${label}`, () => {
    const result = parseWholeNumberSettingInput(input, GAP_SETTING);
    assert.equal(result.ok, accepted);
    if (result.ok) {
      assert.equal(result.value, Number(String(input).trim()));
    } else {
      assert.equal(result.message, 'Enter a whole number from 0 to 100.');
    }
  });
}

test('parse reports the range of the setting being edited', () => {
  assert.deepEqual(parseWholeNumberSettingInput('0', RESIZE_STEP_SETTING), {
    ok: false,
    message: 'Enter a whole number from 1 to 25.',
  });
  assert.deepEqual(
    parseWholeNumberSettingInput('49', ALMOST_MAXIMIZE_SETTING),
    { ok: false, message: 'Enter a whole number from 50 to 100.' },
  );
});

test('store loads normalized settings from extension storage', async () => {
  const storage = memoryExtensionStorage({
    [WINDOW_MANAGEMENT_SETTINGS_KEY]: {
      gap: 500,
      cycle: 'yes',
      resizeStepPercent: 3,
    },
  });
  const store = extensionWindowManagementSettingsStore({
    storage: storage as any,
  });
  assert.deepEqual(await store.load(), {
    ...DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
    gap: 100,
    resizeStepPercent: 3,
  });
});

test('store saves only normalized settings under the settings key', async () => {
  const storage = memoryExtensionStorage();
  const store = extensionWindowManagementSettingsStore({
    storage: storage as any,
  });
  await store.save({ ...DEFAULT_WINDOW_MANAGEMENT_SETTINGS, gap: -3 });
  assert.deepEqual(storage.values.get(WINDOW_MANAGEMENT_SETTINGS_KEY), {
    ...DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
    gap: 0,
  });
});
