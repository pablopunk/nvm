import {
  DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
  normalizeWindowManagementSettings,
  type WindowManagementSettings,
  type WindowManagementSettingsStore,
} from '../../extensions/window-management-settings';
import { windowManagementSettingsView } from '../../extensions/window-management-settings-view';
import type { ExtensionContext } from '../../resources/nevermind-extension-api';

function memoryWindowManagementSettingsStore(): WindowManagementSettingsStore {
  let settings: WindowManagementSettings = {
    ...DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
  };
  return {
    load: () => Promise.resolve(settings),
    save(next) {
      settings = normalizeWindowManagementSettings(next);
      return Promise.resolve();
    },
  };
}

const fixtureSettingsStore = memoryWindowManagementSettingsStore();

export function windowManagementSettingsFixtureView(ctx: ExtensionContext) {
  return windowManagementSettingsView(ctx, fixtureSettingsStore);
}
