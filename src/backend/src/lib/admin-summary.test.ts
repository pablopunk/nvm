import assert from 'node:assert/strict';
import test from 'node:test';
import {
  estimateStripeFeesUsd,
  pricingKnobs,
  resolveSummaryMonth,
  segmentTotals,
} from './admin-summary';

test('resolveSummaryMonth defaults to the current UTC month', () => {
  const month = resolveSummaryMonth(null, new Date('2026-10-27T12:00:00Z'));
  assert.equal(month?.key, '2026-10');
  assert.equal(month?.from.toISOString(), '2026-10-01T00:00:00.000Z');
  assert.equal(month?.to.toISOString(), '2026-11-01T00:00:00.000Z');
});

test('resolveSummaryMonth rolls December into January and rejects junk', () => {
  assert.equal(
    resolveSummaryMonth('2026-12')?.to.toISOString(),
    '2027-01-01T00:00:00.000Z',
  );
  assert.equal(resolveSummaryMonth('2026-13'), null);
  assert.equal(resolveSummaryMonth('october'), null);
});

test('estimateStripeFeesUsd charges nothing without payments', () => {
  assert.equal(estimateStripeFeesUsd(0, 0), 0);
  assert.equal(estimateStripeFeesUsd(20, 1).toFixed(2), '0.88');
});

test('segmentTotals nets revenue against fees and provider cost', () => {
  const totals = segmentTotals([
    {
      paying: true,
      users: 2,
      activeUsers: 1,
      requests: 10,
      costMicrocents: 300_000_000,
      revenueCents: 4000,
      payments: 2,
    },
    {
      paying: false,
      users: 5,
      activeUsers: 3,
      requests: 40,
      costMicrocents: 100_000_000,
      revenueCents: 0,
      payments: 0,
    },
  ]);
  assert.equal(totals.users, 7);
  assert.equal(totals.activeUsers, 4);
  assert.equal(totals.costUsd, 4);
  assert.equal(totals.revenueUsd, 40);
  assert.equal(totals.feesUsd.toFixed(2), '1.76');
  assert.equal(totals.netUsd.toFixed(2), '34.24');
});

test('pricingKnobs shows worst-case cost when every credit is spent', () => {
  const knobs = pricingKnobs(
    {
      subscriptions: [
        {
          kind: 'subscription',
          priceId: 'price_pro',
          tier: 'pro',
          credits: 1000,
          displayPrice: { amount: 2000, currency: 'usd', interval: 'month' },
        },
      ],
      topUps: [{ kind: 'top_up', priceId: 'price_top', credits: 1000 }],
    },
    { creditUsd: 0.01, markup: 5, monthlyFreeCredits: 500 },
  );
  assert.equal(knobs.maxFreeUserCostUsd, 1);
  assert.equal(knobs.subscriptions[0].maxCostUsd, 3);
  assert.equal(knobs.subscriptions[0].worstCaseNetUsd?.toFixed(2), '16.12');
  assert.equal(knobs.topUps[0].priceUsd, null);
  assert.equal(knobs.topUps[0].maxCostUsd, 2);
});
