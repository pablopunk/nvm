import assert from 'node:assert/strict';
import test, { mock } from 'node:test';
import { createMaintenanceActivity } from '../platform/maintenance-activity';

const consumedDeviceCodes: string[] = [];
let signedInAuth: { email: string; baseUrl: string } | null = null;
let pendingAuth: Promise<{ email: string; baseUrl: string } | null> | null =
  null;

function fakeConsumeDeviceCode({ code }: { code: string }) {
  consumedDeviceCodes.push(code);
  return Promise.resolve({ ok: false });
}

function fakeGetNevermindAuth() {
  return pendingAuth ?? Promise.resolve(signedInAuth);
}

mock.module('./nevermind-auth', {
  namedExports: {
    consumeDeviceCode: fakeConsumeDeviceCode,
    getDefaultNevermindBaseUrl: () => 'https://api.nvm.fyi',
    getNevermindAuth: fakeGetNevermindAuth,
    isSigningIn: () => false,
    setActiveNevermindAuthBaseUrl: () => {},
  },
});

mock.module('./nevermind-compatibility', {
  namedExports: {
    warmNevermindCompatibilityCache: () => {},
  },
});

mock.module('../platform/logger', {
  namedExports: {
    error: () => {},
    info: () => {},
    warn: () => {},
  },
});

const { createNevermindDeepLinks } = await import('./nevermind-deep-links');

function createDeepLinkHandlerForTests() {
  return createNevermindDeepLinks({
    parseAuthDeepLink: () => null,
    bufferedDeepLinks: [],
    getActiveBaseUrl: () => null,
    setActiveBaseUrl: () => {},
    invalidateExtensionRootItems: () => {},
    broadcastAuthChanged: () => {},
  });
}

test('deep link never replaces an existing session, even with intent reconnect', async () => {
  signedInAuth = { email: 'owner@example.com', baseUrl: 'https://api.nvm.fyi' };
  consumedDeviceCodes.length = 0;

  await createDeepLinkHandlerForTests().handleAuthDeepLink({
    code: 'attacker-approved-code',
    baseUrl: 'https://api.nvm.fyi',
    intent: 'reconnect',
  });

  assert.deepEqual(consumedDeviceCodes, []);
});

test('deep link consumes its device code when no session exists', async () => {
  signedInAuth = null;
  consumedDeviceCodes.length = 0;

  await createDeepLinkHandlerForTests().handleAuthDeepLink({
    code: 'device-code',
    baseUrl: 'https://api.nvm.fyi',
    intent: 'connect',
  });

  assert.deepEqual(consumedDeviceCodes, ['device-code']);
});

test('maintenance drains an in-flight deep link and preserves links received while paused', async () => {
  const activity = createMaintenanceActivity();
  const buffered = ['nvm://auth'];
  let finish!: () => void;
  pendingAuth = new Promise((resolve) => {
    finish = () => resolve(null);
  });
  const service = createNevermindDeepLinks({
    activity,
    bufferedDeepLinks: buffered,
    parseAuthDeepLink: () => ({
      code: 'device-code',
      baseUrl: 'https://api.nvm.fyi',
      intent: 'connect',
    }),
    getActiveBaseUrl: () => null,
    setActiveBaseUrl: () => {},
    invalidateExtensionRootItems: () => {},
    broadcastAuthChanged: () => {},
  });
  service.flushBufferedDeepLinks();
  let drained = false;
  const quiescence = activity.quiesce().then(() => {
    drained = true;
  });
  buffered.push('nvm://auth');
  service.flushBufferedDeepLinks();
  assert.deepEqual(buffered, ['nvm://auth']);
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(drained, false);
  finish();
  await quiescence;
  activity.resume();
  pendingAuth = null;
});
