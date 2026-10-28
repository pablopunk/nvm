// biome-ignore-all lint: Setting definitions and open-view patches are dynamic payloads moved verbatim from main.ts.
import type { UserStateStore } from '../state/user-state-store';
import {
  choiceSettingLabel,
  choiceSettingValue,
  nextChoiceSettingValue,
  SETTING_DEFINITIONS,
  settingDefinition,
  settingValue,
} from './settings';
import { parseUpdateChannel, type UpdateChannel } from './update-channel';

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
  afterSettingCommitted?: (settingId: string) => void;
  canChangeUpdateChannel?: () => boolean;
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
    afterSettingCommitted,
  } = deps;

  function getPaletteHotkey() {
    return String(getSetting('paletteHotkey') ?? '');
  }

  function getUpdateChannel(): UpdateChannel {
    return parseUpdateChannel(getSetting('updateChannel'));
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
    if (
      definition.id === 'updateChannel' &&
      deps.canChangeUpdateChannel?.() === false
    )
      return {
        ok: false,
        message:
          'Wait for the update installation to finish before changing channels',
      };
    if (!settingIsAvailable(definition))
      return {
        ok: false,
        message: `${definition.title} is not available on ${osLabel()}`,
      };
    if (
      definition.type === 'choice' &&
      choiceSettingValue(definition, value) !== value
    )
      return {
        ok: false,
        message: `${definition.title} has no ${value} option`,
      };
    const result = await commitPortable({
      label: `setting:${definition.id}`,
      mutate: (draft) => {
        draft.settings[definition.id] = value;
      },
    });
    if (!result.ok) return result;
    afterSettingCommitted?.(definition.id);
    return { ok: true, message: `${definition.title} updated` };
  }

  function settingToggleTitle(definition, value) {
    if (definition.type === 'choice')
      return `Switch to ${choiceSettingLabel(definition, nextChoiceSettingValue(definition, value))}`;
    return value ? 'Turn Off' : 'Turn On';
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
          : definition.type === 'choice'
            ? choiceSettingLabel(definition, value)
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
            title: settingToggleTitle(definition, value),
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
    getUpdateChannel,
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
