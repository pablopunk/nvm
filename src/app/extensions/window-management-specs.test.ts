// biome-ignore-all lint/style/noMagicNumbers: layout fraction tables are literal values.
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  WINDOW_LAYOUT_IDS,
  WINDOW_LAYOUT_SPECS,
} from '../electron/os/window-layout-catalog';
import {
  DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
  type WindowManagementSettings,
} from './window-management-settings-model';
import { windowLayoutSpecFor } from './window-management-specs';

const SETTINGS_DEPENDENT_IDS = new Set([
  'almost-maximize',
  'reasonable-size',
  'center-half',
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

test('almost maximize uses the configured width and height percents without caps', () => {
  assert.deepEqual(
    windowLayoutSpecFor(
      'almost-maximize',
      settingsWith({
        almostMaximizeWidthPercent: 85,
        almostMaximizeHeightPercent: 60,
      }),
    ),
    { kind: 'scaled-center', widthFraction: 0.85, heightFraction: 0.6 },
  );
});

test('default almost maximize is 90 percent on both axes', () => {
  assert.deepEqual(
    windowLayoutSpecFor('almost-maximize', DEFAULT_WINDOW_MANAGEMENT_SETTINGS),
    { kind: 'scaled-center', widthFraction: 0.9, heightFraction: 0.9 },
  );
});

test('reasonable size uses its own width and height percents with no cap', () => {
  assert.deepEqual(
    windowLayoutSpecFor(
      'reasonable-size',
      settingsWith({
        reasonableSizeWidthPercent: 100,
        reasonableSizeHeightPercent: 30,
      }),
    ),
    { kind: 'scaled-center', widthFraction: 1, heightFraction: 0.3 },
  );
});

test('default reasonable size is 70 percent on both axes', () => {
  assert.deepEqual(
    windowLayoutSpecFor('reasonable-size', DEFAULT_WINDOW_MANAGEMENT_SETTINGS),
    { kind: 'scaled-center', widthFraction: 0.7, heightFraction: 0.7 },
  );
});

test('center half keeps the configured width and centers it horizontally', () => {
  assert.deepEqual(
    windowLayoutSpecFor(
      'center-half',
      settingsWith({ centerHalfWidthPercent: 50 }),
    ),
    { kind: 'fraction', x: 0.25, y: 0, width: 0.5, height: 1 },
  );
  assert.deepEqual(
    windowLayoutSpecFor(
      'center-half',
      settingsWith({ centerHalfWidthPercent: 60 }),
    ),
    { kind: 'fraction', x: 0.2, y: 0, width: 0.6, height: 1 },
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

test('gap and size settings never change the half, maximize, or center geometry', () => {
  const customized = settingsWith({
    outerGap: 40,
    innerGap: 40,
    almostMaximizeWidthPercent: 55,
    reasonableSizeWidthPercent: 35,
    resizeStepPercent: 20,
  });
  assert.deepEqual(
    windowLayoutSpecFor('left-half', customized),
    WINDOW_LAYOUT_SPECS['left-half'],
  );
  assert.deepEqual(windowLayoutSpecFor('maximize', customized), {
    kind: 'fraction',
    x: 0,
    y: 0,
    width: 1,
    height: 1,
  });
  assert.deepEqual(windowLayoutSpecFor('center', customized), {
    kind: 'center',
  });
});
