import type Stripe from 'stripe';
import { formatBillingPrice, stripeClient } from './billing';
import { log } from './log';

type PriceDisplay = {
  amount: string;
  period: string | null;
  /** The undiscounted price to show crossed out, from the Stripe price's `list_amount` metadata. */
  listAmount?: string;
};
const cache = new Map<
  string,
  { expiresAt: number; value: Promise<PriceDisplay | null> }
>();
const CACHE_MS = 5 * 60 * 1000;
const RETRY_MS = 15 * 1000;

export function publicPriceDisplay(price: Stripe.Price): PriceDisplay | null {
  if (
    !price.active ||
    price.billing_scheme !== 'per_unit' ||
    price.unit_amount === null ||
    !Number.isSafeInteger(price.unit_amount) ||
    price.unit_amount < 0 ||
    price.custom_unit_amount ||
    price.transform_quantity
  )
    return null;
  const amount = formatBillingPrice({
    amount: price.unit_amount,
    currency: price.currency,
  });
  if (!amount) return null;
  const listAmount = listAmountDisplay(price);
  const extras = listAmount ? { listAmount } : {};
  if (price.type === 'one_time') return { amount, period: null, ...extras };
  if (!price.recurring || price.recurring.usage_type !== 'licensed')
    return null;
  const { interval, interval_count: count } = price.recurring;
  return {
    amount,
    period: count === 1 ? interval : `${count} ${interval}s`,
    ...extras,
  };
}

/** A crossed-out price is shown only when it is a whole minor-unit amount above the real one. */
function listAmountDisplay(price: Stripe.Price): string | null {
  const raw = price.metadata?.list_amount;
  if (!raw || !/^\d+$/.test(raw)) return null;
  const listAmount = Number(raw);
  if (
    !Number.isSafeInteger(listAmount) ||
    price.unit_amount === null ||
    listAmount <= price.unit_amount
  )
    return null;
  return formatBillingPrice({ amount: listAmount, currency: price.currency });
}

async function loadPublicPrice(priceId: string): Promise<PriceDisplay | null> {
  try {
    const price = await stripeClient().prices.retrieve(
      priceId,
      {},
      { timeout: 3000, maxNetworkRetries: 0 },
    );
    return publicPriceDisplay(price);
  } catch (error) {
    log.warn('public_pricing.stripe_unavailable', {
      priceId,
      error: error instanceof Error ? error.name : 'UnknownError',
    });
    return null;
  }
}

function cachedPublicPrice(priceId: string): Promise<PriceDisplay | null> {
  const previous = cache.get(priceId);
  if (previous && previous.expiresAt > Date.now()) return previous.value;
  const entry = {
    expiresAt: Date.now() + CACHE_MS,
    value: loadPublicPrice(priceId),
  };
  cache.set(priceId, entry);
  entry.value.then(function updateExpiry(display) {
    entry.expiresAt = Date.now() + (display ? CACHE_MS : RETRY_MS);
  });
  return entry.value;
}

export async function publicBillingPrices(items: { priceId: string }[]) {
  const entries = await Promise.all(
    items.map(async function loadItem(item) {
      return [item.priceId, await cachedPublicPrice(item.priceId)] as const;
    }),
  );
  return new Map(entries);
}
