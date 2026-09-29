// biome-ignore-all lint/suspicious/noExplicitAny: Stubs mirror extension views and captured patches.
import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: {
      isPackaged: false,
      getPath: () => '/tmp/nevermind-account-test',
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

const { clearNevermindAuthCacheForTests } = await import(
  '../electron/nevermind-auth'
);
const { createAccountExtension } = await import('./account');
const { initExtensionContext } = await import('./_context');

test('account sign-in view exposes copyable manual verification details', async () => {
  clearNevermindAuthCacheForTests();
  const code = 'one-time-device-code';
  const verificationUrl = `https://www.nvm.fyi/auth/device?code=${code}`;
  const pendingStatus = {
    state: 'pending' as const,
    verificationUrl,
    code,
    expiresAt: new Date(Date.now() + 60_000).toISOString(),
    browserOpenFailed: true,
  };
  const listeners = new Set<(status: typeof pendingStatus | null) => void>();
  const patches: Array<{ viewId: string; patch: any }> = [];
  let currentStatus: typeof pendingStatus | null = null;
  let completeSignIn: ((result: { ok: false; error: string }) => void) | null =
    null;

  initExtensionContext({
    getNevermindDebugStatus: () => ({
      client: { environment: 'production', baseUrl: 'https://api.nvm.fyi' },
      active: { environment: 'production', baseUrl: 'https://api.nvm.fyi' },
      backend: null,
    }),
    isPackaged: true,
    getNevermindDeviceSignInState: () => currentStatus,
    onNevermindDeviceSignInChanged: (listener) => {
      listeners.add(listener as (status: typeof pendingStatus | null) => void);
      return () =>
        listeners.delete(
          listener as (status: typeof pendingStatus | null) => void,
        );
    },
    signInToNevermind: () =>
      new Promise<{ ok: false; error: string }>((resolve) => {
        completeSignIn = resolve;
      }),
    retryNevermindDeviceSignInBrowser: async () => false,
    cancelNevermindDeviceSignIn: () => true,
    patchOpenView: (viewId, patch) => patches.push({ viewId, patch }),
    invalidateExtensionRootItems: () => {},
    broadcastAuthChanged: () => {},
  });

  const extension = createAccountExtension();
  const loginItem: any = extension
    .rootItems()
    .find((item: any) => item.id === 'account-login');
  assert.ok(loginItem);
  const viewContext = {
    actions: {
      copyText: (text: string, title: string) => ({
        type: 'copyText',
        text,
        title,
      }),
      run: (title: string, handler: (ctx: any) => unknown) => ({
        type: 'runExtensionAction',
        title,
        __handler: handler,
      }),
    },
    ui: {
      list: (view: any) => ({ ...view, type: 'list' }),
      indicator: { show: () => {}, update: () => {} },
    },
  };

  const result = await loginItem.primaryAction.__handler(viewContext);
  assert.equal(result.view.id, 'nevermind-device-sign-in');
  assert.equal(result.view.items[0].title, 'Preparing sign-in');

  currentStatus = pendingStatus;
  for (const listener of listeners) listener(pendingStatus);
  const pendingPatch = patches.at(-1);
  assert.equal(pendingPatch?.viewId, 'nevermind-device-sign-in');
  const statusItem = pendingPatch?.patch.items.find(
    (item: any) => item.id === 'device-sign-in-status',
  );
  const urlItem = pendingPatch?.patch.items.find(
    (item: any) => item.id === 'device-sign-in-url',
  );
  const codeItem = pendingPatch?.patch.items.find(
    (item: any) => item.id === 'device-sign-in-code',
  );
  assert.equal(statusItem?.title, 'Browser could not be opened');
  assert.equal(urlItem?.primaryAction.text, verificationUrl);
  assert.equal(urlItem?.primaryAction.title, 'Copy verification URL');
  assert.equal(codeItem?.primaryAction.text, code);
  assert.equal(codeItem?.primaryAction.title, 'Copy one-time user code');

  completeSignIn?.({ ok: false, error: 'test complete' });
  await new Promise<void>((resolve) => setImmediate(resolve));
  clearNevermindAuthCacheForTests();
});
