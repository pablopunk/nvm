import { and, eq, inArray, isNull, sql } from 'drizzle-orm';
import { db } from '../db/client';
import { authIntents, invites, users, creditLedger } from '../db/schema';
import { createHash } from 'node:crypto';
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

export class InviteRequiredError extends Error {
  constructor() { super('A valid invitation is required'); this.name = 'InviteRequiredError'; }
}

function canonicalEmail(email: string) {
  return email.trim().toLowerCase();
}

async function findOrLinkUserByWorkosIdentity(tx: any, input: { workosUserId: string; email: string }) {
  const [existingByWorkosId] = await tx.select().from(users).where(eq(users.workosUserId, input.workosUserId)).limit(1);
  if (existingByWorkosId) return existingByWorkosId;

  const email = canonicalEmail(input.email);
  const [existingByEmail] = await tx.select().from(users).where(sql`lower(${users.email}) = ${email}`).limit(1);
  if (!existingByEmail) return null;

  const [linked] = await tx.update(users)
    .set({ workosUserId: input.workosUserId })
    .where(and(eq(users.id, existingByEmail.id), eq(users.workosUserId, existingByEmail.workosUserId)))
    .returning();
  if (linked) return linked;

  const [concurrentLink] = await tx.select().from(users).where(eq(users.workosUserId, input.workosUserId)).limit(1);
  if (concurrentLink) return concurrentLink;
  throw new Error('User identity changed while linking authenticated account');
}

export async function createUserFromInviteIntent(input: { intentId: string; nonce: string; workosUserId: string; email: string }) {
  return db.transaction(async (tx) => {
    const [existingByWorkosId] = await tx.select().from(users).where(eq(users.workosUserId, input.workosUserId)).limit(1);
    if (existingByWorkosId) return existingByWorkosId;

    const [intent] = await tx.select().from(authIntents).where(eq(authIntents.id, input.intentId)).limit(1);
    if (!intent || intent.consumedAt || intent.expiresAt <= new Date() || createHash('sha256').update(input.nonce).digest('hex') !== intent.nonceHash) throw new InviteRequiredError();
    const [invite] = await tx.select().from(invites).where(eq(invites.id, intent.inviteId)).limit(1);
    const email = canonicalEmail(input.email);
    if (!invite || invite.email !== email || invite.expiresAt <= new Date() || !['queued', 'sending', 'sent'].includes(invite.status)) throw new InviteRequiredError();
    if (isDisposableEmail(input.email)) throw new DisposableEmailError(input.email);

    const [consumed] = await tx.update(authIntents).set({ consumedAt: new Date() })
      .where(and(eq(authIntents.id, intent.id), eq(authIntents.nonceHash, intent.nonceHash), isNull(authIntents.consumedAt)))
      .returning();
    if (!consumed) throw new InviteRequiredError();

    const existing = await findOrLinkUserByWorkosIdentity(tx, input);
    if (existing) {
      await tx.update(invites).set({ status: 'redeemed', redeemedAt: new Date() }).where(and(eq(invites.id, invite.id), inArray(invites.status, ['queued', 'sending', 'sent'])));
      return existing;
    }

    const [created] = await tx.insert(users).values({ workosUserId: input.workosUserId, email }).returning();
    await tx.insert(creditLedger).values({ userId: created.id, delta: MONTHLY_FREE_CREDITS, kind: 'free', reason: 'grant_free_monthly', refId: currentFreeCreditPeriod() });
    await tx.update(invites).set({ status: 'redeemed', redeemedAt: new Date() }).where(and(eq(invites.id, invite.id), inArray(invites.status, ['queued', 'sending', 'sent'])));
    return created;
  });
}

export async function upsertUserWithFreeGrant(input: {
  workosUserId: string;
  email: string;
}) {
  return db.transaction(async (tx) => {
    const existing = await findOrLinkUserByWorkosIdentity(tx, input);
    if (existing) return existing;

    if (isDisposableEmail(input.email)) throw new DisposableEmailError(input.email);
    const email = canonicalEmail(input.email);

    const [created] = await tx
      .insert(users)
      .values({ workosUserId: input.workosUserId, email })
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

export async function getUserByWorkosId(workosUserId: string) {
  return (await db.select().from(users).where(eq(users.workosUserId, workosUserId)).limit(1))[0] ?? null;
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
