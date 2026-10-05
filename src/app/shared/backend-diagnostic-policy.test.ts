import assert from 'node:assert/strict';
import test from 'node:test';
import {
  backendDiagnosticHeaders,
  isDiagnosticBackend,
  responseRequestId,
} from './backend-diagnostic-policy';

test('diagnostic propagation accepts only canonical first-party API origins', () => {
  for (const url of [
    'https://api.nvm.fyi/api/v1/active-model',
    'https://www.nvm.fyi/api/auth/device/initiate',
  ])
    assert.ok(isDiagnosticBackend(url));
  for (const url of [
    'https://api.nvm.fyi.evil.test/api/x',
    'https://api.nvm.fyi:444/api/x',
    'https://user@api.nvm.fyi/api/x',
    'http://api.nvm.fyi/api/x',
    'https://api.nvm.fyi/dashboard',
    'https://custom.example/api/x',
  ])
    assert.ok(!isDiagnosticBackend(url));
  const context = {
    actionId: 'a'.repeat(32),
    journeyId: 'b'.repeat(32),
    traceHeaders: {
      'sentry-trace': `${'c'.repeat(32)}-${'d'.repeat(16)}-1`,
      baggage: 'secret=value',
      Authorization: 'private',
    },
  };
  assert.deepEqual(
    backendDiagnosticHeaders('https://custom.example/api/x', context),
    {},
  );
  assert.ok(
    !JSON.stringify(
      backendDiagnosticHeaders('https://api.nvm.fyi/api/x', context),
    ).includes('private'),
  );
  assert.ok(
    !(
      'baggage' in
      backendDiagnosticHeaders('https://api.nvm.fyi/api/x', context)
    ),
  );
  assert.equal(
    responseRequestId({ 'x-request-id': 'unsafe\nvalue' }),
    undefined,
  );
});
