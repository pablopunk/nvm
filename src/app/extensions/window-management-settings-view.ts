import type {
  ExtensionContext,
  ExtensionItem,
  ExtensionItemSection,
} from '../resources/nevermind-extension-api';
import { showExtensionFeedback } from './feedback';
import {
  extensionWindowManagementSettingsStore,
  type WindowManagementSettingsStore,
} from './window-management-settings';
import {
  WINDOW_MANAGEMENT_SETTING_DEFINITIONS,
  WINDOW_MANAGEMENT_SETTING_SECTIONS,
  type WindowManagementSettingSection,
} from './window-management-settings-definitions';
import {
  DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
  type WindowManagementSettings,
} from './window-management-settings-model';
import {
  resetSectionRow,
  settingRowFor,
} from './window-management-settings-rows';

const SETTINGS_VIEW_ID = 'window-management-settings';
const SETTINGS_VIEW_TITLE = 'Window Management Settings';
const SETTINGS_VIEW_SUBTITLE = 'Applied to every window layout command';

function settingsSectionFor(
  ctx: ExtensionContext,
  store: WindowManagementSettingsStore,
  section: WindowManagementSettingSection,
  settings: WindowManagementSettings,
): ExtensionItemSection {
  return {
    title: section.title,
    subtitle: section.subtitle,
    items: [
      ...section.settings.map((setting) =>
        settingRowFor(ctx, store, setting, settings, (innerCtx) =>
          windowManagementSettingsView(innerCtx, store),
        ),
      ),
      resetSectionRow(ctx, store, section, (innerCtx) =>
        windowManagementSettingsView(innerCtx, store),
      ),
    ],
  };
}

function resetDefaultsItem(
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
          items: WINDOW_MANAGEMENT_SETTING_DEFINITIONS.map((setting) =>
            settingRowFor(
              innerCtx,
              store,
              setting,
              DEFAULT_WINDOW_MANAGEMENT_SETTINGS,
              (refreshCtx) => windowManagementSettingsView(refreshCtx, store),
            ),
          ),
        },
      };
    }),
  });
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
    sections: [
      ...WINDOW_MANAGEMENT_SETTING_SECTIONS.map((section) =>
        settingsSectionFor(ctx, store, section, settings),
      ),
      { items: [resetDefaultsItem(ctx, store)] },
    ],
  });
}
