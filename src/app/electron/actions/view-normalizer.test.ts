// biome-ignore-all lint: Normalizer harness stubs dynamic extension shapes by design.
import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: false, getPath: () => '/tmp/nvm-view-normalizer-test' },
    shell: {},
    systemPreferences: { isTrustedAccessibilityClient: () => false },
  },
});

mock.module('electron-log/main', {
  defaultExport: { error() {}, info() {}, warn() {}, debug() {} },
});

const { createViewNormalizer } = await import('./view-normalizer');
const { createExtensionUiApi } = await import(
  '../extension-host/extension-ui-api'
);
const { combinedActionSections } = await import('../../palette/local-actions');

function createNormalizer() {
  const extensionActionHandlers = new Map<string, any>();
  let executionCount = 0;
  const normalizer = createViewNormalizer({
    extensionActionRegistry: new Map(),
    extensionActionHandlers,
    extensionDraftRefs: new Map(),
    extensionDraftStore: {} as any,
    viewLoaderRegistry: { register() {} } as any,
    viewRefreshRecords: new Map(),
    pruneViewRefreshRecords() {},
    registerViewActionForRenderer: (action) => ({
      ...action,
      executionId: `execution-${++executionCount}`,
    }),
    userStateStore: { get: () => ({ removedShortcuts: {} }) } as any,
    shortcutForAction: () => null,
    aiChatIdForExtensionFile: () => undefined,
  });
  return { normalizer, extensionActionHandlers };
}

const entry = { extension: { id: 'fixture.collection' } };

function handlerAction(title: string) {
  return { type: 'runExtensionAction', title, __handler: () => undefined };
}

function collectionView(add: any, open: any, remove: any) {
  const ui = createExtensionUiApi({
    buildPreviewItemAction: (item) => item,
    progressView: (input) => input,
    buildConfirmAction: (input) => input,
  });
  return ui.collection({
    id: 'notes',
    title: 'Notes',
    add,
    items: [{ id: 'note-1', title: 'Groceries', preview: open, remove }],
  });
}

test('one action object reused across a view keeps one handler identity', () => {
  const { normalizer, extensionActionHandlers } = createNormalizer();
  const open = handlerAction('Open note');
  const view = normalizer.normalizeView(
    collectionView(
      handlerAction('New note'),
      open,
      handlerAction('Delete note'),
    ),
    entry,
  );
  const note = view.items.find((item: any) => item.id === 'note-1');
  assert.equal(note.primaryAction.handlerId, note.actions[0].handlerId);
  assert.equal(
    note.primaryAction.handlerId,
    note.actionPanel.sections[0].actions[0].handlerId,
  );
  assert.equal(extensionActionHandlers.size, 3);
});

test('collection item and view panels never repeat the primary action', () => {
  const { normalizer } = createNormalizer();
  const view = normalizer.normalizeView(
    collectionView(
      handlerAction('New note'),
      handlerAction('Open note'),
      handlerAction('Delete note'),
    ),
    entry,
  );
  for (const item of view.items) {
    const titles = combinedActionSections(item, view).flatMap((section) =>
      section.actions.map((action: any) => action.title),
    );
    assert.equal(new Set(titles).size, titles.length, item.id);
  }
});

test('distinct actions that share a title stay separate', () => {
  const { normalizer } = createNormalizer();
  const view = normalizer.normalizeView(
    {
      type: 'list',
      title: 'Notes',
      items: [
        {
          id: 'note-1',
          title: 'Groceries',
          actions: [handlerAction('Open'), handlerAction('Open')],
        },
      ],
    },
    entry,
  );
  const [first, second] = view.items[0].actions;
  assert.notEqual(first.handlerId, second.handlerId);
  assert.equal(
    combinedActionSections(view.items[0], view)[0].actions.length,
    2,
  );
});

test('normalized views keep the explicit contextual flag only where hosts set it', () => {
  const { normalizer } = createNormalizer();
  const ui = createExtensionUiApi({
    buildPreviewItemAction: (item) => item,
    progressView: (input) => input,
    buildConfirmAction: (input) => input,
  });
  const error = normalizer.normalizeView(
    ui.error('Invalid temperature', 'Try 100 C.'),
    entry,
  );
  const passive = normalizer.normalizeView(
    ui.list({
      id: 'getting-started',
      title: 'Getting started',
      items: [
        {
          id: 'account',
          title: 'Nevermind account',
          disabled: true,
        },
      ],
    }),
    entry,
  );

  assert.equal(error.items[0].contextual, true);
  assert.equal(passive.items[0].disabled, true);
  assert.equal(passive.items[0].contextual, undefined);
  assert.deepEqual(structuredClone(error), error);
});
