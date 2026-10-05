import assert from 'node:assert/strict';
import test from 'node:test';
import type { Event } from '@sentry/electron/main';
import {
  MAIN_DIAGNOSTIC_INTEGRATIONS,
  RENDERER_DIAGNOSTIC_INTEGRATIONS,
  sanitizeDiagnosticEvent,
} from './diagnostic-privacy';

test('private payloads are removed while generated app stack locations survive', () => {
  const event = sanitizeDiagnosticEvent({
    user: { email: 'PRIVATE_EMAIL' },
    extra: { prompt: 'PRIVATE_PROMPT' },
    request: {
      url: 'https://example.test/PRIVATE_URL',
      headers: { Authorization: 'PRIVATE_TOKEN' },
    },
    breadcrumbs: [{ message: 'PRIVATE_CLIPBOARD' }],
    exception: {
      values: [
        {
          type: 'TypeError',
          value: 'PRIVATE_MESSAGE',
          stacktrace: {
            frames: [
              {
                filename:
                  '/Users/PRIVATE_USER/app/dist/renderer/assets/index-abc.js',
                function: 'PRIVATE_FUNCTION',
                lineno: 4,
                colno: 8,
                vars: { token: 'PRIVATE_VAR' },
              },
              {
                filename: '/Users/PRIVATE_USER/extensions/PRIVATE_EXTENSION.js',
                lineno: 1,
              },
            ],
          },
        },
      ],
    },
  });
  assert.ok(!JSON.stringify(event).includes('PRIVATE_'));
  assert.deepEqual(event.exception?.values?.[0]?.stacktrace?.frames, [
    {
      filename: 'app:///dist/renderer/assets/index-abc.js',
      in_app: true,
      lineno: 4,
      colno: 8,
    },
  ]);
});

test('only required capture and preload integrations are permitted', () => {
  for (const name of [
    'SentryMinidump',
    'ElectronMinidump',
    'LocalVariables',
    'ContextLines',
    'Console',
    'Screenshots',
    'MainProcessSession',
  ])
    assert.ok(!MAIN_DIAGNOSTIC_INTEGRATIONS.has(name));
  assert.ok(MAIN_DIAGNOSTIC_INTEGRATIONS.has('PreloadInjection'));
  assert.ok(!RENDERER_DIAGNOSTIC_INTEGRATIONS.has('ScopeToMain'));
  assert.ok(!RENDERER_DIAGNOSTIC_INTEGRATIONS.has('Dedupe'));
  assert.ok(!RENDERER_DIAGNOSTIC_INTEGRATIONS.has('Breadcrumbs'));
});

test('successful journeys retain bounded correlation and outcomes without private content', () => {
  const event = sanitizeDiagnosticEvent<Event>({
    type: 'transaction' as const,
    transaction: 'command.execute',
    contexts: {
      trace: {
        trace_id: 'a'.repeat(32),
        span_id: 'b'.repeat(16),
        data: {
          'diagnostic.journey_id': 'c'.repeat(32),
          'diagnostic.boot_id': 'f'.repeat(32),
          'diagnostic.action_id': 'd'.repeat(32),
          'diagnostic.outcome': 'success',
          'diagnostic.stage': 'complete',
          'diagnostic.component': 'clipboard',
          'diagnostic.request_id': 'server-request',
          prompt: 'PRIVATE_PROMPT',
        },
      },
    },
  });
  assert.equal(event.level, 'info');
  assert.equal(event.tags?.journey_id, 'c'.repeat(32));
  assert.equal(event.tags?.boot_id, 'f'.repeat(32));
  assert.equal(event.tags?.action_id, 'd'.repeat(32));
  assert.equal(event.tags?.outcome, 'success');
  assert.equal(event.tags?.component, 'clipboard');
  assert.equal(
    event.contexts?.trace?.data?.['diagnostic.request_id'],
    'server-request',
  );
  assert.ok(!JSON.stringify(event).includes('PRIVATE_'));
  assert.deepEqual(sanitizeDiagnosticEvent(event), event);
});

test('journey metadata rejects arbitrary identifiers, components, and request content', () => {
  const event = sanitizeDiagnosticEvent({
    type: 'transaction' as const,
    transaction: 'command.execute',
    contexts: {
      trace: {
        trace_id: 'a'.repeat(32),
        span_id: 'b'.repeat(16),
        data: {
          'diagnostic.journey_id': 'PRIVATE_ACCOUNT',
          'diagnostic.action_id': 'PRIVATE_COMMAND',
          'diagnostic.component': 'PRIVATE_EXTENSION',
          'diagnostic.request_id': 'PRIVATE_REQUEST\nHEADER',
        },
      },
    },
  });
  assert.deepEqual(event.contexts?.trace?.data, {});
  assert.ok(!JSON.stringify(event).includes('PRIVATE_'));
});
