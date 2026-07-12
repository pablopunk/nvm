import assert from 'node:assert/strict';
import test from 'node:test';
import { consumePreviewSessionGrant, createPreviewSessionGrant, decodePreviewState, encodePreviewState, previewTargetFromRequest } from './preview-auth';

process.env.WORKOS_COOKIE_PASSWORD = 'test-cookie-password';

test('creates preview state only for HTTPS Vercel preview domains', () => {
  const target = previewTargetFromRequest(new URL('https://nvm-git-branch-team.vercel.app/api/auth/signin'), '/profile');
  assert.deepEqual(target, { origin: 'https://nvm-git-branch-team.vercel.app', returnTo: '/profile' });
  assert.equal(previewTargetFromRequest(new URL('https://nvm.fyi/api/auth/signin'), '/profile'), null);
  assert.equal(previewTargetFromRequest(new URL('http://nvm-git-branch-team.vercel.app/api/auth/signin'), '/profile'), null);
});

test('round trips an encrypted preview session grant only on its target origin', async () => {
  const target = { origin: 'https://nvm-git-branch-team.vercel.app', returnTo: '/dashboard' };
  const grant = await createPreviewSessionGrant(target, 'sealed-session');
  assert.deepEqual(await consumePreviewSessionGrant(grant, target.origin), { sealedSession: 'sealed-session', returnTo: '/dashboard' });
  assert.equal(await consumePreviewSessionGrant(grant, 'https://other-preview.vercel.app'), null);
});

test('rejects malformed preview state', () => {
  const state = encodePreviewState({ origin: 'https://nvm-git-branch-team.vercel.app', returnTo: '/profile' });
  assert.deepEqual(decodePreviewState(state), { origin: 'https://nvm-git-branch-team.vercel.app', returnTo: '/profile' });
  assert.equal(decodePreviewState('preview:bad'), null);
  assert.equal(decodePreviewState('https://evil.example'), null);
});
