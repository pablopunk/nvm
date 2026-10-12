import assert from 'node:assert/strict';
import { test } from 'node:test';
import { allocateCreditsAcrossPools } from './credit-reservations';

test('credit allocation remains free-first when free spend is allowed', () => {
  assert.deepEqual(
    allocateCreditsAcrossPools({
      credits: 80,
      freeAvailable: 50,
      paidAvailable: 50,
      freeSpendAllowed: true,
    }),
    { ok: true, reservedFree: 50, reservedPaid: 30 },
  );
});

test('credit allocation falls back to paid credits when the free cap is reached', () => {
  assert.deepEqual(
    allocateCreditsAcrossPools({
      credits: 80,
      freeAvailable: 100,
      paidAvailable: 80,
      freeSpendAllowed: false,
    }),
    { ok: true, reservedFree: 0, reservedPaid: 80 },
  );
});

test('credit allocation reports the free cap when paid credits cannot cover it', () => {
  assert.deepEqual(
    allocateCreditsAcrossPools({
      credits: 80,
      freeAvailable: 100,
      paidAvailable: 79,
      freeSpendAllowed: false,
    }),
    { ok: false, reason: 'free_spend_cap_reached' },
  );
});

test('credit allocation reports insufficient credits when combined pools cannot cover it', () => {
  assert.deepEqual(
    allocateCreditsAcrossPools({
      credits: 80,
      freeAvailable: 40,
      paidAvailable: 39,
      freeSpendAllowed: false,
    }),
    { ok: false, reason: 'insufficient_credits' },
  );
});
