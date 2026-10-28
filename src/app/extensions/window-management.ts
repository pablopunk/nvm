import type {
  ExtensionContext,
  NevermindExtension,
} from '../resources/nevermind-extension-api';
import {
  WINDOW_MANAGEMENT_OS_CAPABILITY,
  windowLayoutActions,
} from './window-management-commands';
import { customWindowCommandsCommand } from './window-management-custom-commands';
import { windowDesktopActions } from './window-management-desktops';
import { exactWindowActions } from './window-management-exact-commands';
import { windowManagementSettingsView } from './window-management-settings-view';

function windowManagementSettingsCommand() {
  return {
    id: 'settings',
    title: 'Window Management Settings',
    subtitle: 'Gaps, cycling, and sizes for window layouts',
    icon: 'settings',
    run: (ctx: ExtensionContext) => windowManagementSettingsView(ctx),
  };
}

function windowManagementSupported(ctx: ExtensionContext) {
  return ctx.system.capabilities.has(WINDOW_MANAGEMENT_OS_CAPABILITY);
}

export function createWindowManagementExtension() {
  return {
    id: 'window-management',
    title: 'Window Management',
    capabilities: ['desktop.window-management', 'system'],
    commands: [
      windowManagementSettingsCommand(),
      customWindowCommandsCommand(),
    ],
    actions(ctx) {
      if (!windowManagementSupported(ctx)) {
        return [];
      }
      return [
        ...windowLayoutActions(ctx),
        ...exactWindowActions(ctx),
        ...windowDesktopActions(),
      ];
    },
  } satisfies NevermindExtension;
}
