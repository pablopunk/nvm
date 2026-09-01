import assert from 'node:assert/strict';
import { afterEach, describe, test } from 'node:test';
import { resetDbForTests, setDbForTests } from '../db/client';
import { modelProviders, providers } from '../db/schema';
import {
  getModelProviderChain,
  getModelRoute,
  listKnownProviders,
  modelRouteSlotForAccount,
  modelTierForPlan,
  modelRouteToRef,
  ModelNotConfiguredError,
  parseExtensionAiModelRole,
  parseModelRouteSlot,
  parseModelRouteRef,
  setModelRoute,
  setModelProviderChain,
  tieredModelRouteSlot,
  getSignupsEnabled,
  SignupsPolicyError,
} from './settings';

function promiseChain(result: unknown, onValues?: (v: unknown) => void) {
  const p = () => Promise.resolve(result);
  const chain: any = {
    from: () => chain,
    innerJoin: () => chain,
    where: () => chain,
    limit: () => p(),
    orderBy: () => p(),
    values: (v: unknown) => {
      onValues?.(v);
      return chain;
    },
    set: () => chain,
    returning: () => p(),
    onConflictDoNothing: () => chain,
    onConflictDoUpdate: () => chain,
    then: (r: any, j: any) => p().then(r, j),
  };
  return chain;
}

function fakeDb(selects: unknown[], onInsert?: (v: unknown) => void) {
  const remaining = [...selects];
  const db: any = {
    select: () => promiseChain(remaining.shift()),
    insert: () => promiseChain([], onInsert),
    update: () => promiseChain([]),
    delete: () => promiseChain([]),
    transaction: async (cb: (tx: any) => Promise<void>) => cb(db),
  };
  return db;
}

afterEach(() => resetDbForTests());

describe('getSignupsEnabled', () => {
  const previous = process.env.INVITE_GATE_ENABLED;

  afterEach(() => {
    if (previous === undefined) delete process.env.INVITE_GATE_ENABLED;
    else process.env.INVITE_GATE_ENABLED = previous;
  });

  test('uses the legacy open policy only when the setting row is absent', async () => {
    process.env.INVITE_GATE_ENABLED = 'false';
    setDbForTests(fakeDb([[]]));
    assert.equal(await getSignupsEnabled(), true);

    process.env.INVITE_GATE_ENABLED = 'true';
    setDbForTests(fakeDb([[]]));
    assert.equal(await getSignupsEnabled(), false);
  });

  test('treats a valid persisted policy as authoritative over the legacy flag', async () => {
    process.env.INVITE_GATE_ENABLED = 'true';
    setDbForTests(fakeDb([[{ value: 'true' }]]));
    assert.equal(await getSignupsEnabled(), true);

    process.env.INVITE_GATE_ENABLED = 'false';
    setDbForTests(fakeDb([[{ value: 'false' }]]));
    assert.equal(await getSignupsEnabled(), false);
  });

  test('fails closed for malformed persisted policy data', async () => {
    process.env.INVITE_GATE_ENABLED = 'false';
    setDbForTests(fakeDb([[{ value: 'not-a-boolean' }]]));
    await assert.rejects(() => getSignupsEnabled(), SignupsPolicyError);
  });
});

describe('parseModelRouteRef', () => {
  test('parses valid provider/model ref', () => {
    assert.deepEqual(parseModelRouteRef('openai/gpt-4o'), {
      provider: 'openai',
      modelId: 'gpt-4o',
    });
  });

  test('parses opencode_zen ref', () => {
    assert.deepEqual(parseModelRouteRef('opencode_zen/claude-sonnet-4-6'), {
      provider: 'opencode_zen',
      modelId: 'claude-sonnet-4-6',
    });
  });

  test('preserves slash-qualified OpenRouter model ids', () => {
    assert.deepEqual(
      parseModelRouteRef('openrouter/google/gemini-2.5-flash'),
      {
        provider: 'openrouter',
        modelId: 'google/gemini-2.5-flash',
      },
    );
  });

  test('returns null for missing slash', () => {
    assert.strictEqual(parseModelRouteRef('gpt-4o'), null);
  });

  test('returns null for leading slash', () => {
    assert.strictEqual(parseModelRouteRef('/gpt-4o'), null);
  });

  test('returns null for unknown provider', () => {
    assert.strictEqual(parseModelRouteRef('made-up/gpt-4o'), null);
  });

  test('returns null for empty modelId after slash', () => {
    assert.strictEqual(parseModelRouteRef('openai/'), null);
  });
});

describe('modelRouteToRef', () => {
  test('encodes route as provider/modelId string', () => {
    assert.strictEqual(
      modelRouteToRef({ provider: 'anthropic', modelId: 'claude-sonnet-4-6' }),
      'anthropic/claude-sonnet-4-6',
    );
  });

  test('round-trips with parseModelRouteRef', () => {
    const route = { provider: 'openai' as const, modelId: 'gpt-4o' };
    const ref = modelRouteToRef(route);
    assert.deepEqual(parseModelRouteRef(ref), route);
  });
});

describe('parseExtensionAiModelRole', () => {
  test('recognises smart', () => {
    assert.strictEqual(parseExtensionAiModelRole('smart'), 'smart');
  });

  test('recognises fast', () => {
    assert.strictEqual(parseExtensionAiModelRole('fast'), 'fast');
  });

  test('returns null for null/undefined', () => {
    assert.strictEqual(parseExtensionAiModelRole(null), null);
    assert.strictEqual(parseExtensionAiModelRole(undefined), null);
  });

  test('returns null for unknown values', () => {
    assert.strictEqual(parseExtensionAiModelRole('paid'), null);
    assert.strictEqual(parseExtensionAiModelRole(''), null);
  });
});

test('combines a model tier and extension model role into a route slot', () => {
  assert.strictEqual(tieredModelRouteSlot('pro', 'smart'), 'pro-smart');
  assert.strictEqual(tieredModelRouteSlot('free', 'fast'), 'free-fast');
});

test('parses current and legacy model route slots', () => {
  assert.strictEqual(parseModelRouteSlot('pro-smart'), 'pro-smart');
  assert.strictEqual(parseModelRouteSlot('free-fast'), 'free-fast');
  assert.strictEqual(parseModelRouteSlot('paid'), 'paid');
  assert.strictEqual(parseModelRouteSlot('typo'), null);
});

test('maps only Pro subscriptions to the Pro model tier', () => {
  assert.strictEqual(modelTierForPlan('pro'), 'pro');
  assert.strictEqual(modelTierForPlan('free'), 'free');
  assert.strictEqual(modelTierForPlan('unknown'), 'free');
});

test('selects model routes from subscription entitlement, credits, and requested role', () => {
  assert.strictEqual(modelRouteSlotForAccount('pro', 100, 'smart'), 'pro-smart');
  assert.strictEqual(modelRouteSlotForAccount('pro', 100, 'fast'), 'pro-fast');
  assert.strictEqual(modelRouteSlotForAccount('free', 100, 'smart'), 'free-smart');
  assert.strictEqual(modelRouteSlotForAccount('free', 100, 'fast'), 'free-fast');
  assert.strictEqual(modelRouteSlotForAccount('pro', 0, 'smart'), 'free-smart');
  assert.strictEqual(modelRouteSlotForAccount('pro', 0, 'fast'), 'free-fast');
  assert.strictEqual(modelRouteSlotForAccount('pro', 100, 'smart', true), 'free-smart');
});

describe('listKnownProviders', () => {
  test('returns the set of known providers', () => {
    const known = listKnownProviders();
    assert.ok(known.includes('opencode_zen'));
    assert.ok(known.includes('openrouter'));
    assert.ok(known.includes('anthropic'));
    assert.ok(known.includes('openai'));
    assert.ok(known.includes('google'));
  });
});

describe('getModelRoute', () => {
  test('returns stored JSON route for paid slot', async () => {
    const db = fakeDb([
      [{ value: '{"provider":"anthropic","modelId":"claude-sonnet-4-6"}' }],
    ]);
    setDbForTests(db);

    const route = await getModelRoute('paid');
    assert.deepEqual(route, { provider: 'anthropic', modelId: 'claude-sonnet-4-6', thinkingLevel: 'low' });
  });

  test('returns stored ref-string route', async () => {
    const db = fakeDb([
      [{ value: 'openai/gpt-4o' }],
    ]);
    setDbForTests(db);

    const route = await getModelRoute('paid');
    assert.deepEqual(route, { provider: 'openai', modelId: 'gpt-4o', thinkingLevel: 'low' });
  });

  test('returns the stored thinking level', async () => {
    const db = fakeDb([
      [{ value: '{"provider":"anthropic","modelId":"claude-sonnet-4-6","thinkingLevel":"high"}' }],
    ]);
    setDbForTests(db);

    const route = await getModelRoute('paid');
    assert.deepEqual(route, { provider: 'anthropic', modelId: 'claude-sonnet-4-6', thinkingLevel: 'high' });
  });

  test('falls back to legacy modelId + active provider when no route stored', async () => {
    const db = fakeDb([
      [],                                   // active_model_route: none
      [{ value: 'anthropic' }],             // active_provider
      [{ value: 'claude-sonnet-4-6' }],     // active_model
    ]);
    setDbForTests(db);

    const route = await getModelRoute('paid');
    assert.deepEqual(route, { provider: 'anthropic', modelId: 'claude-sonnet-4-6', thinkingLevel: 'low' });
  });

  test('falls back to default provider when active_provider is unset', async () => {
    const db = fakeDb([
      [],                         // active_model_route: none
      [],                         // active_provider: none → default
      [{ value: 'some-model' }],  // active_model
    ]);
    setDbForTests(db);

    const route = await getModelRoute('paid');
    assert.deepEqual(route, { provider: 'opencode_zen', modelId: 'some-model', thinkingLevel: 'low' });
  });

  test('smart slot falls back to paid chain', async () => {
    const db = fakeDb([
      [],                                 // smart_model_route: none
      [],                                 // (paid) active_model_route: none
      [{ value: 'openai' }],              // active_provider
      [{ value: 'gpt-4o' }],             // active_model
    ]);
    setDbForTests(db);

    const route = await getModelRoute('smart');
    assert.deepEqual(route, { provider: 'openai', modelId: 'gpt-4o', thinkingLevel: 'low' });
  });

  test('fast slot falls back to free chain', async () => {
    const db = fakeDb([
      [],                                // fast_model_route: none
      [],                                // (free) free_model_route: none
      [],                                // active_provider: none → default
      [{ value: 'gemini-flash' }],       // free_model
    ]);
    setDbForTests(db);

    const route = await getModelRoute('fast');
    assert.deepEqual(route, { provider: 'opencode_zen', modelId: 'gemini-flash', thinkingLevel: 'low' });
  });

  test('tiered slot returns its independently stored route', async () => {
    setDbForTests(fakeDb([
      [{ value: '{"provider":"anthropic","modelId":"claude-sonnet-4-6","thinkingLevel":"high"}' }],
    ]));

    const route = await getModelRoute('free-smart');
    assert.deepEqual(route, { provider: 'anthropic', modelId: 'claude-sonnet-4-6', thinkingLevel: 'high' });
  });

  test('Pro Fast falls back to the legacy paid route', async () => {
    setDbForTests(fakeDb([
      [],
      [{ value: '{"provider":"openai","modelId":"gpt-4o","thinkingLevel":"medium"}' }],
    ]));

    const route = await getModelRoute('pro-fast');
    assert.deepEqual(route, { provider: 'openai', modelId: 'gpt-4o', thinkingLevel: 'medium' });
  });

  test('Free Smart never falls back through the legacy paid Smart route', async () => {
    setDbForTests(fakeDb([
      [],
      [{ value: '{"provider":"google","modelId":"gemini-flash","thinkingLevel":"low"}' }],
    ]));

    const route = await getModelRoute('free-smart');
    assert.deepEqual(route, { provider: 'google', modelId: 'gemini-flash', thinkingLevel: 'low' });
  });

  test('Free Smart falls back to the legacy free route', async () => {
    setDbForTests(fakeDb([
      [],
      [],
      [{ value: 'google' }],
      [{ value: 'gemini-pro' }],
    ]));

    const route = await getModelRoute('free-smart');
    assert.deepEqual(route, { provider: 'google', modelId: 'gemini-pro', thinkingLevel: 'low' });
  });

  test('throws ModelNotConfiguredError when legacy modelId is missing', async () => {
    const db = fakeDb([
      [],  // active_model_route: none
      [],  // active_provider: none
      [],  // active_model: none
    ]);
    setDbForTests(db);

    await assert.rejects(
      () => getModelRoute('paid'),
      ModelNotConfiguredError,
    );
  });

  test('JSON stored route with unknown provider falls through to legacy which then fails', async () => {
    const db = fakeDb([
      [{ value: '{"provider":"made-up","modelId":"gpt-4o"}' }],
      [],
      [],
    ]);
    setDbForTests(db);

    await assert.rejects(
      () => getModelRoute('paid'),
      ModelNotConfiguredError,
    );
  });
});

test('persists the thinking level with a model route', async () => {
  let inserted: any;
  setDbForTests(fakeDb([], (values) => { inserted = values; }));

  await setModelRoute('smart', {
    provider: 'anthropic',
    modelId: 'claude-sonnet-4-6',
    thinkingLevel: 'high',
  });

  assert.equal(inserted.key, 'smart_model_route');
  assert.deepEqual(JSON.parse(inserted.value), {
    provider: 'anthropic',
    modelId: 'claude-sonnet-4-6',
    thinkingLevel: 'high',
  });
});

test('persists all four tiered model routes independently', async () => {
  const inserted: any[] = [];
  setDbForTests(fakeDb([], (values) => { inserted.push(values); }));

  for (const slot of ['pro-smart', 'pro-fast', 'free-smart', 'free-fast'] as const) {
    await setModelRoute(slot, {
      provider: 'openai',
      modelId: `model-${slot}`,
      thinkingLevel: 'minimal',
    });
  }

  assert.deepEqual(inserted.map((value) => value.key), [
    'pro_smart_model_route',
    'pro_fast_model_route',
    'free_smart_model_route',
    'free_fast_model_route',
  ]);
});

describe('getModelProviderChain', () => {
  test('returns provider ids ordered by priority', async () => {
    const db = fakeDb([
      [{ providerId: 'opencode_zen' }, { providerId: 'openrouter' }, { providerId: 'anthropic' }],
    ]);
    setDbForTests(db);

    const chain = await getModelProviderChain('paid', 'some-model');
    assert.deepEqual(chain, ['opencode_zen', 'openrouter', 'anthropic']);
  });

  test('returns empty array when no providers match', async () => {
    const db = fakeDb([[]]);
    setDbForTests(db);

    const chain = await getModelProviderChain('paid', 'some-model');
    assert.deepEqual(chain, []);
  });

  test('filters by route slot and modelId', async () => {
    const db = fakeDb([[{ providerId: 'openai' }]]);
    setDbForTests(db);

    const chain = await getModelProviderChain('fast', 'gemini-flash');
    assert.deepEqual(chain, ['openai']);
  });

  test('tiered route falls back to its legacy provider chain', async () => {
    const db = fakeDb([
      [],
      [{ providerId: 'openrouter' }],
    ]);
    setDbForTests(db);

    const chain = await getModelProviderChain('pro-smart', 'some-model');
    assert.deepEqual(chain, ['openrouter']);
  });
});

describe('setModelProviderChain', () => {
  test('deletes old entries and inserts new ordered chain', async () => {
    let deletedTable: unknown = null;
    const inserted: unknown[] = [];

    function deleteChain() {
      const p = () => Promise.resolve();
      const chain: any = {
        from: () => chain,
        innerJoin: () => chain,
        where: () => chain,
        limit: () => p(),
        orderBy: () => p(),
        then: (r: any, j: any) => p().then(r, j),
      };
      return chain;
    }

    const db: any = {
      select: () => promiseChain([]),
      insert: () => promiseChain([], (v) => inserted.push(v)),
      update: () => promiseChain([]),
      delete: (table: unknown) => {
        deletedTable = table;
        return {
          where: () => promiseChain([]),
        };
      },
      transaction: async (cb: (tx: any) => Promise<void>) => cb(db),
    };

    setDbForTests(db);
    await setModelProviderChain('paid', 'test-model', ['opencode_zen', 'anthropic', 'openai']);

    assert.strictEqual(deletedTable, modelProviders);
    assert.strictEqual(inserted.length, 1);
    const chainValues = inserted[0] as any[];
    assert.strictEqual(chainValues.length, 3);
    assert.deepEqual(
      chainValues.map((v: any) => v.providerId),
      ['opencode_zen', 'anthropic', 'openai'],
    );
    assert.deepEqual(
      chainValues.map((v: any) => v.priority),
      [0, 1, 2],
    );
    chainValues.forEach((v: any) => {
      assert.strictEqual(v.routeSlot, 'paid');
      assert.strictEqual(v.modelId, 'test-model');
      assert.strictEqual(v.providerModelId, 'test-model');
    });
  });

  test('stores provider-specific model ids for failover', async () => {
    let inserted: any[] = [];
    const db: any = {
      insert: () => promiseChain([], (values) => { inserted = values as any[]; }),
      delete: () => ({ where: () => promiseChain([]) }),
      transaction: async (cb: (tx: any) => Promise<void>) => cb(db),
    };
    setDbForTests(db);

    await setModelProviderChain('pro-smart', 'openai/gpt-5.6-sol', [
      { providerId: 'opencode_zen', modelId: 'gpt-5.6-sol' },
    ]);

    assert.equal(inserted[0].modelId, 'openai/gpt-5.6-sol');
    assert.equal(inserted[0].providerId, 'opencode_zen');
    assert.equal(inserted[0].providerModelId, 'gpt-5.6-sol');
  });

  test('empty provider list deletes without inserting', async () => {
    let deletedTable: unknown = null;
    const inserted: unknown[] = [];

    const db: any = {
      select: () => promiseChain([]),
      insert: () => promiseChain([], (v) => inserted.push(v)),
      update: () => promiseChain([]),
      delete: (table: unknown) => {
        deletedTable = table;
        return { where: () => promiseChain([]) };
      },
      transaction: async (cb: (tx: any) => Promise<void>) => cb(db),
    };

    setDbForTests(db);
    await setModelProviderChain('free', 'test-model', []);

    assert.strictEqual(deletedTable, modelProviders);
    assert.strictEqual(inserted.length, 0);
  });
});
