import type {
  ExtensionContext,
  ExtensionView,
} from '../resources/nevermind-extension-api';
import type { WindowManagementSettingsStore } from './window-management-settings';
import type { WholeNumberSettingDefinition } from './window-management-settings-definitions';
import { parseWholeNumberSettingInput } from './window-management-settings-input';
import type { WindowManagementSettings } from './window-management-settings-model';
import { windowManagementSettingsView } from './window-management-settings-view';

const SAVE_TITLE = 'Save';
const VALUE_FIELD_ID = 'value';

type SettingsContext = Pick<ExtensionContext, 'actions' | 'input' | 'views'>;

interface WholeNumberSettingEditor {
  definition: WholeNumberSettingDefinition;
  store: WindowManagementSettingsStore;
}

function settingPromptFields(
  definition: WholeNumberSettingDefinition,
  value: string,
  error?: string,
) {
  return [
    {
      id: VALUE_FIELD_ID,
      label: definition.title,
      type: 'number' as const,
      value,
      required: !definition.nullable,
      description: definition.description,
      error,
    },
  ];
}

function settingSaveAction(
  ctx: SettingsContext,
  editor: WholeNumberSettingEditor,
) {
  const { definition, store } = editor;
  return ctx.actions.run(SAVE_TITLE, async (innerCtx, action) => {
    const raw = action.formValues?.[VALUE_FIELD_ID];
    const parsed = parseWholeNumberSettingInput(raw, definition);
    if (parsed.ok === false) {
      return {
        view: settingPromptView(
          innerCtx,
          editor,
          String(raw ?? ''),
          parsed.message,
        ),
        navigation: 'replace' as const,
      };
    }
    const current = await store.load();
    await store.save({
      ...current,
      [definition.key]: parsed.value,
    } as WindowManagementSettings);
    return {
      view: await windowManagementSettingsView(innerCtx, store),
      navigation: 'root' as const,
    };
  });
}

export function settingPromptView(
  ctx: SettingsContext,
  editor: WholeNumberSettingEditor,
  value: string,
  error?: string,
): ExtensionView {
  const { definition } = editor;
  return {
    type: 'prompt',
    title: definition.title,
    subtitle: definition.description,
    fields: settingPromptFields(definition, value, error),
    submitAction: settingSaveAction(ctx, editor),
  };
}

export function settingPromptAction(
  ctx: SettingsContext,
  editor: WholeNumberSettingEditor,
  value: number | null,
) {
  const { definition } = editor;
  return ctx.input.prompt({
    title: definition.title,
    message: definition.description,
    fields: settingPromptFields(
      definition,
      value === null ? '' : String(value),
    ),
    submitTitle: SAVE_TITLE,
    action: settingSaveAction(ctx, editor),
  });
}
