import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { resetDbForTests, setDbForTests } from '../db/client';
import {
  auditLog,
  financialReconciliationJobs,
  financialReconciliationPages,
  stripeBalanceTransactions,
} from '../db/schema';
import {
  beginFinancialReconciliation,
  classifyStripeBalanceTransaction,
  normalizeStripeBalanceTransaction,
  persistFinancialReconciliationPage,
  readStripeBalanceTransactionPage,
  reconciliationCoverageComplete,
  resolveUtcReconciliationRange,
} from './financial-reconciliation';

type ReconciliationFakeState = {
  job: Record<string, any> | null;
  balanceTransactions: Map<string, Record<string, any>>;
  pages: Record<string, any>[];
  audits: Record<string, any>[];
  failAudit: boolean;
};

function createReconciliationFakeDb() {
  const state: ReconciliationFakeState = {
    job: null,
    balanceTransactions: new Map(),
    pages: [],
    audits: [],
    failAudit: false,
  };

  function createSelectChain() {
    let table: unknown;
    const chain: Record<string, any> = {
      from(value: unknown) {
        table = value;
        return chain;
      },
      where() {
        return chain;
      },
      limit() {
        return resolve();
      },
      then(
        onFulfilled: (value: unknown[]) => unknown,
        onRejected?: (reason: unknown) => unknown,
      ) {
        return resolve().then(onFulfilled, onRejected);
      },
    };
    chain.for = () => chain;
    function resolve() {
      if (table === financialReconciliationJobs)
        return Promise.resolve(state.job ? [structuredClone(state.job)] : []);
      if (table === stripeBalanceTransactions)
        return Promise.resolve(
          [...state.balanceTransactions.values()].map((row) =>
            structuredClone(row),
          ),
        );
      return Promise.resolve([]);
    }
    return chain;
  }

  function createInsertChain(table: unknown) {
    let values: any;
    let conflictUpdate = false;
    const chain: Record<string, any> = {
      values(next: any) {
        values = next;
        return chain;
      },
      onConflictDoNothing() {
        return chain;
      },
      onConflictDoUpdate() {
        conflictUpdate = true;
        return chain;
      },
      returning() {
        if (table === financialReconciliationJobs) {
          state.job = {
            id: '00000000-0000-4000-8000-000000000001',
            state: 'pending',
            cursor: null,
            leaseVersion: 0,
            leaseExpiresAt: null,
            importedCount: 0,
            updatedCount: 0,
            unknownCount: 0,
            completedAt: null,
            createdAt: values.createdAt ?? new Date(),
            updatedAt: values.updatedAt ?? new Date(),
            ...values,
          };
          return Promise.resolve([structuredClone(state.job)]);
        }
        return Promise.resolve([]);
      },
      then(
        onFulfilled: (value: unknown[]) => unknown,
        onRejected?: (reason: unknown) => unknown,
      ) {
        try {
          if (table === stripeBalanceTransactions) {
            for (const row of values as Record<string, any>[]) {
              if (conflictUpdate && state.balanceTransactions.has(row.id)) {
                state.balanceTransactions.set(row.id, {
                  ...state.balanceTransactions.get(row.id),
                  ...row,
                });
              } else {
                state.balanceTransactions.set(row.id, row);
              }
            }
          } else if (table === financialReconciliationPages) {
            state.pages.push(values);
          } else if (table === auditLog) {
            if (state.failAudit) throw new Error('audit insert failed');
            state.audits.push(values);
          }
          return Promise.resolve([]).then(onFulfilled, onRejected);
        } catch (error) {
          return Promise.reject(error).then(onFulfilled, onRejected);
        }
      },
    };
    return chain;
  }

  function createUpdateChain(table: unknown) {
    let values: Record<string, any>;
    const chain: Record<string, any> = {
      set(next: Record<string, any>) {
        values = next;
        return chain;
      },
      where() {
        if (table === financialReconciliationJobs && state.job)
          state.job = { ...state.job, ...values };
        return chain;
      },
      returning() {
        return Promise.resolve(state.job ? [structuredClone(state.job)] : []);
      },
    };
    return chain;
  }

  const db = {
    select: () => createSelectChain(),
    insert: (table: unknown) => createInsertChain(table),
    update: (table: unknown) => createUpdateChain(table),
    transaction: async (callback: (tx: any) => Promise<unknown>) => {
      const snapshot = structuredClone({
        job: state.job,
        balanceTransactions: [...state.balanceTransactions.entries()],
        pages: state.pages,
        audits: state.audits,
      });
      try {
        return await callback(db);
      } catch (error) {
        state.job = snapshot.job;
        state.balanceTransactions = new Map(snapshot.balanceTransactions);
        state.pages = snapshot.pages;
        state.audits = snapshot.audits;
        throw error;
      }
    },
  };
  return { db, state };
}

afterEach(() => resetDbForTests());

test('reporting-category classifier preserves cash categories and leaves unknown values visible', () => {
  assert.equal(classifyStripeBalanceTransaction('charge'), 'charge');
  assert.equal(
    classifyStripeBalanceTransaction('partial_capture_reversal'),
    'partial_capture_reversal',
  );
  assert.equal(
    classifyStripeBalanceTransaction('refund_failure'),
    'refund_failure',
  );
  assert.equal(
    classifyStripeBalanceTransaction('dispute_reversal'),
    'dispute_reversal',
  );
  assert.equal(classifyStripeBalanceTransaction('dispute'), 'dispute');
  assert.equal(classifyStripeBalanceTransaction('tax'), 'tax');
  assert.equal(classifyStripeBalanceTransaction('payout'), 'payout');
  assert.equal(classifyStripeBalanceTransaction('transfer'), 'transfer');
  assert.equal(classifyStripeBalanceTransaction('reserve_release'), 'reserve');
  assert.equal(
    classifyStripeBalanceTransaction('future_category'),
    'unclassified',
  );
  assert.equal(classifyStripeBalanceTransaction(null), 'unclassified');
});

test('balance transaction normalization preserves source facts and marks unavailable fee/net fields unknown', () => {
  const normalized = normalizeStripeBalanceTransaction({
    id: 'txn_123',
    source: { id: 'ch_123' },
    amount: 1200,
    fee: null,
    net: 1190,
    currency: 'usd',
    type: 'charge',
    reporting_category: 'charge',
    created: 1_800_000_000,
    available_on: 1_800_086_400,
    status: 'pending',
  });

  assert.ok(normalized);
  assert.equal(normalized.id, 'txn_123');
  assert.equal(normalized.sourceObjectId, 'ch_123');
  assert.equal(normalized.currency, 'USD');
  assert.equal(normalized.amount, 1200);
  assert.equal(normalized.fee, null);
  assert.equal(normalized.net, 1190);
  assert.equal(normalized.feeKnown, false);
  assert.deepEqual(normalized.missingFields, ['fee']);
  assert.equal(normalized.classifiedAs, 'charge');
  assert.equal(normalized.status, 'pending');
});

test('normalization retains unknown categories and unusable required fields instead of dropping the row', () => {
  const normalized = normalizeStripeBalanceTransaction({
    id: 'txn_future',
    amount: 'not-an-integer',
    fee: 0,
    net: 0,
    currency: 'not-a-currency',
    type: 'future_type',
    reporting_category: 'future_reporting_category',
    created: 'invalid',
  });

  assert.ok(normalized);
  assert.equal(normalized.classifiedAs, 'unclassified');
  assert.equal(normalized.type, 'future_type');
  assert.equal(normalized.amount, null);
  assert.equal(normalized.currency, null);
  assert.equal(normalized.createdAtStripe, null);
  assert.deepEqual(normalized.missingFields, [
    'amount',
    'currency',
    'created',
    'available_on',
    'status',
  ]);
});

test('cash fixtures preserve signed amounts and currencies without conversion', () => {
  const rows = [
    {
      id: 'txn_charge',
      amount: 1000,
      fee: 35,
      net: 965,
      currency: 'usd',
      type: 'charge',
      reporting_category: 'charge',
    },
    {
      id: 'txn_refund_partial',
      amount: -400,
      fee: 0,
      net: -400,
      currency: 'usd',
      type: 'refund',
      reporting_category: 'refund',
    },
    {
      id: 'txn_refund_full',
      amount: -600,
      fee: 0,
      net: -600,
      currency: 'usd',
      type: 'refund',
      reporting_category: 'refund',
    },
    {
      id: 'txn_capture_reversal',
      amount: -200,
      fee: 0,
      net: -200,
      currency: 'usd',
      type: 'payment_reversal',
      reporting_category: 'partial_capture_reversal',
    },
    {
      id: 'txn_dispute',
      amount: -1000,
      fee: 0,
      net: -1000,
      currency: 'usd',
      type: 'charge',
      reporting_category: 'dispute',
    },
    {
      id: 'txn_eur',
      amount: 500,
      fee: 20,
      net: 480,
      currency: 'eur',
      type: 'charge',
      reporting_category: 'charge',
    },
    {
      id: 'txn_payout',
      amount: -900,
      fee: 0,
      net: -900,
      currency: 'usd',
      type: 'payout',
      reporting_category: 'payout',
    },
  ];
  const normalized = rows.map((row) => normalizeStripeBalanceTransaction(row));

  assert.ok(normalized.every(Boolean));
  assert.deepEqual(
    normalized.map((row) => row?.currency),
    ['USD', 'USD', 'USD', 'USD', 'USD', 'EUR', 'USD'],
  );
  assert.deepEqual(
    normalized.map((row) => row?.amount),
    [1000, -400, -600, -200, -1000, 500, -900],
  );
  assert.deepEqual(
    normalized.map((row) => row?.classifiedAs),
    [
      'charge',
      'refund',
      'refund',
      'partial_capture_reversal',
      'dispute',
      'charge',
      'payout',
    ],
  );
});

test('UTC range resolver defaults to 90 days and rejects invalid, future, reversed, or over-year ranges', () => {
  const now = new Date('2026-10-07T12:30:45.900Z');
  const defaultRange = resolveUtcReconciliationRange(undefined, undefined, now);
  assert.ok(defaultRange);
  assert.equal(defaultRange.to.toISOString(), '2026-10-07T12:30:45.000Z');
  assert.equal(
    defaultRange.to.getTime() - defaultRange.from.getTime(),
    90 * 24 * 60 * 60 * 1000,
  );

  assert.equal(
    resolveUtcReconciliationRange('2026-02-30', '2026-03-01', now),
    null,
  );
  assert.equal(
    resolveUtcReconciliationRange('2026-10-08', '2026-10-09', now),
    null,
  );
  assert.equal(
    resolveUtcReconciliationRange('2026-10-01', '2026-09-30', now),
    null,
  );
  assert.equal(
    resolveUtcReconciliationRange('2025-10-01', '2026-10-02', now),
    null,
  );
  assert.equal(
    resolveUtcReconciliationRange(
      '2026-10-01T00:00:00+01:00',
      '2026-10-02T00:00:00Z',
      now,
    ),
    null,
  );
});

test('coverage requires a gap-free union of completed intervals', () => {
  const selected = {
    from: new Date('2026-01-01T00:00:00Z'),
    to: new Date('2026-01-04T00:00:00Z'),
  };
  assert.equal(
    reconciliationCoverageComplete(
      [
        {
          from: new Date('2025-12-31T00:00:00Z'),
          to: new Date('2026-01-02T00:00:00Z'),
        },
        {
          from: new Date('2026-01-02T00:00:00Z'),
          to: new Date('2026-01-05T00:00:00Z'),
        },
      ],
      selected,
    ),
    true,
  );
  assert.equal(
    reconciliationCoverageComplete(
      [
        {
          from: new Date('2026-01-01T00:00:00Z'),
          to: new Date('2026-01-02T00:00:00Z'),
        },
        {
          from: new Date('2026-01-03T00:00:00Z'),
          to: new Date('2026-01-04T00:00:00Z'),
        },
      ],
      selected,
    ),
    false,
  );
});

test('Stripe reconciliation issues one bounded balance-transaction GET with the durable cursor', async () => {
  const calls: Array<{ params: unknown; options: unknown }> = [];
  const stripe = {
    balanceTransactions: {
      list: async (params: unknown, options: unknown) => {
        calls.push({ params, options });
        return { data: [], has_more: false };
      },
    },
  } as any;

  await readStripeBalanceTransactionPage(stripe, {
    range: {
      from: new Date('2026-01-01T00:00:00Z'),
      to: new Date('2026-02-01T00:00:00Z'),
    },
    cursor: 'txn_previous',
  });

  assert.deepEqual(calls, [
    {
      params: {
        created: { gte: 1_767_225_600, lt: 1_769_904_000 },
        limit: 100,
        starting_after: 'txn_previous',
      },
      options: { timeout: 2000, maxNetworkRetries: 0 },
    },
  ]);
});

test('reconciliation leases reject stale pages, advance one cursor atomically, and upsert repeated facts', async () => {
  const { db, state } = createReconciliationFakeDb();
  setDbForTests(db as any);
  const base = new Date('2026-10-01T12:00:00Z');
  const range = {
    from: new Date('2026-10-01T00:00:00Z'),
    to: new Date('2026-10-02T00:00:00Z'),
  };
  const begun = await beginFinancialReconciliation({
    actorUserId: 'admin-id',
    range,
    deadlineAt: Date.now() + 10_000,
    now: base,
  });
  assert.equal(begun?.status, 'claimed');
  if (!begun || begun.status !== 'claimed') return;

  const transaction = normalizeStripeBalanceTransaction({
    id: 'txn_replay',
    source: 'ch_unlinked',
    amount: 1000,
    fee: 30,
    net: 970,
    currency: 'usd',
    type: 'charge',
    reporting_category: 'charge',
    created: Math.floor(base.getTime() / 1000),
    available_on: Math.floor(base.getTime() / 1000) + 86_400,
    status: 'pending',
  });
  assert.ok(transaction);

  const expiredPage = await persistFinancialReconciliationPage({
    actorUserId: 'admin-id',
    jobId: begun.job.id,
    leaseVersion: begun.leaseVersion,
    deadlineAt: Date.now() + 10_000,
    cursorBefore: null,
    cursorAfter: 'txn_replay',
    hasMore: true,
    transactions: [transaction],
    now: new Date(base.getTime() + 31_000),
  });
  assert.deepEqual(expiredPage, { status: 'stale' });
  assert.equal(state.balanceTransactions.size, 0);
  assert.equal(state.job?.cursor, null);

  const resumed = await beginFinancialReconciliation({
    actorUserId: 'admin-id',
    jobId: begun.job.id,
    deadlineAt: Date.now() + 10_000,
    now: new Date(base.getTime() + 31_000),
  });
  assert.equal(resumed?.status, 'claimed');
  if (!resumed || resumed.status !== 'claimed') return;

  const firstPage = await persistFinancialReconciliationPage({
    actorUserId: 'admin-id',
    jobId: begun.job.id,
    leaseVersion: resumed.leaseVersion,
    deadlineAt: Date.now() + 10_000,
    cursorBefore: null,
    cursorAfter: 'txn_replay',
    hasMore: true,
    transactions: [transaction, transaction],
    now: new Date(base.getTime() + 32_000),
  });
  assert.equal(firstPage.status, 'saved');
  if (firstPage.status !== 'saved') return;
  assert.equal(firstPage.importedCount, 1);
  assert.equal(firstPage.updatedCount, 0);
  assert.equal(state.balanceTransactions.size, 1);
  assert.equal(firstPage.job.cursor, 'txn_replay');
  assert.equal(firstPage.job.state, 'pending');
  assert.equal(state.pages.length, 1);

  const nextLease = await beginFinancialReconciliation({
    actorUserId: 'admin-id',
    jobId: begun.job.id,
    deadlineAt: Date.now() + 10_000,
    now: new Date(base.getTime() + 33_000),
  });
  assert.equal(nextLease?.status, 'claimed');
  if (!nextLease || nextLease.status !== 'claimed') return;

  const repeatedPage = await persistFinancialReconciliationPage({
    actorUserId: 'admin-id',
    jobId: begun.job.id,
    leaseVersion: nextLease.leaseVersion,
    deadlineAt: Date.now() + 10_000,
    cursorBefore: 'txn_replay',
    cursorAfter: null,
    hasMore: false,
    transactions: [transaction],
    now: new Date(base.getTime() + 34_000),
  });
  assert.equal(repeatedPage.status, 'saved');
  if (repeatedPage.status !== 'saved') return;
  assert.equal(repeatedPage.importedCount, 0);
  assert.equal(repeatedPage.updatedCount, 1);
  assert.equal(repeatedPage.job.state, 'complete');
  assert.equal(state.balanceTransactions.size, 1);
  assert.equal(state.pages.length, 2);
});

test('reconciliation audit failure rolls back imported facts, page activity, and cursor advancement', async () => {
  const { db, state } = createReconciliationFakeDb();
  setDbForTests(db as any);
  const now = new Date();
  const begun = await beginFinancialReconciliation({
    actorUserId: 'admin-id',
    range: {
      from: new Date(now.getTime() - 60_000),
      to: new Date(Math.floor(now.getTime() / 1000) * 1000),
    },
    deadlineAt: Date.now() + 10_000,
    now,
  });
  assert.equal(begun?.status, 'claimed');
  if (!begun || begun.status !== 'claimed') return;
  state.failAudit = true;
  const transaction = normalizeStripeBalanceTransaction({
    id: 'txn_rollback',
    amount: 100,
    fee: 1,
    net: 99,
    currency: 'usd',
    type: 'charge',
    reporting_category: 'charge',
    created: Math.floor(now.getTime() / 1000) - 10,
  });
  assert.ok(transaction);

  await assert.rejects(
    persistFinancialReconciliationPage({
      actorUserId: 'admin-id',
      jobId: begun.job.id,
      leaseVersion: begun.leaseVersion,
      deadlineAt: Date.now() + 10_000,
      cursorBefore: null,
      cursorAfter: null,
      hasMore: false,
      transactions: [transaction],
    }),
    /audit insert failed/,
  );
  assert.equal(state.balanceTransactions.size, 0);
  assert.equal(state.pages.length, 0);
  assert.equal(state.job?.state, 'processing');
  assert.equal(state.job?.cursor, null);
  assert.equal(
    state.audits.length,
    1,
    'the durable job-start audit is retained',
  );
});
