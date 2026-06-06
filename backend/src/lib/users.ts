import { and, eq, sql } from 'drizzle-orm';
import { db } from '../db/client';
import { users, creditLedger } from '../db/schema';
import { isDisposableEmail } from './disposable';
import { env } from './env';

function readNonNegativeIntEnv(key: string, fallback: number): number {
  const raw = env(key);
  const parsed = raw && raw.trim() ? Number(raw) : fallback;
  return Number.isInteger(parsed) && parsed >= 0 ? parsed : fallback;
}

export const MONTHLY_FREE_CREDITS = readNonNegativeIntEnv('MONTHLY_FREE_CREDITS', 500);

function currentFreeCreditPeriod(now = new Date()): string {
  return `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, '0')}`;
}

export class DisposableEmailError extends Error {
  constructor(email: string) {
    super(`Disposable email not allowed: ${email}`);
    this.name = 'DisposableEmailError';
  }
}

export async function upsertUserWithFreeGrant(input: {
  workosUserId: string;
  email: string;
}) {
  return db.transaction(async (tx) => {
    const existing = await tx
      .select()
      .from(users)
      .where(eq(users.workosUserId, input.workosUserId))
      .limit(1);
    if (existing[0]) return existing[0];

    if (isDisposableEmail(input.email)) throw new DisposableEmailError(input.email);

    const [created] = await tx
      .insert(users)
      .values({ workosUserId: input.workosUserId, email: input.email })
      .returning();

    await tx.insert(creditLedger).values({
      userId: created.id,
      delta: MONTHLY_FREE_CREDITS,
      kind: 'free',
      reason: 'grant_free_monthly',
      refId: currentFreeCreditPeriod(),
    });

    return created;
  });
}

export async function ensureMonthlyFreeCredits(userId: string, now = new Date()): Promise<void> {
  const period = currentFreeCreditPeriod(now);
  await db.transaction(async (tx) => {
    const existingGrant = await tx
      .select({ id: creditLedger.id })
      .from(creditLedger)
      .where(and(eq(creditLedger.userId, userId), eq(creditLedger.reason, 'grant_free_monthly'), eq(creditLedger.refId, period)))
      .limit(1);
    if (existingGrant.length > 0) return;

    const [row] = await tx
      .select({
        free: sql<number>`coalesce(sum(case when ${creditLedger.kind} = 'free' then ${creditLedger.delta} else 0 end), 0)::int`,
      })
      .from(creditLedger)
      .where(eq(creditLedger.userId, userId));
    const delta = Math.max(0, MONTHLY_FREE_CREDITS - (row?.free ?? 0));
    await tx.insert(creditLedger).values({
      userId,
      delta,
      kind: 'free',
      reason: 'grant_free_monthly',
      refId: period,
    }).onConflictDoNothing({ target: [creditLedger.userId, creditLedger.reason, creditLedger.refId] });
  });
}

export async function getBalance(userId: string): Promise<number> {
  const rows = await db
    .select({ balance: sql<number>`coalesce(sum(${creditLedger.delta}), 0)::int` })
    .from(creditLedger)
    .where(eq(creditLedger.userId, userId));
  return rows[0]?.balance ?? 0;
}

export async function getBalances(userId: string): Promise<{ free: number; paid: number; total: number }> {
  const [row] = await db
    .select({
      free: sql<number>`coalesce(sum(case when ${creditLedger.kind} = 'free' then ${creditLedger.delta} else 0 end), 0)::int`,
      paid: sql<number>`coalesce(sum(case when ${creditLedger.kind} = 'paid' then ${creditLedger.delta} else 0 end), 0)::int`,
    })
    .from(creditLedger)
    .where(eq(creditLedger.userId, userId));
  const free = row?.free ?? 0;
  const paid = row?.paid ?? 0;
  return { free, paid, total: free + paid };
}
