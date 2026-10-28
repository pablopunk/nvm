// biome-ignore-all lint/style/noMagicNumbers: normalization tables are literal boundary values.
// biome-ignore-all lint/suspicious/noExplicitAny: storage fakes are untyped bridge objects.
import assert from 'node:assert/strict';
import test from 'node:test';
import { DEFAULT_WINDOW_MANAGEMENT_OPTIONS } from '../electron/os/window-management-types';
import {
  extensionWindowManagementSettingsStore,
  WINDOW_MANAGEMENT_SETTINGS_KEY,
} from './window-management-settings';
import { WINDOW_MANAGEMENT_SETTING_DEFINITIONS } from './window-management-settings-definitions';
import {
  DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
  resetSettingsToDefaults,
} from './window-management-settings-model';
import { normalizeWindowManagementSettings } from './window-management-settings-normalize';
import { memoryExtensionStorage } from './window-management-test-doubles';

test('defaults match the documented window management defaults', () => {
  assert.deepEqual(DEFAULT_WINDOW_MANAGEMENT_SETTINGS, {
    outerGap: 8,
    outerGapTop: null,
    outerGapRight: null,
    outerGapBottom: null,
    outerGapLeft: null,
    innerGap: 4,
    applyGapToMaximize: true,
    almostMaximizeWidthPercent: 90,
    almostMaximizeHeightPercent: 90,
    reasonableSizeWidthPercent: 70,
    reasonableSizeHeightPercent: 70,
    centerHalfWidthPercent: 50,
    resizeStepPercent: 5,
    cycle: true,
    cycleOrder: 'half-two-thirds-third',
    displayOrder: 'left-to-right',
    displayMoveSizing: 'proportional',
  });
});

test('every persisted setting has a definition and a default', () => {
  const definitionKeys = WINDOW_MANAGEMENT_SETTING_DEFINITIONS.map(
    (definition) => definition.key,
  ).sort();
  assert.deepEqual(
    definitionKeys,
    Object.keys(DEFAULT_WINDOW_MANAGEMENT_SETTINGS).sort(),
  );
});

test('extension defaults resolve to the host default options', () => {
  const resolved = normalizeWindowManagementSettings({});
  assert.equal(
    resolved.outerGap,
    DEFAULT_WINDOW_MANAGEMENT_OPTIONS.outerGap.top,
  );
  assert.equal(resolved.innerGap, DEFAULT_WINDOW_MANAGEMENT_OPTIONS.innerGap);
  assert.equal(
    resolved.cycleOrder,
    DEFAULT_WINDOW_MANAGEMENT_OPTIONS.cycleOrder,
  );
});

test('missing or non-object stored settings normalize to defaults', () => {
  for (const raw of [undefined, null, 'settings', 42, ['outerGap']]) {
    assert.deepEqual(
      normalizeWindowManagementSettings(raw),
      DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
    );
  }
});

const NORMALIZATION_CASES: [string, unknown, string, unknown][] = [
  ['outer gap above 40 clamps to 40', { outerGap: 50 }, 'outerGap', 40],
  ['negative outer gap clamps to 0', { outerGap: -1 }, 'outerGap', 0],
  ['fractional outer gap rounds', { outerGap: 12.6 }, 'outerGap', 13],
  ['text outer gap falls back to 8', { outerGap: 'wide' }, 'outerGap', 8],
  [
    'numeric string outer gap falls back to 8',
    { outerGap: '12' },
    'outerGap',
    8,
  ],
  ['top override in range is kept', { outerGapTop: 12 }, 'outerGapTop', 12],
  [
    'top override above 40 clamps to 40',
    { outerGapTop: 99 },
    'outerGapTop',
    40,
  ],
  ['top override below 0 clamps to 0', { outerGapTop: -5 }, 'outerGapTop', 0],
  [
    'top override null follows outer gap',
    { outerGapTop: null },
    'outerGapTop',
    null,
  ],
  [
    'top override text is cleared to null',
    { outerGapTop: 'x' },
    'outerGapTop',
    null,
  ],
  ['right override in range is kept', { outerGapRight: 3 }, 'outerGapRight', 3],
  [
    'bottom override in range is kept',
    { outerGapBottom: 30 },
    'outerGapBottom',
    30,
  ],
  ['left override in range is kept', { outerGapLeft: 0 }, 'outerGapLeft', 0],
  ['inner gap above 40 clamps to 40', { innerGap: 41 }, 'innerGap', 40],
  [
    'non-finite inner gap falls back to 4',
    { innerGap: Number.NaN },
    'innerGap',
    4,
  ],
  [
    'false apply gap to maximize is kept',
    { applyGapToMaximize: false },
    'applyGapToMaximize',
    false,
  ],
  [
    'non-boolean apply gap to maximize falls back to true',
    { applyGapToMaximize: 'no' },
    'applyGapToMaximize',
    true,
  ],
  [
    'almost maximize width below 50 clamps to 50',
    { almostMaximizeWidthPercent: 10 },
    'almostMaximizeWidthPercent',
    50,
  ],
  [
    'almost maximize width above 100 clamps to 100',
    { almostMaximizeWidthPercent: 120 },
    'almostMaximizeWidthPercent',
    100,
  ],
  [
    'almost maximize height in range is kept',
    { almostMaximizeHeightPercent: 75 },
    'almostMaximizeHeightPercent',
    75,
  ],
  [
    'almost maximize height text falls back to 90',
    { almostMaximizeHeightPercent: 'x' },
    'almostMaximizeHeightPercent',
    90,
  ],
  [
    'reasonable width below 30 clamps to 30',
    { reasonableSizeWidthPercent: 5 },
    'reasonableSizeWidthPercent',
    30,
  ],
  [
    'reasonable width missing falls back to 70',
    { reasonableSizeWidthPercent: undefined },
    'reasonableSizeWidthPercent',
    70,
  ],
  [
    'reasonable width above 100 clamps to 100 with no cap',
    { reasonableSizeWidthPercent: 150 },
    'reasonableSizeWidthPercent',
    100,
  ],
  [
    'reasonable height below 30 clamps to 30',
    { reasonableSizeHeightPercent: 29 },
    'reasonableSizeHeightPercent',
    30,
  ],
  [
    'reasonable height in range is kept',
    { reasonableSizeHeightPercent: 55 },
    'reasonableSizeHeightPercent',
    55,
  ],
  [
    'center half width below 20 clamps to 20',
    { centerHalfWidthPercent: 19 },
    'centerHalfWidthPercent',
    20,
  ],
  [
    'center half width above 90 clamps to 90',
    { centerHalfWidthPercent: 91 },
    'centerHalfWidthPercent',
    90,
  ],
  [
    'center half width text falls back to 50',
    { centerHalfWidthPercent: 'x' },
    'centerHalfWidthPercent',
    50,
  ],
  [
    'resize step of 0 clamps to 1',
    { resizeStepPercent: 0 },
    'resizeStepPercent',
    1,
  ],
  [
    'resize step above 25 clamps to 25',
    { resizeStepPercent: 40 },
    'resizeStepPercent',
    25,
  ],
  [
    'non-numeric resize step falls back to 5',
    { resizeStepPercent: 'big' },
    'resizeStepPercent',
    5,
  ],
  ['non-boolean cycle falls back to true', { cycle: 'no' }, 'cycle', true],
  ['false cycle is kept', { cycle: false }, 'cycle', false],
  [
    'known cycle order is kept',
    { cycleOrder: 'half-third-two-thirds' },
    'cycleOrder',
    'half-third-two-thirds',
  ],
  [
    'unknown cycle order falls back to the first order',
    { cycleOrder: 'third' },
    'cycleOrder',
    'half-two-thirds-third',
  ],
  [
    'known display order is kept',
    { displayOrder: 'top-to-bottom' },
    'displayOrder',
    'top-to-bottom',
  ],
  [
    'unknown display order falls back to left-to-right',
    { displayOrder: 'diagonal' },
    'displayOrder',
    'left-to-right',
  ],
  [
    'keep-size display move sizing is kept',
    { displayMoveSizing: 'keep-size' },
    'displayMoveSizing',
    'keep-size',
  ],
  [
    'unknown display move sizing falls back to proportional',
    { displayMoveSizing: 'grow' },
    'displayMoveSizing',
    'proportional',
  ],
];

for (const [label, raw, key, expected] of NORMALIZATION_CASES) {
  test(`normalization: ${label}`, () => {
    const normalized = normalizeWindowManagementSettings(raw);
    assert.deepEqual(normalized[key as keyof typeof normalized], expected);
  });
}

test('resetSettingsToDefaults restores only the requested keys', () => {
  const customized = normalizeWindowManagementSettings({
    outerGap: 30,
    outerGapTop: 2,
    cycle: false,
  });
  const reset = resetSettingsToDefaults(customized, [
    'outerGap',
    'outerGapTop',
  ]);
  assert.equal(reset.outerGap, DEFAULT_WINDOW_MANAGEMENT_SETTINGS.outerGap);
  assert.equal(reset.outerGapTop, null);
  assert.equal(reset.cycle, false);
});

test('store loads normalized settings from extension storage', async () => {
  const storage = memoryExtensionStorage({
    [WINDOW_MANAGEMENT_SETTINGS_KEY]: {
      outerGap: 500,
      cycle: 'yes',
      resizeStepPercent: 3,
    },
  });
  const store = extensionWindowManagementSettingsStore({
    storage: storage as any,
  });
  assert.deepEqual(await store.load(), {
    ...DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
    outerGap: 40,
    resizeStepPercent: 3,
  });
});

test('store saves only normalized settings under the settings key', async () => {
  const storage = memoryExtensionStorage();
  const store = extensionWindowManagementSettingsStore({
    storage: storage as any,
  });
  await store.save({ ...DEFAULT_WINDOW_MANAGEMENT_SETTINGS, innerGap: -3 });
  assert.deepEqual(storage.values.get(WINDOW_MANAGEMENT_SETTINGS_KEY), {
    ...DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
    innerGap: 0,
  });
});
