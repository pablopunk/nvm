// biome-ignore-all lint/style/noMagicNumbers: option tables are literal boundary values.
import assert from 'node:assert/strict';
import test from 'node:test';
import { DEFAULT_WINDOW_MANAGEMENT_OPTIONS } from '../electron/os/window-management-types';
import { windowManagementOptionsFor } from './window-management-options';
import {
  DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
  type WindowManagementSettings,
} from './window-management-settings-model';
import { normalizeWindowManagementSettings } from './window-management-settings-normalize';

function settingsWith(
  overrides: Partial<WindowManagementSettings>,
): WindowManagementSettings {
  return normalizeWindowManagementSettings({
    ...DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
    ...overrides,
  });
}

test('default settings build the host default options', () => {
  assert.deepEqual(
    windowManagementOptionsFor(DEFAULT_WINDOW_MANAGEMENT_SETTINGS),
    DEFAULT_WINDOW_MANAGEMENT_OPTIONS,
  );
});

test('the outer gap fills every edge that has no override', () => {
  const options = windowManagementOptionsFor(settingsWith({ outerGap: 12 }));
  assert.deepEqual(options.outerGap, {
    top: 12,
    right: 12,
    bottom: 12,
    left: 12,
  });
});

test('per-edge overrides win over the outer gap and leave other edges alone', () => {
  const options = windowManagementOptionsFor(
    settingsWith({ outerGap: 10, outerGapTop: 0, outerGapLeft: 24 }),
  );
  assert.deepEqual(options.outerGap, {
    top: 0,
    right: 10,
    bottom: 10,
    left: 24,
  });
});

test('a zero override is a real override, not a missing one', () => {
  const options = windowManagementOptionsFor(
    settingsWith({ outerGap: 8, outerGapBottom: 0 }),
  );
  assert.equal(options.outerGap.bottom, 0);
});

test('gap, cycle, and display options pass through unchanged', () => {
  const options = windowManagementOptionsFor(
    settingsWith({
      innerGap: 14,
      applyGapToMaximize: false,
      cycle: false,
      cycleOrder: 'half-third-two-thirds',
      displayOrder: 'top-to-bottom',
      displayMoveSizing: 'keep-size',
    }),
  );
  assert.deepEqual(options, {
    outerGap: { top: 8, right: 8, bottom: 8, left: 8 },
    innerGap: 14,
    applyGapToMaximize: false,
    cycle: false,
    cycleOrder: 'half-third-two-thirds',
    displayOrder: 'top-to-bottom',
    displayMoveSizing: 'keep-size',
  });
});
