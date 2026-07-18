import { and, eq, lte, sql } from 'drizzle-orm';
import { db } from '../db/client';
import { creditLedger, creditReservations, usage, users } from '../db/schema';
import { computeUsdCost, usdToCredits, usdToMicrocents, type ModelCost } from './cost';
import { log } from './log';

const CREDIT_GRACE_THRESHOLD = Math.max(0, Number(process.env.CREDIT_GRACE_THRESHOLD ?? 100));
const RESERVATION_TTL_MS = Math.max(60_000, Number(process.env.CREDIT_RESERVATION_TTL_MS ?? 6 * 60_000));

export type CreditKind = 'free' | 'paid';
export type ReservationResult =
  | { ok: true; reservation: typeof creditReservations.$inferSelect; balance: number; reserved: number }
  | { ok: false; reason: 'insufficient_credits' | 'request_already_reserved'; balance: number; reserved: number };

export async function reserveCredits(input: {
  requestId: string;
  userId: string;
  kind: CreditKind;
  credits: number;
  now?: Date;
}): Promise<ReservationResult> {
  const now = input.now ?? new Date();
  const credits = Math.max(1, Math.ceil(input.credits));
  return db.transaction(async (tx) => {
    // Locking the parent user serializes all admission decisions for that user
    // without blocking unrelated users.
    const [user] = await tx.select({ id: users.id }).from(users).where(eq(users.id, input.userId)).limit(1).for('update');
    if (!user) throw new Error('Cannot reserve credits for unknown user');

    const [existing] = await tx.select().from(creditReservations)
      .where(eq(creditReservations.requestId, input.requestId)).limit(1).for('update');
    if (existing) {
      if (existing.userId !== input.userId || existing.kind !== input.kind || existing.reservedCredits !== credits) {
        throw new Error('Reservation request ID was reused with different billing parameters');
      }
      return { ok: false, reason: 'request_already_reserved', balance: 0, reserved: existing.reservedCredits };
    }

    const [ledger] = await tx.select({
      balance: sql<number>`coalesce(sum(case when ${creditLedger.kind} = ${input.kind} then ${creditLedger.delta} else 0 end), 0)::int`,
    }).from(creditLedger).where(eq(creditLedger.userId, input.userId));
    const [active] = await tx.select({
      reserved: sql<number>`coalesce(sum(${creditReservations.reservedCredits}), 0)::int`,
    }).from(creditReservations).where(and(
      eq(creditReservations.userId, input.userId),
      eq(creditReservations.kind, input.kind),
      eq(creditReservations.status, 'pending'),
    ));
    const balance = ledger?.balance ?? 0;
    const reserved = active?.reserved ?? 0;
    if (credits + reserved > balance + CREDIT_GRACE_THRESHOLD) {
      log.warn('credit_reservation_rejected', { request_id: input.requestId, user_id: input.userId, kind: input.kind, credits, balance, reserved });
      return { ok: false, reason: 'insufficient_credits', balance, reserved };
    }

    const [reservation] = await tx.insert(creditReservations).values({
      requestId: input.requestId,
      userId: input.userId,
      kind: input.kind,
      reservedCredits: credits,
      expiresAt: new Date(now.getTime() + RESERVATION_TTL_MS),
    }).returning();
    log.info('credit_reserved', { request_id: input.requestId, user_id: input.userId, kind: input.kind, credits, balance, reserved });
    return { ok: true, reservation, balance, reserved };
  });
}

export type ReservationFinalization = {
  requestId: string;
  outcome: 'settle' | 'release';
  model?: string;
  provider?: string;
  tokens?: { inputTokens: number; outputTokens: number };
  costRow?: ModelCost;
  status?: number;
  latencyMs?: number;
};

async function lockReservationAfterUser(
  tx: Parameters<Parameters<typeof db.transaction>[0]>[0],
  requestId: string,
): Promise<typeof creditReservations.$inferSelect> {
  const [user] = await tx.select({ id: users.id }).from(users)
    .where(sql`${users.id} = (
      select ${creditReservations.userId}
      from ${creditReservations}
      where ${creditReservations.requestId} = ${requestId}
      limit 1
    )`).limit(1).for('update');
  if (!user) throw new Error(`Missing user for credit reservation ${requestId}`);

  const [reservation] = await tx.select().from(creditReservations)
    .where(eq(creditReservations.requestId, requestId)).limit(1).for('update');
  if (!reservation) throw new Error(`Missing credit reservation ${requestId}`);
  return reservation;
}

/** Finalize exactly once. Terminal rows are intentionally no-ops on retries. */
export async function finalizeReservation(input: ReservationFinalization): Promise<'settled' | 'released' | 'already_terminal'> {
  return db.transaction(async (tx) => {
    // Every reservation transition takes row locks in the same order: parent
    // user first, reservation second. This matches admission and prevents a
    // same-request admission/finalization cycle through the FK-backed rows.
    const reservation = await lockReservationAfterUser(tx, input.requestId);
    if (reservation.status !== 'pending') return 'already_terminal';
    const now = new Date();
    if (input.outcome === 'release') {
      await tx.update(creditReservations).set({ status: 'released', actualCredits: 0, releasedAt: now, updatedAt: now })
        .where(eq(creditReservations.requestId, input.requestId));
      log.info('credit_reservation_released', { request_id: input.requestId, user_id: reservation.userId, reserved_credits: reservation.reservedCredits });
      return 'released';
    }
    if (!input.model || !input.provider || !input.tokens || !input.costRow || input.status == null || input.latencyMs == null) {
      throw new Error('Settlement requires usage details');
    }
    const costUsd = computeUsdCost(input.costRow, input.tokens.inputTokens, input.tokens.outputTokens);
    const credits = usdToCredits(costUsd);
    const microcents = usdToMicrocents(costUsd);
    await tx.insert(creditLedger).values({
      userId: reservation.userId,
      delta: -credits,
      kind: reservation.kind,
      reason: 'ai_usage',
      refId: reservation.requestId,
    }).onConflictDoNothing();
    await tx.insert(usage).values({
      userId: reservation.userId,
      model: input.model,
      provider: input.provider,
      inputTokens: input.tokens.inputTokens,
      outputTokens: input.tokens.outputTokens,
      costCredits: credits,
      upstreamCostMicrocents: microcents,
      requestId: reservation.requestId,
      status: input.status,
      latencyMs: input.latencyMs,
    });
    await tx.update(creditReservations).set({ status: 'settled', actualCredits: credits, settledAt: now, updatedAt: now })
      .where(eq(creditReservations.requestId, input.requestId));
    log.info('credit_reservation_settled', { request_id: input.requestId, user_id: reservation.userId, reserved_credits: reservation.reservedCredits, actual_credits: credits });
    return 'settled';
  });
}

/** Pending rows have no verified usage after their proxy lease expires, so the
 * conservative recovery policy releases them instead of charging unknown work. */
export async function reconcileStaleReservations(limit = 100, now = new Date()): Promise<{ released: number }> {
  const rows = await db.select({ requestId: creditReservations.requestId }).from(creditReservations)
    .where(and(eq(creditReservations.status, 'pending'), lte(creditReservations.expiresAt, now)))
    .limit(Math.max(1, Math.min(limit, 500)));
  let released = 0;
  for (const row of rows) {
    if (await finalizeReservation({ requestId: row.requestId, outcome: 'release' }) === 'released') released += 1;
  }
  if (released > 0) log.info('credit_reservations_stale_reconciled', { released });
  return { released };
}
