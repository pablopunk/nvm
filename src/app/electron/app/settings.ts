import {
  DEFAULT_UPDATE_CHANNEL,
  UPDATE_CHANNEL_OPTIONS,
  type UpdateChannel,
} from './update-channel';

export type ChoiceSettingOption = { value: string; label: string };

export type SettingDefinition =
  | {
      id: 'paletteHotkey' | 'hyperKey';
      title: string;
      description: string;
      icon: string;
      type: 'shortcut';
      default: string;
    }
  | {
      id:
        | 'showClipboardInRoot'
        | 'showClipboardInRootSearch'
        | 'startAtLogin'
        | 'errorReporting';
      title: string;
      description: string;
      icon: string;
      type: 'boolean';
      default: boolean;
      capability?: string;
    }
  | {
      id: 'updateChannel';
      title: string;
      description: string;
      icon: string;
      type: 'choice';
      default: UpdateChannel;
      options: ReadonlyArray<ChoiceSettingOption>;
    };

export type SettingId = SettingDefinition['id'];
export type SettingsState = Partial<Record<SettingId, string | boolean>>;

export const SETTING_DEFINITIONS: SettingDefinition[] = [
  {
    id: 'errorReporting',
    title: 'Send Technical Diagnostics',
    description:
      'Send errors, usage and performance data to PostHog, without prompts or clipboard content',
    icon: 'shield-check',
    type: 'boolean',
    default: true,
  },
  {
    id: 'paletteHotkey',
    title: 'Open Nevermind Shortcut',
    description: 'Optional global keyboard shortcut to open the palette',
    icon: 'keyboard',
    type: 'shortcut',
    default: '',
  },
  {
    id: 'hyperKey',
    title: 'Hyper Key',
    description: 'Key combination displayed as Hyper in shortcut labels',
    icon: 'command',
    type: 'shortcut',
    default: 'Command+Control+Alt+Shift',
  },
  {
    id: 'showClipboardInRoot',
    title: 'Show Clipboard Items in Root List',
    description: 'Show copied items inline in the root list',
    icon: 'clipboard',
    type: 'boolean',
    default: true,
  },
  {
    id: 'showClipboardInRootSearch',
    title: 'Show Clipboard Items in Root Search',
    description: 'Show copied items when searching from the root list',
    icon: 'clipboard',
    type: 'boolean',
    default: true,
  },
  {
    id: 'startAtLogin',
    title: 'Start at Login',
    description: 'Open Nevermind automatically after you sign in',
    icon: 'log-in',
    type: 'boolean',
    default: false,
    capability: 'launch-at-login',
  },
  {
    id: 'updateChannel',
    title: 'Update Channel',
    description:
      'Choose stable releases, nightly builds, or frequent dev builds.',
    icon: 'restart',
    type: 'choice',
    default: DEFAULT_UPDATE_CHANNEL,
    options: UPDATE_CHANNEL_OPTIONS,
  },
];

export function settingDefinition(id: string) {
  return SETTING_DEFINITIONS.find((entry) => entry.id === id);
}

export function settingValue(
  settings: SettingsState | undefined,
  id: SettingId,
) {
  const definition = settingDefinition(id);
  if (!definition) return;
  const stored = settings?.[id];
  return stored === undefined ? definition.default : stored;
}

type ChoiceSettingShape = {
  default: string;
  options: ReadonlyArray<ChoiceSettingOption>;
};

export function choiceSettingValue(
  definition: ChoiceSettingShape,
  value: unknown,
) {
  return definition.options.some((option) => option.value === value)
    ? String(value)
    : definition.default;
}

export function choiceSettingLabel(
  definition: ChoiceSettingShape,
  value: unknown,
) {
  const effective = choiceSettingValue(definition, value);
  return (
    definition.options.find((option) => option.value === effective)?.label ??
    effective
  );
}

export function nextChoiceSettingValue(
  definition: ChoiceSettingShape,
  current: unknown,
) {
  const effective = choiceSettingValue(definition, current);
  const index = definition.options.findIndex(
    (option) => option.value === effective,
  );
  return definition.options[(index + 1) % definition.options.length].value;
}

export function toggledSettingValue(
  definition: SettingDefinition,
  current: unknown,
) {
  if (definition.type === 'boolean') return !current;
  if (definition.type === 'choice')
    return nextChoiceSettingValue(definition, current);
  return current;
}
