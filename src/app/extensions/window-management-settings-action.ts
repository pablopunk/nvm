import type { ExtensionContext } from '../resources/nevermind-extension-api';
import { windowManagementSettingsView } from './window-management-settings-view';

const SETTINGS_ACTION_TITLE = 'Window Management Settings';

export function windowManagementSettingsAction(ctx: ExtensionContext) {
  return ctx.actions.run(
    SETTINGS_ACTION_TITLE,
    async (innerCtx) => ({
      view: await windowManagementSettingsView(innerCtx),
      navigation: 'push' as const,
    }),
    { icon: 'settings' },
  );
}
