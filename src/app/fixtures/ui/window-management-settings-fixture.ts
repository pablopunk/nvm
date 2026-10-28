import type { WindowManagementSettingsStore } from '../../extensions/window-management-settings';
import { wholeNumberSettingFor } from '../../extensions/window-management-settings-definitions';
import type { WindowManagementSettings } from '../../extensions/window-management-settings-model';
import { normalizeWindowManagementSettings } from '../../extensions/window-management-settings-normalize';
import { settingPromptView } from '../../extensions/window-management-settings-prompts';
import { windowManagementSettingsView } from '../../extensions/window-management-settings-view';
import type { ExtensionContext } from '../../resources/nevermind-extension-api';

const SAMPLE_SETTINGS = normalizeWindowManagementSettings({
  outerGap: 10,
  outerGapTop: 16,
  innerGap: 6,
  applyGapToMaximize: false,
  almostMaximizeWidthPercent: 85,
  cycleOrder: 'half-third-two-thirds',
  displayMoveSizing: 'keep-size',
});

function memoryWindowManagementSettingsStore(
  seed: WindowManagementSettings,
): WindowManagementSettingsStore {
  let settings = normalizeWindowManagementSettings(seed);
  return {
    load: () => Promise.resolve(settings),
    save(next) {
      settings = normalizeWindowManagementSettings(next);
      return Promise.resolve();
    },
  };
}

const fixtureSettingsStore =
  memoryWindowManagementSettingsStore(SAMPLE_SETTINGS);

export function windowManagementSettingsFixtureView(ctx: ExtensionContext) {
  return windowManagementSettingsView(ctx, fixtureSettingsStore);
}

export function windowManagementSettingsErrorFixtureView(
  ctx: ExtensionContext,
) {
  return settingPromptView(
    ctx,
    {
      store: fixtureSettingsStore,
      definition: wholeNumberSettingFor('outerGap'),
      buildView: (innerCtx) =>
        windowManagementSettingsView(innerCtx, fixtureSettingsStore),
    },
    '150',
    'Enter a whole number from 0 to 40.',
  );
}
