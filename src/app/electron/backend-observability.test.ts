import assert from 'node:assert/strict';
import test from 'node:test';
import {
  fetchDiagnosticBackend,
  observeProviderOptions,
} from './backend-observability';
import {
  configureObservability,
  currentOperation,
  recordOperationFailure,
  recordServerOutcome,
  runOperation,
} from './observability';

test('first-party requests fail closed on redirects and keep existing auth headers', async (t) => {
  configureObservability({ build: 'test' });
  t.mock.method(globalThis, 'fetch', async (_input, init) => {
    assert.equal(init?.redirect, 'error');
    assert.equal(
      new Headers(init?.headers).get('Authorization'),
      'Bearer test',
    );
    assert.match(
      new Headers(init?.headers).get('x-nevermind-action-id') ?? '',
      /^[a-f0-9]{32}$/,
    );
    assert.equal(new Headers(init?.headers).get('baggage'), null);
    return new Response(null, {
      headers: { 'x-request-id': 'server-request' },
    });
  });
  await runOperation('command.execute', async () => {
    await fetchDiagnosticBackend('https://api.nvm.fyi/api/compatibility', {
      headers: { Authorization: 'Bearer test' },
    });
    assert.equal(currentOperation()?.requestId, 'server-request');
  });
});

test('an optional HTTP failure does not mark the containing journey as failed', async (t) => {
  configureObservability({ build: 'test' });
  t.mock.method(
    globalThis,
    'fetch',
    async () =>
      new Response(null, {
        status: 404,
        headers: { 'x-request-id': 'server-request' },
      }),
  );
  const context = await runOperation('command.execute', async () => {
    await fetchDiagnosticBackend('https://api.nvm.fyi/api/compatibility');
    return currentOperation()!;
  });
  assert.equal(context.outcome, 'success');
});

test('classified server failures are local outcomes while distinct client failures are captured', async () => {
  const captures: unknown[] = [];
  configureObservability({
    build: 'test',
    capture: (error) => {
      captures.push(error);
    },
  });
  await runOperation('ai.stream', async () => {
    const options = observeProviderOptions();
    await options.onResponse(
      { status: 503, headers: { 'x-request-id': 'server-failure' } },
      { baseUrl: 'https://api.nvm.fyi/api/v1' } as never,
    );
    const record = recordOperationFailure(
      new Error('server failed'),
      'stream',
      'backend',
    );
    assert.equal(record?.requestId, 'server-failure');
  });
  assert.equal(captures.length, 0);
  await runOperation('ai.stream', async () => {
    await observeProviderOptions().onResponse({ status: 200, headers: {} }, {
      baseUrl: 'https://api.nvm.fyi/api/v1',
    } as never);
    recordOperationFailure(new Error('client processing failed'));
  });
  assert.equal(captures.length, 1);
});

test('a new provider attempt does not inherit a failed metadata response', () => {
  configureObservability({ build: 'test' });
  runOperation('ai.stream', () => {
    recordServerOutcome(404, 'metadata-request');
    observeProviderOptions();
    assert.equal(currentOperation()?.serverFailure, false);
    assert.equal(currentOperation()?.responseStatus, undefined);
    assert.equal(currentOperation()?.requestId, undefined);
  });
});
