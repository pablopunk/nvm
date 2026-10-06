import assert from 'node:assert/strict';
import test from 'node:test';
import {
  diagnosticTestCollectorOrigin,
  isDiagnosticTestCollectorRequest,
} from './test-diagnostic-collector';

test('diagnostic tests accept only a dedicated loopback collector', function loopbackCollector() {
  assert.equal(
    diagnosticTestCollectorOrigin('http://qa@127.0.0.1:54321/1'),
    'http://127.0.0.1:54321',
  );
  for (const dsn of [
    undefined,
    'https://qa@example.com/1',
    'http://qa@localhost:54321/1',
    'http://qa:secret@127.0.0.1:54321/1',
    'http://qa@127.0.0.1:54321/2',
  ])
    assert.throws(function rejectCollector() {
      diagnosticTestCollectorOrigin(dsn);
    });
});

test('test network policy does not allow unrelated URLs or origins', function collectorRequests() {
  const origin = 'http://127.0.0.1:54321';
  assert.equal(
    isDiagnosticTestCollectorRequest(
      `${origin}/api/1/envelope/?sentry_key=qa`,
      origin,
    ),
    true,
  );
  for (const url of [
    'https://sentry.io/api/1/envelope/',
    `${origin}/api/auth`,
    'http://127.0.0.1:54322/api/1/envelope/',
    `${origin}@example.com/api/1/envelope/`,
  ])
    assert.equal(isDiagnosticTestCollectorRequest(url, origin), false);
  assert.equal(
    isDiagnosticTestCollectorRequest(`${origin}/api/1/envelope/`, undefined),
    false,
  );
});
