import assert from 'node:assert/strict';
import test from 'node:test';
import { correlateBackendResponse, safeDiagnosticId, safeRequestId } from './diagnostics';

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
