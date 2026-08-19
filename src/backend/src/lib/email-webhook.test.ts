import assert from 'node:assert/strict';
import { createHmac } from 'node:crypto';
import test from 'node:test';
import { verifyProviderWebhook } from './email';

const secret = `whsec_${Buffer.from('test-webhook-secret').toString('base64')}`;
const raw = JSON.stringify({ id: 'evt_1', type: 'email.delivered' });
const timestamp = Math.floor(Date.now() / 1000);
const id = 'msg_123';
function signedHeaders(body = raw, ts = timestamp) {
  const signature = createHmac('sha256', Buffer.from('test-webhook-secret')).update(`${id}.${ts}.${body}`).digest('base64');
  return new Headers({ 'svix-id': id, 'svix-timestamp': String(ts), 'svix-signature': `v1,${signature}` });
}

test('accepts a valid Resend/Svix raw-body signature', () => {
  process.env.RESEND_WEBHOOK_SECRET = secret;
  assert.equal(verifyProviderWebhook(raw, signedHeaders(), timestamp * 1000), true);
});

test('rejects missing or invalid Svix headers', () => {
  process.env.RESEND_WEBHOOK_SECRET = secret;
  assert.equal(verifyProviderWebhook(raw, new Headers(), timestamp * 1000), false);
  const headers = signedHeaders();
  headers.set('svix-signature', 'v1,invalid');
  assert.equal(verifyProviderWebhook(raw, headers, timestamp * 1000), false);
});

test('rejects stale/replayed timestamps before parsing', () => {
  process.env.RESEND_WEBHOOK_SECRET = secret;
  assert.equal(verifyProviderWebhook(raw, signedHeaders(), (timestamp + 301) * 1000), false);
});

test('supports multiple Svix signatures during secret rotation', () => {
  process.env.RESEND_WEBHOOK_SECRET = secret;
  const headers = signedHeaders();
  headers.set('svix-signature', `v1,invalid ${headers.get('svix-signature')}`);
  assert.equal(verifyProviderWebhook(raw, headers, timestamp * 1000), true);
});
