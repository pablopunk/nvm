// biome-ignore-all lint: Registration harness stubs dynamic extension shapes by design.
import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: false, getPath: () => '/tmp/nvm-registration-test' },
    shell: {},
  },
});

mock.module('electron-log/main', {
  defaultExport: { error() {}, info() {}, warn() {}, debug() {} },
});

const { createExtensionRegistration } = await import(
  '../extension-host/extension-registration'
);
const { createViewNormalizer } = await import('./view-normalizer');
const { createExtensionActionLookup } = await import(
  './extension-action-lookup'
);
const { normalizeItemAppearance } = await import('./item-appearance');

function createRegistrationHarness() {
  const extensionModules = new Map<string, any>();
  const extensionActionRegistry = new Map<string, any>();
  const extensionActionHandlers = new Map<string, any>();
  const effects = {
    shortcutSyncs: 0,
    rootInvalidations: 0,
    jobsUnregistered: [] as string[],
  };
  const normalizer = createViewNormalizer({
    extensionActionRegistry,
    extensionActionHandlers,
    extensionDraftRefs: new Map(),
    extensionDraftStore: {} as any,
    viewLoaderRegistry: { register() {} } as any,
    viewRefreshRecords: new Map(),
    pruneViewRefreshRecords() {},
    registerViewActionForRenderer: (action) => action,
    userStateStore: { get: () => ({ removedShortcuts: {} }) } as any,
    shortcutForAction: () => null,
    aiChatIdForExtensionFile: () => undefined,
  });
  const registration = createExtensionRegistration({
    extensionModules,
    extensionActionRegistry,
    INTERNAL_EXTENSION_FACTORIES: [],
    REQUIRED_INTERNAL_EXTENSIONS: [],
    REQUIRED_INTERNAL_COMMANDS: [],
    normalizeItemAppearance,
    normalizeViewItems: normalizer.normalizeViewItems,
    createExtensionContext: () => ({}),
    registerExtensionBackgroundJob() {},
    unregisterExtensionBackgroundJob(_extension, item) {
      effects.jobsUnregistered.push(item.id);
    },
    registerActionShortcuts() {
      effects.shortcutSyncs += 1;
    },
    invalidateExtensionRootItems() {
      effects.rootInvalidations += 1;
    },
  });
  const lookup = createExtensionActionLookup({
    extensionActionRegistry,
    extensionModules,
    userStateStore: { get: () => ({ aiChats: {} }) } as any,
    chatTouchedExtensionFiles: () => [],
    extensionActionFromContribution: normalizer.extensionActionFromContribution,
    extensionRootActionFromItem: (_entry: any, item: any) => item,
  });
  return {
    extensionModules,
    extensionActionRegistry,
    effects,
    lookup,
    normalizer,
    registration,
  };
}

function registeredActionFor(harness, extensionId: string, localId: string) {
  const entry = harness.extensionActionRegistry.get(
    `${extensionId}:${localId}`,
  );
  return harness.normalizer.extensionActionFromContribution(entry);
}

function layoutAction(id: string) {
  return { id, title: id, run: () => ({}) };
}

test('durable action exposes secondary actions in its Cmd+K panel', async () => {
  const harness = createRegistrationHarness();
  const extension = {
    id: 'synthetic',
    title: 'Synthetic',
    actions: () => [
      {
        id: 'layout',
        title: 'Layout',
        run: () => ({}),
        actions: [
          {
            type: 'runExtensionRegisteredAction',
            title: 'Edit Layout',
            extensionId: 'synthetic',
            registeredActionId: 'editor',
          },
          {
            type: 'runExtensionAction',
            title: 'Remove',
            __handler: () => ({}),
          },
        ],
      },
      { id: 'editor', title: 'Editor', run: () => ({}) },
    ],
  };

  await harness.registration.registerExtension(extension);
  const action = registeredActionFor(harness, 'synthetic', 'layout');

  assert.equal(action.kind, 'extension-action');
  assert.equal(action.actionId, 'extension-action:synthetic:layout');
  assert.deepEqual(
    action.actionPanel.sections[0].actions.map((item) => item.title),
    ['Edit Layout', 'Remove'],
  );
});

test('secondary actions cross IPC as clone-safe tokenized payloads', async () => {
  const harness = createRegistrationHarness();
  await harness.registration.registerExtension({
    id: 'synthetic',
    title: 'Synthetic',
    actions: () => [
      {
        id: 'layout',
        title: 'Layout',
        run: () => ({}),
        actions: [
          {
            type: 'runExtensionAction',
            title: 'Remove',
            __handler: () => ({}),
          },
        ],
      },
    ],
  });
  const action = registeredActionFor(harness, 'synthetic', 'layout');
  const [remove] = action.actionPanel.sections[0].actions;

  assert.equal(typeof remove.handlerId, 'string');
  assert.equal(remove.__handler, undefined);
  assert.doesNotThrow(() => structuredClone(action));
});

test('async actions(ctx) are awaited before durable actions register', async () => {
  const harness = createRegistrationHarness();

  await harness.registration.registerExtension({
    id: 'async',
    title: 'Async',
    actions: async () => {
      await Promise.resolve();
      return { actions: [layoutAction('later')] };
    },
  });

  assert.equal(harness.extensionActionRegistry.has('async:later'), true);
});

test('refresh adds new actions, drops removed ones, and keeps unchanged identities', async () => {
  const harness = createRegistrationHarness();
  let layouts = ['keep', 'drop'];
  const extension = {
    id: 'custom',
    title: 'Custom',
    actions: () => layouts.map(layoutAction),
  };
  await harness.registration.registerExtension(extension);
  const keptActionId = registeredActionFor(harness, 'custom', 'keep').actionId;

  layouts = ['keep', 'added'];
  const result =
    await harness.registration.refreshExtensionContributions(extension);

  assert.deepEqual(result, { ok: true });
  assert.equal(harness.extensionActionRegistry.has('custom:added'), true);
  assert.equal(harness.extensionActionRegistry.has('custom:drop'), false);
  assert.deepEqual(harness.effects.jobsUnregistered, ['drop', 'keep', 'added']);
  assert.equal(
    registeredActionFor(harness, 'custom', 'keep').actionId,
    keptActionId,
  );
  assert.equal(harness.effects.shortcutSyncs, 1);
  assert.equal(harness.effects.rootInvalidations, 1);
});

test('a removed action no longer binds its stored shortcut to a remaining sibling', async () => {
  const harness = createRegistrationHarness();
  let layouts = ['drop', 'keep'];
  const extension = {
    id: 'custom',
    title: 'Custom',
    __filePath: '/tmp/custom-extension.ts',
    actions: () => layouts.map(layoutAction),
  };
  await harness.registration.registerExtension(extension);
  const storedShortcutAction = registeredActionFor(harness, 'custom', 'drop');

  layouts = ['keep'];
  await harness.registration.refreshExtensionContributions(extension);

  assert.equal(
    harness.lookup.currentActionForStoredShortcut(storedShortcutAction),
    null,
  );
});

test('a failing actions(ctx) during refresh keeps the previous set', async () => {
  const harness = createRegistrationHarness();
  let failing = false;
  const extension = {
    id: 'flaky',
    title: 'Flaky',
    actions: () => {
      if (failing) throw new Error('storage unavailable');
      return [layoutAction('one')];
    },
  };
  await harness.registration.registerExtension(extension);

  failing = true;
  const result =
    await harness.registration.refreshExtensionContributions(extension);

  assert.equal(result.ok, false);
  assert.equal(harness.extensionActionRegistry.has('flaky:one'), true);
  assert.equal(harness.effects.shortcutSyncs, 0);
  assert.equal(harness.effects.rootInvalidations, 0);
});

test('a refresh from a replaced extension module cannot overwrite the reloaded one', async () => {
  const harness = createRegistrationHarness();
  const previous = {
    id: 'reloaded',
    title: 'Reloaded',
    actions: () => [layoutAction('old')],
  };
  await harness.registration.registerExtension(previous);
  const replacement = {
    id: 'reloaded',
    title: 'Reloaded',
    actions: () => [layoutAction('new')],
  };
  harness.extensionModules.set('reloaded', replacement);

  const result =
    await harness.registration.refreshExtensionContributions(previous);

  assert.equal(result.ok, false);
  assert.equal(harness.extensionActionRegistry.has('reloaded:old'), true);
  assert.equal(harness.effects.shortcutSyncs, 0);
});
