import assert from 'node:assert/strict';
import fs from 'node:fs';
import { eq, sql } from 'drizzle-orm';
import { db } from '../src/db/client';
import {
  creditLedger,
  creditReservations,
  freeSpendMonthlyBudgets,
  usage,
  users,
} from '../src/db/schema';
import { canReserveFreeSpend } from '../src/lib/free-spend-cap';

export async function runFreeSpendAttributionAssertions() {
  const [user, otherUser] = await db
    .insert(users)
    .values([
      {
        workosUserId: 'attribution-repair',
        email: 'attribution-repair@example.test',
      },
      {
        workosUserId: 'attribution-other',
        email: 'attribution-other@example.test',
      },
    ])
    .returning();
  const createdAt = new Date('2025-02-10T12:00:00Z');
  const monthKey = '2025-02';
  const ids = ['repair-mixed', 'repair-paid', 'repair-unknown'];
  await db.insert(creditReservations).values(
    ids.map((requestId) => ({
      requestId,
      userId: user.id,
      kind: 'free',
      reservedCredits: 10,
      actualCredits: 10,
      status: 'settled',
      createdAt,
      expiresAt: createdAt,
      settledAt: createdAt,
    })),
  );
  await db.insert(usage).values(
    ids.map((requestId, index) => ({
      requestId,
      userId: user.id,
      model: 'repair-model',
      provider: 'integration',
      inputTokens: 1,
      outputTokens: 1,
      costCredits: 10,
      upstreamCostMicrocents: index === 0 ? 11 : 10,
      upstreamCostSource: 'provider_reported',
      createdAt,
    })),
  );
  await db.insert(creditLedger).values([
    {
      userId: user.id,
      delta: -3,
      kind: 'free',
      reason: 'ai_usage',
      refId: `${ids[0]}:free`,
    },
    {
      userId: user.id,
      delta: -7,
      kind: 'paid',
      reason: 'ai_usage',
      refId: `${ids[0]}:paid`,
    },
    {
      userId: user.id,
      delta: -10,
      kind: 'paid',
      reason: 'ai_usage',
      refId: ids[1],
    },
    {
      userId: otherUser.id,
      delta: -10,
      kind: 'free',
      reason: 'ai_usage',
      refId: ids[2],
    },
  ]);
  await db.insert(creditReservations).values([
    {
      requestId: 'repair-pending-known',
      userId: user.id,
      kind: 'free',
      reservedCredits: 2,
      status: 'pending',
      admissionMonth: monthKey,
      reservedFreeCostMicrocents: 3,
      createdAt,
      expiresAt: createdAt,
    },
    {
      requestId: 'repair-pending-legacy',
      userId: user.id,
      kind: 'free',
      reservedCredits: 2,
      status: 'pending',
      createdAt,
      expiresAt: createdAt,
    },
  ]);
  await db.insert(freeSpendMonthlyBudgets).values({
    monthKey,
    capMicrocents: 20,
    baselineComplete: false,
    actualFreeCostMicrocents: 0,
    reservedFreeCostMicrocents: 3,
  });
  const migration = fs.readFileSync(
    new URL(
      '../drizzle/0025_legacy_free_spend_attribution.sql',
      import.meta.url,
    ),
    'utf8',
  );
  async function applyRepair() {
    await db.transaction(async (tx) => {
      for (const statement of migration.split('--> statement-breakpoint')) {
        await tx.execute(sql.raw(statement));
      }
    });
  }
  async function budget() {
    const [row] = await db
      .select()
      .from(freeSpendMonthlyBudgets)
      .where(eq(freeSpendMonthlyBudgets.monthKey, monthKey));
    return row;
  }
  await applyRepair();
  const [mixed] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, ids[0]));
  assert.equal(mixed.admissionMonth, monthKey);
  assert.equal(mixed.actualFreeCredits, 3);
  assert.equal(mixed.actualPaidCredits, 7);
  assert.equal(
    mixed.actualFreeCostMicrocents,
    4,
    'mixed attribution must round up exactly',
  );
  const [paid] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, ids[1]));
  assert.equal(
    paid.actualFreeCostMicrocents,
    0,
    'a legacy unsuffixed paid debit must not count as free spend',
  );
  const [unknown] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, ids[2]));
  assert.equal(
    unknown.admissionMonth,
    null,
    'another user’s debit must not supply attribution',
  );
  assert.equal((await budget()).baselineComplete, false);
  assert.equal((await budget()).halted, true);

  await db.insert(creditLedger).values({
    userId: user.id,
    delta: -10,
    kind: 'free',
    reason: 'ai_usage',
    refId: ids[2],
  });
  await applyRepair();
  const repaired = await budget();
  assert.equal(repaired.baselineComplete, true);
  assert.equal(repaired.actualFreeCostMicrocents, 14);
  assert.equal(repaired.reservedFreeCostMicrocents, 3);
  assert.equal(repaired.halted, false);
  assert.equal(
    canReserveFreeSpend({ budget: repaired, requestedMicrocents: 3 }),
    true,
  );
  assert.equal(
    canReserveFreeSpend({ budget: repaired, requestedMicrocents: 4 }),
    false,
  );
  const [pending] = await db
    .select()
    .from(creditReservations)
    .where(eq(creditReservations.requestId, 'repair-pending-legacy'));
  assert.equal(
    pending.admissionMonth,
    null,
    'pending legacy requests must remain guarded',
  );
  await applyRepair();
  assert.equal(
    (await budget()).actualFreeCostMicrocents,
    14,
    'rerunning the repair must not double count costs',
  );
  assert.equal(
    (
      await db
        .select()
        .from(creditLedger)
        .where(eq(creditLedger.userId, user.id))
    ).length,
    4,
    'repair must not change credit debits',
  );
  await db.delete(users).where(eq(users.id, user.id));
  await db.delete(users).where(eq(users.id, otherUser.id));
  await db
    .delete(freeSpendMonthlyBudgets)
    .where(eq(freeSpendMonthlyBudgets.monthKey, monthKey));
}
