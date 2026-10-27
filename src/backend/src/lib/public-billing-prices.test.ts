import assert from 'node:assert/strict';
import { test } from 'node:test';
import type Stripe from 'stripe';
import { setStripeForTests, type StripeLike } from './billing';
import {
  publicBillingPrices,
  publicPriceDisplay,
} from './public-billing-prices';

function price(overrides: Partial<Stripe.Price> = {}): Stripe.Price {
  return {
    active: true,
    billing_scheme: 'per_unit',
    unit_amount: 1000,
    currency: 'usd',
    type: 'recurring',
    recurring: { interval: 'month', interval_count: 1, usage_type: 'licensed' },
    ...overrides,
  } as Stripe.Price;
}

test('displays the Stripe amount and billing interval', () => {
  assert.deepEqual(publicPriceDisplay(price()), {
    amount: '$10.00',
    period: 'month',
  });
  assert.deepEqual(
    publicPriceDisplay(price({ currency: 'eur', unit_amount: 1550 })),
    { amount: '€15.50', period: 'month' },
  );
});

test('preserves multi-month periods and one-time top-ups', () => {
  const quarterly = price();
  quarterly.recurring!.interval_count = 3;
  assert.equal(publicPriceDisplay(quarterly)?.period, '3 months');
  assert.deepEqual(
    publicPriceDisplay(price({ type: 'one_time', recurring: null })),
    { amount: '$10.00', period: null },
  );
});

test('does not advertise inactive or unsupported variable prices', () => {
  assert.equal(publicPriceDisplay(price({ active: false })), null);
  assert.equal(publicPriceDisplay(price({ unit_amount: null })), null);
  assert.equal(publicPriceDisplay(price({ billing_scheme: 'tiered' })), null);
});

test('uses and caches the Stripe amount instead of catalog display metadata', async () => {
  let calls = 0;
  setStripeForTests({
    prices: {
      retrieve: async () => {
        calls += 1;
        return price({ unit_amount: 1250 });
      },
    },
  } as unknown as StripeLike);
  try {
    const items = [{ priceId: 'price_public_display_cache_test' }];
    const first = await publicBillingPrices(items);
    const second = await publicBillingPrices(items);
    assert.deepEqual(first.get(items[0].priceId), {
      amount: '$12.50',
      period: 'month',
    });
    assert.deepEqual(second.get(items[0].priceId), first.get(items[0].priceId));
    assert.equal(calls, 1);
  } finally {
    setStripeForTests(null);
  }
});

test('Stripe lookup failures hide the quote without crashing the page', async () => {
  setStripeForTests({
    prices: {
      retrieve: async () => {
        throw new Error('Stripe unavailable');
      },
    },
  } as unknown as StripeLike);
  try {
    const result = await publicBillingPrices([
      { priceId: 'price_public_display_failure_test' },
    ]);
    assert.equal(result.get('price_public_display_failure_test'), null);
  } finally {
    setStripeForTests(null);
  }
});
