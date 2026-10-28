import type {
  WindowCycleOrder,
  WindowDisplayMoveSizing,
  WindowDisplayOrder,
} from '../electron/os/window-management-types';

export interface WindowManagementSettings {
  outerGap: number;
  outerGapTop: number | null;
  outerGapRight: number | null;
  outerGapBottom: number | null;
  outerGapLeft: number | null;
  innerGap: number;
  applyGapToMaximize: boolean;
  almostMaximizeWidthPercent: number;
  almostMaximizeHeightPercent: number;
  reasonableSizeWidthPercent: number;
  reasonableSizeHeightPercent: number;
  centerHalfWidthPercent: number;
  resizeStepPercent: number;
  cycle: boolean;
  cycleOrder: WindowCycleOrder;
  displayOrder: WindowDisplayOrder;
  displayMoveSizing: WindowDisplayMoveSizing;
}

export type WindowManagementSettingKey = keyof WindowManagementSettings;

export const DEFAULT_WINDOW_MANAGEMENT_SETTINGS: WindowManagementSettings = {
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
};

export function resetSettingsToDefaults(
  settings: WindowManagementSettings,
  keys: WindowManagementSettingKey[],
): WindowManagementSettings {
  const next: Record<string, unknown> = { ...settings };
  for (const key of keys) {
    next[key] = DEFAULT_WINDOW_MANAGEMENT_SETTINGS[key];
  }
  return next as unknown as WindowManagementSettings;
}
