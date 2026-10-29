import { and, eq, gt, inArray, lte, sql } from 'drizzle-orm';
import { db } from '../db/client';
import {
  creditLedger,
  creditReservations,
  freeSpendMonthlyBudgets,
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
import {
  canReserveFreeSpend,
  canResizeFreeSpend,
  capHaltedAfterSettlement,
  ensureFreeSpendMonthlyBudget,
  type FreeSpendBudget,
  lockFreeSpendCapConfig,
  microcentsForFreePool,
  utcMonthKey,
} from './free-spend-cap';
import { log } from './log';
import {
  absorbedSettlementCredits,
  recordAbsorbedOverrunCredits,
} from './pricing-overrun-cap';
import { trustedReportedCostUsd } from './provider-pricing-contract';

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
      reason:
        | 'insufficient_credits'
        | 'request_already_reserved'
        | 'invalid_audio_preparation'
        | 'free_spend_cap_reached';
      balance: number;
      reserved: number;
    };

export type CreditPoolAllocation =
  | { ok: true; reservedFree: number; reservedPaid: number }
  | {
      ok: false;
      reason: 'insufficient_credits' | 'free_spend_cap_reached';
    };

export function allocateCreditsAcrossPools(input: {
  credits: number;
  freeAvailable: number;
  paidAvailable: number;
  freeSpendAllowed: boolean;
}): CreditPoolAllocation {
  const reservedFree = Math.min(
    input.credits,
    Math.max(0, input.freeAvailable),
  );
  const reservedPaid = input.credits - reservedFree;
  if (reservedPaid > Math.max(0, input.paidAvailable)) {
    return { ok: false, reason: 'insufficient_credits' };
  }
  if (reservedFree > 0 && !input.freeSpendAllowed) {
    if (input.credits <= Math.max(0, input.paidAvailable)) {
      return {
        ok: true,
        reservedFree: 0,
        reservedPaid: input.credits,
      };
    }
    return { ok: false, reason: 'free_spend_cap_reached' };
  }
  return { ok: true, reservedFree, reservedPaid };
}

export async function reserveCredits(input: {
  requestId: string;
  userId: string;
  kind?: CreditKind;
  credits: number;
  estimatedProviderCostUsd: number;
  estimateCostSource: 'provider_reported' | 'catalog_estimate';
  now?: Date;
  allowExistingPending?: boolean;
}): Promise<ReservationResult> {
  return reserveCreditsInternal(input);
}

type ReserveCreditInput = Parameters<typeof reserveCredits>[0];
type AudioReservationInput = {
  reservationId: string;
  operationId?: string;
  userId: string;
  /** The configured audio charge ceiling; preparation and admission must match. */
  credits: number;
  now?: Date;
};

/** Untagged legacy rows are deliberately not reusable as audio preparation. */
export function isAudioPreparation(
  reservation: typeof creditReservations.$inferSelect,
  userId: string,
  credits: number,
  now: Date,
): boolean {
  return (
    reservation.userId === userId &&
    reservation.purpose === 'audio' &&
    reservation.audioState === 'prepared' &&
    reservation.status === 'pending' &&
    reservation.expiresAt.getTime() > now.getTime() &&
    reservation.reservedCredits === credits &&
    reservation.estimatedTotalCredits === credits &&
    reservation.admissionMonth === null &&
    reservation.estimatedProviderCostMicrocents === null &&
    reservation.estimateCostSource === null &&
    reservation.reservedFreeCostMicrocents === 0 &&
    reservation.reservedFreeCredits !== null &&
    reservation.reservedPaidCredits !== null &&
    reservation.reservedFreeCredits >= 0 &&
    reservation.reservedPaidCredits >= 0 &&
    reservation.reservedFreeCredits + reservation.reservedPaidCredits ===
      credits
  );
}

function validateAudioCreditCeiling(credits: number) {
  if (!Number.isSafeInteger(credits) || credits !== 5)
    throw new Error(
      'Audio reservations require the configured five-credit ceiling',
    );
}

function isReleasedAudioCancellation(
  reservation: typeof creditReservations.$inferSelect,
): boolean {
  return (
    reservation.purpose === 'audio' &&
    reservation.audioState === 'cancelled' &&
    reservation.status === 'released'
  );
}

function isActiveAudioCancellation(
  reservation: typeof creditReservations.$inferSelect,
  now: Date,
): boolean {
  return (
    isReleasedAudioCancellation(reservation) &&
    reservation.expiresAt.getTime() > now.getTime()
  );
}

/** Credit-only preparation is not authorization to contact a provider. */
export async function prepareAudioReservation(input: AudioReservationInput) {
  validateAudioCreditCeiling(input.credits);
  const result = await reserveCreditsInternal(
    {
      requestId: input.reservationId,
      userId: input.userId,
      credits: input.credits,
      estimatedProviderCostUsd: 0,
      estimateCostSource: 'catalog_estimate',
      allowExistingPending: true,
      now: input.now,
    },
    true,
    input.operationId ?? input.reservationId,
  );
  return result.ok
    ? { ...result, reservationId: result.reservation.requestId }
    : result;
}

async function reserveCreditsInternal(
  input: ReserveCreditInput,
  audioPreparation = false,
  audioOperationId = input.requestId,
): Promise<ReservationResult> {
  let now = input.now ?? new Date();
  const credits = Math.max(1, Math.ceil(input.credits));
  const estimateCostMicrocents = usdToMicrocents(
    input.estimatedProviderCostUsd,
  );
  if (
    !Number.isSafeInteger(estimateCostMicrocents) ||
    estimateCostMicrocents < 0
  ) {
    throw new Error(
      'Reservation requires a safe non-negative provider-cost estimate',
    );
  }
  if (estimateCostMicrocents > 0 && credits <= 0) {
    throw new Error(
      'Positive provider-cost estimate requires an estimated credit allocation',
    );
  }
  const monthKey = utcMonthKey(now);
  return db.transaction(async (tx) => {
    const config = audioPreparation
      ? null
      : await lockFreeSpendCapConfig(tx, 'share');
    const budget = config
      ? await ensureFreeSpendMonthlyBudget(tx, monthKey, config.capMicrocents)
      : null;
    // Locking the parent user serializes all admission decisions for that user
    // without blocking unrelated users.
    const [user] = await tx
      .select({ id: users.id })
      .from(users)
      .where(eq(users.id, input.userId))
      .limit(1)
      .for('update');
    if (!user) throw new Error('Cannot reserve credits for unknown user');
    if (audioPreparation && input.now === undefined) now = new Date();

    if (audioPreparation && audioOperationId !== input.requestId) {
      const [cancellation] = await tx
        .select()
        .from(creditReservations)
        .where(
          and(
            eq(creditReservations.requestId, audioOperationId),
            eq(creditReservations.userId, input.userId),
            eq(creditReservations.status, 'released'),
            eq(creditReservations.purpose, 'audio'),
            eq(creditReservations.audioState, 'cancelled'),
            gt(creditReservations.expiresAt, now),
          ),
        )
        .limit(1)
        .for('update');
      if (cancellation && isActiveAudioCancellation(cancellation, now))
        return {
          ok: false,
          reason: 'invalid_audio_preparation',
          balance: 0,
          reserved: 0,
        };
    }

    let existing: typeof creditReservations.$inferSelect | undefined;
    [existing] = await tx
      .select()
      .from(creditReservations)
      .where(eq(creditReservations.requestId, input.requestId))
      .limit(1)
      .for('update');
    if (audioPreparation && existing && isReleasedAudioCancellation(existing)) {
      if (existing.expiresAt.getTime() <= now.getTime()) {
        await tx
          .delete(creditReservations)
          .where(eq(creditReservations.requestId, input.requestId));
        existing = undefined;
      } else if (audioOperationId !== input.requestId) {
        return {
          ok: false,
          reason: 'invalid_audio_preparation',
          balance: 0,
          reserved: existing.reservedCredits,
        };
      }
    }
    if (existing) {
      if (audioPreparation) {
        if (!isAudioPreparation(existing, input.userId, credits, now))
          return {
            ok: false,
            reason: 'invalid_audio_preparation',
            balance: 0,
            reserved: existing.reservedCredits,
          };
        return {
          ok: true,
          reservation: existing,
          balance: 0,
          reserved: credits,
        };
      }
      if (existing.purpose === 'audio')
        return {
          ok: false,
          reason: 'request_already_reserved',
          balance: 0,
          reserved: existing.reservedCredits,
        };
      if (
        existing.userId !== input.userId ||
        existing.reservedCredits !== credits
      ) {
        throw new Error(
          'Reservation request ID was reused with different billing parameters',
        );
      }
      const legacyPending =
        existing.status === 'pending' &&
        existing.admissionMonth == null &&
        existing.reservedFreeCredits == null &&
        existing.reservedPaidCredits == null &&
        existing.estimatedProviderCostMicrocents == null &&
        existing.estimatedTotalCredits == null &&
        existing.estimateCostSource == null &&
        existing.reservedFreeCostMicrocents == null;
      if (legacyPending && input.allowExistingPending) {
        return {
          ok: true,
          reservation: existing,
          balance: 0,
          reserved: existing.reservedCredits,
        };
      }
      if (
        existing.admissionMonth == null ||
        existing.estimatedProviderCostMicrocents !== estimateCostMicrocents ||
        existing.estimatedTotalCredits !== credits ||
        existing.estimateCostSource !== input.estimateCostSource ||
        existing.reservedFreeCredits == null ||
        existing.reservedPaidCredits == null ||
        existing.reservedFreeCredits + existing.reservedPaidCredits !==
          credits ||
        existing.reservedFreeCostMicrocents !==
          microcentsForFreePool({
            totalCostMicrocents: estimateCostMicrocents,
            freeCredits: existing.reservedFreeCredits ?? 0,
            totalCredits: credits,
          })
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
    if (freeBalance < 0 || paidBalance < 0) {
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

    const candidateFree = Math.min(credits, Math.max(0, freeAvailable));
    const candidateFreeCost = microcentsForFreePool({
      totalCostMicrocents: estimateCostMicrocents,
      freeCredits: candidateFree,
      totalCredits: credits,
    });
    const freeSpendAllowed =
      audioPreparation ||
      candidateFree === 0 ||
      canReserveFreeSpend({
        budget: budget!,
        requestedMicrocents: candidateFreeCost,
      });
    const allocation = allocateCreditsAcrossPools({
      credits,
      freeAvailable,
      paidAvailable,
      freeSpendAllowed,
    });
    if (!allocation.ok) {
      if (allocation.reason === 'insufficient_credits') {
        const balance = freeBalance + paidBalance;
        const reserved = (active?.free ?? 0) + (active?.paid ?? 0);
        log.warn('credit_reservation_rejected', {
          request_id: input.requestId,
          user_id: input.userId,
          credits,
          balance,
          reserved,
        });
        return { ok: false, reason: allocation.reason, balance, reserved };
      }
      return {
        ok: false,
        reason: allocation.reason,
        balance: freeBalance + paidBalance,
        reserved: (active?.free ?? 0) + (active?.paid ?? 0),
      };
    }
    const { reservedFree, reservedPaid } = allocation;
    const kind = reservedFree > 0 ? 'free' : 'paid';
    const reservedFreeCostMicrocents = microcentsForFreePool({
      totalCostMicrocents: estimateCostMicrocents,
      freeCredits: reservedFree,
      totalCredits: credits,
    });

    const [reservation] = await tx
      .insert(creditReservations)
      .values({
        requestId: input.requestId,
        userId: input.userId,
        kind,
        reservedCredits: credits,
        reservedFreeCredits: reservedFree,
        reservedPaidCredits: reservedPaid,
        purpose: audioPreparation ? 'audio' : null,
        audioState: audioPreparation ? 'prepared' : null,
        admissionMonth: audioPreparation ? null : monthKey,
        estimatedProviderCostMicrocents: audioPreparation
          ? null
          : estimateCostMicrocents,
        estimatedTotalCredits: credits,
        estimateCostSource: audioPreparation ? null : input.estimateCostSource,
        reservedFreeCostMicrocents,
        expiresAt: new Date(now.getTime() + RESERVATION_TTL_MS),
      })
      .returning();
    if (reservedFreeCostMicrocents > 0) {
      await tx
        .update(freeSpendMonthlyBudgets)
        .set({
          reservedFreeCostMicrocents: sql`${freeSpendMonthlyBudgets.reservedFreeCostMicrocents} + ${reservedFreeCostMicrocents}`,
          updatedAt: now,
        })
        .where(eq(freeSpendMonthlyBudgets.monthKey, monthKey));
    }
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
    .select()
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
  return Boolean(
    reservation &&
      isAudioPreparation(
        reservation,
        input.userId,
        Math.max(1, Math.ceil(input.credits)),
        new Date(),
      ),
  );
}

export type AudioPreparationStatus =
  | 'prepared'
  | 'cancelled'
  | 'unavailable'
  | 'missing';

export function classifyAudioPreparationStatus(
  reservation: typeof creditReservations.$inferSelect | undefined,
  input: { userId: string; credits: number },
  now: Date,
): AudioPreparationStatus {
  if (!reservation) return 'missing';
  if (reservation.userId !== input.userId) return 'unavailable';
  if (isAudioPreparation(reservation, input.userId, input.credits, now))
    return 'prepared';
  if (reservation.purpose === 'audio' && reservation.audioState === 'cancelled')
    return reservation.expiresAt.getTime() > now.getTime()
      ? 'cancelled'
      : 'missing';
  return reservation.status === 'pending' ? 'unavailable' : 'missing';
}

/** A read-only hint, not dispatch authorization; admission revalidates under lock. */
export async function getAudioPreparationStatus(input: {
  requestId: string;
  userId: string;
  credits: number;
  operationId?: string;
}): Promise<AudioPreparationStatus> {
  const [reservation] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, input.requestId))
    .limit(1);
  const now = new Date();
  const status = classifyAudioPreparationStatus(reservation, input, now);
  if (
    status !== 'missing' ||
    !input.operationId ||
    input.operationId === input.requestId
  )
    return status;
  const [cancellation] = await db
    .select({ requestId: creditReservations.requestId })
    .from(creditReservations)
    .where(
      and(
        eq(creditReservations.requestId, input.operationId),
        eq(creditReservations.userId, input.userId),
        eq(creditReservations.status, 'released'),
        eq(creditReservations.purpose, 'audio'),
        eq(creditReservations.audioState, 'cancelled'),
        gt(creditReservations.expiresAt, now),
      ),
    )
    .limit(1);
  return cancellation ? 'cancelled' : status;
}

export async function getPendingCreditReservations(
  userId: string,
): Promise<{ free: number; paid: number }> {
  const [reserved] = await db
    .select({
      free: sql<number>`coalesce(sum(coalesce(${creditReservations.reservedFreeCredits}, case when ${creditReservations.kind} = 'free' then ${creditReservations.reservedCredits} else 0 end)), 0)::int`,
      paid: sql<number>`coalesce(sum(coalesce(${creditReservations.reservedPaidCredits}, case when ${creditReservations.kind} = 'paid' then ${creditReservations.reservedCredits} else 0 end)), 0)::int`,
    })
    .from(creditReservations)
    .where(
      and(
        eq(creditReservations.userId, userId),
        eq(creditReservations.status, 'pending'),
      ),
    );
  return { free: reserved?.free ?? 0, paid: reserved?.paid ?? 0 };
}

async function lockReservationAndBudget(
  tx: Parameters<Parameters<typeof db.transaction>[0]>[0],
  requestId: string,
  legacySettlementMonth?: string,
): Promise<{
  reservation: typeof creditReservations.$inferSelect;
  budget: FreeSpendBudget | null;
}> {
  const [snapshot] = await tx
    .select({
      userId: creditReservations.userId,
      admissionMonth: creditReservations.admissionMonth,
    })
    .from(creditReservations)
    .where(eq(creditReservations.requestId, requestId))
    .limit(1);
  if (!snapshot) throw new Error(`Missing credit reservation ${requestId}`);

  let budget: FreeSpendBudget | null = null;
  const budgetMonth = snapshot.admissionMonth ?? legacySettlementMonth;
  if (budgetMonth) {
    const config = await lockFreeSpendCapConfig(tx, 'share');
    if (snapshot.admissionMonth) {
      const [lockedBudget] = await tx
        .select()
        .from(freeSpendMonthlyBudgets)
        .where(eq(freeSpendMonthlyBudgets.monthKey, budgetMonth))
        .for('update');
      if (!lockedBudget)
        throw new Error(`Missing monthly free-spend budget for ${budgetMonth}`);
      budget = lockedBudget;
    } else {
      budget = await ensureFreeSpendMonthlyBudget(
        tx,
        budgetMonth,
        config.capMicrocents,
      );
    }
  }

  const reservation = await lockReservationAfterUser(tx, requestId);
  if (
    reservation.userId !== snapshot.userId ||
    reservation.admissionMonth !== snapshot.admissionMonth
  ) {
    throw new Error(
      `Credit reservation identity changed while locking ${requestId}`,
    );
  }
  return { reservation, budget };
}

export type ReservationFinalization = {
  requestId: string;
  expectedUserId?: string;
  /** Reconciliation must recheck the current lease under lock before releasing. */
  expectedExpiredAtBefore?: Date;
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
  /** Upstream-reported cost; settles only when the provider contract trusts it. */
  providerCostUsd?: number;
  /**
   * Catalog-derived cost chosen by the caller, such as a reservation ceiling kept
   * when usage is missing. It is never labelled as provider-reported.
   */
  catalogCostUsd?: number;
  modality?: 'text' | 'audio';
  audioDurationMs?: number;
  audioTokenCountsAvailable?: boolean;
  costSource?: 'provider_reported' | 'catalog_estimate';
  routeSlot?: string;
  status?: number;
  latencyMs?: number;
  ttftMs?: number;
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
  estimatedProviderCostUsd?: number,
  estimateCostSource:
    | 'provider_reported'
    | 'catalog_estimate' = 'catalog_estimate',
): Promise<ReservationResult> {
  return resizeReservationInternal(
    requestId,
    creditsInput,
    estimatedProviderCostUsd,
    estimateCostSource,
  );
}

/** Success claims exactly one dispatch. Never dispatch on preparation alone or retry a successful claim. */
export async function admitAudioReservation(
  input: AudioReservationInput & {
    estimatedProviderCostUsd: number;
    estimateCostSource: 'provider_reported' | 'catalog_estimate';
  },
) {
  validateAudioCreditCeiling(input.credits);
  if (
    !Number.isFinite(input.estimatedProviderCostUsd) ||
    input.estimatedProviderCostUsd < 0
  )
    throw new Error(
      'Audio admission requires a known non-negative provider estimate',
    );
  const result = await resizeReservationInternal(
    input.reservationId,
    input.credits,
    input.estimatedProviderCostUsd,
    input.estimateCostSource,
    input,
  );
  return result.ok
    ? { ...result, reservationId: result.reservation.requestId }
    : result;
}

async function resizeReservationInternal(
  requestId: string,
  creditsInput: number,
  estimatedProviderCostUsd?: number,
  estimateCostSource:
    | 'provider_reported'
    | 'catalog_estimate' = 'catalog_estimate',
  audioAdmission?: AudioReservationInput,
): Promise<ReservationResult> {
  const credits = Math.max(1, Math.ceil(creditsInput));
  let now = audioAdmission?.now ?? new Date();
  return db.transaction(async (tx) => {
    let locked: Awaited<ReturnType<typeof lockReservationAndBudget>>;
    if (audioAdmission) {
      const config = await lockFreeSpendCapConfig(tx, 'share');
      const budget = await ensureFreeSpendMonthlyBudget(
        tx,
        utcMonthKey(now),
        config.capMicrocents,
      );
      const [user] = await tx
        .select({ id: users.id })
        .from(users)
        .where(eq(users.id, audioAdmission.userId))
        .for('update');
      if (!user)
        return {
          ok: false,
          reason: 'invalid_audio_preparation',
          balance: 0,
          reserved: 0,
        };
      const operationId = audioAdmission.operationId ?? requestId;
      if (operationId !== requestId) {
        const [cancellation] = await tx
          .select()
          .from(creditReservations)
          .where(
            and(
              eq(creditReservations.requestId, operationId),
              eq(creditReservations.userId, audioAdmission.userId),
              eq(creditReservations.status, 'released'),
              eq(creditReservations.purpose, 'audio'),
              eq(creditReservations.audioState, 'cancelled'),
              gt(creditReservations.expiresAt, now),
            ),
          )
          .limit(1)
          .for('update');
        if (cancellation && isActiveAudioCancellation(cancellation, now))
          return {
            ok: false,
            reason: 'invalid_audio_preparation',
            balance: 0,
            reserved: 0,
          };
      }
      const [reservation] = await tx
        .select()
        .from(creditReservations)
        .where(
          and(
            eq(creditReservations.requestId, requestId),
            eq(creditReservations.userId, audioAdmission.userId),
          ),
        )
        .for('update');
      if (audioAdmission.now === undefined) now = new Date();
      if (
        utcMonthKey(now) !== budget.monthKey ||
        !reservation ||
        !isAudioPreparation(reservation, audioAdmission.userId, credits, now)
      )
        return {
          ok: false,
          reason: 'invalid_audio_preparation',
          balance: 0,
          reserved: reservation?.reservedCredits ?? 0,
        };
      locked = {
        reservation: { ...reservation, admissionMonth: budget.monthKey },
        budget,
      };
    } else {
      locked = await lockReservationAndBudget(tx, requestId);
      if (locked.reservation.purpose === 'audio')
        return {
          ok: false,
          reason: 'request_already_reserved',
          balance: 0,
          reserved: locked.reservation.reservedCredits,
        };
    }
    const { reservation, budget } = locked;
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
    const candidateFree = Math.min(credits, Math.max(0, freeAvailable));
    const totalCostMicrocents =
      estimatedProviderCostUsd === undefined
        ? (reservation.estimatedProviderCostMicrocents ?? 0)
        : usdToMicrocents(estimatedProviderCostUsd);
    if (!Number.isSafeInteger(totalCostMicrocents) || totalCostMicrocents < 0) {
      throw new Error(
        'Reservation resize requires a safe provider-cost estimate',
      );
    }
    const oldFreeCost = reservation.reservedFreeCostMicrocents ?? 0;
    const candidateFreeCost = reservation.admissionMonth
      ? microcentsForFreePool({
          totalCostMicrocents,
          freeCredits: candidateFree,
          totalCredits: credits,
        })
      : 0;
    const balance = freeBalance + paidBalance;
    const reservedByOthers = freeReserved + paidReserved;
    if (freeBalance < 0 || paidBalance < 0) {
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
    const freeSpendAllowed =
      !reservation.admissionMonth ||
      candidateFree === 0 ||
      Boolean(
        budget &&
          (audioAdmission
            ? canReserveFreeSpend({
                budget,
                requestedMicrocents: candidateFreeCost,
              })
            : canResizeFreeSpend({
                budget,
                previousFreeCredits: oldFree,
                nextFreeCredits: candidateFree,
                previousHoldMicrocents: oldFreeCost,
                nextHoldMicrocents: candidateFreeCost,
              })),
      );
    const allocation = allocateCreditsAcrossPools({
      credits,
      freeAvailable,
      paidAvailable,
      freeSpendAllowed,
    });
    if (!allocation.ok) {
      if (allocation.reason === 'insufficient_credits') {
        log.warn('credit_reservation_resize_rejected', {
          request_id: requestId,
          user_id: reservation.userId,
          credits,
          balance,
          reserved: reservedByOthers,
        });
      }
      return {
        ok: false,
        reason: allocation.reason,
        balance,
        reserved: reservedByOthers,
      };
    }
    const { reservedFree, reservedPaid } = allocation;
    const newFreeCost = reservation.admissionMonth
      ? microcentsForFreePool({
          totalCostMicrocents,
          freeCredits: reservedFree,
          totalCredits: credits,
        })
      : 0;
    const [updated] = await tx
      .update(creditReservations)
      .set({
        kind: reservedFree > 0 ? 'free' : 'paid',
        reservedCredits: credits,
        reservedFreeCredits: reservedFree,
        reservedPaidCredits: reservedPaid,
        estimatedProviderCostMicrocents: totalCostMicrocents,
        estimatedTotalCredits: credits,
        estimateCostSource,
        reservedFreeCostMicrocents: newFreeCost,
        ...(audioAdmission
          ? {
              admissionMonth: reservation.admissionMonth,
              audioState: 'dispatched',
              expiresAt: new Date(now.getTime() + RESERVATION_TTL_MS),
            }
          : {}),
        updatedAt: now,
      })
      .where(eq(creditReservations.requestId, requestId))
      .returning();
    const freeCostDifference = newFreeCost - oldFreeCost;
    if (freeCostDifference !== 0 && reservation.admissionMonth) {
      const nextReservedCost =
        budget!.reservedFreeCostMicrocents + freeCostDifference;
      await tx
        .update(freeSpendMonthlyBudgets)
        .set({
          reservedFreeCostMicrocents: nextReservedCost,
          halted: capHaltedAfterSettlement({
            capMicrocents: budget!.capMicrocents,
            actualFreeCostMicrocents: budget!.actualFreeCostMicrocents,
            reservedFreeCostMicrocents: nextReservedCost,
          }),
          updatedAt: new Date(),
        })
        .where(
          eq(freeSpendMonthlyBudgets.monthKey, reservation.admissionMonth),
        );
    }
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
): Promise<'settled' | 'released' | 'already_terminal' | 'not_expired'> {
  const result = await finalizeReservationInternal(input);
  if (result === 'not_prepared')
    throw new Error('Unexpected preparation-only finalization');
  return result;
}

/** Cancellation releases provisional holds or tombstones an earlier cancel. */
export async function cancelAudioPreparation(input: {
  reservationId: string;
  userId: string;
  reason?: 'cancelled' | 'failed';
  tombstoneMissing?: boolean;
}): Promise<'released' | 'already_terminal' | 'not_prepared'> {
  const result = await finalizeReservationInternal(
    {
      requestId: input.reservationId,
      expectedUserId: input.userId,
      outcome: 'release',
    },
    true,
    input.reason ?? 'failed',
    input.tombstoneMissing ?? false,
  );
  if (result === 'settled' || result === 'not_expired')
    throw new Error('Unexpected audio preparation cancellation result');
  return result;
}

async function finalizeReservationInternal(
  input: ReservationFinalization,
  onlyAudioPreparation = false,
  cancellationReason: 'cancelled' | 'failed' = 'failed',
  tombstoneMissing = false,
): Promise<
  'settled' | 'released' | 'already_terminal' | 'not_prepared' | 'not_expired'
> {
  if (
    input.expectedExpiredAtBefore &&
    (input.outcome !== 'release' ||
      !Number.isFinite(input.expectedExpiredAtBefore.getTime()))
  )
    throw new Error('Expiry reconciliation requires a valid release cutoff');
  return db.transaction(async (tx) => {
    // Every reservation transition takes row locks in the same order: parent
    // user first, reservation second. This matches admission and prevents a
    // same-request admission/finalization cycle through the FK-backed rows.
    const now = new Date();
    const legacySettlementMonth =
      input.outcome === 'settle' ? utcMonthKey(now) : undefined;
    if (onlyAudioPreparation) {
      if (!input.expectedUserId)
        throw new Error('Audio preparation cancellation requires an owner');
      await lockFreeSpendCapConfig(tx, 'share');
      const [user] = await tx
        .select({ id: users.id })
        .from(users)
        .where(eq(users.id, input.expectedUserId))
        .limit(1)
        .for('update');
      if (!user) throw new Error('Cannot cancel preparation for unknown user');
      const [existing] = await tx
        .select()
        .from(creditReservations)
        .where(eq(creditReservations.requestId, input.requestId))
        .limit(1)
        .for('update');
      if (!existing) {
        if (cancellationReason !== 'cancelled' || !tombstoneMissing)
          return 'not_prepared';
        const [tombstone] = await tx
          .insert(creditReservations)
          .values({
            requestId: input.requestId,
            userId: input.expectedUserId,
            kind: 'free',
            reservedCredits: 0,
            status: 'released',
            purpose: 'audio',
            audioState: 'cancelled',
            actualCredits: 0,
            reservedFreeCredits: 0,
            reservedPaidCredits: 0,
            actualFreeCredits: 0,
            actualPaidCredits: 0,
            estimatedProviderCostMicrocents: null,
            estimatedTotalCredits: null,
            estimateCostSource: null,
            reservedFreeCostMicrocents: 0,
            actualFreeCostMicrocents: 0,
            expiresAt: new Date(now.getTime() + RESERVATION_TTL_MS),
            releasedAt: now,
            updatedAt: now,
          })
          .onConflictDoNothing({ target: creditReservations.requestId })
          .returning({ requestId: creditReservations.requestId });
        if (tombstone) return 'released';
        const [conflict] = await tx
          .select()
          .from(creditReservations)
          .where(eq(creditReservations.requestId, input.requestId))
          .limit(1)
          .for('update');
        if (!conflict || conflict.userId !== input.expectedUserId)
          throw new Error(`User does not own reservation ${input.requestId}`);
        if (conflict.purpose === 'audio' && conflict.audioState === 'cancelled')
          return 'already_terminal';
        return 'not_prepared';
      }
      if (existing.userId !== input.expectedUserId)
        throw new Error(`User does not own reservation ${input.requestId}`);
      if (existing.status !== 'pending') {
        if (
          cancellationReason === 'cancelled' &&
          input.outcome === 'release' &&
          existing.purpose === 'audio' &&
          existing.status === 'released' &&
          (existing.audioState === 'failed' ||
            (existing.audioState === 'cancelled' &&
              existing.expiresAt.getTime() <= now.getTime()))
        ) {
          await tx
            .update(creditReservations)
            .set({
              audioState: 'cancelled',
              expiresAt: new Date(now.getTime() + RESERVATION_TTL_MS),
              updatedAt: now,
            })
            .where(eq(creditReservations.requestId, input.requestId));
          return 'released';
        }
        return 'already_terminal';
      }
      if (existing.purpose !== 'audio' || existing.audioState !== 'prepared')
        return 'not_prepared';
    }
    const { reservation, budget } = await lockReservationAndBudget(
      tx,
      input.requestId,
      legacySettlementMonth,
    );
    if (input.expectedUserId && input.expectedUserId !== reservation.userId) {
      throw new Error(`User does not own reservation ${input.requestId}`);
    }
    if (input.dedup && input.dedup.userId !== reservation.userId) {
      throw new Error(
        `Idempotency claim user does not own reservation ${input.requestId}`,
      );
    }
    if (reservation.status !== 'pending') return 'already_terminal';
    if (
      input.expectedExpiredAtBefore &&
      reservation.expiresAt.getTime() > input.expectedExpiredAtBefore.getTime()
    )
      return 'not_expired';
    if (
      onlyAudioPreparation &&
      (reservation.purpose !== 'audio' || reservation.audioState !== 'prepared')
    )
      return 'not_prepared';
    if (
      input.outcome === 'settle' &&
      reservation.purpose === 'audio' &&
      reservation.audioState !== 'dispatched'
    )
      throw new Error('Audio settlement requires an admitted dispatch claim');
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
          audioTokenCountsAvailable: input.audioTokenCountsAvailable ?? false,
          costCredits: 0,
          upstreamCostMicrocents: 0,
          upstreamCostSource: 'not_billed',
          costOverrun: false,
          routeSlot: input.routeSlot ?? null,
          requestId: reservation.requestId,
          status: input.status,
          latencyMs: input.latencyMs,
          ttftMs: input.ttftMs ?? null,
        });
      }
      await tx
        .update(creditReservations)
        .set({
          status: 'released',
          ...(onlyAudioPreparation
            ? {
                audioState: cancellationReason,
                ...(cancellationReason === 'cancelled'
                  ? { expiresAt: new Date(now.getTime() + RESERVATION_TTL_MS) }
                  : {}),
              }
            : {}),
          actualCredits: 0,
          actualFreeCredits: 0,
          actualPaidCredits: 0,
          reservedFreeCostMicrocents: reservation.admissionMonth
            ? 0
            : reservation.reservedFreeCostMicrocents,
          actualFreeCostMicrocents: reservation.admissionMonth ? 0 : null,
          releasedAt: now,
          updatedAt: now,
        })
        .where(eq(creditReservations.requestId, input.requestId));
      if (reservation.admissionMonth) {
        if (!budget)
          throw new Error(
            `Missing monthly free-spend budget for ${reservation.admissionMonth}`,
          );
        const held = reservation.reservedFreeCostMicrocents;
        if (held == null || held > budget.reservedFreeCostMicrocents) {
          throw new Error(
            `Invalid free-spend reservation hold for ${input.requestId}`,
          );
        }
        const remainingReserved = budget.reservedFreeCostMicrocents - held;
        await tx
          .update(freeSpendMonthlyBudgets)
          .set({
            reservedFreeCostMicrocents: remainingReserved,
            halted: capHaltedAfterSettlement({
              capMicrocents: budget.capMicrocents,
              actualFreeCostMicrocents: budget.actualFreeCostMicrocents,
              reservedFreeCostMicrocents: remainingReserved,
            }),
            updatedAt: now,
          })
          .where(
            eq(freeSpendMonthlyBudgets.monthKey, reservation.admissionMonth),
          );
      }
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
      (input.costRow == null &&
        input.providerCostUsd == null &&
        input.catalogCostUsd == null) ||
      input.status == null ||
      input.latencyMs == null
    ) {
      throw new Error('Settlement requires usage details');
    }
    const reportedCostUsd = trustedReportedCostUsd(
      input.provider,
      input.providerCostUsd,
    );
    const hasProviderCost = reportedCostUsd !== undefined;
    const costRow = input.costRow;
    let costUsd: number;
    if (reportedCostUsd !== undefined) {
      costUsd = reportedCostUsd;
    } else if (input.catalogCostUsd !== undefined) {
      costUsd = input.catalogCostUsd;
    } else if (costRow) {
      costUsd = computeUsdCost(
        costRow,
        input.tokens.inputTokens,
        input.tokens.outputTokens,
        input.tokens,
      );
    } else {
      throw new Error('Settlement requires a valid cost source');
    }
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
    const actualTotalCredits = actualFree + actualPaid;
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
    if (!Number.isSafeInteger(microcents) || microcents < 0) {
      throw new Error('Settlement provider cost exceeds safe integer storage');
    }
    const actualFreeCostMicrocents = reservation.admissionMonth
      ? microcentsForFreePool({
          totalCostMicrocents: microcents,
          freeCredits: actualFree,
          totalCredits: actualTotalCredits,
        })
      : null;
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
    if (actualTotalCredits < credits)
      log.warn('credit_settlement_debit_limited', {
        request_id: reservation.requestId,
        user_id: reservation.userId,
        credits,
        actual_credits: actualTotalCredits,
        free_balance: Math.max(0, balances?.free ?? 0),
        paid_balance: Math.max(0, balances?.paid ?? 0),
      });
    await recordAbsorbedOverrunCredits(
      tx,
      absorbedSettlementCredits(calculatedCredits, actualTotalCredits),
      now,
    );
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
      audioTokenCountsAvailable: input.audioTokenCountsAvailable ?? false,
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
      ttftMs: input.ttftMs ?? null,
    });
    await tx
      .update(creditReservations)
      .set({
        status: 'settled',
        actualCredits: actualFree + actualPaid,
        actualFreeCredits: actualFree,
        actualPaidCredits: actualPaid,
        reservedFreeCostMicrocents: reservation.admissionMonth
          ? 0
          : reservation.reservedFreeCostMicrocents,
        actualFreeCostMicrocents,
        settledAt: now,
        updatedAt: now,
      })
      .where(eq(creditReservations.requestId, input.requestId));
    if (reservation.admissionMonth) {
      if (!budget)
        throw new Error(
          `Missing monthly free-spend budget for ${reservation.admissionMonth}`,
        );
      const held = reservation.reservedFreeCostMicrocents;
      if (
        held == null ||
        held > budget.reservedFreeCostMicrocents ||
        actualFreeCostMicrocents == null
      ) {
        throw new Error(
          `Invalid free-spend reservation hold for ${input.requestId}`,
        );
      }
      const actualFreeCost =
        budget.actualFreeCostMicrocents + actualFreeCostMicrocents;
      const remainingReserved = budget.reservedFreeCostMicrocents - held;
      await tx
        .update(freeSpendMonthlyBudgets)
        .set({
          actualFreeCostMicrocents: actualFreeCost,
          reservedFreeCostMicrocents: remainingReserved,
          halted: capHaltedAfterSettlement({
            capMicrocents: budget.capMicrocents,
            actualFreeCostMicrocents: actualFreeCost,
            reservedFreeCostMicrocents: remainingReserved,
          }),
          updatedAt: now,
        })
        .where(
          eq(freeSpendMonthlyBudgets.monthKey, reservation.admissionMonth),
        );
    } else if (budget) {
      await tx
        .update(freeSpendMonthlyBudgets)
        .set({
          baselineComplete: false,
          halted: budget.capMicrocents != null,
          updatedAt: now,
        })
        .where(eq(freeSpendMonthlyBudgets.monthKey, budget.monthKey));
    }
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
  const boundedLimit = Math.max(1, Math.min(limit, 500));
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
    .limit(boundedLimit);
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
        expectedUserId: row.userId,
        outcome: 'release',
        expectedExpiredAtBefore: now,
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
  await pruneExpiredAudioCancellationTombstones(limit, now);
  if (released > 0)
    log.info('credit_reservations_stale_reconciled', { released });
  return { released };
}

export async function pruneExpiredAudioCancellationTombstones(
  limit = 100,
  now = new Date(),
): Promise<number> {
  const boundedLimit = Math.max(1, Math.min(limit, 500));
  const expiredCancellationTombstones = await db
    .select({ requestId: creditReservations.requestId })
    .from(creditReservations)
    .where(
      and(
        eq(creditReservations.status, 'released'),
        eq(creditReservations.purpose, 'audio'),
        eq(creditReservations.audioState, 'cancelled'),
        eq(creditReservations.reservedCredits, 0),
        lte(creditReservations.expiresAt, now),
      ),
    )
    .limit(boundedLimit);
  if (expiredCancellationTombstones.length === 0) return 0;
  const deleted = await db
    .delete(creditReservations)
    .where(
      and(
        inArray(
          creditReservations.requestId,
          expiredCancellationTombstones.map(
            function tombstoneRequestId(tombstone) {
              return tombstone.requestId;
            },
          ),
        ),
        eq(creditReservations.status, 'released'),
        eq(creditReservations.purpose, 'audio'),
        eq(creditReservations.audioState, 'cancelled'),
        eq(creditReservations.reservedCredits, 0),
        lte(creditReservations.expiresAt, now),
      ),
    )
    .returning({ requestId: creditReservations.requestId });
  if (deleted.length > 0)
    log.info('audio_cancellation_tombstones_pruned', {
      deleted: deleted.length,
    });
  return deleted.length;
}
