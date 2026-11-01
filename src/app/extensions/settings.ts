import { extensionContext } from './_context';

export function appSettingsView(ctx: any) {
  return ctx.ui.list({
    type: 'list',
    id: 'app-settings',
    title: 'Settings',
    presentation: 'root',
    selectedItemId: ctx.state.selectedItemId,
    searchBarPlaceholder: 'Search Settings',
    items: extensionContext.settingsItems(),
  });
}

export function createSettingsExtension() {
  return {
    id: 'nevermind.settings',
    title: 'Settings',
    capabilities: ['settings.write'] as const,
    commands: [
      {
        id: 'app-settings',
        actionId: 'app-settings',
        title: 'Settings',
        subtitle: 'Configure Nevermind',
        icon: 'settings',
        score: 16,
        run: appSettingsView,
      },
    ],
  };
}
