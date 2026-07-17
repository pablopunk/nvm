// biome-ignore-all lint/suspicious/noExplicitAny: the extension host currently exposes these callbacks without public types.
import { extensionContext } from './_context';

function uninstallResult(ctx: any, result: any) {
  if (result.status === 'complete')
    return ctx.ui.toast({
      message: `${result.moved.length} item${result.moved.length === 1 ? '' : 's'} moved to Trash`,
    });
  const untouched = result.untouched
    .map((item: any) => `- ${item.path}: ${item.message}`)
    .join('\n');
  return ctx.ui.preview({
    title:
      result.status === 'partial'
        ? 'Uninstall partially completed'
        : 'Nothing was moved to Trash',
    content: `# ${result.status === 'partial' ? 'Some selected items were left untouched' : 'Selected items were left untouched'}\n\n${untouched || result.notes.map((note: any) => `- ${note.message}`).join('\n')}`,
    appearance: { foreground: 'red' },
  });
}

function uninstallAction(item: any, ctx: any) {
  if (
    !extensionContext.hasCapability?.('app-uninstall') ||
    !extensionContext.appUninstallService ||
    String(item.path || '').startsWith('/System/')
  )
    return null;
  return ctx.actions.run(
    `Uninstall ${item.name}…`,
    async () => {
      const service = extensionContext.appUninstallService;
      const discovery = await service.discover(String(item.path));
      if (discovery.status !== 'ready')
        return ctx.ui.error('Uninstall unavailable', discovery.message);
      return ctx.ui.form({
        id: `uninstall:${item.id}`,
        title: `Uninstall ${item.name}`,
        subtitle: 'Choose items to move to Trash. Associated data is optional.',
        fields: [
          ...discovery.notes.map((note: any) => ({
            id: `note:${note.code}`,
            type: 'description',
            description: note.message,
          })),
          ...discovery.candidates.map((candidate: any) => ({
            id: candidate.id,
            type: 'checkbox',
            label: candidate.path,
            description:
              candidate.kind === 'app'
                ? 'Application bundle'
                : 'Conventional associated data',
            value: candidate.kind === 'app',
          })),
        ],
        submitAction: ctx.actions.run(
          'Review selected items',
          (_inner: any, action: any) => {
            const selection = service.selected(
              discovery.snapshot,
              action.formValues,
            );
            if (!selection.length)
              return ctx.ui.toast({
                message: 'Select at least one item to move to Trash',
                tone: 'error',
              });
            const selectedPaths = selection.map(
              (candidate: any) => candidate.path,
            );
            return ctx.ui.confirm({
              title: `Move ${selectedPaths.length} item${selectedPaths.length === 1 ? '' : 's'} to Trash`,
              message: selectedPaths.join('\n'),
              confirmLabel: 'Move to Trash',
              cancelLabel: 'Cancel',
              destructive: true,
              onConfirm: ctx.actions.run('Move to Trash', async () =>
                uninstallResult(
                  ctx,
                  await service.trash(discovery.snapshot, action.formValues),
                ),
              ),
            });
          },
        ),
      });
    },
    { style: 'destructive', icon: 'trash-2' },
  );
}

function appRootItem(item, ctx: any) {
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
    actions: [uninstallAction(item, ctx)].filter(Boolean),
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
        run: (ctx: any) =>
          ctx.ui.list({
            id: 'force-quit-apps',
            title: 'Force Quit Apps',
            presentation: 'root',
            searchBarPlaceholder: 'Search running apps',
            items: ctx.data.loader(async () => {
              const runningPaths =
                await extensionContext.runningAppStatus.refresh(
                  'force-quit-command',
                );
              return extensionContext.appIndexService
                .get()
                .filter(
                  (app) =>
                    app.path &&
                    runningPaths.has(String(app.path).toLowerCase()),
                )
                .map(createForceQuitAppItem);
            }),
            emptyView: {
              title: 'No running apps',
              subtitle: 'No running applications to force quit.',
            },
          }),
      },
    ],
    rootItems(ctx) {
      const items = ctx.desktop.apps
        .list()
        .map((item) => appRootItem(item, ctx));
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
        .map((item) => appRootItem(item, ctx))
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
