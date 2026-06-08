import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { resetDbForTests, setDbForTests } from '../db/client';
import { creditLedger } from '../db/schema';
import { ensureMonthlyFreeCredits } from './users';

type FakeDb = {
  insertedValues: unknown[];
  select: () => ReturnType<typeof createChain>;
  insert: (table: unknown) => ReturnType<typeof createChain>;
  transaction: (callback: (tx: FakeDb) => Promise<void>) => Promise<void>;
};

function createChain(result: unknown, onValues?: (values: unknown) => void) {
  const promise = () => Promise.resolve(result);
  const chain = {
    from: () => chain,
    where: () => chain,
    limit: () => promise(),
    values: (values: unknown) => {
      onValues?.(values);
      return chain;
    },
    onConflictDoNothing: () => chain,
    then: (resolve: Parameters<Promise<unknown>['then']>[0], reject: Parameters<Promise<unknown>['then']>[1]) => promise().then(resolve, reject),
    catch: (reject: Parameters<Promise<unknown>['catch']>[0]) => promise().catch(reject),
  };
  return chain;
}

function createFakeDb(selects: unknown[]): FakeDb {
  const remainingSelects = [...selects];
  const insertedValues: unknown[] = [];
  let db: FakeDb;
  db = {
    insertedValues,
    select: () => createChain(remainingSelects.shift() ?? []),
    insert: (table: unknown) => table === creditLedger ? createChain([], (values) => insertedValues.push(values)) : createChain([]),
    transaction: async (callback: (tx: FakeDb) => Promise<void>) => callback(db),
  };
  return db;
}

afterEach(() => resetDbForTests());

test('monthly free credits top up to allowance once per UTC month', async () => {
  const db = createFakeDb([[], [{ free: 125 }]]);
  setDbForTests(db as any);

  await ensureMonthlyFreeCredits('user_1', new Date('2026-06-15T12:00:00Z'));

  assert.deepEqual(db.insertedValues, [{
    userId: 'user_1',
    delta: 375,
    kind: 'free',
    reason: 'grant_free_monthly',
    refId: '2026-06',
  }]);
});

test('monthly free credits record a zero marker instead of rolling over above allowance', async () => {
  const db = createFakeDb([[], [{ free: 800 }]]);
  setDbForTests(db as any);

  await ensureMonthlyFreeCredits('user_1', new Date('2026-06-15T12:00:00Z'));

  assert.deepEqual(db.insertedValues, [{
    userId: 'user_1',
    delta: 0,
    kind: 'free',
    reason: 'grant_free_monthly',
    refId: '2026-06',
  }]);
});

test('monthly free credits skip when the current period was already granted', async () => {
  const db = createFakeDb([[{ id: 1 }]]);
  setDbForTests(db as any);

  await ensureMonthlyFreeCredits('user_1', new Date('2026-06-15T12:00:00Z'));

  assert.deepEqual(db.insertedValues, []);
});
