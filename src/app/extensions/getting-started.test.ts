import assert from 'node:assert/strict';
import test, { mock } from 'node:test';
import type { GettingStartedDependencies } from './getting-started';

mock.module('electron', {
  namedExports: {
    app: {
      getPath: () => '/tmp/nevermind-getting-started-test',
      getVersion: () => '0.19.0',
    },
    safeStorage: {
      isEncryptionAvailable: () => false,
      encryptString: () => Buffer.from(''),
      decryptString: () => '',
    },
    shell: { openExternal: async () => {} },
  },
});

const { createGettingStartedExtension } = await import('./getting-started');

function testDependencies() {
  const state: Record<string, unknown> = { hasCompletedOnboarding: false };
  const calls = { saves: 0, rootInvalidations: 0 };
  const overrides: Partial<GettingStartedDependencies> = {
    getUserState: () => state,
    getAccountRootItem: () => ({
      id: 'account-login',
      primaryAction: { type: 'runExtensionAction', title: 'Log in' },
    }),
    scheduleSaveState: () => {
      calls.saves += 1;
    },
    invalidateRootItems: () => {
      calls.rootInvalidations += 1;
    },
    matchesSearch: (item, query) =>
      [item.title, item.subtitle, ...item.aliases]
        .join(' ')
        .toLowerCase()
        .includes(query.toLowerCase()),
  };
  return { state, calls, overrides };
}

function paletteContext() {
  return {
    actions: {
      push: (title: string, view: any) => ({ type: 'pushView', title, view }),
      run: (title: string, handler: (ctx: any) => unknown) => ({
        type: 'runExtensionAction',
        title,
        __handler: handler,
      }),
    },
    navigation: { pop: () => ({ navigation: 'pop' }) },
    ui: { list: (view: any) => view },
  };
}

test('fresh profiles see compact palette guidance and a direct sign-in path', () => {
  const { overrides } = testDependencies();
  const extension = createGettingStartedExtension(overrides);
  const [welcome] = extension.rootItems(paletteContext());
  const view = welcome.primaryAction.view;
  const signIn = view.items.find(
    (item: { id: string }) => item.id === 'getting-started-sign-in',
  ) as any;
  const permissions = view.items.find(
    (item: { id: string }) => item.id === 'getting-started-permissions',
  ) as any;

  assert.equal(welcome.title, 'Welcome to Nevermind');
  assert.match(welcome.subtitle, /command palette: search apps/);
  assert.ok(
    welcome.actions.some(
      (action: { title: string }) => action.title === 'Dismiss welcome',
    ),
  );
  assert.match(
    (
      view.items.find(
        (item: { id: string }) => item.id === 'getting-started-run',
      ) as any
    ).subtitle,
    /press Enter/,
  );
  assert.equal(signIn.primaryAction.title, 'Sign in');
  assert.match(signIn.subtitle, /Optional/);
  assert.match(permissions.subtitle, /only when a command needs access/);
});

test('finishing onboarding persists completion and keeps help searchable', async () => {
  const { state, calls, overrides } = testDependencies();
  const extension = createGettingStartedExtension(overrides);
  const ctx = paletteContext();
  const [welcome] = extension.rootItems(ctx);
  const done = welcome.primaryAction.view.items.find(
    (item: { id: string }) => item.id === 'getting-started-done',
  ) as any;

  assert.deepEqual(await done.primaryAction.__handler(ctx), {
    navigation: 'pop',
  });
  assert.equal(state.hasCompletedOnboarding, true);
  assert.deepEqual(calls, { saves: 1, rootInvalidations: 1 });

  const [help] = extension.rootItems(ctx);
  assert.equal(help.title, 'Help');
  assert.equal(extension.searchItems(ctx, 'help')[0].id, help.id);
});
