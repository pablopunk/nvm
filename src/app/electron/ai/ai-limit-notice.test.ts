import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: false, getPath: () => '/tmp/nvm-ai-limit-notice-test' },
    safeStorage: { isEncryptionAvailable: () => false },
  },
});
mock.module('electron-log/main', {
  defaultExport: { error() {}, info() {}, warn() {}, debug() {} },
});

const { aiLimitNoticeFromError } = await import('./ai-limit-notice');

const nevermindRateLimitBody = JSON.stringify({
  error: {
    type: 'rate_limited',
    message: 'Rate limit exceeded (user:chat)',
    retry_after: 12,
    dashboard_url: 'https://example.test/dashboard',
  },
});

test('Nevermind rate limits point to the dashboard with the retry delay', () => {
  const notice = aiLimitNoticeFromError(
    new Error(`429 ${nevermindRateLimitBody}`),
  );

  assert.equal(notice?.kind, 'rate_limited');
  assert.equal(notice?.retryAfterSec, 12);
  assert.equal(notice?.action?.type, 'openUrl');
});

test('provider rate limits are not reported as the user reaching a Nevermind limit', () => {
  const anthropic = aiLimitNoticeFromError(
    new Error(
      '429 {"type":"error","error":{"type":"rate_limit_error","message":"Number of request tokens has exceeded your per-minute rate limit"}}',
    ),
  );
  const openAi = aiLimitNoticeFromError(
    new Error(
      '429 Rate limit reached for requests. Please retry after 20 seconds.',
    ),
  );

  assert.equal(anthropic?.kind, 'provider_busy');
  assert.equal(anthropic?.action, undefined);
  assert.match(anthropic?.message || '', /not your Nevermind usage limit/);
  assert.equal(openAi?.kind, 'provider_busy');
  assert.equal(openAi?.retryAfterSec, 20);
});

test('a stack frame on line 429 is not mistaken for a rate limit', () => {
  const error = new Error('Cannot read properties of undefined');
  error.stack = 'TypeError: Cannot read\n    at run (file:///ext.ts:429:12)';

  assert.equal(aiLimitNoticeFromError(error), null);
});

test('credit and free-pool limits keep their own guidance', () => {
  assert.equal(
    aiLimitNoticeFromError(new Error('402 insufficient_credits'))?.kind,
    'insufficient_credits',
  );
  const freePool = aiLimitNoticeFromError(
    new Error(
      '429 {"error":{"type":"free_spend_cap_reached","resetsAt":"2026-11-01T00:00:00Z"}}',
    ),
  );
  assert.equal(freePool?.kind, 'free_spend_cap_reached');
  assert.match(freePool?.message || '', /2026-11-01T00:00:00Z/);
});

test('status codes inside stack locations never trigger credit or update notices', () => {
  const error = new Error('boom');
  error.stack =
    'Error: boom\n    at a (x.ts:402:1)\n    at b (y.ts:413:2)\n    at c (z.ts:426:3)';

  assert.equal(aiLimitNoticeFromError(error), null);
  assert.equal(
    aiLimitNoticeFromError(new Error('Request failed with status 426'))?.kind,
    'unsupported_client',
  );
});
