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
const { normalizeItemAppearance } = await import('./item-appearance');

function createRegistrationHarness() {
  const extensionModules = new Map<string, any>();
  const extensionActionRegistry = new Map<string, any>();
  const extensionActionHandlers = new Map<string, any>();
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
  });
  return { extensionActionRegistry, normalizer, registration };
}

function registeredActionFor(harness, extensionId: string, localId: string) {
  const entry = harness.extensionActionRegistry.get(
    `${extensionId}:${localId}`,
  );
  return harness.normalizer.extensionActionFromContribution(entry);
}

test('durable action exposes secondary actions in its Cmd+K panel', () => {
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

  harness.registration.registerExtension(extension);
  const action = registeredActionFor(harness, 'synthetic', 'layout');

  assert.equal(action.kind, 'extension-action');
  assert.equal(action.actionId, 'extension-action:synthetic:layout');
  assert.deepEqual(
    action.actionPanel.sections[0].actions.map((item) => item.title),
    ['Edit Layout', 'Remove'],
  );
});

test('secondary actions cross IPC as clone-safe tokenized payloads', () => {
  const harness = createRegistrationHarness();
  harness.registration.registerExtension({
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
