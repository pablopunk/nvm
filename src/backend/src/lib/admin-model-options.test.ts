import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { resetDbForTests, setDbForTests } from '../db/client';
import { listAdminModelRefs, openRouterAccessibleModels } from './admin-model-options';
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
  globalThis.fetch = (async (input: string | URL | Request) => String(input).endsWith('/models') ? ({
    ok: true,
    json: async () => ({ data: [{ id: 'google/gemini-2.5-flash' }] }),
  }) : ({
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

test('OpenRouter model access uses the configured key and fails closed', async () => {
  const originalFetch = globalThis.fetch;
  const originalKey = process.env.OPENROUTER_API_KEY;
  process.env.OPENROUTER_API_KEY = 'test-key';
  try {
    globalThis.fetch = (async (_input: string | URL | Request, init?: RequestInit) => {
      assert.equal(new Headers(init?.headers).get('authorization'), 'Bearer test-key');
      return Response.json({ data: [{ id: 'available' }] });
    }) as typeof fetch;
    assert.deepEqual(await openRouterAccessibleModels(), new Set(['available']));
    globalThis.fetch = (async () => Response.json({ error: 'unavailable' }, { status: 503 })) as typeof fetch;
    assert.equal(await openRouterAccessibleModels(), null);
    globalThis.fetch = (async () => Response.json({ data: [{}] })) as typeof fetch;
    assert.equal(await openRouterAccessibleModels(), null);
  } finally {
    globalThis.fetch = originalFetch;
    if (originalKey === undefined) delete process.env.OPENROUTER_API_KEY;
    else process.env.OPENROUTER_API_KEY = originalKey;
  }
});
