import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { creditReservations } from '../db/schema';
import {
  allocateCreditsAcrossPools,
  classifyAudioPreparationStatus,
  isAudioPreparation,
} from './credit-reservations';

function audioPreparationRow(): typeof creditReservations.$inferSelect {
  return {
    requestId: 'operation-not-correlation',
    userId: 'owner',
    kind: 'free',
    status: 'pending',
    purpose: 'audio',
    audioState: 'prepared',
    reservedCredits: 5,
    reservedFreeCredits: 5,
    reservedPaidCredits: 0,
    estimatedTotalCredits: 5,
    admissionMonth: null,
    estimatedProviderCostMicrocents: null,
    estimateCostSource: null,
    reservedFreeCostMicrocents: 0,
    actualFreeCostMicrocents: null,
    actualCredits: null,
    actualFreeCredits: null,
    actualPaidCredits: null,
    expiresAt: new Date('2030-01-01T00:06:00Z'),
    settledAt: null,
    releasedAt: null,
    createdAt: new Date('2030-01-01T00:00:00Z'),
    updatedAt: new Date('2030-01-01T00:00:00Z'),
  };
}

test('audio preparation requires explicit purpose, unknown cost, ownership and live lease', () => {
  const row = audioPreparationRow();
  const now = row.createdAt;
  assert.equal(isAudioPreparation(row, 'owner', 5, now), true);
  assert.equal(isAudioPreparation(row, 'other', 5, now), false);
  assert.equal(isAudioPreparation(row, 'owner', 4, now), false);
  assert.equal(isAudioPreparation(row, 'owner', 5, row.expiresAt), false);
  for (const change of [
    { purpose: null, audioState: null },
    { purpose: 'text' },
    { audioState: 'dispatched' },
    { status: 'released' },
    { estimatedProviderCostMicrocents: 0 },
    { admissionMonth: '2030-01' },
    { reservedFreeCostMicrocents: 1 },
    { reservedPaidCredits: 1 },
  ]) {
    assert.equal(
      isAudioPreparation({ ...row, ...change }, 'owner', 5, now),
      false,
    );
  }
});

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

test('audio status preserves cancellation tombstones and rejects unverified pending provenance', () => {
  const row = audioPreparationRow();
  const input = { userId: 'owner', credits: 5 };
  assert.equal(
    classifyAudioPreparationStatus(undefined, input, row.createdAt),
    'missing',
  );
  assert.equal(
    classifyAudioPreparationStatus(row, input, row.createdAt),
    'prepared',
  );
  assert.equal(
    classifyAudioPreparationStatus(row, input, row.expiresAt),
    'unavailable',
  );
  assert.equal(
    classifyAudioPreparationStatus(
      row,
      { ...input, userId: 'other' },
      row.createdAt,
    ),
    'unavailable',
  );
  assert.equal(
    classifyAudioPreparationStatus(
      row,
      { ...input, credits: 4 },
      row.createdAt,
    ),
    'unavailable',
  );
  for (const change of [
    { purpose: null, audioState: null },
    { purpose: 'text' },
    { audioState: 'dispatched' },
    { audioState: 'failed' },
  ]) {
    assert.equal(
      classifyAudioPreparationStatus(
        { ...row, ...change },
        input,
        row.createdAt,
      ),
      'unavailable',
    );
  }
  const cancelled = { ...row, status: 'released', audioState: 'cancelled' };
  assert.equal(
    classifyAudioPreparationStatus(
      cancelled,
      input,
      new Date(row.expiresAt.getTime() - 1),
    ),
    'cancelled',
  );
  assert.equal(
    classifyAudioPreparationStatus(cancelled, input, row.expiresAt),
    'missing',
  );
  assert.equal(
    classifyAudioPreparationStatus(
      cancelled,
      { ...input, userId: 'other' },
      row.createdAt,
    ),
    'unavailable',
  );
  assert.equal(
    classifyAudioPreparationStatus(
      { ...cancelled, purpose: 'text' },
      input,
      row.createdAt,
    ),
    'missing',
  );
  for (const change of [
    { audioState: 'failed' },
    { audioState: 'prepared' },
    { status: 'settled', audioState: 'dispatched' },
  ]) {
    assert.equal(
      classifyAudioPreparationStatus(
        { ...row, status: 'released', ...change },
        input,
        row.createdAt,
      ),
      'missing',
    );
  }
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
