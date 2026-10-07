import { and, eq, inArray, or, sql } from 'drizzle-orm';
import type Stripe from 'stripe';
import { db } from '../db/client';
import {
  billingOrders,
  financialReconciliationJobs,
  financialReconciliationPages,
  stripeBalanceTransactions,
} from '../db/schema';
import { recordAuditStrict } from './audit';

const DAY_MS = 24 * 60 * 60 * 1000;
const DEFAULT_RANGE_DAYS = 90;
const MAX_RANGE_DAYS = 365;
const LEASE_MS = 30_000;

export type FinancialCategory =
  | 'charge'
  | 'partial_capture_reversal'
  | 'refund'
  | 'refund_failure'
  | 'dispute'
  | 'dispute_reversal'
  | 'fee'
  | 'tax'
  | 'payout'
  | 'transfer'
  | 'reserve'
  | 'charge_failure'
  | 'other_adjustment'
  | 'unclassified';

export type ReconciliationRange = { from: Date; to: Date };

export type NormalizedBalanceTransaction = {
  id: string;
  sourceObjectId: string | null;
  currency: string | null;
  amount: number | null;
  fee: number | null;
  net: number | null;
  type: string | null;
  reportingCategory: string | null;
  classifiedAs: FinancialCategory;
  classifierVersion: number;
  createdAtStripe: Date | null;
  availableOn: Date | null;
  status: string | null;
  feeKnown: boolean;
  missingFields: string[];
};

type ReconciliationJob = typeof financialReconciliationJobs.$inferSelect;

type ClaimedJob = {
  status: 'claimed';
  job: ReconciliationJob;
  leaseVersion: number;
};

export type BeginReconciliationResult =
  | ClaimedJob
  | { status: 'processing' | 'complete' | 'deferred'; job: ReconciliationJob };

export function classifyStripeBalanceTransaction(
  reportingCategory: string | null,
): FinancialCategory {
  switch (reportingCategory) {
    case 'charge':
    case 'partial_capture_reversal':
    case 'refund':
    case 'refund_failure':
    case 'dispute':
    case 'dispute_reversal':
    case 'fee':
    case 'tax':
    case 'charge_failure':
    case 'other_adjustment':
      return reportingCategory;
    case 'payout':
      return 'payout';
    case 'transfer':
      return 'transfer';
    case 'reserve_transaction':
    case 'reserve_hold':
    case 'reserve_release':
    case 'reserved_funds':
      return 'reserve';
    default:
      return 'unclassified';
  }
}

function safeInteger(value: unknown): number | null {
  return typeof value === 'number' && Number.isSafeInteger(value)
    ? value
    : null;
}

function stripeTimestamp(value: unknown): Date | null {
  const seconds = safeInteger(value);
  if (seconds === null) return null;
  const date = new Date(seconds * 1000);
  return Number.isNaN(date.getTime()) ? null : date;
}

function sourceId(value: Stripe.BalanceTransaction['source']): string | null {
  if (typeof value === 'string') return value;
  if (value && typeof value.id === 'string') return value.id;
  return null;
}

export function normalizeStripeBalanceTransaction(
  transaction: Stripe.BalanceTransaction | Record<string, unknown>,
): NormalizedBalanceTransaction | null {
  const value = transaction as Record<string, unknown>;
  if (typeof value.id !== 'string' || value.id.trim() === '') return null;

  const amount = safeInteger(value.amount);
  const fee = safeInteger(value.fee);
  const net = safeInteger(value.net);
  const rawCurrency =
    typeof value.currency === 'string'
      ? value.currency.trim().toUpperCase()
      : '';
  const currency = /^[A-Z]{3}$/.test(rawCurrency) ? rawCurrency : null;
  const reportingCategory =
    typeof value.reporting_category === 'string' &&
    value.reporting_category.trim() !== ''
      ? value.reporting_category
      : null;
  const rawType =
    typeof value.type === 'string' && value.type.trim() !== ''
      ? value.type
      : null;
  const rawSource = value.source as
    | Stripe.BalanceTransaction['source']
    | undefined;
  const createdAtStripe = stripeTimestamp(value.created);
  const availableOn = stripeTimestamp(value.available_on);
  const missingFields: string[] = [];

  if (amount === null) missingFields.push('amount');
  if (fee === null) missingFields.push('fee');
  if (net === null) missingFields.push('net');
  if (currency === null) missingFields.push('currency');
  if (!reportingCategory) missingFields.push('reporting_category');
  if (!rawType) missingFields.push('type');
  if (createdAtStripe === null) missingFields.push('created');
  if (availableOn === null) missingFields.push('available_on');
  if (typeof value.status !== 'string') missingFields.push('status');

  return {
    id: value.id,
    sourceObjectId: sourceId(rawSource ?? null),
    currency,
    amount,
    fee,
    net,
    type: rawType,
    reportingCategory,
    classifiedAs: classifyStripeBalanceTransaction(reportingCategory),
    classifierVersion: 1,
    createdAtStripe,
    availableOn,
    status: typeof value.status === 'string' ? value.status : null,
    feeKnown: fee !== null,
    missingFields,
  };
}

export function readStripeBalanceTransactionPage(
  stripe: Pick<Stripe, 'balanceTransactions'>,
  input: { range: ReconciliationRange; cursor: string | null },
) {
  return stripe.balanceTransactions.list(
    {
      created: {
        gte: Math.floor(input.range.from.getTime() / 1000),
        lt: Math.floor(input.range.to.getTime() / 1000),
      },
      limit: 100,
      ...(input.cursor ? { starting_after: input.cursor } : {}),
    },
    { timeout: 2000, maxNetworkRetries: 0 },
  );
}

function parseUtcBound(value: string): Date | null {
  const dateOnly = /^\d{4}-\d{2}-\d{2}$/.test(value);
  const utcTimestamp =
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?Z$/.test(value);
  if (!dateOnly && !utcTimestamp) return null;
  const date = new Date(dateOnly ? `${value}T00:00:00.000Z` : value);
  if (
    Number.isNaN(date.getTime()) ||
    date.getUTCMilliseconds() !== 0 ||
    date.toISOString().slice(0, 10) !== value.slice(0, 10)
  ) {
    return null;
  }
  return date;
}

export function resolveUtcReconciliationRange(
  fromInput: string | undefined,
  toInput: string | undefined,
  now = new Date(),
): ReconciliationRange | null {
  const from = fromInput ? parseUtcBound(fromInput) : null;
  const to = toInput ? parseUtcBound(toInput) : null;
  if ((fromInput && !from) || (toInput && !to)) return null;

  const effectiveNow = new Date(Math.floor(now.getTime() / 1000) * 1000);
  const end = to ?? effectiveNow;
  const start = from ?? new Date(end.getTime() - DEFAULT_RANGE_DAYS * DAY_MS);
  const duration = end.getTime() - start.getTime();
  if (
    duration <= 0 ||
    duration > MAX_RANGE_DAYS * DAY_MS ||
    end.getTime() > now.getTime()
  )
    return null;
  return { from: start, to: end };
}

export async function beginFinancialReconciliation(input: {
  actorUserId: string;
  jobId?: string;
  range?: ReconciliationRange;
  deadlineAt: number;
  now?: Date;
}): Promise<BeginReconciliationResult | null> {
  const now = input.now ?? new Date();
  return db.transaction(async (tx) => {
    let job: ReconciliationJob | undefined;
    let created = false;

    if (input.jobId) {
      [job] = await tx
        .select()
        .from(financialReconciliationJobs)
        .where(eq(financialReconciliationJobs.id, input.jobId))
        .for('update');
      if (!job) return null;
      if (
        input.range &&
        (job.rangeFrom.getTime() !== input.range.from.getTime() ||
          job.rangeTo.getTime() !== input.range.to.getTime())
      ) {
        return null;
      }
    } else {
      if (!input.range)
        throw new Error(
          'A range is required to create a financial reconciliation job',
        );
      [job] = await tx
        .insert(financialReconciliationJobs)
        .values({
          requestedBy: input.actorUserId,
          rangeFrom: input.range.from,
          rangeTo: input.range.to,
        })
        .onConflictDoNothing()
        .returning();
      created = Boolean(job);
      if (!job) {
        [job] = await tx
          .select()
          .from(financialReconciliationJobs)
          .where(
            and(
              eq(financialReconciliationJobs.rangeFrom, input.range.from),
              eq(financialReconciliationJobs.rangeTo, input.range.to),
              inArray(financialReconciliationJobs.state, [
                'pending',
                'processing',
              ]),
            ),
          )
          .for('update');
      }
      if (!job)
        throw new Error(
          'Could not create or resume the financial reconciliation job',
        );
    }

    await recordAuditStrict(tx, {
      actorUserId: input.actorUserId,
      action: 'financials.reconciliation_started',
      targetType: 'financial_reconciliation_job',
      targetId: job.id,
      meta: {
        rangeFrom: job.rangeFrom.toISOString(),
        rangeTo: job.rangeTo.toISOString(),
        created,
      },
    });

    if (job.state === 'complete') return { status: 'complete', job };
    if (job.leaseExpiresAt && job.leaseExpiresAt.getTime() > now.getTime())
      return { status: 'processing', job };
    if (input.deadlineAt - Date.now() < 7000)
      return { status: 'deferred', job };

    const leaseVersion = job.leaseVersion + 1;
    [job] = await tx
      .update(financialReconciliationJobs)
      .set({
        state: 'processing',
        leaseVersion,
        leaseExpiresAt: new Date(now.getTime() + LEASE_MS),
        updatedAt: now,
      })
      .where(eq(financialReconciliationJobs.id, job.id))
      .returning();

    return { status: 'claimed', job, leaseVersion };
  });
}

export async function persistFinancialReconciliationPage(input: {
  actorUserId: string;
  jobId: string;
  leaseVersion: number;
  deadlineAt: number;
  cursorBefore: string | null;
  cursorAfter: string | null;
  hasMore: boolean;
  transactions: NormalizedBalanceTransaction[];
  now?: Date;
}): Promise<
  | {
      status: 'saved';
      job: ReconciliationJob;
      importedCount: number;
      updatedCount: number;
      unknownCount: number;
    }
  | { status: 'stale' }
> {
  if (input.hasMore && !input.cursorAfter)
    throw new Error('Stripe returned has_more without a continuation cursor');
  const now = input.now ?? new Date();

  return db.transaction(async (tx) => {
    const [job] = await tx
      .select()
      .from(financialReconciliationJobs)
      .where(eq(financialReconciliationJobs.id, input.jobId))
      .for('update');
    if (
      !job ||
      job.state !== 'processing' ||
      job.leaseVersion !== input.leaseVersion ||
      !job.leaseExpiresAt ||
      job.leaseExpiresAt.getTime() <= now.getTime() ||
      job.cursor !== input.cursorBefore
    ) {
      return { status: 'stale' };
    }

    const sourceIds = [
      ...new Set(
        input.transactions
          .map((transaction) => transaction.sourceObjectId)
          .filter((id): id is string => Boolean(id)),
      ),
    ];
    const orderMatches = sourceIds.length
      ? await tx
          .select({
            id: billingOrders.id,
            userId: billingOrders.userId,
            sessionId: billingOrders.sessionId,
            paymentIntentId: billingOrders.paymentIntentId,
          })
          .from(billingOrders)
          .where(
            or(
              inArray(billingOrders.sessionId, sourceIds),
              inArray(billingOrders.paymentIntentId, sourceIds),
            ),
          )
      : [];
    const ordersBySource = new Map<string, { id: string; userId: string }>();
    for (const order of orderMatches) {
      if (order.sessionId)
        ordersBySource.set(order.sessionId, {
          id: order.id,
          userId: order.userId,
        });
      if (order.paymentIntentId)
        ordersBySource.set(order.paymentIntentId, {
          id: order.id,
          userId: order.userId,
        });
    }

    const rows = input.transactions.map((transaction) => {
      const linkedOrder = transaction.sourceObjectId
        ? ordersBySource.get(transaction.sourceObjectId)
        : undefined;
      return {
        ...transaction,
        orderId: linkedOrder?.id ?? null,
        userId: linkedOrder?.userId ?? null,
        updatedAt: now,
      };
    });
    const uniqueRows = [...new Map(rows.map((row) => [row.id, row])).values()];
    const rowIds = uniqueRows.map((row) => row.id);
    const existingIds = rowIds.length
      ? await tx
          .select({ id: stripeBalanceTransactions.id })
          .from(stripeBalanceTransactions)
          .where(inArray(stripeBalanceTransactions.id, rowIds))
      : [];
    const existingIdSet = new Set(existingIds.map((row) => row.id));
    const importedCount = uniqueRows.length - existingIdSet.size;
    const updatedCount = existingIdSet.size;
    const unknownCount = uniqueRows.filter(
      (row) =>
        row.classifiedAs === 'unclassified' || row.missingFields.length > 0,
    ).length;

    if (input.deadlineAt - Date.now() < 3000) {
      throw new Error(
        'Financial reconciliation page exceeded its invocation budget before commit',
      );
    }

    if (uniqueRows.length) {
      await tx
        .insert(stripeBalanceTransactions)
        .values(uniqueRows)
        .onConflictDoUpdate({
          target: stripeBalanceTransactions.id,
          set: {
            sourceObjectId: sql`excluded.source_object_id`,
            orderId: sql`coalesce(excluded.order_id, ${stripeBalanceTransactions.orderId})`,
            userId: sql`coalesce(excluded.user_id, ${stripeBalanceTransactions.userId})`,
            currency: sql`excluded.currency`,
            amount: sql`excluded.amount`,
            fee: sql`excluded.fee`,
            net: sql`excluded.net`,
            type: sql`excluded.type`,
            reportingCategory: sql`excluded.reporting_category`,
            classifiedAs: sql`excluded.classified_as`,
            classifierVersion: sql`excluded.classifier_version`,
            createdAtStripe: sql`excluded.created_at_stripe`,
            availableOn: sql`excluded.available_on`,
            status: sql`excluded.status`,
            feeKnown: sql`excluded.fee_known`,
            missingFields: sql`excluded.missing_fields`,
            updatedAt: sql`excluded.updated_at`,
          },
        });
    }

    await tx.insert(financialReconciliationPages).values({
      jobId: job.id,
      leaseVersion: input.leaseVersion,
      cursorBefore: input.cursorBefore,
      cursorAfter: input.cursorAfter,
      hasMore: input.hasMore,
      importedCount,
      updatedCount,
      unknownCount,
      createdAt: now,
    });

    const [updatedJob] = await tx
      .update(financialReconciliationJobs)
      .set({
        cursor: input.hasMore ? input.cursorAfter : null,
        state: input.hasMore ? 'pending' : 'complete',
        leaseExpiresAt: null,
        importedCount: job.importedCount + importedCount,
        updatedCount: job.updatedCount + updatedCount,
        unknownCount: job.unknownCount + unknownCount,
        updatedAt: now,
        completedAt: input.hasMore ? null : now,
      })
      .where(eq(financialReconciliationJobs.id, job.id))
      .returning();
    if (!updatedJob)
      throw new Error(
        'Financial reconciliation job disappeared before page commit',
      );

    await recordAuditStrict(tx, {
      actorUserId: input.actorUserId,
      action: 'financials.reconciliation_page_imported',
      targetType: 'financial_reconciliation_job',
      targetId: job.id,
      meta: {
        leaseVersion: input.leaseVersion,
        importedCount,
        updatedCount,
        unknownCount,
        hasMore: input.hasMore,
      },
    });

    if (input.deadlineAt - Date.now() < 250) {
      throw new Error(
        'Financial reconciliation transaction exceeded its invocation budget',
      );
    }

    return {
      status: 'saved',
      job: updatedJob,
      importedCount,
      updatedCount,
      unknownCount,
    };
  });
}

export function reconciliationCoverageComplete(
  ranges: Array<{ from: Date; to: Date }>,
  selected: ReconciliationRange,
): boolean {
  const ordered = ranges
    .map((range) => ({ from: range.from.getTime(), to: range.to.getTime() }))
    .filter(
      (range) =>
        range.to > selected.from.getTime() &&
        range.from < selected.to.getTime(),
    )
    .sort((left, right) => left.from - right.from);
  let coveredUntil = selected.from.getTime();
  for (const range of ordered) {
    if (range.from > coveredUntil) return false;
    coveredUntil = Math.max(coveredUntil, range.to);
    if (coveredUntil >= selected.to.getTime()) return true;
  }
  return false;
}
