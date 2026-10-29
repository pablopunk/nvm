import { eq, sql } from 'drizzle-orm';
import { appSettings } from '../db/schema';
import type { FreeSpendCapTransaction } from './free-spend-cap';
import { env } from './env';
import { log } from './log';

/**
 * Credits Nevermind may absorb in one UTC day from settlements whose list cost its debit
 * did not collect. 500 credits is about $5 of list charge, or about $1 of provider cost at
 * the default markup. Override with PRICING_OVERRUN_DAILY_CAP_CREDITS.
 */
export const DEFAULT_ABSORBED_OVERRUN_DAILY_CAP_CREDITS = 500;

export function absorbedOverrunDailyCapCredits(): number {
  const raw = env('PRICING_OVERRUN_DAILY_CAP_CREDITS');
  const parsed = raw && raw.trim() ? Number(raw) : Number.NaN;
  return Number.isSafeInteger(parsed) && parsed >= 0
    ? parsed
    : DEFAULT_ABSORBED_OVERRUN_DAILY_CAP_CREDITS;
}

/** One durable counter per UTC day, so the bound resets with the day's admissions. */
export function absorbedOverrunSettingKey(now: Date): string {
  return `pricing_absorbed_overrun_credits:${now.toISOString().slice(0, 10)}`;
}

export function overrunCapReached(absorbedCredits: number, capCredits: number): boolean {
  return absorbedCredits >= capCredits;
}

/**
 * The platform's exposure from one settlement: the list cost its debit did not collect.
 * That includes cost above the hold and any part the debit could not take because the
 * balance or the held pool ran short. It is one difference, so an overrun that is also
 * short of balance counts once.
 */
export function absorbedSettlementCredits(
  calculatedCredits: number,
  collectedCredits: number,
): number {
  return Math.max(0, calculatedCredits - collectedCredits);
}

/**
 * Adds one settlement's absorbed credits to today's counter. The upsert is a single
 * statement, so concurrent settlements never lose an increment, and it runs inside the
 * settlement transaction so a rolled-back settlement adds nothing.
 */
export async function recordAbsorbedOverrunCredits(
  tx: FreeSpendCapTransaction,
  absorbedCredits: number,
  now: Date,
): Promise<void> {
  if (!Number.isSafeInteger(absorbedCredits) || absorbedCredits <= 0) return;
  await tx
    .insert(appSettings)
    .values({
      key: absorbedOverrunSettingKey(now),
      value: String(absorbedCredits),
      updatedAt: now,
    })
    .onConflictDoUpdate({
      target: appSettings.key,
      set: {
        value: sql`(${appSettings.value}::bigint + ${absorbedCredits})::text`,
        updatedAt: now,
      },
    });
}

async function absorbedOverrunCreditsToday(now: Date): Promise<number> {
  const { getDb } = await import('../db/client');
  const [row] = await getDb()
    .select({ value: appSettings.value })
    .from(appSettings)
    .where(eq(appSettings.key, absorbedOverrunSettingKey(now)))
    .limit(1);
  if (!row) return 0;
  if (!/^\d+$/.test(row.value))
    throw new Error(`absorbed overrun counter is not a whole credit count: ${row.value}`);
  return Number(row.value);
}

let haltedForTests: boolean | null = null;

/** Forces the halt decision without a database. Pass null to restore the real read. */
export function setStaleAdmissionsHaltedForTests(halted: boolean | null) {
  haltedForTests = halted;
}

/**
 * Whether stale-priced admissions are refused for the rest of the UTC day. Without a
 * database no settlement can run, so nothing has been absorbed. A failed read refuses
 * stale admissions rather than assuming the bound holds.
 */
export async function staleAdmissionsHalted(now = new Date()): Promise<boolean> {
  if (haltedForTests !== null) return haltedForTests;
  if (!env('DATABASE_URL')) return false;
  try {
    return overrunCapReached(
      await absorbedOverrunCreditsToday(now),
      absorbedOverrunDailyCapCredits(),
    );
  } catch (error) {
    log.warn('pricing_overrun_cap_read_failed', { error: String(error) });
    return true;
  }
}
