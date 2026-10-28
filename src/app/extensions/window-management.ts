import type {
  ExtensionContext,
  NevermindExtension,
} from '../resources/nevermind-extension-api';
import { windowLayoutActions } from './window-management-commands';
import { windowManagementSettingsView } from './window-management-settings-view';

const WINDOW_MANAGEMENT_OS_CAPABILITY = 'window-management';

function windowManagementSettingsCommand() {
  return {
    id: 'settings',
    title: 'Window Management Settings',
    subtitle: 'Gaps, cycling, and sizes for window layouts',
    icon: 'settings',
    run: (ctx: ExtensionContext) => windowManagementSettingsView(ctx),
  };
}

export function createWindowManagementExtension() {
  return {
    id: 'window-management',
    title: 'Window Management',
    capabilities: ['desktop.window-management'],
    commands: [windowManagementSettingsCommand()],
    actions(ctx) {
      if (!ctx.system.capabilities.has(WINDOW_MANAGEMENT_OS_CAPABILITY)) {
        return [];
      }
      return windowLayoutActions();
    },
  } satisfies NevermindExtension;
}
