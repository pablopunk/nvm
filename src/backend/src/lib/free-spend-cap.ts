import { and, eq, sql } from 'drizzle-orm';
import { db } from '../db/client';
import {
  creditReservations,
  freeSpendCapConfig,
  freeSpendMonthlyBudgets,
} from '../db/schema';

export type FreeSpendCapTransaction = Parameters<
  Parameters<typeof db.transaction>[0]
>[0];
export type FreeSpendBudget = typeof freeSpendMonthlyBudgets.$inferSelect;

function resultRows<T>(result: unknown): T[] {
  if (Array.isArray(result)) return result as T[];
  if (
    result &&
    typeof result === 'object' &&
    Array.isArray((result as { rows?: unknown }).rows)
  ) {
    return (result as { rows: T[] }).rows;
  }
  return [];
}

export function utcMonthKey(date: Date): string {
  return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, '0')}`;
}

export function nextUtcMonth(date: Date): Date {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + 1, 1));
}

export function microcentsForFreePool(input: {
  totalCostMicrocents: number;
  freeCredits: number;
  totalCredits: number;
}): number {
  if (
    !Number.isSafeInteger(input.totalCostMicrocents) ||
    input.totalCostMicrocents < 0 ||
    !Number.isSafeInteger(input.freeCredits) ||
    !Number.isSafeInteger(input.totalCredits) ||
    input.freeCredits < 0 ||
    input.totalCredits < 0 ||
    input.freeCredits > input.totalCredits
  ) {
    throw new Error('Invalid free-pool cost allocation');
  }
  if (input.totalCostMicrocents > 0 && input.totalCredits === 0) {
    throw new Error('Positive provider cost requires a credit allocation');
  }
  if (
    input.totalCostMicrocents === 0 ||
    input.freeCredits === 0 ||
    input.totalCredits === 0
  )
    return 0;
  const numerator =
    BigInt(input.totalCostMicrocents) * BigInt(input.freeCredits);
  const denominator = BigInt(input.totalCredits);
  const rounded = (numerator + denominator - 1n) / denominator;
  const value = Number(rounded);
  if (!Number.isSafeInteger(value))
    throw new Error('Free-pool provider cost exceeds safe integer storage');
  return value;
}

export async function lockFreeSpendCapConfig(
  tx: FreeSpendCapTransaction,
  mode: 'share' | 'update' = 'share',
) {
  const query = tx
    .select()
    .from(freeSpendCapConfig)
    .where(eq(freeSpendCapConfig.id, 'global'));
  const [config] =
    mode === 'share' ? await query.for('share') : await query.for('update');
  if (!config)
    throw new Error('Global free-spend cap configuration is not initialized');
  return config;
}

export async function ensureFreeSpendMonthlyBudget(
  tx: FreeSpendCapTransaction,
  monthKey: string,
  inheritedCapMicrocents: number | null,
): Promise<FreeSpendBudget> {
  const [existing] = await tx
    .select()
    .from(freeSpendMonthlyBudgets)
    .where(eq(freeSpendMonthlyBudgets.monthKey, monthKey))
    .for('update');
  if (existing) return existing;

  const monthStart = new Date(`${monthKey}-01T00:00:00.000Z`);
  if (
    Number.isNaN(monthStart.getTime()) ||
    utcMonthKey(monthStart) !== monthKey
  ) {
    throw new Error(`Invalid UTC month key: ${monthKey}`);
  }
  const nextMonth = nextUtcMonth(monthStart);
  const baselineResult = await tx.execute(sql`
    select
      coalesce(sum(r.actual_free_cost_microcents) filter (
        where r.status = 'settled'
          and r.admission_month = ${monthKey}
          and r.actual_free_credits is not null
          and r.actual_paid_credits is not null
          and r.actual_free_cost_microcents is not null
          and u.upstream_cost_source in ('provider_reported', 'catalog_estimate', 'not_billed')
      ), 0)::bigint as actual_free_cost_microcents,
      count(*) filter (
        where r.request_id is null
          or r.admission_month is null
          or (r.admission_month = ${monthKey} and (
            r.status <> 'settled'
            or r.actual_free_credits is null
            or r.actual_paid_credits is null
            or r.actual_free_cost_microcents is null
            or u.upstream_cost_source not in ('provider_reported', 'catalog_estimate', 'not_billed')
          ))
      )::int as unknown_usage_count
    from usage u
    left join credit_reservations r on r.request_id = u.request_id
    where u.created_at >= ${monthStart}
      and u.created_at < ${nextMonth}
      and (u.cost_credits > 0 or u.upstream_cost_microcents > 0)
  `);
  const [baseline] = resultRows<{
    actual_free_cost_microcents: number | string;
    unknown_usage_count: number | string;
  }>(baselineResult);

  const [pending] = await tx
    .select({
      reserved: sql<number>`coalesce(sum(coalesce(${creditReservations.reservedFreeCostMicrocents}, 0)), 0)::bigint`,
    })
    .from(creditReservations)
    .where(
      and(
        eq(creditReservations.status, 'pending'),
        eq(creditReservations.admissionMonth, monthKey),
      ),
    );
  const actual = Number(baseline?.actual_free_cost_microcents ?? 0);
  const reserved = Number(pending?.reserved ?? 0);
  const unknownUsageCount = Number(baseline?.unknown_usage_count ?? 0);
  if (!Number.isSafeInteger(actual) || !Number.isSafeInteger(reserved)) {
    throw new Error('Free-spend baseline exceeds safe integer storage');
  }

  const [created] = await tx
    .insert(freeSpendMonthlyBudgets)
    .values({
      monthKey,
      capMicrocents: inheritedCapMicrocents,
      actualFreeCostMicrocents: actual,
      reservedFreeCostMicrocents: reserved,
      halted:
        inheritedCapMicrocents != null &&
        (unknownUsageCount > 0 ||
          actual >= inheritedCapMicrocents ||
          actual + reserved > inheritedCapMicrocents),
      baselineComplete: unknownUsageCount === 0,
    })
    .onConflictDoNothing()
    .returning();
  if (created) return created;

  const [raced] = await tx
    .select()
    .from(freeSpendMonthlyBudgets)
    .where(eq(freeSpendMonthlyBudgets.monthKey, monthKey))
    .for('update');
  if (!raced) throw new Error('Could not materialize UTC free-spend budget');
  return raced;
}

async function findFreeSpendMonthlyBudget(
  executor: Pick<FreeSpendCapTransaction, 'select'>,
  monthKey: string,
): Promise<FreeSpendBudget | undefined> {
  const [budget] = await executor
    .select()
    .from(freeSpendMonthlyBudgets)
    .where(eq(freeSpendMonthlyBudgets.monthKey, monthKey))
    .limit(1);
  return budget;
}

export function canReserveFreeSpend(input: {
  budget: Pick<
    FreeSpendBudget,
    | 'capMicrocents'
    | 'actualFreeCostMicrocents'
    | 'reservedFreeCostMicrocents'
    | 'halted'
    | 'baselineComplete'
  >;
  requestedMicrocents: number;
}): boolean {
  if (
    !Number.isSafeInteger(input.requestedMicrocents) ||
    input.requestedMicrocents < 0
  )
    return false;
  if (input.budget.halted) return false;
  if (input.budget.capMicrocents == null) return true;
  if (!input.budget.baselineComplete) return false;
  const remaining =
    input.budget.capMicrocents -
    input.budget.actualFreeCostMicrocents -
    input.budget.reservedFreeCostMicrocents;
  return remaining > 0 && input.requestedMicrocents <= remaining;
}

export function canResizeFreeSpend(input: {
  budget: Pick<
    FreeSpendBudget,
    | 'capMicrocents'
    | 'actualFreeCostMicrocents'
    | 'reservedFreeCostMicrocents'
    | 'halted'
    | 'baselineComplete'
  >;
  previousFreeCredits: number;
  nextFreeCredits: number;
  previousHoldMicrocents: number;
  nextHoldMicrocents: number;
}): boolean {
  if (
    !Number.isSafeInteger(input.previousFreeCredits) ||
    !Number.isSafeInteger(input.nextFreeCredits) ||
    !Number.isSafeInteger(input.previousHoldMicrocents) ||
    !Number.isSafeInteger(input.nextHoldMicrocents) ||
    input.previousFreeCredits < 0 ||
    input.nextFreeCredits < 0 ||
    input.previousHoldMicrocents < 0 ||
    input.nextHoldMicrocents < 0
  ) {
    return false;
  }

  const addsFreeAllocation = input.nextFreeCredits > input.previousFreeCredits;
  const increasesHold = input.nextHoldMicrocents > input.previousHoldMicrocents;
  if (!addsFreeAllocation && !increasesHold) return true;

  return canReserveFreeSpend({
    budget: {
      ...input.budget,
      reservedFreeCostMicrocents: Math.max(
        0,
        input.budget.reservedFreeCostMicrocents - input.previousHoldMicrocents,
      ),
    },
    requestedMicrocents: input.nextHoldMicrocents,
  });
}

export function capHaltedAfterSettlement(input: {
  capMicrocents: number | null;
  actualFreeCostMicrocents: number;
  reservedFreeCostMicrocents: number;
}): boolean {
  return (
    input.capMicrocents != null &&
    (input.actualFreeCostMicrocents >= input.capMicrocents ||
      input.actualFreeCostMicrocents + input.reservedFreeCostMicrocents >
        input.capMicrocents)
  );
}

export function capResetsAt(monthKey: string): string {
  const monthStart = new Date(`${monthKey}-01T00:00:00.000Z`);
  return nextUtcMonth(monthStart).toISOString();
}

export function freeSpendCapErrorBody(now = new Date()) {
  return {
    error: {
      type: 'free_spend_cap_reached',
      message:
        'Free-spend capacity is currently unavailable. Capacity may return earlier when reservations settle or release.',
      retryable: true,
      resetsAt: capResetsAt(utcMonthKey(now)),
    },
  };
}

export async function getCurrentFreeSpendBudget(now = new Date()) {
  const monthKey = utcMonthKey(now);
  const existing = await findFreeSpendMonthlyBudget(db, monthKey);
  if (existing) return existing;
  return db.transaction(async (tx) => {
    const config = await lockFreeSpendCapConfig(tx, 'share');
    const budget = await ensureFreeSpendMonthlyBudget(
      tx,
      monthKey,
      config.capMicrocents,
    );
    return budget;
  });
}

export async function getFreeSpendBudgetSnapshots(now = new Date()) {
  const currentMonth = utcMonthKey(now);
  const nextMonth = utcMonthKey(nextUtcMonth(now));
  return db.transaction(async (tx) => {
    const config = await lockFreeSpendCapConfig(tx, 'share');
    const current =
      (await findFreeSpendMonthlyBudget(tx, currentMonth)) ??
      (await ensureFreeSpendMonthlyBudget(
        tx,
        currentMonth,
        config.capMicrocents,
      ));
    const storedNext = await findFreeSpendMonthlyBudget(tx, nextMonth);
    const next = storedNext ?? {
      monthKey: nextMonth,
      capMicrocents: config.capMicrocents,
      actualFreeCostMicrocents: 0,
      reservedFreeCostMicrocents: 0,
      halted: false,
      baselineComplete: true,
      updatedAt: now,
    };
    return { config, current, next, currentMonth, nextMonth };
  });
}
