import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { resetDbForTests, setDbForTests } from '../db/client';
import { creditLedger } from '../db/schema';
import { ensureMonthlyFreeCredits, getBalances, upsertUserWithFreeGrant, DisposableEmailError } from './users';
import { users } from '../db/schema';

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

// ── getBalances ──

test('getBalances returns free/paid/total split', async () => {
  const db = createFakeDb([[{ free: 500, paid: 1000 }]]);
  setDbForTests(db as any);

  const balances = await getBalances('user_1');
  assert.deepEqual(balances, { free: 500, paid: 1000, total: 1500 });
});

test('getBalances returns zeros when no ledger rows exist', async () => {
  const db = createFakeDb([[{}]]);
  setDbForTests(db as any);

  const balances = await getBalances('user_1');
  assert.deepEqual(balances, { free: 0, paid: 0, total: 0 });
});

test('getBalances returns zeros on empty result', async () => {
  const db = createFakeDb([[]]);
  setDbForTests(db as any);

  const balances = await getBalances('user_1');
  assert.deepEqual(balances, { free: 0, paid: 0, total: 0 });
});

// ── upsertUserWithFreeGrant ──

function createFlexibleFakeDb(
  selects: unknown[],
  userReturningValues?: unknown[],
  updateReturningValues?: unknown[],
) {
  const remainingSelects = [...selects];
  const returningStack = [...(userReturningValues ?? [])];
  const updateReturningStack = [...(updateReturningValues ?? [])];
  const insertedValues: unknown[] = [];
  let db: any;

  function chain(result: unknown, onValues?: (v: unknown) => void) {
    const p = () => Promise.resolve(result);
    const c: any = {
      from: () => c,
      where: () => c,
      limit: () => p(),
      values: (v: unknown) => { onValues?.(v); return c; },
      set: (v: unknown) => { onValues?.(v); return c; },
      returning: () => p(),
      onConflictDoNothing: () => c,
      then: (r: any, j: any) => p().then(r, j),
    };
    return c;
  }

  db = {
    insertedValues,
    select: () => chain(remainingSelects.shift() ?? []),
    insert: (table: unknown) => {
      if (table === creditLedger) {
        return chain([], (v) => insertedValues.push(v));
      }
      const val = returningStack.shift() ?? [];
      return chain(val, (v) => insertedValues.push(v));
    },
    update: () => chain(updateReturningStack.shift() ?? []),
    transaction: async (cb: (tx: any) => Promise<void>) => cb(db),
  };

  return db;
}

test('upsertUserWithFreeGrant returns existing user without creating', async () => {
  const existingUser = { id: 'existing-id', workosUserId: 'wos_1', email: 'existing@example.com' };
  const db = createFlexibleFakeDb([[existingUser]]);
  setDbForTests(db as any);

  const result = await upsertUserWithFreeGrant({
    workosUserId: 'wos_1',
    email: 'existing@example.com',
  });

  assert.deepEqual(result, existingUser);
  assert.strictEqual(db.insertedValues.length, 0);
});

test('upsertUserWithFreeGrant creates user and grants free credits', async () => {
  const createdUser = { id: 'new-id', workosUserId: 'wos_2', email: 'new@legitdomain.com' };
  const db = createFlexibleFakeDb([[], []], [[createdUser]]);
  setDbForTests(db as any);

  const result = await upsertUserWithFreeGrant({
    workosUserId: 'wos_2',
    email: 'new@legitdomain.com',
  });

  assert.deepEqual(result, createdUser);
  assert.strictEqual(db.insertedValues.length, 2);
  const creditValues = db.insertedValues[1] as Record<string, unknown>;
  assert.strictEqual(creditValues.userId, 'new-id');
  assert.strictEqual(creditValues.kind, 'free');
  assert.strictEqual(creditValues.reason, 'grant_free_monthly');
});

test('upsertUserWithFreeGrant links a new WorkOS identity to an existing verified email account', async () => {
  const existingUser = { id: 'existing-id', workosUserId: 'wos_old', email: 'existing@example.com' };
  const linkedUser = { ...existingUser, workosUserId: 'wos_new' };
  const db = createFlexibleFakeDb([[], [existingUser]], [], [[linkedUser]]);
  setDbForTests(db as any);

  const result = await upsertUserWithFreeGrant({
    workosUserId: 'wos_new',
    email: 'Existing@Example.com',
  });

  assert.deepEqual(result, linkedUser);
  assert.strictEqual(db.insertedValues.length, 0);
});

test('DisposableEmailError is constructible', () => {
  const err = new DisposableEmailError('test@mailinator.com');
  assert.ok(err instanceof Error);
  assert.strictEqual(err.name, 'DisposableEmailError');
  assert.ok(err.message.includes('test@mailinator.com'));
});
