import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

const consumedDeviceCodes: string[] = [];
let signedInAuth: { email: string; baseUrl: string } | null = null;

function fakeConsumeDeviceCode({ code }: { code: string }) {
  consumedDeviceCodes.push(code);
  return Promise.resolve({ ok: false });
}

function fakeGetNevermindAuth() {
  return Promise.resolve(signedInAuth);
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
