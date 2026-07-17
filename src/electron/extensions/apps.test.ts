// biome-ignore-all lint/suspicious/noExplicitAny: built-in extension contexts are intentionally untyped at this bridge boundary.
import assert from 'node:assert/strict';
import test from 'node:test';
import { initExtensionContext } from './_context';
import { createAppsExtension } from './apps';

test('force quit paints its view before running the OS process scan', async () => {
  let refreshCount = 0;
  initExtensionContext({
    appIndexService: {
      get: () => [
        { id: 'notes', name: 'Notes', path: '/Applications/Notes.app' },
      ],
    },
    runningAppStatus: {
      refresh: () => {
        refreshCount += 1;
        return Promise.resolve(new Set(['/applications/notes.app']));
      },
    },
  });

  const command = createAppsExtension().commands[0];
  let loader: (() => Promise<unknown[]>) | undefined;
  const view = command.run({
    ui: { list: (input: unknown) => input },
    data: {
      loader: (work: () => Promise<unknown[]>) => {
        loader = work;
        return { _loader: true };
      },
    },
  }) as { items: { _loader: boolean }; emptyView: { title: string } };

  assert.equal(refreshCount, 0);
  assert.equal(view.items._loader, true);
  assert.equal(view.emptyView.title, 'No running apps');
  assert.ok(loader);

  const items = await loader();
  assert.equal(refreshCount, 1);
  assert.deepEqual(
    items.map((item) => (item as { title: string }).title),
    ['Notes'],
  );
});

test('app root/search work stays synchronous while Uninstall is a macOS-only lazy secondary action', async () => {
  let discoveries = 0;
  const app = {
    id: 'example',
    name: 'Example',
    path: '/Applications/Example.app',
  };
  initExtensionContext({
    appIndexService: { get: () => [app] },
    hasCapability: (capability) => capability === 'app-uninstall',
    appUninstallService: {
      discover: () => {
        discoveries += 1;
        return Promise.resolve({
          status: 'unavailable' as const,
          message: 'Unavailable in this test',
        });
      },
      selected: () => [],
      trash: async () => ({
        status: 'failed',
        moved: [],
        untouched: [],
        notes: [],
      }),
    },
    actionAliases: () => [],
    rankAction: () => true,
  });
  const ctx = {
    desktop: { apps: { list: () => [app] } },
    actions: {
      run: (title, handler, options = {}) => ({
        title,
        __handler: handler,
        ...options,
      }),
    },
    ui: { error: (title, message) => ({ type: 'preview', title, message }) },
  };
  const extension = createAppsExtension();
  const root = extension.rootItems(ctx as any);
  const appItem = root.find((item: any) => item.id === 'app:example') as any;
  assert.equal(discoveries, 0);
  assert.equal(appItem.primaryAction.title, 'Open Example');
  assert.equal(appItem.actions[0].title, 'Uninstall Example…');
  await appItem.actions[0].__handler();
  assert.equal(discoveries, 1);

  initExtensionContext({
    hasCapability: () => false,
    appUninstallService: null,
  });
  const unsupported = createAppsExtension().rootItems(ctx as any);
  assert.equal(
    (unsupported.find((item: any) => item.id === 'app:example') as any).actions
      .length,
    0,
  );
});

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: this scenario exercises the complete destructive-action journey.
test('Uninstall candidate list defaults to the app, patches selection, and confirms host snapshot paths', async () => {
  const app = {
    id: 'example',
    name: 'Example',
    path: '/Applications/Example.app',
  };
  const candidates = [
    { id: 'app-id', kind: 'app', path: app.path },
    {
      id: 'cache-id',
      kind: 'associated',
      path: '/Users/tester/Library/Caches/com.example.App',
    },
  ];
  const snapshot = { candidates };
  initExtensionContext({
    appIndexService: { get: () => [app] },
    hasCapability: (capability) => capability === 'app-uninstall',
    appUninstallService: {
      discover: async () => ({
        status: 'ready' as const,
        snapshot,
        candidates,
        notes: [
          { code: 'missing-associated', message: 'Some locations missing' },
        ],
      }),
      selected: (_snapshot, values = {}) =>
        candidates.filter((candidate) => values[candidate.id] === true),
      trash: async () => ({
        status: 'complete',
        moved: [app.path],
        untouched: [],
        notes: [],
      }),
    },
    actionAliases: () => [],
    rankAction: () => true,
  });
  const ctx = {
    desktop: { apps: { list: () => [app] } },
    actions: {
      run: (title, handler, options = {}) => ({
        title,
        __handler: handler,
        ...options,
      }),
    },
    ui: {
      item: (input) => input,
      list: (input) => input,
      confirm: (input) => input,
      toast: (input) => ({ toast: input }),
      preview: (input) => input,
      error: (title, message) => ({ type: 'preview', title, message }),
    },
  };
  const root = createAppsExtension().rootItems(ctx as any);
  const action = (root.find((item: any) => item.id === 'app:example') as any)
    .actions[0];
  const list = await action.__handler();
  assert.equal(list.id, 'uninstall:example');
  const candidateItems = list.sections[1].items;
  assert.equal(
    candidateItems.find((candidate: any) => candidate.id === 'app-id')
      .accessories[0].text,
    'Selected',
  );
  assert.equal(
    candidateItems.find((candidate: any) => candidate.id === 'cache-id')
      .accessories[0].text,
    'Optional',
  );
  const deselected = candidateItems
    .find((candidate: any) => candidate.id === 'app-id')
    .primaryAction.__handler();
  assert.equal(deselected.patch.items[0].accessories[0].text, 'Optional');
  const review = list.sections[0].items[0].primaryAction;
  const empty = review.__handler();
  assert.equal(empty.toast.tone, 'error');
  const selected = deselected.patch.items[0].primaryAction.__handler();
  assert.equal(selected.patch.items[0].accessories[0].text, 'Selected');
  const confirmation = review.__handler();
  assert.equal(confirmation.message, app.path);
  assert.equal(confirmation.onConfirm.requiresConfirmation, undefined);
  const complete = await confirmation.onConfirm.__handler();
  assert.equal(complete.toast.message, '1 item moved to Trash');
  assert.equal(complete.toast.tone, 'success');
});
