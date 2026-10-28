import assert from 'node:assert/strict';
import test from 'node:test';
import {
  WINDOW_LAYOUT_IDS,
  WINDOW_LAYOUT_SPECS,
} from '../electron/os/window-layout-catalog';
import {
  DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
  type WindowManagementSettings,
} from './window-management-settings';
import { windowLayoutSpecFor } from './window-management-specs';

const SETTINGS_DEPENDENT_IDS = new Set([
  'almost-maximize',
  'reasonable-size',
  'make-larger',
  'make-smaller',
]);

function settingsWith(
  overrides: Partial<WindowManagementSettings>,
): WindowManagementSettings {
  return { ...DEFAULT_WINDOW_MANAGEMENT_SETTINGS, ...overrides };
}

test('layouts without settings keep their catalog spec', () => {
  for (const id of WINDOW_LAYOUT_IDS) {
    if (SETTINGS_DEPENDENT_IDS.has(id)) {
      continue;
    }
    assert.deepEqual(
      windowLayoutSpecFor(id, DEFAULT_WINDOW_MANAGEMENT_SETTINGS),
      WINDOW_LAYOUT_SPECS[id],
      id,
    );
  }
});

test('almost maximize uses the configured percent on both axes without caps', () => {
  assert.deepEqual(
    windowLayoutSpecFor(
      'almost-maximize',
      settingsWith({ almostMaximizePercent: 90 }),
    ),
    { kind: 'scaled-center', widthFraction: 0.9, heightFraction: 0.9 },
  );
  assert.deepEqual(
    windowLayoutSpecFor(
      'almost-maximize',
      settingsWith({ almostMaximizePercent: 50 }),
    ),
    { kind: 'scaled-center', widthFraction: 0.5, heightFraction: 0.5 },
  );
});

test('reasonable size uses the configured percent and keeps the 1025x900 cap', () => {
  assert.deepEqual(
    windowLayoutSpecFor(
      'reasonable-size',
      settingsWith({ reasonableSizePercent: 60 }),
    ),
    {
      kind: 'scaled-center',
      widthFraction: 0.6,
      heightFraction: 0.6,
      maxWidth: 1025,
      maxHeight: 900,
    },
  );
  assert.deepEqual(
    windowLayoutSpecFor(
      'reasonable-size',
      settingsWith({ reasonableSizePercent: 30 }),
    ),
    {
      kind: 'scaled-center',
      widthFraction: 0.3,
      heightFraction: 0.3,
      maxWidth: 1025,
      maxHeight: 900,
    },
  );
});

test('make larger and make smaller use the configured resize step', () => {
  assert.deepEqual(
    windowLayoutSpecFor('make-larger', settingsWith({ resizeStepPercent: 5 })),
    { kind: 'resize-step', direction: 'larger', step: 0.05 },
  );
  assert.deepEqual(
    windowLayoutSpecFor(
      'make-smaller',
      settingsWith({ resizeStepPercent: 12 }),
    ),
    { kind: 'resize-step', direction: 'smaller', step: 0.12 },
  );
});

test('settings never change the half, maximize, or center geometry', () => {
  const customized = settingsWith({
    gap: 40,
    almostMaximizePercent: 55,
    reasonableSizePercent: 35,
    resizeStepPercent: 20,
  });
  assert.deepEqual(
    windowLayoutSpecFor('left-half', customized),
    WINDOW_LAYOUT_SPECS['left-half'],
  );
  assert.deepEqual(windowLayoutSpecFor('center', customized), {
    kind: 'center',
  });
});
