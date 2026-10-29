import assert from 'node:assert/strict';
import test from 'node:test';
import {
  diagnosticTestCollectorOrigin,
  isDiagnosticTestCollectorRequest,
} from './test-diagnostic-collector';

test('diagnostic tests accept only a dedicated loopback collector', function loopbackCollector() {
  assert.equal(
    diagnosticTestCollectorOrigin('http://127.0.0.1:54321'),
    'http://127.0.0.1:54321',
  );
  for (const host of [
    undefined,
    'https://eu.i.posthog.com',
    'http://localhost:54321',
    'http://qa:secret@127.0.0.1:54321',
    'http://127.0.0.1:54321/path',
    'http://127.0.0.1',
  ])
    assert.throws(function rejectCollector() {
      diagnosticTestCollectorOrigin(host);
    });
});

test('test network policy does not allow unrelated URLs or origins', function collectorRequests() {
  const origin = 'http://127.0.0.1:54321';
  assert.equal(
    isDiagnosticTestCollectorRequest(`${origin}/batch/`, origin),
    true,
  );
  for (const url of [
    'https://eu.i.posthog.com/batch/',
    'http://127.0.0.1:54322/batch/',
    `${origin}@example.com/batch/`,
  ])
    assert.equal(isDiagnosticTestCollectorRequest(url, origin), false);
  assert.equal(
    isDiagnosticTestCollectorRequest(`${origin}/batch/`, undefined),
    false,
  );
});
