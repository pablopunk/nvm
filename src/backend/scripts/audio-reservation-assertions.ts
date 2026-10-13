import assert from 'node:assert/strict';
import { and, eq, lte } from 'drizzle-orm';
import { db } from '../src/db/client';
import {
  creditLedger,
  creditReservations,
  freeSpendMonthlyBudgets,
  requestDedup,
  usage,
  users,
} from '../src/db/schema';
import { CREDIT_USD, MARKUP, usdToMicrocents } from '../src/lib/cost';
import {
  ensureFreeSpendMonthlyBudget,
  lockFreeSpendCapConfig,
  utcMonthKey,
} from '../src/lib/free-spend-cap';
import {
  admitAudioReservation,
  cancelAudioPreparation,
  finalizeReservation,
  getAudioPreparationStatus,
  prepareAudioReservation,
  pruneExpiredAudioCancellationTombstones,
  resizeReservation,
} from '../src/lib/credit-reservations';

/** Runs only inside the existing isolated PostgreSQL integration database. */
export async function runAudioReservationAssertions(suffix: string) {
  const [user] = await db
    .insert(users)
    .values({
      workosUserId: `audio-${suffix}`,
      email: `audio-${suffix}@example.test`,
    })
    .returning();
  await db.insert(creditLedger).values({
    userId: user.id,
    delta: 40,
    kind: 'free',
    reason: 'integration.audio_grant',
  });
  const now = new Date('2032-01-31T23:59:30Z');
  const input = { userId: user.id, credits: 5, now };
  const reservationId = `audio-operation-${suffix}`;
  const estimate = (CREDIT_USD * 5) / MARKUP;
  await db.insert(freeSpendMonthlyBudgets).values({
    monthKey: '2032-02',
    capMicrocents: null,
    baselineComplete: true,
  });
  const preparations = await Promise.all([
    prepareAudioReservation({ ...input, reservationId }),
    prepareAudioReservation({ ...input, reservationId }),
  ]);
  for (const preparation of preparations) {
    assert.ok(preparation.ok);
    assert.equal(preparation.reservationId, reservationId);
    assert.equal(preparation.reservation.estimatedProviderCostMicrocents, null);
    assert.equal(preparation.reservation.admissionMonth, null);
    assert.equal(preparation.reservation.reservedFreeCostMicrocents, 0);
  }
  const [preparationBudget] = await db
    .select()
    .from(freeSpendMonthlyBudgets)
    .where(eq(freeSpendMonthlyBudgets.monthKey, '2032-02'));
  assert.equal(preparationBudget.reservedFreeCostMicrocents, 0);
  assert.equal((await resizeReservation(reservationId, 5, estimate)).ok, false);
  await assert.rejects(
    () =>
      finalizeReservation({
        requestId: reservationId,
        outcome: 'settle',
      }),
    /Audio settlement requires/,
  );
  const admission = {
    ...input,
    reservationId,
    now: new Date('2032-02-01T00:00:00Z'),
    estimatedProviderCostUsd: estimate,
    estimateCostSource: 'catalog_estimate' as const,
  };
  const claims = await Promise.all([
    admitAudioReservation(admission),
    admitAudioReservation(admission),
  ]);
  assert.equal(
    claims.filter((claim) => claim.ok).length,
    1,
    'exactly one dispatch claim',
  );
  const winner = claims.find((claim) => claim.ok);
  assert.ok(winner?.ok);
  assert.equal(winner.reservationId, reservationId);
  assert.equal(winner.reservation.admissionMonth, '2032-02');
  assert.equal(winner.reservation.audioState, 'dispatched');
  assert.equal(
    await cancelAudioPreparation({ reservationId, userId: user.id }),
    'not_prepared',
  );
  const [claimedBudget] = await db
    .select()
    .from(freeSpendMonthlyBudgets)
    .where(eq(freeSpendMonthlyBudgets.monthKey, '2032-02'));
  assert.equal(
    claimedBudget.reservedFreeCostMicrocents,
    usdToMicrocents(estimate),
  );
  const settlement = {
    requestId: reservationId,
    expectedUserId: user.id,
    outcome: 'settle' as const,
    model: 'audio-test',
    provider: 'test',
    modality: 'audio' as const,
    providerCostUsd: estimate * 2,
    tokens: { inputTokens: 0, outputTokens: 0 },
    status: 200,
    latencyMs: 1,
  };
  const terminal = await Promise.all([
    finalizeReservation(settlement),
    finalizeReservation(settlement),
  ]);
  assert.deepEqual(terminal.sort(), ['already_terminal', 'settled']);
  const [settled] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, reservationId));
  assert.equal(
    settled.actualCredits,
    5,
    'settlement preserves the five-credit ceiling',
  );
  assert.equal(
    (await db.select().from(usage).where(eq(usage.requestId, reservationId)))
      .length,
    1,
  );
  const [settledBudget] = await db
    .select()
    .from(freeSpendMonthlyBudgets)
    .where(eq(freeSpendMonthlyBudgets.monthKey, '2032-02'));
  assert.equal(settledBudget.reservedFreeCostMicrocents, 0);
  assert.equal(
    settledBudget.actualFreeCostMicrocents,
    usdToMicrocents(estimate * 2),
  );

  const canceledId = `audio-canceled-${suffix}`;
  assert.ok(
    (await prepareAudioReservation({ ...input, reservationId: canceledId })).ok,
  );
  assert.equal(
    await cancelAudioPreparation({
      reservationId: canceledId,
      userId: user.id,
      reason: 'cancelled',
    }),
    'released',
  );
  assert.equal(
    await cancelAudioPreparation({
      reservationId: canceledId,
      userId: user.id,
      reason: 'cancelled',
    }),
    'already_terminal',
  );
  assert.equal(
    (await admitAudioReservation({ ...admission, reservationId: canceledId }))
      .ok,
    false,
  );
  assert.equal(
    await getAudioPreparationStatus({
      requestId: canceledId,
      userId: user.id,
      credits: 5,
    }),
    'cancelled',
  );
  assert.equal(
    await cancelAudioPreparation({
      reservationId: canceledId,
      userId: user.id,
      reason: 'failed',
    }),
    'already_terminal',
  );
  assert.equal(
    await getAudioPreparationStatus({
      requestId: canceledId,
      userId: user.id,
      credits: 5,
    }),
    'cancelled',
    'failure retry cannot erase cancellation',
  );

  const failedId = `audio-failed-preparation-${suffix}`;
  await prepareAudioReservation({ ...input, reservationId: failedId });
  const failedKey = `audio-failed-key-${suffix}`;
  await db.insert(requestDedup).values({
    userId: user.id,
    idempotencyKey: failedKey,
    requestId: failedId,
    requestHash: `audio-failed-hash-${suffix}`,
    status: 'in_flight',
  });
  assert.equal(
    await getAudioPreparationStatus({
      requestId: failedId,
      userId: user.id,
      credits: 5,
    }),
    'prepared',
  );
  assert.equal(
    await cancelAudioPreparation({
      reservationId: failedId,
      userId: user.id,
      reason: 'failed',
    }),
    'released',
  );
  const [failed] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, failedId));
  assert.equal(failed.audioState, 'failed');
  const [failureClaim] = await db
    .select()
    .from(requestDedup)
    .where(eq(requestDedup.idempotencyKey, failedKey));
  assert.equal(
    failureClaim.status,
    'in_flight',
    'cleanup commits before the caller publishes dedup failure',
  );
  assert.equal(
    await getAudioPreparationStatus({
      requestId: failedId,
      userId: user.id,
      credits: 5,
    }),
    'missing',
    'ordinary predispatch failure allows fresh fallback',
  );
  assert.equal(
    await cancelAudioPreparation({ reservationId: failedId, userId: user.id }),
    'already_terminal',
  );
  assert.equal(
    await getAudioPreparationStatus({
      requestId: failedId,
      userId: user.id,
      credits: 5,
    }),
    'missing',
    'terminal cleanup is idempotent without changing the original reason',
  );
  assert.equal(
    await getAudioPreparationStatus({
      requestId: failedId,
      userId: '00000000-0000-0000-0000-000000000001',
      credits: 5,
    }),
    'unavailable',
  );
  assert.equal(
    await getAudioPreparationStatus({
      requestId: `audio-missing-${suffix}`,
      userId: user.id,
      credits: 5,
    }),
    'missing',
  );

  const expiredId = `audio-expired-${suffix}`;
  const prepared = await prepareAudioReservation({
    ...input,
    reservationId: expiredId,
  });
  assert.ok(prepared.ok);
  assert.equal(
    (
      await admitAudioReservation({
        ...admission,
        reservationId: expiredId,
        now: prepared.reservation.expiresAt,
      })
    ).ok,
    false,
  );
  assert.equal(
    (
      await admitAudioReservation({
        ...admission,
        reservationId: expiredId,
        userId: '00000000-0000-0000-0000-000000000001',
      })
    ).ok,
    false,
  );
  await db
    .update(creditReservations)
    .set({ expiresAt: new Date(0) })
    .where(eq(creditReservations.requestId, expiredId));
  assert.equal(
    await getAudioPreparationStatus({
      requestId: expiredId,
      userId: user.id,
      credits: 5,
    }),
    'unavailable',
  );
  await cancelAudioPreparation({ reservationId: expiredId, userId: user.id });

  const legacyId = `audio-legacy-${suffix}`;
  await db.insert(creditReservations).values({
    requestId: legacyId,
    userId: user.id,
    kind: 'free',
    reservedCredits: 5,
    expiresAt: new Date('2032-02-01T00:05:00Z'),
  });
  assert.equal(
    (await admitAudioReservation({ ...admission, reservationId: legacyId })).ok,
    false,
  );
  assert.equal(
    await cancelAudioPreparation({ reservationId: legacyId, userId: user.id }),
    'not_prepared',
  );
  assert.equal(
    await getAudioPreparationStatus({
      requestId: legacyId,
      userId: user.id,
      credits: 5,
    }),
    'unavailable',
  );
  await finalizeReservation({ requestId: legacyId, outcome: 'release' });

  // A reconciliation snapshot can become stale while admission renews its lease.
  const staleId = `audio-renewed-lease-${suffix}`;
  const stalePreparation = await prepareAudioReservation({
    ...input,
    reservationId: staleId,
  });
  assert.ok(stalePreparation.ok);
  const cutoff = stalePreparation.reservation.expiresAt;
  const [selected] = await db
    .select()
    .from(creditReservations)
    .where(
      and(
        eq(creditReservations.requestId, staleId),
        eq(creditReservations.status, 'pending'),
        lte(creditReservations.expiresAt, cutoff),
      ),
    );
  assert.ok(selected);
  const renewed = await admitAudioReservation({
    ...admission,
    reservationId: staleId,
    now: new Date(cutoff.getTime() - 1),
  });
  assert.ok(renewed.ok);
  assert.ok(renewed.reservation.expiresAt.getTime() > cutoff.getTime());
  const dedup = {
    userId: user.id,
    idempotencyKey: `audio-renewed-key-${suffix}`,
    requestHash: `audio-renewed-hash-${suffix}`,
    status: 'failed' as const,
  };
  await db
    .insert(requestDedup)
    .values({ ...dedup, requestId: staleId, status: 'in_flight' });
  const [beforeReconciliation] = await db
    .select()
    .from(freeSpendMonthlyBudgets)
    .where(eq(freeSpendMonthlyBudgets.monthKey, '2032-02'));
  assert.equal(
    await finalizeReservation({
      requestId: selected.requestId,
      expectedUserId: user.id,
      outcome: 'release',
      expectedExpiredAtBefore: cutoff,
      dedup,
    }),
    'not_expired',
  );
  const [afterReconciliation] = await db
    .select()
    .from(freeSpendMonthlyBudgets)
    .where(eq(freeSpendMonthlyBudgets.monthKey, '2032-02'));
  assert.equal(
    afterReconciliation.reservedFreeCostMicrocents,
    beforeReconciliation.reservedFreeCostMicrocents,
    'stale selection cannot release the renewed provider hold',
  );
  const [stillActive] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, staleId));
  assert.equal(stillActive.status, 'pending');
  assert.equal(stillActive.audioState, 'dispatched');
  const [stillInFlight] = await db
    .select()
    .from(requestDedup)
    .where(eq(requestDedup.idempotencyKey, dedup.idempotencyKey));
  assert.equal(
    stillInFlight.status,
    'in_flight',
    'stale expiry must not publish dedup failure',
  );
  assert.equal(
    await finalizeReservation({
      requestId: staleId,
      outcome: 'release',
      expectedExpiredAtBefore: renewed.reservation.expiresAt,
      dedup,
    }),
    'released',
    'expiry equality releases only after the renewed lease',
  );
  assert.equal(
    await finalizeReservation({
      requestId: staleId,
      outcome: 'release',
      expectedExpiredAtBefore: renewed.reservation.expiresAt,
      dedup,
    }),
    'already_terminal',
  );

  const zeroId = `audio-known-zero-${suffix}`;
  assert.ok(
    (await prepareAudioReservation({ ...input, reservationId: zeroId })).ok,
  );
  const zero = await admitAudioReservation({
    ...admission,
    reservationId: zeroId,
    estimatedProviderCostUsd: 0,
  });
  assert.ok(zero.ok);
  assert.equal(zero.reservation.estimatedProviderCostMicrocents, 0);
  await finalizeReservation({ requestId: zeroId, outcome: 'release' });

  const [limitedUser] = await db
    .insert(users)
    .values({
      workosUserId: `audio-limited-${suffix}`,
      email: `audio-limited-${suffix}@example.test`,
    })
    .returning();
  await db.insert(creditLedger).values({
    userId: limitedUser.id,
    delta: 5,
    kind: 'free',
    reason: 'integration.audio_limited_grant',
  });
  const limited = await Promise.all(
    ['a', 'b'].map((key) =>
      prepareAudioReservation({
        ...input,
        userId: limitedUser.id,
        reservationId: `audio-limited-${key}-${suffix}`,
      }),
    ),
  );
  assert.equal(
    limited.filter((result) => result.ok).length,
    1,
    'credit-only preparations still serialize balance admission',
  );
  for (const result of limited) {
    if (result.ok)
      await cancelAudioPreparation({
        reservationId: result.reservationId,
        userId: limitedUser.id,
      });
  }

  // Provisional free credits do not bypass cap admission, including a known zero estimate.
  await db
    .update(freeSpendMonthlyBudgets)
    .set({ capMicrocents: 0, halted: true })
    .where(eq(freeSpendMonthlyBudgets.monthKey, '2032-02'));
  const cappedId = `audio-capped-${suffix}`;
  assert.ok(
    (await prepareAudioReservation({ ...input, reservationId: cappedId })).ok,
  );
  const capped = await admitAudioReservation({
    ...admission,
    reservationId: cappedId,
    estimatedProviderCostUsd: 0,
  });
  assert.equal(capped.ok, false);
  if (!capped.ok) assert.equal(capped.reason, 'free_spend_cap_reached');
  await db.insert(creditLedger).values({
    userId: user.id,
    delta: 5,
    kind: 'paid',
    reason: 'integration.audio_paid_grant',
  });
  const paid = await admitAudioReservation({
    ...admission,
    reservationId: cappedId,
  });
  assert.ok(paid.ok);
  assert.equal(paid.reservation.reservedFreeCredits, 0);
  assert.equal(paid.reservation.reservedPaidCredits, 5);
  assert.equal(paid.reservation.reservedFreeCostMicrocents, 0);
  assert.equal(
    await finalizeReservation({ requestId: cappedId, outcome: 'release' }),
    'released',
  );
  assert.equal(
    await finalizeReservation({ requestId: cappedId, outcome: 'release' }),
    'already_terminal',
  );

  // Both serial orderings of cancellation versus dispatch are safe (covered above).
  const racingId = `audio-race-${suffix}`;
  await prepareAudioReservation({ ...input, reservationId: racingId });
  const race = await Promise.allSettled([
    admitAudioReservation({ ...admission, reservationId: racingId }),
    cancelAudioPreparation({
      reservationId: racingId,
      userId: user.id,
      reason: 'cancelled',
    }),
  ]);
  const [raced] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, racingId));
  const claimed = race[0].status === 'fulfilled' && race[0].value.ok;
  assert.equal(raced.status, claimed ? 'pending' : 'released');
  if (claimed) {
    assert.equal(raced.audioState, 'dispatched');
    await finalizeReservation({ requestId: racingId, outcome: 'release' });
  }

  const cancelledBeforePreparationId = `audio-cancel-before-prepare-${suffix}`;
  assert.equal(
    await cancelAudioPreparation({
      reservationId: cancelledBeforePreparationId,
      userId: user.id,
      reason: 'cancelled',
      tombstoneMissing: true,
    }),
    'released',
  );
  assert.equal(
    await getAudioPreparationStatus({
      requestId: cancelledBeforePreparationId,
      userId: user.id,
      credits: 5,
    }),
    'cancelled',
  );
  const latePreparation = await prepareAudioReservation({
    reservationId: cancelledBeforePreparationId,
    userId: user.id,
    credits: 5,
  });
  assert.equal(latePreparation.ok, false);
  if (!latePreparation.ok)
    assert.equal(latePreparation.reason, 'invalid_audio_preparation');
  const [cancelTombstone] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, cancelledBeforePreparationId));
  assert.ok(cancelTombstone);
  assert.equal(cancelTombstone.status, 'released');
  assert.equal(cancelTombstone.audioState, 'cancelled');
  assert.equal(cancelTombstone.reservedCredits, 0);
  assert.equal(cancelTombstone.reservedFreeCostMicrocents, 0);

  const legacyCleanupId = `audio-legacy-cleanup-${suffix}`;
  assert.equal(
    await cancelAudioPreparation({
      reservationId: legacyCleanupId,
      userId: user.id,
      reason: 'failed',
    }),
    'not_prepared',
  );
  assert.equal(
    await getAudioPreparationStatus({
      requestId: legacyCleanupId,
      userId: user.id,
      credits: 5,
    }),
    'missing',
  );
  const legacyRetryPreparation = await prepareAudioReservation({
    ...input,
    reservationId: legacyCleanupId,
  });
  assert.ok(legacyRetryPreparation.ok);
  assert.equal(
    await cancelAudioPreparation({
      reservationId: legacyCleanupId,
      userId: user.id,
      reason: 'failed',
    }),
    'released',
  );
  const legacyRetryBillingId = `${legacyCleanupId}-billing`;
  const legacyRetryHold = await prepareAudioReservation({
    reservationId: legacyRetryBillingId,
    operationId: legacyCleanupId,
    userId: user.id,
    credits: 5,
  });
  assert.ok(legacyRetryHold.ok);
  const legacyRetryAdmission = await admitAudioReservation({
    reservationId: legacyRetryBillingId,
    operationId: legacyCleanupId,
    userId: user.id,
    credits: 5,
    estimatedProviderCostUsd: estimate,
    estimateCostSource: 'catalog_estimate',
  });
  assert.ok(legacyRetryAdmission.ok);
  assert.equal(
    await finalizeReservation({
      requestId: legacyRetryBillingId,
      outcome: 'release',
    }),
    'released',
  );
  assert.equal(
    await cancelAudioPreparation({
      reservationId: legacyCleanupId,
      userId: user.id,
      reason: 'cancelled',
      tombstoneMissing: true,
    }),
    'released',
  );
  assert.equal(
    await getAudioPreparationStatus({
      requestId: legacyCleanupId,
      userId: user.id,
      credits: 5,
    }),
    'cancelled',
    'explicit cancellation upgrades a cleanup that won the race',
  );

  const rootOperationId = `audio-cancel-fallback-${suffix}`;
  const fallbackReservationId = `${rootOperationId}-billing`;
  const fallbackPreparation = await prepareAudioReservation({
    reservationId: fallbackReservationId,
    operationId: rootOperationId,
    userId: user.id,
    credits: 5,
  });
  assert.ok(fallbackPreparation.ok);
  const ledgerBeforeCancel = await db
    .select()
    .from(creditLedger)
    .where(eq(creditLedger.userId, user.id));
  const monthKey = utcMonthKey(new Date());
  const budgetBeforeCancel = await db.transaction(async (tx) => {
    const config = await lockFreeSpendCapConfig(tx, 'share');
    return ensureFreeSpendMonthlyBudget(tx, monthKey, config.capMicrocents);
  });
  assert.equal(
    await cancelAudioPreparation({
      reservationId: rootOperationId,
      userId: user.id,
      reason: 'cancelled',
      tombstoneMissing: true,
    }),
    'released',
  );
  const rejectedFallbackAdmission = await admitAudioReservation({
    reservationId: fallbackReservationId,
    operationId: rootOperationId,
    userId: user.id,
    credits: 5,
    estimatedProviderCostUsd: estimate,
    estimateCostSource: 'catalog_estimate',
  });
  assert.equal(rejectedFallbackAdmission.ok, false);
  if (!rejectedFallbackAdmission.ok)
    assert.equal(rejectedFallbackAdmission.reason, 'invalid_audio_preparation');
  assert.equal(
    await cancelAudioPreparation({
      reservationId: fallbackReservationId,
      userId: user.id,
      reason: 'failed',
    }),
    'released',
  );
  const ledgerAfterCancel = await db
    .select()
    .from(creditLedger)
    .where(eq(creditLedger.userId, user.id));
  assert.equal(ledgerAfterCancel.length, ledgerBeforeCancel.length);
  const [budgetAfterCancel] = await db
    .select()
    .from(freeSpendMonthlyBudgets)
    .where(eq(freeSpendMonthlyBudgets.monthKey, monthKey));
  assert.equal(
    budgetAfterCancel?.actualFreeCostMicrocents ?? 0,
    budgetBeforeCancel.actualFreeCostMicrocents,
  );
  assert.equal(
    budgetAfterCancel?.reservedFreeCostMicrocents ?? 0,
    budgetBeforeCancel.reservedFreeCostMicrocents,
  );
  const [fallbackTombstone] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, rootOperationId));
  assert.ok(fallbackTombstone);
  assert.equal(fallbackTombstone.reservedCredits, 0);
  assert.equal(fallbackTombstone.reservedFreeCostMicrocents, 0);
  await db
    .update(creditReservations)
    .set({ expiresAt: new Date(Date.now() - 1) })
    .where(eq(creditReservations.requestId, rootOperationId));
  const operationAfterTombstoneExpiry = await prepareAudioReservation({
    reservationId: rootOperationId,
    userId: user.id,
    credits: 5,
  });
  assert.ok(operationAfterTombstoneExpiry.ok);
  assert.equal(
    await cancelAudioPreparation({
      reservationId: rootOperationId,
      userId: user.id,
      reason: 'failed',
    }),
    'released',
  );

  const pruneTombstoneId = `audio-prune-tombstone-${suffix}`;
  assert.equal(
    await cancelAudioPreparation({
      reservationId: pruneTombstoneId,
      userId: user.id,
      reason: 'cancelled',
      tombstoneMissing: true,
    }),
    'released',
  );
  await db
    .update(creditReservations)
    .set({ expiresAt: new Date(Date.now() - 1) })
    .where(eq(creditReservations.requestId, pruneTombstoneId));
  assert.equal(await pruneExpiredAudioCancellationTombstones(), 1);
  const [prunedTombstone] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, pruneTombstoneId));
  assert.equal(prunedTombstone, undefined);
}
