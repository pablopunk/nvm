import assert from 'node:assert/strict';
import test from 'node:test';
import { setPreviewAuthStoreForTests } from '../../../lib/preview-auth';

const keys = ['VERCEL_ENV', 'PRODUCTION_ORIGIN', 'WORKOS_REDIRECT_URI', 'GATEWAY_STATE_KEY', 'PREVIEW_GATEWAY_ORIGIN', 'PREVIEW_START_KEY'];
const original = Object.fromEntries(keys.map((key) => [key, process.env[key]]));
Object.assign(process.env, {
  WORKOS_API_KEY: 'sk_test_redirect_config',
  WORKOS_CLIENT_ID: 'client_redirect_config',
  GATEWAY_STATE_KEY: 'gateway-state-test',
  PREVIEW_START_KEY: 'preview-start-test',
  PREVIEW_GATEWAY_ORIGIN: 'https://www.nvm.fyi',
  VERCEL_ENV: 'production',
});

const [{ GET: productionSignIn }, { GET: previewStart }] = await Promise.all([
  import('./signin'),
  import('./preview-start'),
]);

function noSideEffectResponse(response: Response) {
  assert.equal(response.status, 503);
  assert.equal(response.headers.has('location'), false);
  assert.equal(response.headers.has('set-cookie'), false);
}

test('production sign-in validates callback configuration before writing state', async () => {
  const store = new Map<string, unknown>();
  setPreviewAuthStoreForTests(store);
  process.env.PRODUCTION_ORIGIN = 'https://www.nvm.fyi';
  process.env.WORKOS_REDIRECT_URI = 'https://nvm.fyi/api/auth/callback';
  noSideEffectResponse(await productionSignIn({
    url: new URL('https://www.nvm.fyi/api/auth/signin'),
    request: new Request('https://www.nvm.fyi/api/auth/signin'),
    redirect: (location: string) => Response.redirect(location),
  } as any));
  assert.equal(store.size, 0);
});

test('Preview gateway validates callback configuration before writing state', async () => {
  const store = new Map<string, unknown>();
  setPreviewAuthStoreForTests(store);
  process.env.PRODUCTION_ORIGIN = 'https://www.nvm.fyi';
  process.env.WORKOS_REDIRECT_URI = 'https://www.nvm.fyi/api/auth/callback/path';
  noSideEffectResponse(await previewStart({
    url: new URL('https://www.nvm.fyi/api/auth/preview-start?intent=invalid'),
    request: new Request('https://www.nvm.fyi/api/auth/preview-start?intent=invalid'),
    redirect: (location: string) => Response.redirect(location),
  } as any));
  assert.equal(store.size, 0);
});

test.after(() => {
  setPreviewAuthStoreForTests(null);
  for (const key of keys) {
    if (original[key] === undefined) delete process.env[key];
    else process.env[key] = original[key];
  }
});
