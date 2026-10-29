import assert from 'node:assert/strict';
import { afterEach, beforeEach, test } from 'node:test';
import type { SQL } from 'drizzle-orm';
import { PgDialect } from 'drizzle-orm/pg-core';
import { appSettings } from '../db/schema';
import { type Database, withTestDb } from '../db/client';
import type { FreeSpendCapTransaction } from './free-spend-cap';
import {
  absorbedOverrunDailyCapCredits,
  absorbedOverrunSettingKey,
  absorbedSettlementCredits,
  DEFAULT_ABSORBED_OVERRUN_DAILY_CAP_CREDITS,
  overrunCapReached,
  recordAbsorbedOverrunCredits,
  setStaleAdmissionsHaltedForTests,
  staleAdmissionsHalted,
} from './pricing-overrun-cap';

const CAP_ENV = 'PRICING_OVERRUN_DAILY_CAP_CREDITS';
const ENV_KEYS = [CAP_ENV, 'DATABASE_URL'] as const;
const NOON = new Date('2026-10-29T12:00:00.000Z');

let savedEnv: Record<string, string | undefined>;

beforeEach(() => {
  savedEnv = Object.fromEntries(ENV_KEYS.map((key) => [key, process.env[key]]));
  process.env.DATABASE_URL = 'postgres://overrun-cap-test.invalid/nvm';
  setStaleAdmissionsHaltedForTests(null);
});

afterEach(() => {
  for (const key of ENV_KEYS) {
    if (savedEnv[key] === undefined) delete process.env[key];
    else process.env[key] = savedEnv[key];
  }
  setStaleAdmissionsHaltedForTests(null);
});

/** A settings-only database whose counter read returns `rows` or throws. */
function counterDb(read: () => Array<{ value: string }>): Database {
  const query = {
    from: () => query,
    where: () => query,
    limit: async () => read(),
  };
  return { select: () => query } as unknown as Database;
}

function haltedWithCounter(rows: Array<{ value: string }>, now = NOON) {
  return withTestDb(counterDb(() => rows), () => staleAdmissionsHalted(now));
}

/** Records every write the settlement transaction makes to the counter table. */
function recordingTransaction() {
  const writes: Array<{
    table: unknown;
    values: Record<string, unknown>;
    conflict: { target: unknown; set: Record<string, unknown> };
  }> = [];
  const tx = {
    insert: (table: unknown) => ({
      values: (values: Record<string, unknown>) => ({
        onConflictDoUpdate: async (conflict: { target: unknown; set: Record<string, unknown> }) => {
          writes.push({ table, values, conflict });
        },
      }),
    }),
  } as unknown as FreeSpendCapTransaction;
  return { tx, writes };
}

test('the daily cap defaults to 500 credits and accepts only a whole-number override', () => {
  delete process.env[CAP_ENV];
  assert.equal(absorbedOverrunDailyCapCredits(), DEFAULT_ABSORBED_OVERRUN_DAILY_CAP_CREDITS);
  assert.equal(DEFAULT_ABSORBED_OVERRUN_DAILY_CAP_CREDITS, 500);

  process.env[CAP_ENV] = '1200';
  assert.equal(absorbedOverrunDailyCapCredits(), 1200);
  process.env[CAP_ENV] = '0';
  assert.equal(absorbedOverrunDailyCapCredits(), 0, 'zero is a valid cap that halts every stale admission');

  for (const invalid of ['', '   ', '-1', '1.5', 'abc']) {
    process.env[CAP_ENV] = invalid;
    assert.equal(
      absorbedOverrunDailyCapCredits(),
      DEFAULT_ABSORBED_OVERRUN_DAILY_CAP_CREDITS,
      `"${invalid}" falls back to the default`,
    );
  }
});

test('the counter is keyed by the UTC day of the settlement, so it resets at UTC midnight', () => {
  assert.equal(
    absorbedOverrunSettingKey(new Date('2026-10-29T23:59:59.999Z')),
    'pricing_absorbed_overrun_credits:2026-10-29',
  );
  assert.equal(
    absorbedOverrunSettingKey(new Date('2026-10-30T00:00:00.000Z')),
    'pricing_absorbed_overrun_credits:2026-10-30',
  );
});

test('the cap is reached exactly at its credit count, and never below it', () => {
  assert.equal(overrunCapReached(499, 500), false);
  assert.equal(overrunCapReached(500, 500), true, 'equal to the cap is already reached');
  assert.equal(overrunCapReached(501, 500), true);
  assert.equal(overrunCapReached(0, 0), true, 'a zero cap halts before any overrun');
  assert.equal(overrunCapReached(0, 500), false);
});

test('a settlement absorbs exactly the list cost its debit did not collect', () => {
  // A 3-credit hold collects at most 3 credits, so a 5-credit cost leaves 2 uncollected.
  assert.equal(absorbedSettlementCredits(5, 3), 2, 'overrun above a fully collected hold');
  assert.equal(
    absorbedSettlementCredits(3, 0),
    3,
    'a debit limited by balance is exposure even without overrun',
  );
  assert.equal(
    absorbedSettlementCredits(5, 1),
    4,
    'overrun and a balance shortfall together are counted once, as the uncollected difference',
  );
  assert.equal(absorbedSettlementCredits(2, 2), 0, 'a fully collected settlement absorbs nothing');
  assert.equal(absorbedSettlementCredits(1, 3), 0, 'never negative when collection exceeds cost');
});

test('a settlement adds its absorbed credits in one additive upsert inside its transaction', async () => {
  const { tx, writes } = recordingTransaction();
  await recordAbsorbedOverrunCredits(tx, 125, NOON);

  assert.equal(writes.length, 1);
  const [write] = writes;
  assert.equal(write.table, appSettings);
  assert.equal(write.values.key, absorbedOverrunSettingKey(NOON));
  assert.equal(write.values.value, '125');
  assert.equal(write.values.updatedAt, NOON);
  assert.equal(write.conflict.target, appSettings.key);

  const rendered = new PgDialect().sqlToQuery(write.conflict.set.value as SQL);
  assert.match(
    rendered.sql,
    /::bigint \+ \$1\)::text/,
    'the sum is computed by the database, so concurrent settlements never overwrite each other',
  );
  assert.deepEqual(rendered.params, [125]);
});

test('settlements that absorb nothing, or a value that is not a whole credit count, write nothing', async () => {
  for (const absorbed of [0, -3, 1.5, Number.NaN, Number.POSITIVE_INFINITY]) {
    const { tx, writes } = recordingTransaction();
    await recordAbsorbedOverrunCredits(tx, absorbed, NOON);
    assert.equal(writes.length, 0, `absorbed=${absorbed}`);
  }
});

test('stale admissions halt exactly when the day counter reaches the cap', async () => {
  process.env[CAP_ENV] = '500';
  assert.equal(await haltedWithCounter([{ value: '499' }]), false, 'one credit under the cap admits');
  assert.equal(await haltedWithCounter([{ value: '500' }]), true, 'at the cap refuses');
  assert.equal(await haltedWithCounter([{ value: '501' }]), true);
  assert.equal(await haltedWithCounter([]), false, 'no counter row means nothing absorbed today');
});

test('a zero cap halts stale admissions even before anything is absorbed', async () => {
  process.env[CAP_ENV] = '0';
  assert.equal(await haltedWithCounter([]), true);
});

test('each halt check reads the counter again, so an overrun settled mid-day halts the next admission', async () => {
  process.env[CAP_ENV] = '500';
  let absorbed = '0';
  const db = counterDb(() => [{ value: absorbed }]);
  assert.equal(await withTestDb(db, () => staleAdmissionsHalted(NOON)), false);
  absorbed = '500';
  assert.equal(await withTestDb(db, () => staleAdmissionsHalted(NOON)), true);
});

test('an unreadable or corrupt counter refuses stale admissions rather than assuming the bound holds', async () => {
  process.env[CAP_ENV] = '500';
  assert.equal(
    await withTestDb(
      counterDb(() => {
        throw new Error('connection reset');
      }),
      () => staleAdmissionsHalted(NOON),
    ),
    true,
  );
  assert.equal(await haltedWithCounter([{ value: 'abc' }]), true, 'NaN must not admit');
  assert.equal(await haltedWithCounter([{ value: '' }]), true, 'an empty value must not admit');
  assert.equal(await haltedWithCounter([{ value: '-5' }]), true);
});

test('without a database nothing can settle, so nothing is absorbed and stale admissions stay open', async () => {
  delete process.env.DATABASE_URL;
  process.env[CAP_ENV] = '0';
  assert.equal(await staleAdmissionsHalted(NOON), false);
});

test('a test override decides the halt without reading the counter', async () => {
  process.env[CAP_ENV] = '0';
  const failing = counterDb(() => {
    throw new Error('must not be read');
  });
  setStaleAdmissionsHaltedForTests(false);
  assert.equal(await withTestDb(failing, () => staleAdmissionsHalted(NOON)), false);
  setStaleAdmissionsHaltedForTests(true);
  assert.equal(await withTestDb(failing, () => staleAdmissionsHalted(NOON)), true);
  setStaleAdmissionsHaltedForTests(null);
  assert.equal(await withTestDb(counterDb(() => [{ value: '0' }]), () => staleAdmissionsHalted(NOON)), true);
});
