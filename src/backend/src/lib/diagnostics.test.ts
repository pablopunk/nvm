import assert from 'node:assert/strict';
import test from 'node:test';
import type { Event } from '@sentry/astro';
import { correlateBackendResponse, safeDiagnosticId, safeRequestId, sanitizeBackendEvent } from './diagnostics';

test('backend diagnostics strips prompts, request data and user identity', () => {
  const event = sanitizeBackendEvent<Event>({
    user: { email: 'PRIVATE_EMAIL', id: 'PRIVATE_ACCOUNT' }, extra: { prompt: 'PRIVATE_PROMPT' },
    tags: { request_id: 'server-request', desktop_action_id: 'a'.repeat(32), private: 'PRIVATE_LABEL' },
    request: { url: 'https://api.nvm.fyi/api?secret=PRIVATE_CODE', data: 'PRIVATE_BODY', headers: { Authorization: 'PRIVATE_TOKEN' } },
    exception: { values: [{ type: 'Error', value: 'PRIVATE_MESSAGE', stacktrace: { frames: [{ filename: '/var/task/dist/server/entry.mjs', lineno: 3, vars: { key: 'PRIVATE_VARIABLE' } }, { filename: '/Users/PRIVATE_USER/code.ts' }] } }] },
  });
  assert.ok(!JSON.stringify(event).includes('PRIVATE_'));
  assert.equal(event.tags?.request_id, 'server-request');
  assert.equal(event.contexts?.diagnostic?.desktop_action_id, 'a'.repeat(32));
  assert.equal(event.exception?.values?.[0]?.stacktrace?.frames?.length, 1);
});

test('correlation headers are bounded and never authentication', () => {
  assert.equal(safeDiagnosticId('a'.repeat(32)), 'a'.repeat(32));
  assert.equal(safeDiagnosticId('a'.repeat(33)), undefined);
  assert.equal(safeRequestId('bad\nheader'), undefined);
  assert.equal(safeRequestId('x'.repeat(81)), undefined);
});

test('correlation preserves server billing identity and the original stream', async () => {
  const response = new Response('data: original\n\n', { headers: { 'x-request-id': 'billing-request', 'Content-Type': 'text/event-stream' } });
  assert.equal(correlateBackendResponse(response, 'desktop-attempt'), response);
  assert.equal(response.headers.get('x-request-id'), 'billing-request');
  assert.equal(await response.text(), 'data: original\n\n');
});

test('older clients without diagnostic headers retain unchanged error bodies', async () => {
  const response = Response.json({ error: { type: 'unauthorized' } }, { status: 401 });
  const correlated = correlateBackendResponse(response, 'server-request');
  assert.equal(correlated.status, 401);
  assert.equal(correlated.headers.get('x-request-id'), 'server-request');
  assert.deepEqual(await correlated.json(), { error: { type: 'unauthorized' } });
});
