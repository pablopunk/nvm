import type { ExtensionContext } from '../resources/nevermind-extension-api';
import type { WindowManagementSettings } from './window-management-settings-model';
import { normalizeWindowManagementSettings } from './window-management-settings-normalize';

export const WINDOW_MANAGEMENT_SETTINGS_KEY = 'window-management.settings';

export interface WindowManagementSettingsStore {
  load(): Promise<WindowManagementSettings>;
  save(settings: WindowManagementSettings): Promise<void>;
}

export function extensionWindowManagementSettingsStore(
  ctx: Pick<ExtensionContext, 'storage'>,
): WindowManagementSettingsStore {
  return {
    async load() {
      return normalizeWindowManagementSettings(
        await ctx.storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY),
      );
    },
    async save(settings) {
      await ctx.storage.set(
        WINDOW_MANAGEMENT_SETTINGS_KEY,
        normalizeWindowManagementSettings(settings),
      );
    },
  };
}
