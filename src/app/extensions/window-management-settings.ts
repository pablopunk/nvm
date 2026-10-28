import {
  DEFAULT_WINDOW_MANAGEMENT_OPTIONS,
  type WindowManagementOptions,
} from '../electron/os/window-management-types';
import type { ExtensionContext } from '../resources/nevermind-extension-api';

const WHOLE_NUMBER_TEXT_PATTERN = /^\d+$/;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function clampWholeNumber(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function normalizeWholeNumberSetting(
  value: unknown,
  definition: WholeNumberSettingDefinition,
) {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return DEFAULT_WINDOW_MANAGEMENT_SETTINGS[definition.key];
  }
  return clampWholeNumber(Math.round(value), definition.min, definition.max);
}

function normalizeCycleSetting(value: unknown) {
  return typeof value === 'boolean'
    ? value
    : DEFAULT_WINDOW_MANAGEMENT_SETTINGS.cycle;
}

export const WINDOW_MANAGEMENT_SETTINGS_KEY = 'window-management.settings';

export interface WindowManagementSettings extends WindowManagementOptions {
  almostMaximizePercent: number;
  reasonableSizePercent: number;
  resizeStepPercent: number;
}

export const DEFAULT_WINDOW_MANAGEMENT_SETTINGS: WindowManagementSettings = {
  ...DEFAULT_WINDOW_MANAGEMENT_OPTIONS,
  almostMaximizePercent: 90,
  reasonableSizePercent: 60,
  resizeStepPercent: 5,
};

export type WholeNumberSettingKey = Exclude<
  keyof WindowManagementSettings,
  'cycle'
>;

export interface WholeNumberSettingDefinition {
  key: WholeNumberSettingKey;
  title: string;
  description: string;
  unit: 'px' | '%';
  min: number;
  max: number;
}

export const GAP_SETTING: WholeNumberSettingDefinition = {
  key: 'gap',
  title: 'Window Gap',
  description: 'Space between tiled windows and the screen edges.',
  unit: 'px',
  min: 0,
  max: 100,
};

export const ALMOST_MAXIMIZE_SETTING: WholeNumberSettingDefinition = {
  key: 'almostMaximizePercent',
  title: 'Almost Maximize Size',
  description: 'Share of the work area that Almost Maximize fills.',
  unit: '%',
  min: 50,
  max: 100,
};

export const REASONABLE_SIZE_SETTING: WholeNumberSettingDefinition = {
  key: 'reasonableSizePercent',
  title: 'Reasonable Size',
  description:
    'Share of the work area that Reasonable Size fills, capped at 1025×900.',
  unit: '%',
  min: 30,
  max: 100,
};

export const RESIZE_STEP_SETTING: WholeNumberSettingDefinition = {
  key: 'resizeStepPercent',
  title: 'Resize Step',
  description:
    'Share of the work area that Make Larger and Make Smaller change by.',
  unit: '%',
  min: 1,
  max: 25,
};

export type WholeNumberParseResult =
  | { ok: true; value: number }
  | { ok: false; message: string };

export function parseWholeNumberSettingInput(
  input: unknown,
  definition: WholeNumberSettingDefinition,
): WholeNumberParseResult {
  const text = String(input ?? '').trim();
  const value = Number(text);
  const isWholeNumberText = WHOLE_NUMBER_TEXT_PATTERN.test(text);
  if (!isWholeNumberText || value < definition.min || value > definition.max) {
    return {
      ok: false,
      message: `Enter a whole number from ${definition.min} to ${definition.max}.`,
    };
  }
  return { ok: true, value };
}

export function normalizeWindowManagementSettings(
  raw: unknown,
): WindowManagementSettings {
  const record = isRecord(raw) ? raw : {};
  return {
    gap: normalizeWholeNumberSetting(record.gap, GAP_SETTING),
    cycle: normalizeCycleSetting(record.cycle),
    almostMaximizePercent: normalizeWholeNumberSetting(
      record.almostMaximizePercent,
      ALMOST_MAXIMIZE_SETTING,
    ),
    reasonableSizePercent: normalizeWholeNumberSetting(
      record.reasonableSizePercent,
      REASONABLE_SIZE_SETTING,
    ),
    resizeStepPercent: normalizeWholeNumberSetting(
      record.resizeStepPercent,
      RESIZE_STEP_SETTING,
    ),
  };
}

export interface WindowManagementSettingsStore {
  load(): Promise<WindowManagementSettings>;
  save(settings: WindowManagementSettings): Promise<void>;
}

export function extensionWindowManagementSettingsStore(
  ctx: Pick<ExtensionContext, 'storage'>,
): WindowManagementSettingsStore {
  return {
    async load() {
      return normalizeWindowManagementSettings(
        await ctx.storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY),
      );
    },
    async save(settings) {
      await ctx.storage.set(
        WINDOW_MANAGEMENT_SETTINGS_KEY,
        normalizeWindowManagementSettings(settings),
      );
    },
  };
}
