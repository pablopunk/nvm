import { MAX_WINDOW_GAP_POINTS } from '../electron/os/window-management-types';
import {
  isFiniteNumber,
  type SettingsRecord,
} from './window-management-settings-values';

function legacyGapFields(record: SettingsRecord): SettingsRecord {
  if (!isFiniteNumber(record.gap)) {
    return {};
  }
  const gap = Math.min(record.gap, MAX_WINDOW_GAP_POINTS);
  return { outerGap: gap, innerGap: gap };
}

function legacyAlmostMaximizeFields(record: SettingsRecord): SettingsRecord {
  const percent = record.almostMaximizePercent;
  if (!isFiniteNumber(percent)) {
    return {};
  }
  return {
    almostMaximizeWidthPercent: percent,
    almostMaximizeHeightPercent: percent,
  };
}

function legacyReasonableSizeFields(record: SettingsRecord): SettingsRecord {
  const percent = record.reasonableSizePercent;
  if (!isFiniteNumber(percent)) {
    return {};
  }
  return {
    reasonableSizeWidthPercent: percent,
    reasonableSizeHeightPercent: percent,
  };
}

export function migrateLegacyWindowManagementSettings(
  record: SettingsRecord,
): SettingsRecord {
  return {
    ...legacyGapFields(record),
    ...legacyAlmostMaximizeFields(record),
    ...legacyReasonableSizeFields(record),
    ...record,
  };
}
