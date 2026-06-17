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
      id: 'showClipboardInRoot' | 'startAtLogin';
      title: string;
      description: string;
      icon: string;
      type: 'boolean';
      default: boolean;
      capability?: string;
    };

export type SettingId = SettingDefinition['id'];
export type SettingsState = Partial<Record<SettingId, string | boolean>>;

export const SETTING_DEFINITIONS: SettingDefinition[] = [
  {
    id: 'paletteHotkey',
    title: 'Open Nevermind Shortcut',
    description: 'Global keyboard shortcut that toggles the palette',
    icon: 'keyboard',
    type: 'shortcut',
    default: 'Alt+Space',
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
    title: 'Show Clipboard Items in Main List',
    description: 'Show copied items inline in the root list',
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
];

export function settingDefinition(id: string) {
  return SETTING_DEFINITIONS.find((entry) => entry.id === id);
}

export function settingValue(
  settings: SettingsState | undefined,
  id: SettingId,
) {
  const definition = settingDefinition(id);
  if (!definition) return undefined;
  const stored = settings?.[id];
  return stored === undefined ? definition.default : stored;
}

export function toggledSettingValue(
  definition: SettingDefinition,
  current: unknown,
) {
  return definition.type === 'boolean' ? !current : current;
}
