import assert from 'node:assert/strict';
import test from 'node:test';
import {
  consumeGatewayState,
  consumePreviewSessionGrant,
  createPreviewGatewayState,
  createPreviewSessionGrant,
  createPreviewStartIntent,
  createProductionState,
  previewTargetFromEnvironment,
  setPreviewAuthStoreForTests,
} from './preview-auth';

process.env.GATEWAY_STATE_KEY = 'gateway-state-test-key';
process.env.PREVIEW_START_KEY = 'preview-start-test-key';
process.env.VERCEL_ENV = 'preview';
process.env.VERCEL_URL = 'nvm-git-branch-team-pablo-varelas-projects-4f86af8b.vercel.app';
const target = { origin: `https://${process.env.VERCEL_URL}`, returnTo: '/' };
const store = new Map<string, string>();
setPreviewAuthStoreForTests(store);

test('derives only the canonical preview origin from Vercel environment', () => {
  assert.deepEqual(previewTargetFromEnvironment(), target);
  process.env.VERCEL_URL = 'attacker.vercel.app';
  assert.equal(previewTargetFromEnvironment(), null);
  process.env.VERCEL_URL = target.origin.slice('https://'.length);
});

test('production v2 state is one-use and rejects legacy material', async () => {
  process.env.VERCEL_ENV = 'production';
  const state = await createProductionState('/dashboard');
  assert.ok(state?.startsWith('v2.'));
  assert.equal((await consumeGatewayState(state))?.flow, 'production');
  assert.equal(await consumeGatewayState(state), null);
  assert.equal(await consumeGatewayState('preview:legacy-state'), null);
  process.env.VERCEL_ENV = 'preview';
});

test('preview start and gateway state are mutually exclusive and replay-safe', async () => {
  const intent = await createPreviewStartIntent(target);
  assert.ok(intent?.startsWith('v2.'));
  if (!intent) throw new Error('expected preview start intent');
  const gateway = await createPreviewGatewayState(intent);
  assert.ok(gateway?.state.startsWith('v2.'));
  if (!gateway) throw new Error('expected preview gateway state');
  assert.deepEqual((await consumeGatewayState(gateway.state))?.flow, 'preview_gateway');
  assert.equal(await createPreviewGatewayState(intent), null, 'start intent is one-use');
  assert.equal(await consumeGatewayState(gateway.state), null, 'gateway state is one-use');
});

test('preview grants are opaque, origin-bound, and one-use', async () => {
  const grant = await createPreviewSessionGrant(target, { id: 'wos_1', email: 'user@example.test' });
  assert.ok(grant?.startsWith('v2.'));
  if (!grant) throw new Error('expected preview grant');
  assert.equal(grant.includes('sealedSession'), false);
  assert.equal(await consumePreviewSessionGrant(grant, 'https://other-preview.vercel.app'), null);
  assert.deepEqual(await consumePreviewSessionGrant(grant, target.origin), { identity: { id: 'wos_1', email: 'user@example.test' }, returnTo: '/' });
  assert.equal(await consumePreviewSessionGrant(grant, target.origin), null);
});

test('rejects malformed origins and legacy query-style grants', async () => {
  assert.equal(await createPreviewSessionGrant({ origin: 'http://evil.example', returnTo: '/' }, { id: 'x', email: 'x@example.test' }), null);
  assert.equal(await consumePreviewSessionGrant('sealedSession=production', target.origin), null);
});
