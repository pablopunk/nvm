import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  canReserveFreeSpend,
  canResizeFreeSpend,
  capHaltedAfterSettlement,
  capResetsAt,
  microcentsForFreePool,
  utcMonthKey,
} from './free-spend-cap';

test('free-pool provider cost allocation rounds upward without floating-point loss', () => {
  assert.equal(
    microcentsForFreePool({
      totalCostMicrocents: 1,
      freeCredits: 1,
      totalCredits: 3,
    }),
    1,
  );
  assert.equal(
    microcentsForFreePool({
      totalCostMicrocents: 100,
      freeCredits: 1,
      totalCredits: 3,
    }),
    34,
  );
  assert.equal(
    microcentsForFreePool({
      totalCostMicrocents: Number.MAX_SAFE_INTEGER,
      freeCredits: 1,
      totalCredits: 2,
    }),
    Math.ceil(Number.MAX_SAFE_INTEGER / 2),
  );
  assert.throws(() =>
    microcentsForFreePool({
      totalCostMicrocents: 1,
      freeCredits: 0,
      totalCredits: 0,
    }),
  );
  assert.throws(() =>
    microcentsForFreePool({
      totalCostMicrocents: 1,
      freeCredits: 1,
      totalCredits: 0,
    }),
  );
});

test('free-spend admission requires the whole positive hold and rejects halted or exhausted free allocations', () => {
  const budget = {
    capMicrocents: 100,
    actualFreeCostMicrocents: 40,
    reservedFreeCostMicrocents: 30,
    halted: false,
    baselineComplete: true,
  };
  assert.equal(canReserveFreeSpend({ budget, requestedMicrocents: 30 }), true);
  assert.equal(canReserveFreeSpend({ budget, requestedMicrocents: 31 }), false);
  assert.equal(
    canReserveFreeSpend({
      budget: { ...budget, actualFreeCostMicrocents: 70 },
      requestedMicrocents: 0,
    }),
    false,
  );
  assert.equal(
    canReserveFreeSpend({
      budget: { ...budget, halted: true },
      requestedMicrocents: 0,
    }),
    false,
  );
  assert.equal(
    canReserveFreeSpend({
      budget: { ...budget, baselineComplete: false },
      requestedMicrocents: 0,
    }),
    false,
  );
  assert.equal(
    canReserveFreeSpend({
      budget: { ...budget, capMicrocents: null, baselineComplete: false },
      requestedMicrocents: 1,
    }),
    true,
  );
});

test('reservation resize cannot add free allocation after a halt, even with a zero or smaller hold', () => {
  const haltedBudget = {
    capMicrocents: 100,
    actualFreeCostMicrocents: 100,
    reservedFreeCostMicrocents: 20,
    halted: true,
    baselineComplete: true,
  };

  assert.equal(
    canResizeFreeSpend({
      budget: haltedBudget,
      previousFreeCredits: 0,
      nextFreeCredits: 1,
      previousHoldMicrocents: 0,
      nextHoldMicrocents: 0,
    }),
    false,
  );
  assert.equal(
    canResizeFreeSpend({
      budget: haltedBudget,
      previousFreeCredits: 1,
      nextFreeCredits: 2,
      previousHoldMicrocents: 10,
      nextHoldMicrocents: 5,
    }),
    false,
  );
  assert.equal(
    canResizeFreeSpend({
      budget: haltedBudget,
      previousFreeCredits: 2,
      nextFreeCredits: 1,
      previousHoldMicrocents: 10,
      nextHoldMicrocents: 5,
    }),
    true,
    'reducing a reservation remains allowed so its unused hold can be released',
  );
});

test('UTC month keys, reset boundaries, and overrun halt semantics are explicit', () => {
  assert.equal(utcMonthKey(new Date('2026-10-31T23:59:59.999Z')), '2026-10');
  assert.equal(capResetsAt('2026-10'), '2026-11-01T00:00:00.000Z');
  assert.equal(
    capHaltedAfterSettlement({
      capMicrocents: 100,
      actualFreeCostMicrocents: 100,
      reservedFreeCostMicrocents: 0,
    }),
    true,
  );
  assert.equal(
    capHaltedAfterSettlement({
      capMicrocents: 100,
      actualFreeCostMicrocents: 99,
      reservedFreeCostMicrocents: 1,
    }),
    false,
  );
  assert.equal(
    capHaltedAfterSettlement({
      capMicrocents: null,
      actualFreeCostMicrocents: 1000,
      reservedFreeCostMicrocents: 0,
    }),
    false,
  );
});
