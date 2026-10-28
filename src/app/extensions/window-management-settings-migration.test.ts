// biome-ignore-all lint/style/noMagicNumbers: migration tables are literal legacy values.
import assert from 'node:assert/strict';
import test from 'node:test';
import { DEFAULT_WINDOW_MANAGEMENT_SETTINGS } from './window-management-settings-model';
import { normalizeWindowManagementSettings } from './window-management-settings-normalize';

test('legacy gap seeds both outer and inner gap', () => {
  const migrated = normalizeWindowManagementSettings({ gap: 12 });
  assert.equal(migrated.outerGap, 12);
  assert.equal(migrated.innerGap, 12);
  assert.equal(migrated.outerGapTop, null);
});

test('legacy gap above the new maximum clamps to 40 on both gaps', () => {
  const migrated = normalizeWindowManagementSettings({ gap: 100 });
  assert.equal(migrated.outerGap, 40);
  assert.equal(migrated.innerGap, 40);
});

test('invalid legacy gap leaves the gap defaults in place', () => {
  const migrated = normalizeWindowManagementSettings({ gap: 'wide' });
  assert.equal(migrated.outerGap, DEFAULT_WINDOW_MANAGEMENT_SETTINGS.outerGap);
  assert.equal(migrated.innerGap, DEFAULT_WINDOW_MANAGEMENT_SETTINGS.innerGap);
});

test('legacy almost maximize percent applies to both axes', () => {
  const migrated = normalizeWindowManagementSettings({
    almostMaximizePercent: 75,
  });
  assert.equal(migrated.almostMaximizeWidthPercent, 75);
  assert.equal(migrated.almostMaximizeHeightPercent, 75);
});

test('legacy reasonable size percent applies to both axes', () => {
  const migrated = normalizeWindowManagementSettings({
    reasonableSizePercent: 60,
  });
  assert.equal(migrated.reasonableSizeWidthPercent, 60);
  assert.equal(migrated.reasonableSizeHeightPercent, 60);
});

test('legacy cycle and resize step are kept', () => {
  const migrated = normalizeWindowManagementSettings({
    cycle: false,
    resizeStepPercent: 10,
  });
  assert.equal(migrated.cycle, false);
  assert.equal(migrated.resizeStepPercent, 10);
});

test('a complete legacy record migrates to the new shape', () => {
  const migrated = normalizeWindowManagementSettings({
    gap: 6,
    cycle: false,
    almostMaximizePercent: 80,
    reasonableSizePercent: 40,
    resizeStepPercent: 2,
  });
  assert.deepEqual(migrated, {
    ...DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
    outerGap: 6,
    innerGap: 6,
    cycle: false,
    almostMaximizeWidthPercent: 80,
    almostMaximizeHeightPercent: 80,
    reasonableSizeWidthPercent: 40,
    reasonableSizeHeightPercent: 40,
    resizeStepPercent: 2,
  });
});

test('explicit new keys win over legacy keys in the same record', () => {
  const migrated = normalizeWindowManagementSettings({
    gap: 20,
    outerGap: 5,
    almostMaximizePercent: 60,
    almostMaximizeWidthPercent: 70,
  });
  assert.equal(migrated.outerGap, 5);
  assert.equal(migrated.innerGap, 20);
  assert.equal(migrated.almostMaximizeWidthPercent, 70);
  assert.equal(migrated.almostMaximizeHeightPercent, 60);
});

test('a record without legacy keys is not migrated', () => {
  const current = normalizeWindowManagementSettings({ innerGap: 9 });
  assert.equal(current.outerGap, DEFAULT_WINDOW_MANAGEMENT_SETTINGS.outerGap);
  assert.equal(current.innerGap, 9);
});
