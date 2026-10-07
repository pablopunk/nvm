import { and, eq, lte, sql } from 'drizzle-orm';
import { db } from '../db/client';
import {
  creditLedger,
  creditReservations,
  requestDedup,
  routeBreakers,
  usage,
  users,
} from '../db/schema';
import {
  computeUsdCost,
  type ModelCost,
  usdToCredits,
  usdToMicrocents,
} from './cost';
import { log } from './log';

export const CREDIT_GRACE_THRESHOLD = 0;
const RESERVATION_TTL_MS = Math.max(
  60_000,
  Number(process.env.CREDIT_RESERVATION_TTL_MS ?? 6 * 60_000),
);

export type CreditKind = 'free' | 'paid';
export type ReservationResult =
  | {
      ok: true;
      reservation: typeof creditReservations.$inferSelect;
      balance: number;
      reserved: number;
    }
  | {
      ok: false;
      reason: 'insufficient_credits' | 'request_already_reserved';
      balance: number;
      reserved: number;
    };

export async function reserveCredits(input: {
  requestId: string;
  userId: string;
  kind?: CreditKind;
  credits: number;
  now?: Date;
  allowExistingPending?: boolean;
}): Promise<ReservationResult> {
  const now = input.now ?? new Date();
  const credits = Math.max(1, Math.ceil(input.credits));
  return db.transaction(async (tx) => {
    // Locking the parent user serializes all admission decisions for that user
    // without blocking unrelated users.
    const [user] = await tx
      .select({ id: users.id })
      .from(users)
      .where(eq(users.id, input.userId))
      .limit(1)
      .for('update');
    if (!user) throw new Error('Cannot reserve credits for unknown user');

    const [existing] = await tx
      .select()
      .from(creditReservations)
      .where(eq(creditReservations.requestId, input.requestId))
      .limit(1)
      .for('update');
    if (existing) {
      if (
        existing.userId !== input.userId ||
        existing.reservedCredits !== credits
      ) {
        throw new Error(
          'Reservation request ID was reused with different billing parameters',
        );
      }
      if (input.allowExistingPending && existing.status === 'pending') {
        return {
          ok: true,
          reservation: existing,
          balance: 0,
          reserved: existing.reservedCredits,
        };
      }
      return {
        ok: false,
        reason: 'request_already_reserved',
        balance: 0,
        reserved: existing.reservedCredits,
      };
    }

    const [ledger] = await tx
      .select({
        free: sql<number>`coalesce(sum(case when ${creditLedger.kind} = 'free' then ${creditLedger.delta} else 0 end), 0)::int`,
        paid: sql<number>`coalesce(sum(case when ${creditLedger.kind} = 'paid' then ${creditLedger.delta} else 0 end), 0)::int`,
      })
      .from(creditLedger)
      .where(eq(creditLedger.userId, input.userId));
    const [active] = await tx
      .select({
        free: sql<number>`coalesce(sum(coalesce(${creditReservations.reservedFreeCredits}, case when ${creditReservations.kind} = 'free' then ${creditReservations.reservedCredits} else 0 end)), 0)::int`,
        paid: sql<number>`coalesce(sum(coalesce(${creditReservations.reservedPaidCredits}, case when ${creditReservations.kind} = 'paid' then ${creditReservations.reservedCredits} else 0 end)), 0)::int`,
      })
      .from(creditReservations)
      .where(
        and(
          eq(creditReservations.userId, input.userId),
          eq(creditReservations.status, 'pending'),
        ),
      );
    const freeBalance = ledger?.free ?? 0;
    const paidBalance = ledger?.paid ?? 0;
    const freeAvailable = freeBalance - (active?.free ?? 0);
    const paidAvailable = paidBalance - (active?.paid ?? 0);
    const reservedFree = Math.min(credits, Math.max(0, freeAvailable));
    const reservedPaid = credits - reservedFree;
    const kind = reservedFree > 0 ? 'free' : 'paid';
    if (
      freeBalance < 0 ||
      paidBalance < 0 ||
      reservedPaid > Math.max(0, paidAvailable)
    ) {
      const balance = freeBalance + paidBalance;
      const reserved = (active?.free ?? 0) + (active?.paid ?? 0);
      log.warn('credit_reservation_rejected', {
        request_id: input.requestId,
        user_id: input.userId,
        credits,
        balance,
        reserved,
      });
      return { ok: false, reason: 'insufficient_credits', balance, reserved };
    }

    const [reservation] = await tx
      .insert(creditReservations)
      .values({
        requestId: input.requestId,
        userId: input.userId,
        kind,
        reservedCredits: credits,
        reservedFreeCredits: reservedFree,
        reservedPaidCredits: reservedPaid,
        expiresAt: new Date(now.getTime() + RESERVATION_TTL_MS),
      })
      .returning();
    const balance = freeBalance + paidBalance;
    const reserved = (active?.free ?? 0) + (active?.paid ?? 0);
    log.info('credit_reserved', {
      request_id: input.requestId,
      user_id: input.userId,
      kind,
      credits,
      balance,
      reserved,
      reserved_free: reservedFree,
      reserved_paid: reservedPaid,
    });
    return { ok: true, reservation, balance, reserved };
  });
}

export async function preparedReservationExists(input: {
  requestId: string;
  userId: string;
  credits: number;
}) {
  const [reservation] = await db
    .select({ requestId: creditReservations.requestId })
    .from(creditReservations)
    .where(
      and(
        eq(creditReservations.requestId, input.requestId),
        eq(creditReservations.userId, input.userId),
        eq(
          creditReservations.reservedCredits,
          Math.max(1, Math.ceil(input.credits)),
        ),
        eq(creditReservations.status, 'pending'),
      ),
    )
    .limit(1);
  return Boolean(reservation);
}

export type ReservationFinalization = {
  requestId: string;
  expectedUserId?: string;
  outcome: 'settle' | 'release';
  model?: string;
  provider?: string;
  tokens?: {
    inputTokens: number;
    outputTokens: number;
    cachedInputTokens?: number;
    cacheWriteInputTokens?: number;
    reasoningTokens?: number;
  };
  costRow?: ModelCost;
  providerCostUsd?: number;
  modality?: 'text' | 'audio';
  audioDurationMs?: number;
  costSource?: 'provider_reported' | 'catalog_estimate';
  routeSlot?: string;
  status?: number;
  latencyMs?: number;
  dedup?: {
    userId: string;
    idempotencyKey: string;
    requestHash: string;
    status: 'completed' | 'failed';
    responseJson?: unknown;
    responseHeaders?: Record<string, string>;
    upstreamStatus?: number;
  };
};

export async function resizeReservation(
  requestId: string,
  creditsInput: number,
): Promise<ReservationResult> {
  const credits = Math.max(1, Math.ceil(creditsInput));
  return db.transaction(async (tx) => {
    const reservation = await lockReservationAfterUser(tx, requestId);
    if (reservation.status !== 'pending') {
      return {
        ok: false,
        reason: 'request_already_reserved',
        balance: 0,
        reserved: reservation.reservedCredits,
      };
    }
    const [ledger] = await tx
      .select({
        free: sql<number>`coalesce(sum(case when ${creditLedger.kind} = 'free' then ${creditLedger.delta} else 0 end), 0)::int`,
        paid: sql<number>`coalesce(sum(case when ${creditLedger.kind} = 'paid' then ${creditLedger.delta} else 0 end), 0)::int`,
      })
      .from(creditLedger)
      .where(eq(creditLedger.userId, reservation.userId));
    const [active] = await tx
      .select({
        free: sql<number>`coalesce(sum(coalesce(${creditReservations.reservedFreeCredits}, case when ${creditReservations.kind} = 'free' then ${creditReservations.reservedCredits} else 0 end)), 0)::int`,
        paid: sql<number>`coalesce(sum(coalesce(${creditReservations.reservedPaidCredits}, case when ${creditReservations.kind} = 'paid' then ${creditReservations.reservedCredits} else 0 end)), 0)::int`,
      })
      .from(creditReservations)
      .where(
        and(
          eq(creditReservations.userId, reservation.userId),
          eq(creditReservations.status, 'pending'),
        ),
      );
    const oldFree =
      reservation.reservedFreeCredits ??
      (reservation.kind === 'free' ? reservation.reservedCredits : 0);
    const oldPaid =
      reservation.reservedPaidCredits ??
      (reservation.kind === 'paid' ? reservation.reservedCredits : 0);
    const freeReserved = Math.max(0, (active?.free ?? 0) - oldFree);
    const paidReserved = Math.max(0, (active?.paid ?? 0) - oldPaid);
    const freeBalance = ledger?.free ?? 0;
    const paidBalance = ledger?.paid ?? 0;
    const freeAvailable = freeBalance - freeReserved;
    const paidAvailable = paidBalance - paidReserved;
    const reservedFree = Math.min(credits, Math.max(0, freeAvailable));
    const reservedPaid = credits - reservedFree;
    const balance = freeBalance + paidBalance;
    const reservedByOthers = freeReserved + paidReserved;
    if (
      freeBalance < 0 ||
      paidBalance < 0 ||
      reservedPaid > Math.max(0, paidAvailable)
    ) {
      log.warn('credit_reservation_resize_rejected', {
        request_id: requestId,
        user_id: reservation.userId,
        credits,
        balance,
        reserved: reservedByOthers,
      });
      return {
        ok: false,
        reason: 'insufficient_credits',
        balance,
        reserved: reservedByOthers,
      };
    }
    const [updated] = await tx
      .update(creditReservations)
      .set({
        kind: reservedFree > 0 ? 'free' : 'paid',
        reservedCredits: credits,
        reservedFreeCredits: reservedFree,
        reservedPaidCredits: reservedPaid,
        updatedAt: new Date(),
      })
      .where(eq(creditReservations.requestId, requestId))
      .returning();
    log.info('credit_reservation_resized', {
      request_id: requestId,
      user_id: reservation.userId,
      credits,
      balance,
      reserved: reservedByOthers,
      reserved_free: reservedFree,
      reserved_paid: reservedPaid,
    });
    return {
      ok: true,
      reservation: updated,
      balance,
      reserved: reservedByOthers,
    };
  });
}

async function finalizeRequestDedup(
  tx: Parameters<Parameters<typeof db.transaction>[0]>[0],
  input: ReservationFinalization,
  now: Date,
) {
  if (!input.dedup) return;
  const [updated] = await tx
    .update(requestDedup)
    .set({
      status: input.dedup.status,
      responseJson: input.dedup.responseJson,
      responseHeaders: input.dedup.responseHeaders,
      upstreamStatus: input.dedup.upstreamStatus,
      completedAt: now,
    })
    .where(
      and(
        eq(requestDedup.userId, input.dedup.userId),
        eq(requestDedup.idempotencyKey, input.dedup.idempotencyKey),
        eq(requestDedup.requestId, input.requestId),
        eq(requestDedup.requestHash, input.dedup.requestHash),
        eq(requestDedup.status, 'in_flight'),
      ),
    )
    .returning({ id: requestDedup.id });
  if (!updated) {
    throw new Error(`Missing matching idempotency claim ${input.requestId}`);
  }
}

async function lockReservationAfterUser(
  tx: Parameters<Parameters<typeof db.transaction>[0]>[0],
  requestId: string,
): Promise<typeof creditReservations.$inferSelect> {
  const [user] = await tx
    .select({ id: users.id })
    .from(users)
    .where(sql`${users.id} = (
      select ${creditReservations.userId}
      from ${creditReservations}
      where ${creditReservations.requestId} = ${requestId}
      limit 1
    )`)
    .limit(1)
    .for('update');
  if (!user)
    throw new Error(`Missing user for credit reservation ${requestId}`);

  const [reservation] = await tx
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, requestId))
    .limit(1)
    .for('update');
  if (!reservation) throw new Error(`Missing credit reservation ${requestId}`);
  return reservation;
}

/** Finalize exactly once. Terminal rows are intentionally no-ops on retries. */
export async function finalizeReservation(
  input: ReservationFinalization,
): Promise<'settled' | 'released' | 'already_terminal'> {
  return db.transaction(async (tx) => {
    // Every reservation transition takes row locks in the same order: parent
    // user first, reservation second. This matches admission and prevents a
    // same-request admission/finalization cycle through the FK-backed rows.
    const reservation = await lockReservationAfterUser(tx, input.requestId);
    if (input.expectedUserId && input.expectedUserId !== reservation.userId) {
      throw new Error(`User does not own reservation ${input.requestId}`);
    }
    if (input.dedup && input.dedup.userId !== reservation.userId) {
      throw new Error(
        `Idempotency claim user does not own reservation ${input.requestId}`,
      );
    }
    if (reservation.status !== 'pending') return 'already_terminal';
    const now = new Date();
    if (input.outcome === 'release') {
      if (
        input.model &&
        input.provider &&
        input.status != null &&
        input.latencyMs != null
      ) {
        await tx.insert(usage).values({
          userId: reservation.userId,
          model: input.model,
          provider: input.provider,
          inputTokens: input.tokens?.inputTokens ?? 0,
          outputTokens: input.tokens?.outputTokens ?? 0,
          cachedInputTokens: input.tokens?.cachedInputTokens ?? 0,
          cacheWriteInputTokens: input.tokens?.cacheWriteInputTokens ?? 0,
          reasoningTokens: input.tokens?.reasoningTokens ?? 0,
          modality: input.modality ?? 'text',
          audioDurationMs: input.audioDurationMs ?? 0,
          costCredits: 0,
          upstreamCostMicrocents: 0,
          upstreamCostSource: 'not_billed',
          costOverrun: false,
          routeSlot: input.routeSlot ?? null,
          requestId: reservation.requestId,
          status: input.status,
          latencyMs: input.latencyMs,
        });
      }
      await tx
        .update(creditReservations)
        .set({
          status: 'released',
          actualCredits: 0,
          releasedAt: now,
          updatedAt: now,
        })
        .where(eq(creditReservations.requestId, input.requestId));
      await finalizeRequestDedup(tx, input, now);
      log.info('credit_reservation_released', {
        request_id: input.requestId,
        user_id: reservation.userId,
        reserved_credits: reservation.reservedCredits,
      });
      return 'released';
    }
    if (
      !input.model ||
      !input.provider ||
      !input.tokens ||
      (input.costRow == null && input.providerCostUsd == null) ||
      input.status == null ||
      input.latencyMs == null
    ) {
      throw new Error('Settlement requires usage details');
    }
    const providerCostUsd = input.providerCostUsd;
    const hasProviderCost =
      providerCostUsd != null &&
      Number.isFinite(providerCostUsd) &&
      providerCostUsd >= 0;
    const costRow = input.costRow;
    if (!hasProviderCost && !costRow)
      throw new Error('Settlement requires a valid cost source');
    const costUsd = hasProviderCost
      ? providerCostUsd
      : computeUsdCost(
          costRow,
          input.tokens.inputTokens,
          input.tokens.outputTokens,
          input.tokens,
        );
    const calculatedCredits = usdToCredits(costUsd);
    const credits = Math.min(calculatedCredits, reservation.reservedCredits);
    const reservedFree =
      reservation.reservedFreeCredits ??
      (reservation.kind === 'free' ? reservation.reservedCredits : 0);
    const reservedPaid =
      reservation.reservedPaidCredits ??
      (reservation.kind === 'paid' ? reservation.reservedCredits : 0);
    const [balances] = await tx
      .select({
        free: sql<number>`coalesce(sum(case when ${creditLedger.kind} = 'free' then ${creditLedger.delta} else 0 end), 0)::int`,
        paid: sql<number>`coalesce(sum(case when ${creditLedger.kind} = 'paid' then ${creditLedger.delta} else 0 end), 0)::int`,
      })
      .from(creditLedger)
      .where(eq(creditLedger.userId, reservation.userId));
    const actualFree = Math.min(
      credits,
      reservedFree,
      Math.max(0, balances?.free ?? 0),
    );
    const actualPaid = Math.min(
      credits - actualFree,
      reservedPaid,
      Math.max(0, balances?.paid ?? 0),
    );
    if (calculatedCredits > reservation.reservedCredits) {
      log.error('provider_cost_exceeded_reservation', {
        request_id: input.requestId,
        user_id: reservation.userId,
        reserved_credits: reservation.reservedCredits,
        calculated_credits: calculatedCredits,
        provider_cost_usd: costUsd,
      });
      if (input.routeSlot && input.provider && input.model)
        await tx
          .insert(routeBreakers)
          .values({
            routeSlot: input.routeSlot,
            provider: input.provider,
            modelId: input.model,
            reason: `settlement_overrun:${hasProviderCost ? 'provider_reported' : 'catalog_estimate'}`,
          })
          .onConflictDoNothing();
    }
    const microcents = usdToMicrocents(costUsd);
    if (actualFree > 0)
      await tx
        .insert(creditLedger)
        .values({
          userId: reservation.userId,
          delta: -actualFree,
          kind: 'free',
          reason: 'ai_usage',
          refId: `${reservation.requestId}:free`,
        })
        .onConflictDoNothing();
    if (actualPaid > 0)
      await tx
        .insert(creditLedger)
        .values({
          userId: reservation.userId,
          delta: -actualPaid,
          kind: 'paid',
          reason: 'ai_usage',
          refId: `${reservation.requestId}:paid`,
        })
        .onConflictDoNothing();
    await tx.insert(usage).values({
      userId: reservation.userId,
      model: input.model,
      provider: input.provider,
      inputTokens: input.tokens.inputTokens,
      outputTokens: input.tokens.outputTokens,
      cachedInputTokens: input.tokens.cachedInputTokens ?? 0,
      cacheWriteInputTokens: input.tokens.cacheWriteInputTokens ?? 0,
      reasoningTokens: input.tokens.reasoningTokens ?? 0,
      modality: input.modality ?? 'text',
      audioDurationMs: input.audioDurationMs ?? 0,
      costCredits: credits,
      upstreamCostMicrocents: microcents,
      upstreamCostSource: hasProviderCost
        ? 'provider_reported'
        : (input.costSource ?? 'catalog_estimate'),
      costOverrun: calculatedCredits > reservation.reservedCredits,
      routeSlot: input.routeSlot ?? null,
      requestId: reservation.requestId,
      status: input.status,
      latencyMs: input.latencyMs,
    });
    await tx
      .update(creditReservations)
      .set({
        status: 'settled',
        actualCredits: actualFree + actualPaid,
        actualFreeCredits: actualFree,
        actualPaidCredits: actualPaid,
        settledAt: now,
        updatedAt: now,
      })
      .where(eq(creditReservations.requestId, input.requestId));
    await finalizeRequestDedup(tx, input, now);
    log.info('credit_reservation_settled', {
      request_id: input.requestId,
      user_id: reservation.userId,
      reserved_credits: reservation.reservedCredits,
      actual_credits: credits,
    });
    return 'settled';
  });
}

/** Pending rows have no verified usage after their proxy lease expires, so the
 * conservative recovery policy releases them instead of charging unknown work. */
export async function reconcileStaleReservations(
  limit = 100,
  now = new Date(),
): Promise<{ released: number }> {
  const rows = await db
    .select({
      requestId: creditReservations.requestId,
      userId: creditReservations.userId,
    })
    .from(creditReservations)
    .where(
      and(
        eq(creditReservations.status, 'pending'),
        lte(creditReservations.expiresAt, now),
      ),
    )
    .limit(Math.max(1, Math.min(limit, 500)));
  let released = 0;
  for (const row of rows) {
    const [dedup] = await db
      .select({
        idempotencyKey: requestDedup.idempotencyKey,
        requestHash: requestDedup.requestHash,
      })
      .from(requestDedup)
      .where(
        and(
          eq(requestDedup.userId, row.userId),
          eq(requestDedup.requestId, row.requestId),
          eq(requestDedup.status, 'in_flight'),
        ),
      )
      .limit(1);
    if (
      (await finalizeReservation({
        requestId: row.requestId,
        outcome: 'release',
        dedup: dedup?.requestHash
          ? {
              userId: row.userId,
              idempotencyKey: dedup.idempotencyKey,
              requestHash: dedup.requestHash,
              status: 'failed',
            }
          : undefined,
      })) === 'released'
    )
      released += 1;
  }
  if (released > 0)
    log.info('credit_reservations_stale_reconciled', { released });
  return { released };
}
