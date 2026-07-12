import assert from 'node:assert/strict';
import { afterEach, describe, test } from 'node:test';
import { resetDbForTests, setDbForTests } from '../db/client';
import { modelProviders, providers } from '../db/schema';
import {
  getModelProviderChain,
  getModelRoute,
  listKnownProviders,
  modelRouteToRef,
  ModelNotConfiguredError,
  parseExtensionAiModelRole,
  parseModelRouteRef,
  setModelProviderChain,
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
    assert.deepEqual(route, { provider: 'anthropic', modelId: 'claude-sonnet-4-6' });
  });

  test('returns stored ref-string route', async () => {
    const db = fakeDb([
      [{ value: 'openai/gpt-4o' }],
    ]);
    setDbForTests(db);

    const route = await getModelRoute('paid');
    assert.deepEqual(route, { provider: 'openai', modelId: 'gpt-4o' });
  });

  test('falls back to legacy modelId + active provider when no route stored', async () => {
    const db = fakeDb([
      [],                                   // active_model_route: none
      [{ value: 'anthropic' }],             // active_provider
      [{ value: 'claude-sonnet-4-6' }],     // active_model
    ]);
    setDbForTests(db);

    const route = await getModelRoute('paid');
    assert.deepEqual(route, { provider: 'anthropic', modelId: 'claude-sonnet-4-6' });
  });

  test('falls back to default provider when active_provider is unset', async () => {
    const db = fakeDb([
      [],                         // active_model_route: none
      [],                         // active_provider: none → default
      [{ value: 'some-model' }],  // active_model
    ]);
    setDbForTests(db);

    const route = await getModelRoute('paid');
    assert.deepEqual(route, { provider: 'opencode_zen', modelId: 'some-model' });
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
    assert.deepEqual(route, { provider: 'openai', modelId: 'gpt-4o' });
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
    assert.deepEqual(route, { provider: 'opencode_zen', modelId: 'gemini-flash' });
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
    });
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
