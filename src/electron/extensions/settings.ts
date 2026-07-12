import { extensionContext } from './_context';

export function createSettingsExtension() {
  return {
    id: 'nevermind.settings',
    title: 'Settings',
    permissions: ['settings.write'] as const,
    commands: [
      {
        id: 'app-settings',
        actionId: 'app-settings',
        title: 'Settings',
        subtitle: 'Configure Nevermind',
        icon: 'settings',
        score: 16,
        run: (ctx) =>
          ctx.ui.list({
            type: 'list',
            id: 'app-settings',
            title: 'Settings',
            presentation: 'root',
            selectedItemId: ctx.state.selectedItemId,
            searchBarPlaceholder: 'Search Settings',
            items: extensionContext.settingsItems(),
          }),
      },
    ],
  };
}
