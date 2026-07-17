// biome-ignore-all lint/suspicious/noExplicitAny: the extension host currently exposes these callbacks without public types.
import { feedbackView } from '../../feedback';
import { extensionContext } from './_context';

function uninstallResult(ctx: any, result: any) {
  if (result.status === 'complete') {
    return ctx.ui.toast({
      message: `${result.moved.length} item${result.moved.length === 1 ? '' : 's'} moved to Trash`,
      tone: 'success',
    });
  }
  const details = result.untouched.length
    ? result.untouched.map((item: any) => ({
        title: item.path,
        subtitle: item.message,
      }))
    : result.notes.map((note: any) => ({ title: note.message }));
  return feedbackView({
    id: 'uninstall-result',
    title:
      result.status === 'partial'
        ? 'Uninstall partially completed'
        : 'Nothing was moved to Trash',
    message:
      result.status === 'partial'
        ? 'Some selected items were left untouched.'
        : 'Selected items were left untouched.',
    tone: 'error',
    details,
  });
}

function selectedCandidateValues(candidates: any[], selectedIds: Set<string>) {
  return Object.fromEntries(
    candidates.map((candidate) => [
      candidate.id,
      selectedIds.has(candidate.id),
    ]),
  );
}

function uninstallCandidateItem(
  ctx: any,
  candidate: any,
  selectedIds: Set<string>,
) {
  const selected = selectedIds.has(candidate.id);
  const toggle = ctx.actions.run(
    `${selected ? 'Deselect' : 'Select'} ${candidate.path}`,
    () => {
      if (selectedIds.has(candidate.id)) {
        selectedIds.delete(candidate.id);
      } else {
        selectedIds.add(candidate.id);
      }
      return {
        patch: {
          items: [uninstallCandidateItem(ctx, candidate, selectedIds)],
        },
      };
    },
  );
  return ctx.ui.item({
    id: candidate.id,
    title: candidate.path,
    subtitle:
      candidate.kind === 'app'
        ? 'Application bundle'
        : 'Conventional associated data',
    icon: candidate.kind === 'app' ? 'app-window' : 'folder',
    accessories: [
      {
        text: selected ? 'Selected' : 'Optional',
        tone: selected ? 'success' : 'muted',
      },
    ],
    primaryAction: toggle,
    actions: [toggle],
  });
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: the chooser keeps its selection closure and review action together.
function uninstallCandidateChooser(ctx: any, item: any, discovery: any) {
  const selectedIds = new Set<string>(
    discovery.candidates
      .filter((candidate: any) => candidate.kind === 'app')
      .map((candidate: any) => candidate.id),
  );
  const review = ctx.actions.run('Review selected items', () => {
    const values = selectedCandidateValues(discovery.candidates, selectedIds);
    const selection = extensionContext.appUninstallService.selected(
      discovery.snapshot,
      values,
    );
    if (selection.length === 0) {
      return ctx.ui.toast({
        message: 'Select at least one item to move to Trash',
        tone: 'error',
      });
    }
    const selectedPaths = selection.map((candidate: any) => candidate.path);
    return ctx.ui.confirm({
      title: `Move ${selectedPaths.length} item${selectedPaths.length === 1 ? '' : 's'} to Trash`,
      message: selectedPaths.join('\n'),
      confirmLabel: 'Move to Trash',
      cancelLabel: 'Cancel',
      destructive: true,
      onConfirm: ctx.actions.run('Move to Trash', async () =>
        uninstallResult(
          ctx,
          await extensionContext.appUninstallService.trash(
            discovery.snapshot,
            values,
          ),
        ),
      ),
    });
  });
  const notes = discovery.notes.map((note: any) => note.message).join(' ');
  return ctx.ui.list({
    id: `uninstall:${item.id}`,
    title: `Uninstall ${item.name}`,
    subtitle: [
      'Press Enter to select or deselect an item, then review your selection.',
      notes,
    ]
      .filter(Boolean)
      .join(' '),
    searchBarPlaceholder: 'Filter items to move to Trash',
    sections: [
      {
        title: 'Next step',
        items: [
          ctx.ui.item({
            id: 'review-uninstall-selection',
            title: 'Review selected items',
            subtitle: 'Confirm the items to move to Trash',
            icon: 'trash-2',
            appearance: { foreground: 'red' },
            primaryAction: review,
            actions: [review],
          }),
        ],
      },
      {
        title: 'Items to move to Trash',
        items: discovery.candidates.map((candidate: any) =>
          uninstallCandidateItem(ctx, candidate, selectedIds),
        ),
      },
    ],
  });
}

function uninstallAction(item: any, ctx: any) {
  if (
    !(
      extensionContext.hasCapability?.('app-uninstall') &&
      extensionContext.appUninstallService
    ) ||
    String(item.path || '').startsWith('/System/')
  ) {
    return null;
  }
  return ctx.actions.run(
    `Uninstall ${item.name}…`,
    async () => {
      const service = extensionContext.appUninstallService;
      const discovery = await service.discover(String(item.path));
      if (discovery.status !== 'ready') {
        return ctx.ui.error('Uninstall unavailable', discovery.message);
      }
      return uninstallCandidateChooser(ctx, item, discovery);
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
