// biome-ignore-all lint: Setting definitions and open-view patches are dynamic payloads moved verbatim from main.ts.
import type { UserStateStore } from '../state/user-state-store';
import {
  SETTING_DEFINITIONS,
  settingDefinition,
  settingValue,
} from './settings';

export type SettingsServiceDeps = {
  userStateStore: Pick<UserStateStore, 'get'>;
  commitPortable: (input: {
    label: string;
    mutate: (draft: any) => string | void;
  }) => Promise<{ ok: true } | { ok: false; message: string }>;
  activationFailure: (key: string) => string | null;
  hasCapability: (capability: string) => boolean;
  getLaunchAtLoginEnabled: () => boolean;
  setLaunchAtLoginEnabled: (enabled: boolean) => {
    ok: boolean;
    message?: string;
  };
  osLabel: () => string;
  patchOpenView: (viewId: string, patch: any) => void;
  buildRecordShortcutAction: (input: any, options: any) => any;
};

export function createSettingsService(deps: SettingsServiceDeps) {
  const {
    userStateStore,
    hasCapability,
    getLaunchAtLoginEnabled,
    setLaunchAtLoginEnabled,
    osLabel,
    commitPortable,
    activationFailure,
    patchOpenView,
    buildRecordShortcutAction,
  } = deps;

  function getPaletteHotkey() {
    return String(getSetting('paletteHotkey') ?? '');
  }

  function settingIsAvailable(definition: any) {
    return !definition?.capability || hasCapability(definition.capability);
  }

  function availableSettingDefinitions() {
    return SETTING_DEFINITIONS.filter(settingIsAvailable);
  }

  function getSetting(id: any) {
    const definition = settingDefinition(String(id));
    if (!(definition && settingIsAvailable(definition))) return;
    if (definition.id === 'startAtLogin') return getLaunchAtLoginEnabled();
    return settingValue(userStateStore.get().settings, definition.id);
  }

  async function setSetting(id: any, value: any) {
    const definition = settingDefinition(String(id));
    if (!definition) return { ok: false, message: 'Setting not found' };
    if (!settingIsAvailable(definition))
      return {
        ok: false,
        message: `${definition.title} is not available on ${osLabel()}`,
      };
    const result = await commitPortable({
      label: `setting:${definition.id}`,
      mutate: (draft) => {
        draft.settings[definition.id] = value;
      },
    });
    if (!result.ok) return result;
    return { ok: true, message: `${definition.title} updated` };
  }

  function settingItemPatch(definition) {
    const value = getSetting(definition.id);
    const accessoryText = activationFailure(`setting:${definition.id}`)
      ? 'Not active'
      : definition.type === 'boolean'
        ? value
          ? 'On'
          : 'Off'
        : definition.type === 'shortcut'
          ? ''
          : String(value);
    const shortcutInput =
      definition.id === 'paletteHotkey'
        ? {
            scope: 'palette',
            title: 'Change Shortcut',
            shortcut: String(value || ''),
          }
        : {
            action: { id: '__hyper-key__' },
            title: 'Change Hyper Key',
            shortcut: String(value || ''),
          };
    const primaryAction =
      definition.type === 'shortcut'
        ? buildRecordShortcutAction(shortcutInput, {})
        : {
            type: 'toggleSetting',
            title: value ? 'Turn Off' : 'Turn On',
            settingId: definition.id,
          };
    return {
      id: `setting:${definition.id}`,
      accessories: accessoryText ? [{ text: accessoryText }] : [],
      primaryAction,
      actionPanel: { sections: [{ actions: [primaryAction] }] },
    };
  }

  function settingsItems() {
    return availableSettingDefinitions().map((definition) => ({
      id: `setting:${definition.id}`,
      title: definition.title,
      subtitle: definition.description,
      icon: definition.icon || 'settings',
      ...settingItemPatch(definition),
    }));
  }

  function settingsView(selectedItemId = '') {
    return {
      type: 'list',
      id: 'app-settings',
      title: 'Settings',
      presentation: 'root',
      selectedItemId,
      searchBarPlaceholder: 'Search Settings',
      items: settingsItems(),
    };
  }

  function patchSettingsView(settingId: string, options: any = {}) {
    const definition = SETTING_DEFINITIONS.find(
      (item) => item.id === settingId,
    );
    if (!(definition && settingIsAvailable(definition))) return;
    patchOpenView('app-settings', {
      mode: 'patch',
      items: [
        { id: `setting:${definition.id}`, ...settingItemPatch(definition) },
      ],
      ...options,
    });
  }

  return {
    getPaletteHotkey,
    settingIsAvailable,
    availableSettingDefinitions,
    getSetting,
    setSetting,
    settingItemPatch,
    settingsItems,
    settingsView,
    patchSettingsView,
  };
}
