import assert from 'node:assert/strict';
import { test } from 'node:test';
import { resolveBillableTokens } from './proxy';

// A minimal BillContext for testing
const ctx = {
  user: { id: 'test-user' },
  provider: 'opencode_zen',
  activeModelId: 'gemini-3-flash',
  costRow: { provider: 'opencode_zen', modelId: 'gemini-3-flash', inputUsdPerMtok: 0, outputUsdPerMtok: 0 },
  kind: 'free' as const,
  requestId: 'req_123',
  client: { name: null, version: null, apiVersion: null, platform: null, arch: null },
  estimatedInputTokens: 50,
};

test('passes through tokens when output is non-zero', function passesThroughWhenOutputNonZero() {
  const result = resolveBillableTokens(ctx, { inputTokens: 100, outputTokens: 50 }, 200);
  assert.deepEqual(result, { inputTokens: 100, outputTokens: 50 });
});

test('passes through when estimatedInputTokens is 0', function passesThroughZeroEstimate() {
  const noEstimate = { ...ctx, estimatedInputTokens: 0 };
  const result = resolveBillableTokens(noEstimate, { inputTokens: 0, outputTokens: 0 }, 200);
  assert.deepEqual(result, { inputTokens: 0, outputTokens: 0 });
});

test('falls back to estimated input + minimum output on 2xx with zero output', function fallsBackOnMissingUsage() {
  const result = resolveBillableTokens(ctx, { inputTokens: 0, outputTokens: 0 }, 200);
  assert.deepEqual(result, { inputTokens: 50, outputTokens: 1 });
});

test('preserves actual input tokens when only output tokens are missing', function preservesActualInput() {
  const result = resolveBillableTokens(ctx, { inputTokens: 75, outputTokens: 0 }, 200);
  assert.deepEqual(result, { inputTokens: 75, outputTokens: 1 });
});

test('does NOT fall back for non-2xx responses (zero-cost errors)', function keepsZeroCostForErrors() {
  for (const status of [400, 401, 403, 404, 429, 500, 502, 503]) {
    const result = resolveBillableTokens(ctx, { inputTokens: 0, outputTokens: 0 }, status);
    assert.deepEqual(result, { inputTokens: 0, outputTokens: 0 }, `should not fall back for status ${status}`);
  }
});

test('does NOT override non-zero output tokens even on 2xx', function doesNotOverrideNonZeroOutput() {
  const result = resolveBillableTokens(ctx, { inputTokens: 100, outputTokens: 25 }, 200);
  assert.deepEqual(result, { inputTokens: 100, outputTokens: 25 });
});
