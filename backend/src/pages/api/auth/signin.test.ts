import assert from 'node:assert/strict';
import test from 'node:test';

process.env.VERCEL_ENV = 'preview';
process.env.VERCEL_URL = 'nvm-feature-pablo-varelas-projects-4f86af8b.vercel.app';
process.env.WORKOS_API_KEY = 'sk_test_signin';
process.env.WORKOS_CLIENT_ID = 'client_test_signin';

const { GET } = await import('./signin');

test('Preview sign-in rejects a mismatched request origin before signing or redirecting', async () => {
  const response = await GET({
    url: new URL('https://attacker.invalid/api/auth/signin'),
    request: new Request('https://attacker.invalid/api/auth/signin'),
    redirect: (location: string) => Response.redirect(location),
  } as any);
  assert.equal(response.status, 503);
  assert.equal(response.headers.has('location'), false);
  assert.equal(response.headers.has('set-cookie'), false);
});
