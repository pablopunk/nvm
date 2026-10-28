import type {
  ExtensionContext,
  ExtensionItem,
  ExtensionView,
} from '../resources/nevermind-extension-api';
import { showExtensionFeedback } from './feedback';
import {
  ALMOST_MAXIMIZE_SETTING,
  DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
  extensionWindowManagementSettingsStore,
  GAP_SETTING,
  parseWholeNumberSettingInput,
  REASONABLE_SIZE_SETTING,
  RESIZE_STEP_SETTING,
  type WholeNumberSettingDefinition,
  type WholeNumberSettingKey,
  type WindowManagementSettings,
  type WindowManagementSettingsStore,
} from './window-management-settings';

const SETTINGS_VIEW_ID = 'window-management-settings';
const SETTINGS_VIEW_TITLE = 'Window Management Settings';
const SETTINGS_VIEW_SUBTITLE = 'Applied to every window layout command';
const VALUE_FIELD_ID = 'value';
const SAVE_TITLE = 'Save';

interface WholeNumberSettingEditor {
  definition: WholeNumberSettingDefinition;
  store: WindowManagementSettingsStore;
}

function wholeNumberEditor(
  definition: WholeNumberSettingDefinition,
  store: WindowManagementSettingsStore,
): WholeNumberSettingEditor {
  return { definition, store };
}

function formatWholeNumberSetting(
  definition: WholeNumberSettingDefinition,
  value: number,
) {
  return definition.unit === 'px' ? `${value} px` : `${value}%`;
}

function wholeNumberSettingField(
  definition: WholeNumberSettingDefinition,
  value: string,
  error?: string,
) {
  return {
    id: VALUE_FIELD_ID,
    label: definition.title,
    type: 'number' as const,
    value,
    required: true,
    description: definition.description,
    error,
  };
}

function withWholeNumberSetting(
  settings: WindowManagementSettings,
  key: WholeNumberSettingKey,
  value: number,
): WindowManagementSettings {
  return { ...settings, [key]: value };
}

function wholeNumberSaveAction(
  ctx: ExtensionContext,
  editor: WholeNumberSettingEditor,
) {
  return ctx.actions.run(SAVE_TITLE, async (innerCtx, action) => {
    const raw = action.formValues?.[VALUE_FIELD_ID];
    const parsed = parseWholeNumberSettingInput(raw, editor.definition);
    if (parsed.ok === false) {
      return {
        view: wholeNumberPromptView(
          innerCtx,
          editor,
          String(raw ?? ''),
          parsed.message,
        ),
        navigation: 'replace' as const,
      };
    }
    const current = await editor.store.load();
    await editor.store.save(
      withWholeNumberSetting(current, editor.definition.key, parsed.value),
    );
    return {
      navigation: 'pop' as const,
      action: innerCtx.views.refresh(),
    };
  });
}

function wholeNumberPromptView(
  ctx: ExtensionContext,
  editor: WholeNumberSettingEditor,
  value: string,
  error?: string,
): ExtensionView {
  return {
    type: 'prompt',
    title: editor.definition.title,
    subtitle: editor.definition.description,
    fields: [wholeNumberSettingField(editor.definition, value, error)],
    submitAction: wholeNumberSaveAction(ctx, editor),
  };
}

function wholeNumberSettingItem(
  ctx: ExtensionContext,
  editor: WholeNumberSettingEditor,
  settings: WindowManagementSettings,
): ExtensionItem {
  const { definition } = editor;
  const value = settings[definition.key];
  return ctx.ui.item({
    id: definition.key,
    title: definition.title,
    subtitle: definition.description,
    icon: 'ruler',
    accessories: [
      { text: formatWholeNumberSetting(definition, value), tone: 'muted' },
    ],
    primaryAction: ctx.input.prompt({
      title: definition.title,
      message: definition.description,
      fields: [wholeNumberSettingField(definition, String(value))],
      submitTitle: SAVE_TITLE,
      action: wholeNumberSaveAction(ctx, editor),
    }),
  });
}

function cycleSettingItem(
  ctx: ExtensionContext,
  store: WindowManagementSettingsStore,
  settings: WindowManagementSettings,
): ExtensionItem {
  return ctx.ui.item({
    id: 'cycle',
    title: 'Cycle Half Commands',
    subtitle: 'Repeat a half command on the same window to cycle ½, ⅔, ⅓.',
    icon: 'refresh-cw',
    accessories: [
      settings.cycle
        ? { text: 'On', tone: 'success' }
        : { text: 'Off', tone: 'muted' },
    ],
    primaryAction: ctx.actions.run('Toggle Cycle', async (innerCtx) => {
      const current = await store.load();
      const next = { ...current, cycle: !current.cycle };
      await store.save(next);
      return { patch: { items: settingItems(innerCtx, store, next) } };
    }),
  });
}

function resetSettingsItem(
  ctx: ExtensionContext,
  store: WindowManagementSettingsStore,
): ExtensionItem {
  return ctx.ui.item({
    id: 'reset-defaults',
    title: 'Reset to Defaults',
    subtitle: 'Restore every window management setting to its default.',
    icon: 'rotate-ccw',
    primaryAction: ctx.actions.run('Reset to Defaults', async (innerCtx) => {
      await store.save(DEFAULT_WINDOW_MANAGEMENT_SETTINGS);
      showExtensionFeedback(
        innerCtx,
        SETTINGS_VIEW_TITLE,
        'Settings reset to defaults.',
        'success',
      );
      return {
        patch: {
          items: settingItems(
            innerCtx,
            store,
            DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
          ),
        },
      };
    }),
  });
}

function settingItems(
  ctx: ExtensionContext,
  store: WindowManagementSettingsStore,
  settings: WindowManagementSettings,
): ExtensionItem[] {
  return [
    wholeNumberSettingItem(
      ctx,
      wholeNumberEditor(GAP_SETTING, store),
      settings,
    ),
    cycleSettingItem(ctx, store, settings),
    wholeNumberSettingItem(
      ctx,
      wholeNumberEditor(ALMOST_MAXIMIZE_SETTING, store),
      settings,
    ),
    wholeNumberSettingItem(
      ctx,
      wholeNumberEditor(REASONABLE_SIZE_SETTING, store),
      settings,
    ),
    wholeNumberSettingItem(
      ctx,
      wholeNumberEditor(RESIZE_STEP_SETTING, store),
      settings,
    ),
  ];
}

export async function windowManagementSettingsView(
  ctx: ExtensionContext,
  store: WindowManagementSettingsStore = extensionWindowManagementSettingsStore(
    ctx,
  ),
) {
  const settings = await store.load();
  return ctx.ui.list({
    id: SETTINGS_VIEW_ID,
    title: SETTINGS_VIEW_TITLE,
    subtitle: SETTINGS_VIEW_SUBTITLE,
    presentation: 'root',
    searchBarPlaceholder: 'Search window settings',
    items: [
      ...settingItems(ctx, store, settings),
      resetSettingsItem(ctx, store),
    ],
  });
}
