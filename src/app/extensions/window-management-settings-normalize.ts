import {
  type ChoiceSettingDefinition,
  type WholeNumberSettingDefinition,
  WINDOW_MANAGEMENT_SETTING_DEFINITIONS,
  type WindowManagementSettingDefinition,
} from './window-management-settings-definitions';
import { migrateLegacyWindowManagementSettings } from './window-management-settings-migration';
import {
  DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
  type WindowManagementSettings,
} from './window-management-settings-model';
import {
  clampWholeNumber,
  isFiniteNumber,
  isRecord,
} from './window-management-settings-values';

function normalizeWholeNumberValue(
  value: unknown,
  definition: WholeNumberSettingDefinition,
): number | null {
  if (!isFiniteNumber(value)) {
    return null;
  }
  return clampWholeNumber(Math.round(value), definition.min, definition.max);
}

function isChoiceValue(value: unknown, definition: ChoiceSettingDefinition) {
  return definition.options.some((option) => option.value === value);
}

function normalizeSettingValue(
  definition: WindowManagementSettingDefinition,
  value: unknown,
): unknown {
  const fallback = DEFAULT_WINDOW_MANAGEMENT_SETTINGS[definition.key];
  if (definition.kind === 'whole-number') {
    return normalizeWholeNumberValue(value, definition) ?? fallback;
  }
  if (definition.kind === 'boolean') {
    return typeof value === 'boolean' ? value : fallback;
  }
  if (definition.kind === 'choice') {
    return isChoiceValue(value, definition) ? value : fallback;
  }
  return fallback;
}

export function normalizeWindowManagementSettings(
  raw: unknown,
): WindowManagementSettings {
  const record = migrateLegacyWindowManagementSettings(
    isRecord(raw) ? raw : {},
  );
  const settings: Record<string, unknown> = {};
  for (const definition of WINDOW_MANAGEMENT_SETTING_DEFINITIONS) {
    settings[definition.key] = normalizeSettingValue(
      definition,
      record[definition.key],
    );
  }
  return settings as unknown as WindowManagementSettings;
}
