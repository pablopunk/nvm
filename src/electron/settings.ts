export type SettingDefinition =
  | {
      id: 'paletteHotkey' | 'hyperKey'
      title: string
      description: string
      type: 'shortcut'
      default: string
    }
  | {
      id: 'showClipboardInRoot'
      title: string
      description: string
      type: 'boolean'
      default: boolean
    }

export type SettingId = SettingDefinition['id']
export type SettingsState = Partial<Record<SettingId, string | boolean>>

export const SETTING_DEFINITIONS: SettingDefinition[] = [
  {
    id: 'paletteHotkey',
    title: 'Open Nevermind Shortcut',
    description: 'Global keyboard shortcut that toggles the palette',
    type: 'shortcut',
    default: 'Alt+Space',
  },
  {
    id: 'hyperKey',
    title: 'Hyper Key',
    description: 'Key combination displayed as Hyper in shortcut labels',
    type: 'shortcut',
    default: 'Command+Control+Alt+Shift',
  },
  {
    id: 'showClipboardInRoot',
    title: 'Show Clipboard Items in Main List',
    description: 'Show copied items inline in the root list',
    type: 'boolean',
    default: true,
  },
]

export function settingDefinition(id: string) {
  return SETTING_DEFINITIONS.find((entry) => entry.id === id)
}

export function settingValue(settings: SettingsState | undefined, id: SettingId) {
  const definition = settingDefinition(id)
  if (!definition) return undefined
  const stored = settings?.[id]
  return stored === undefined ? definition.default : stored
}

export function toggledSettingValue(definition: SettingDefinition, current: unknown) {
  return definition.type === 'boolean' ? !current : current
}
