import assert from 'node:assert/strict';
import { eq } from 'drizzle-orm';
import { db } from '../src/db/client';
import { appSettings, creditLedger, users } from '../src/db/schema';
import { CREDIT_USD, MARKUP } from '../src/lib/cost';
import { finalizeReservation, reserveCredits } from '../src/lib/credit-reservations';
import { absorbedOverrunSettingKey, staleAdmissionsHalted } from '../src/lib/pricing-overrun-cap';

const CAP_ENV = 'PRICING_OVERRUN_DAILY_CAP_CREDITS';
const HOLD_CREDITS = 1;
// Two and a half credits of list cost rounds up to three, so each settlement absorbs two.
const SETTLED_CREDITS = 3;
const ABSORBED_PER_SETTLEMENT = SETTLED_CREDITS - HOLD_CREDITS;
const PROVIDER_COST_USD = (CREDIT_USD * 2.5) / MARKUP;

async function absorbedToday(): Promise<number> {
  const [row] = await db
    .select({ value: appSettings.value })
    .from(appSettings)
    .where(eq(appSettings.key, absorbedOverrunSettingKey(new Date())))
    .limit(1);
  return row ? Number(row.value) : 0;
}

async function withCapEnv<T>(cap: number, action: () => Promise<T>): Promise<T> {
  const previous = process.env[CAP_ENV];
  process.env[CAP_ENV] = String(cap);
  try {
    return await action();
  } finally {
    if (previous === undefined) delete process.env[CAP_ENV];
    else process.env[CAP_ENV] = previous;
  }
}

function reserveHold(requestId: string, userId: string, credits = HOLD_CREDITS) {
  return reserveCredits({
    requestId,
    userId,
    credits,
    estimatedProviderCostUsd: 0,
    estimateCostSource: 'catalog_estimate',
  });
}

function settleOverrun(requestId: string) {
  return finalizeReservation({
    requestId,
    outcome: 'settle',
    model: 'overrun-cap-model',
    provider: 'openrouter',
    tokens: { inputTokens: 1, outputTokens: 1 },
    providerCostUsd: PROVIDER_COST_USD,
    status: 200,
    latencyMs: 1,
  });
}

/**
 * Holds `holdCredits`, draws the balance down to zero before settling, and returns how
 * much the counter grew. The debit collects nothing, so the whole list cost is absorbed.
 */
async function absorbedByBalanceDrawnSettlement(
  suffix: string,
  label: string,
  holdCredits: number,
): Promise<number> {
  const [user] = await db
    .insert(users)
    .values({
      workosUserId: `overrun-cap-${suffix}-${label}`,
      email: `overrun-cap-${suffix}-${label}@example.test`,
    })
    .returning();
  assert.ok(user);
  await db.insert(creditLedger).values({
    userId: user.id,
    delta: holdCredits,
    kind: 'paid',
    reason: 'integration.overrun_cap_grant',
    refId: `${suffix}-${label}`,
  });
  const requestId = `overrun-cap-${suffix}-${label}`;
  assert.equal((await reserveHold(requestId, user.id, holdCredits)).ok, true);
  await db.insert(creditLedger).values({
    userId: user.id,
    delta: -holdCredits,
    kind: 'paid',
    reason: 'integration.overrun_cap_draw',
    refId: `${suffix}-${label}`,
  });

  const before = await absorbedToday();
  assert.equal(await settleOverrun(requestId), 'settled');
  return (await absorbedToday()) - before;
}

/**
 * Runs inside the isolated integration database. Counter values are compared as deltas
 * because earlier assertions in the same run may absorb overrun on the same UTC day.
 */
export async function runPricingOverrunCapAssertions(suffix: string) {
  const [user] = await db
    .insert(users)
    .values({
      workosUserId: `overrun-cap-${suffix}`,
      email: `overrun-cap-${suffix}@example.test`,
    })
    .returning();
  assert.ok(user);
  await db.insert(creditLedger).values({
    userId: user.id,
    delta: 1000,
    kind: 'paid',
    reason: 'integration.overrun_cap_grant',
    refId: suffix,
  });

  const requestIds = Array.from({ length: 5 }, (_, index) => `overrun-cap-${suffix}-${index}`);
  for (const requestId of requestIds) {
    assert.equal((await reserveHold(requestId, user.id)).ok, true);
  }
  const startingAbsorbed = await absorbedToday();
  const settlements = await Promise.all(requestIds.map((requestId) => settleOverrun(requestId)));
  assert.deepEqual(settlements, requestIds.map(() => 'settled'));
  let expectedAbsorbed = startingAbsorbed + requestIds.length * ABSORBED_PER_SETTLEMENT;
  assert.equal(
    await absorbedToday(),
    expectedAbsorbed,
    'concurrent settlements each add their absorbed credits, with no lost update',
  );

  assert.equal(await settleOverrun(requestIds[0]), 'already_terminal');
  assert.equal(await absorbedToday(), expectedAbsorbed, 'a repeated settlement adds nothing');

  assert.equal(
    await withCapEnv(expectedAbsorbed, () => staleAdmissionsHalted()),
    true,
    'a counter at the cap halts stale admissions',
  );
  assert.equal(
    await withCapEnv(expectedAbsorbed + 1, () => staleAdmissionsHalted()),
    false,
    'a counter one credit under the cap admits',
  );

  const inFlightId = `overrun-cap-${suffix}-in-flight`;
  assert.equal((await reserveHold(inFlightId, user.id)).ok, true);
  await withCapEnv(expectedAbsorbed, async () => {
    assert.equal(await staleAdmissionsHalted(), true);
    assert.equal(
      await settleOverrun(inFlightId),
      'settled',
      'a hold admitted before the cap was reached still settles after it',
    );
  });
  expectedAbsorbed += ABSORBED_PER_SETTLEMENT;
  assert.equal(
    await absorbedToday(),
    expectedAbsorbed,
    'the counter can exceed the cap by overrun from holds already in flight',
  );

  assert.equal(
    await absorbedByBalanceDrawnSettlement(suffix, 'shortfall', SETTLED_CREDITS),
    SETTLED_CREDITS,
    'a debit the drawn-down balance cannot take is exposure even without overrun',
  );
  assert.equal(
    await absorbedByBalanceDrawnSettlement(suffix, 'overrun-and-shortfall', HOLD_CREDITS),
    SETTLED_CREDITS,
    'overrun and a drawn-down balance are counted once, as the list cost left uncollected',
  );
  expectedAbsorbed += SETTLED_CREDITS + SETTLED_CREDITS;
  assert.equal(
    await absorbedToday(),
    expectedAbsorbed,
    'the counter records what the platform did not collect, not only cost above the hold',
  );
}
