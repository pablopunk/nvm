import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { resetDbForTests, setDbForTests } from '../db/client';
import { listAdminModelRefs } from './admin-model-options';
import { resetPricingCacheForTests } from './pricing';
import { refreshPricingSnapshots } from './pricing-refresh';

test('model options include only enabled providers with configured keys', async () => {
  const originalFetch = globalThis.fetch;
  const originalOpenRouterKey = process.env.OPENROUTER_API_KEY;
  const originalOpenCodeKey = process.env.OPENCODE_API_KEY;
  setDbForTests({
    select: () => ({
      from: () => ({
        where: () => ({
          orderBy: async () => [{ id: 'openrouter' }, { id: 'opencode_zen' }],
        }),
      }),
    }),
  } as never);
  process.env.OPENROUTER_API_KEY = 'test';
  delete process.env.OPENCODE_API_KEY;
  globalThis.fetch = (async () => ({
    ok: true,
    json: async () => ({
      openrouter: { models: { 'google/gemini-2.5-flash': { id: 'google/gemini-2.5-flash', tool_call: true, modalities: { input: ['text', 'image'], output: ['text'] }, limit: { context: 100_000 }, cost: { input: 1, output: 1 } } } },
      opencode: { models: { 'test-model': { id: 'test-model', tool_call: true, modalities: { input: ['text', 'image'], output: ['text'] }, limit: { context: 100_000 }, cost: { input: 1, output: 1 } } } },
    }),
  })) as unknown as typeof fetch;
  try {
    assert.equal((await refreshPricingSnapshots([])).catalog, 'refreshed');
    const refs = await listAdminModelRefs('fast');
    assert.ok(refs.includes('openrouter/google/gemini-2.5-flash'));
    assert.ok(refs.every((ref) => !ref.startsWith('opencode_zen/')));
  } finally {
    globalThis.fetch = originalFetch;
    if (originalOpenRouterKey === undefined) delete process.env.OPENROUTER_API_KEY;
    else process.env.OPENROUTER_API_KEY = originalOpenRouterKey;
    if (originalOpenCodeKey === undefined) delete process.env.OPENCODE_API_KEY;
    else process.env.OPENCODE_API_KEY = originalOpenCodeKey;
  }
});

afterEach(() => {
  resetDbForTests();
  resetPricingCacheForTests();
});
