// biome-ignore-all lint/suspicious/noExplicitAny: the extension host currently exposes these callbacks without public types.
import { extensionContext } from './_context';

function appRootItem(item) {
  const id = `app:${item.id}`;
  return {
    id,
    isAppResult: true,
    title: item.name,
    subtitle: 'Launch application',
    aliases: extensionContext.actionAliases(
      `extension-root:nevermind.apps:${id}`,
    ),
    icon: 'app',
    image: undefined as string | undefined,
    score: 30,
    dismissAfterRun: 'auto',
    customizable: true,
    primaryAction: {
      type: 'openPath',
      title: `Open ${item.name}`,
      path: item.path,
      dismissAfterRun: 'auto',
    },
  };
}

function createForceQuitAppItem(app: any) {
  return {
    id: `force-quit:${app.id}`,
    title: app.name,
    subtitle: 'Force quit this application',
    icon: 'stop-circle',
    appearance: { foreground: 'red' as const },
    primaryAction: {
      type: 'forceQuitApp' as const,
      title: `Force Quit ${app.name}`,
      path: app.path,
      app: { name: app.name, path: app.path },
      style: 'destructive' as const,
      requiresConfirmation: true,
      confirmMessage: `Force quit ${app.name}? Unsaved data may be lost.`,
      confirmLabel: 'Force Quit',
      cancelLabel: 'Cancel',
      dismissAfterRun: 'auto' as const,
    },
  };
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: the extension definition is deliberately kept as one declarative object.
export function createAppsExtension() {
  return {
    id: 'nevermind.apps',
    title: 'Applications',
    permissions: ['desktop.apps'] as const,
    commands: [
      {
        id: 'force-quit-apps',
        actionId: 'force-quit-apps',
        title: 'Force Quit Apps',
        subtitle: 'Force quit running applications',
        icon: 'stop-circle',
        score: 14,
        appearance: { foreground: 'red' as const },
        run: async (ctx: any) => {
          const runningPaths =
            await extensionContext.runningAppStatus.refresh(
              'force-quit-command',
            );
          const apps = extensionContext.appIndexService
            .get()
            .filter(
              (app) =>
                app.path && runningPaths.has(String(app.path).toLowerCase()),
            );
          if (apps.length === 0) {
            return ctx.ui.empty(
              'No running apps',
              'No running applications to force quit.',
            );
          }
          return ctx.ui.list({
            id: 'force-quit-apps',
            title: 'Force Quit Apps',
            presentation: 'root',
            searchBarPlaceholder: 'Search running apps',
            items: apps.map(createForceQuitAppItem),
          });
        },
      },
    ],
    rootItems(ctx) {
      const items = ctx.desktop.apps.list().map(appRootItem);
      return [
        {
          id: 'force-quit-apps-command',
          title: 'Force Quit Apps',
          subtitle: 'Force quit running applications',
          aliases: ['force quit', 'kill', 'quit apps'],
          icon: 'stop-circle',
          score: 14,
          dismissAfterRun: 'auto',
          appearance: { foreground: 'red' as const },
          primaryAction: {
            type: 'runExtensionRegisteredAction' as const,
            title: 'Force Quit Apps',
            extensionId: 'nevermind.apps',
            registeredActionId: 'force-quit-apps',
          },
        },
        ...items,
      ];
    },
    searchItems(ctx, query) {
      const items = ctx.desktop.apps
        .list()
        .map(appRootItem)
        .filter((item: any) => extensionContext.rankAction(item, query));
      const forceQuitItem = {
        id: 'force-quit-apps-command',
        title: 'Force Quit Apps',
        subtitle: 'Force quit running applications',
        aliases: ['force quit', 'kill', 'quit apps'],
        icon: 'stop-circle',
        score: 14,
        dismissAfterRun: 'auto',
        appearance: { foreground: 'red' as const },
        primaryAction: {
          type: 'runExtensionRegisteredAction' as const,
          title: 'Force Quit Apps',
          extensionId: 'nevermind.apps',
          registeredActionId: 'force-quit-apps',
        },
      };
      if (extensionContext.rankAction(forceQuitItem, query)) {
        return [forceQuitItem, ...items];
      }
      return items;
    },
  };
}
