import assert from 'node:assert/strict';
import { test } from 'node:test';
import { resolveBillableTokens } from './proxy';

// A minimal BillContext for testing
const ctx = {
  user: { id: 'test-user' },
  provider: 'opencode_zen',
  activeModelId: 'gemini-3-flash',
  costRow: { inputUsdPerMtok: 0, outputUsdPerMtok: 0 },
  kind: 'free' as const,
  requestId: 'req_123',
  client: { name: null, version: null, apiVersion: null, platform: null, arch: null },
  estimatedInputTokens: 50,
};

test('passes through tokens when output is non-zero', () => {
  const result = resolveBillableTokens(ctx, { inputTokens: 100, outputTokens: 50 }, 200);
  assert.deepEqual(result, { inputTokens: 100, outputTokens: 50 });
});

test('passes through when estimatedInputTokens is 0', () => {
  const noEstimate = { ...ctx, estimatedInputTokens: 0 };
  const result = resolveBillableTokens(noEstimate, { inputTokens: 0, outputTokens: 0 }, 200);
  assert.deepEqual(result, { inputTokens: 0, outputTokens: 0 });
});

test('falls back to estimated input + minimum output on 2xx with zero output tokens', () => {
  const result = resolveBillableTokens(ctx, { inputTokens: 0, outputTokens: 0 }, 200);
  assert.deepEqual(result, { inputTokens: 50, outputTokens: 1 });
});

test('falls back for streaming tokens on 2xx with zero output', () => {
  const result = resolveBillableTokens(ctx, { inputTokens: 0, outputTokens: 0 }, 201);
  assert.deepEqual(result, { inputTokens: 50, outputTokens: 1 });
});

test('does NOT fall back for non-2xx responses (zero-cost errors)', () => {
  for (const status of [400, 401, 403, 404, 429, 500, 502, 503]) {
    const result = resolveBillableTokens(ctx, { inputTokens: 0, outputTokens: 0 }, status);
    assert.deepEqual(result, { inputTokens: 0, outputTokens: 0 }, `should not fall back for status ${status}`);
  }
});

test('does NOT override non-zero output tokens even on 2xx', () => {
  const result = resolveBillableTokens(ctx, { inputTokens: 100, outputTokens: 25 }, 200);
  assert.deepEqual(result, { inputTokens: 100, outputTokens: 25 });
});
