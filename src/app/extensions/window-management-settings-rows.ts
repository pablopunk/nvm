import type {
  ExtensionContext,
  ExtensionItem,
} from '../resources/nevermind-extension-api';
import { showExtensionFeedback } from './feedback';
import type { WindowManagementSettingsStore } from './window-management-settings';
import type {
  BooleanSettingDefinition,
  ChoiceSettingDefinition,
  WholeNumberSettingDefinition,
  WindowManagementSettingDefinition,
  WindowManagementSettingSection,
} from './window-management-settings-definitions';
import {
  resetSettingsToDefaults,
  type WindowManagementSettings,
} from './window-management-settings-model';
import { settingPromptAction } from './window-management-settings-prompts';

type RowContext = Pick<ExtensionContext, 'ui' | 'actions' | 'input' | 'views'>;

const SAME_AS_OUTER_GAP_TEXT = 'Same as outer gap';

function wholeNumberValueText(
  definition: WholeNumberSettingDefinition,
  value: number | null,
) {
  if (value === null) {
    return { text: SAME_AS_OUTER_GAP_TEXT, tone: 'muted' as const };
  }
  return { text: `${value}${definition.unit === 'px' ? ' px' : '%'}` };
}

function choiceLabel(definition: ChoiceSettingDefinition, value: string) {
  return definition.options.find((option) => option.value === value)?.label;
}

function nextChoiceValue(definition: ChoiceSettingDefinition, current: string) {
  const { options } = definition;
  const currentIndex = options.findIndex((option) => option.value === current);
  return options[(currentIndex + 1) % options.length].value;
}

function settingAccessory(
  definition: WindowManagementSettingDefinition,
  settings: WindowManagementSettings,
) {
  const value = settings[definition.key];
  if (definition.kind === 'whole-number') {
    return wholeNumberValueText(definition, value as number | null);
  }
  if (definition.kind === 'boolean') {
    return value
      ? { text: 'On', tone: 'success' as const }
      : { text: 'Off', tone: 'muted' as const };
  }
  return {
    text: choiceLabel(definition as ChoiceSettingDefinition, value as string),
  };
}

function wholeNumberRow(
  ctx: RowContext,
  store: WindowManagementSettingsStore,
  definition: WholeNumberSettingDefinition,
  settings: WindowManagementSettings,
): ExtensionItem {
  return ctx.ui.item({
    id: definition.key,
    title: definition.title,
    subtitle: definition.description,
    icon: 'ruler',
    accessories: [settingAccessory(definition, settings)],
    primaryAction: settingPromptAction(
      ctx,
      { definition, store },
      settings[definition.key] as number | null,
    ),
  });
}

function booleanRow(
  ctx: RowContext,
  store: WindowManagementSettingsStore,
  definition: BooleanSettingDefinition,
  settings: WindowManagementSettings,
): ExtensionItem {
  return ctx.ui.item({
    id: definition.key,
    title: definition.title,
    subtitle: definition.description,
    icon: 'toggle-right',
    accessories: [settingAccessory(definition, settings)],
    primaryAction: ctx.actions.run(
      `Toggle ${definition.title}`,
      async (innerCtx) => {
        const current = await store.load();
        const next = {
          ...current,
          [definition.key]: !current[definition.key],
        } as WindowManagementSettings;
        await store.save(next);
        return {
          patch: { items: [booleanRow(innerCtx, store, definition, next)] },
        };
      },
    ),
  });
}

function choiceRow(
  ctx: RowContext,
  store: WindowManagementSettingsStore,
  definition: ChoiceSettingDefinition,
  settings: WindowManagementSettings,
): ExtensionItem {
  return ctx.ui.item({
    id: definition.key,
    title: definition.title,
    subtitle: definition.description,
    icon: 'list-ordered',
    accessories: [settingAccessory(definition, settings)],
    primaryAction: ctx.actions.run(
      `Change ${definition.title}`,
      async (innerCtx) => {
        const current = await store.load();
        const next = {
          ...current,
          [definition.key]: nextChoiceValue(
            definition,
            current[definition.key] as string,
          ),
        } as WindowManagementSettings;
        await store.save(next);
        return {
          patch: { items: [choiceRow(innerCtx, store, definition, next)] },
        };
      },
    ),
  });
}

export function settingRowFor(
  ctx: RowContext,
  store: WindowManagementSettingsStore,
  definition: WindowManagementSettingDefinition,
  settings: WindowManagementSettings,
): ExtensionItem {
  if (definition.kind === 'whole-number') {
    return wholeNumberRow(ctx, store, definition, settings);
  }
  if (definition.kind === 'boolean') {
    return booleanRow(ctx, store, definition, settings);
  }
  return choiceRow(ctx, store, definition as ChoiceSettingDefinition, settings);
}

export function resetSectionRow(
  ctx: RowContext,
  store: WindowManagementSettingsStore,
  section: WindowManagementSettingSection,
): ExtensionItem {
  return ctx.ui.item({
    id: `reset-${section.id}`,
    title: `Reset ${section.title}`,
    subtitle: `Restore ${section.title.toLowerCase()} settings to their defaults.`,
    icon: 'rotate-ccw',
    primaryAction: ctx.actions.run(
      `Reset ${section.title}`,
      async (innerCtx) => {
        const keys = section.settings.map((setting) => setting.key);
        const next = resetSettingsToDefaults(await store.load(), keys);
        await store.save(next);
        showExtensionFeedback(
          innerCtx,
          section.title,
          `${section.title} settings reset to defaults.`,
          'success',
        );
        return {
          patch: {
            items: section.settings.map((setting) =>
              settingRowFor(innerCtx, store, setting, next),
            ),
          },
        };
      },
    ),
  });
}
