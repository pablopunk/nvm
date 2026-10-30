import { sql, type SQL } from 'drizzle-orm';
import { db } from '../db/client';
import { billingCatalog, stripeClient } from './billing';
import { CREDIT_USD, MARKUP } from './cost';
import { MONTHLY_FREE_CREDITS } from './users';
import { loadModelPerformance, type ModelPerformanceRow } from './admin-model-performance';

/** Stripe's standard US card rate; an estimate, not settled fees. */
export const STRIPE_FEE_RATE = 0.029;
export const STRIPE_FEE_FIXED_USD = 0.3;
export const TOP_USERS_LIMIT = 25;

const MICROCENTS_PER_USD = 100_000_000;
const PAYING_SUBSCRIPTION_STATUSES = ['active', 'trialing', 'past_due'];

export type SummaryMonth = { key: string; from: Date; to: Date };

export type SegmentTotals = {
  users: number;
  activeUsers: number;
  requests: number;
  costUsd: number;
  revenueUsd: number;
  feesUsd: number;
  netUsd: number;
};

export type ModelCostRow = {
  model: string;
  provider: string | null;
  paying: boolean;
  requests: number;
  costUsd: number;
};

export type UserEconomicsRow = {
  email: string;
  paying: boolean;
  requests: number;
  costUsd: number;
  revenueUsd: number;
  netUsd: number;
};

export type PricingKnobs = {
  creditUsd: number;
  markup: number;
  monthlyFreeCredits: number;
  maxFreeUserCostUsd: number;
  subscriptions: Array<{
    tier: string;
    credits: number;
    priceUsd: number | null;
    maxCostUsd: number;
    worstCaseNetUsd: number | null;
  }>;
  topUps: Array<{ credits: number; priceUsd: number | null; maxCostUsd: number }>;
};

export type AdminSummary = {
  month: string;
  total: SegmentTotals;
  paying: SegmentTotals;
  free: SegmentTotals;
  models: ModelCostRow[];
  performance: ModelPerformanceRow[];
  topUsers: UserEconomicsRow[];
  knobs: PricingKnobs | null;
};

export function resolveSummaryMonth(
  value: string | null,
  now = new Date(),
): SummaryMonth | null {
  const key =
    value ??
    `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, '0')}`;
  const match = /^(\d{4})-(0[1-9]|1[0-2])$/.exec(key);
  if (!match) return null;
  const year = Number(match[1]);
  const monthIndex = Number(match[2]) - 1;
  return {
    key,
    from: new Date(Date.UTC(year, monthIndex, 1)),
    to: new Date(Date.UTC(year, monthIndex + 1, 1)),
  };
}

export function estimateStripeFeesUsd(
  revenueUsd: number,
  payments: number,
): number {
  if (payments <= 0) return 0;
  return revenueUsd * STRIPE_FEE_RATE + payments * STRIPE_FEE_FIXED_USD;
}

export function microcentsToUsd(value: number): number {
  return value / MICROCENTS_PER_USD;
}

type SegmentRow = {
  paying: boolean;
  users: number;
  activeUsers: number;
  requests: number;
  costMicrocents: number;
  revenueCents: number;
  payments: number;
};

export function segmentTotals(rows: SegmentRow[]): SegmentTotals {
  const users = sum(rows, (row) => row.users);
  const activeUsers = sum(rows, (row) => row.activeUsers);
  const requests = sum(rows, (row) => row.requests);
  const costUsd = microcentsToUsd(sum(rows, (row) => row.costMicrocents));
  const revenueUsd = sum(rows, (row) => row.revenueCents) / 100;
  const feesUsd = estimateStripeFeesUsd(
    revenueUsd,
    sum(rows, (row) => row.payments),
  );
  return {
    users,
    activeUsers,
    requests,
    costUsd,
    revenueUsd,
    feesUsd,
    netUsd: revenueUsd - feesUsd - costUsd,
  };
}

/**
 * Worst case assumes every granted credit is spent at the configured markup.
 * `pricesUsd` holds live Stripe amounts by price ID.
 */
export function pricingKnobs(
  catalog: ReturnType<typeof billingCatalog>,
  pricesUsd: Map<string, number>,
  config = { creditUsd: CREDIT_USD, markup: MARKUP, monthlyFreeCredits: MONTHLY_FREE_CREDITS },
): PricingKnobs {
  const creditsToMaxCostUsd = (credits: number) =>
    (credits * config.creditUsd) / config.markup;
  const maxFreeUserCostUsd = creditsToMaxCostUsd(config.monthlyFreeCredits);
  return {
    ...config,
    maxFreeUserCostUsd,
    subscriptions: catalog.subscriptions.map((item) => {
      const priceUsd = pricesUsd.get(item.priceId) ?? null;
      const maxCostUsd = creditsToMaxCostUsd(item.credits) + maxFreeUserCostUsd;
      return {
        tier: item.tier,
        credits: item.credits,
        priceUsd,
        maxCostUsd,
        worstCaseNetUsd:
          priceUsd == null
            ? null
            : priceUsd - estimateStripeFeesUsd(priceUsd, 1) - maxCostUsd,
      };
    }),
    topUps: catalog.topUps.map((item) => ({
      credits: item.credits,
      priceUsd: pricesUsd.get(item.priceId) ?? null,
      maxCostUsd: creditsToMaxCostUsd(item.credits),
    })),
  };
}

function sum<T>(rows: T[], pick: (row: T) => number): number {
  return rows.reduce((total, row) => total + pick(row), 0);
}

function rowsOf<T>(result: unknown): T[] {
  return Array.isArray((result as { rows?: unknown }).rows)
    ? (result as { rows: T[] }).rows
    : (result as T[]);
}

/**
 * Per-user cost and Stripe revenue for the month. Revenue comes from stored
 * webhook payloads: subscription invoices plus paid one-off (top-up) Checkouts.
 */
function userEconomicsQuery(month: SummaryMonth): SQL {
  const from = Math.floor(month.from.getTime() / 1000);
  const to = Math.floor(month.to.getTime() / 1000);
  return sql`
    with cost as (
      select user_id, count(*) as requests,
        coalesce(sum(upstream_cost_microcents), 0) as cost_microcents
      from usage
      where created_at >= ${month.from} and created_at < ${month.to}
      group by user_id
    ),
    payments as (
      select payload->'data'->'object'->>'customer' as customer,
        case when type = 'invoice.paid'
          then (payload->'data'->'object'->>'amount_paid')::bigint
          else (payload->'data'->'object'->>'amount_total')::bigint end as cents
      from stripe_events
      where (type = 'invoice.paid'
          or (type = 'checkout.session.completed'
            and payload->'data'->'object'->>'mode' = 'payment'
            and payload->'data'->'object'->>'payment_status' = 'paid'))
        and lower(payload->'data'->'object'->>'currency') = 'usd'
        and (payload->>'created')::bigint >= ${from}
        and (payload->>'created')::bigint < ${to}
    ),
    revenue as (
      select u.id as user_id, count(*) filter (where p.cents > 0) as payments,
        coalesce(sum(p.cents), 0) as cents
      from payments p join users u on u.stripe_customer_id = p.customer
      group by u.id
    ),
    subscribed as (
      select distinct user_id from subscriptions
      where status in (${sql.join(PAYING_SUBSCRIPTION_STATUSES.map((status) => sql`${status}`), sql`, `)})
    ),
    economics as (
      select u.id, u.email,
        (s.user_id is not null or coalesce(r.cents, 0) > 0) as paying,
        coalesce(c.requests, 0)::int as requests,
        coalesce(c.cost_microcents, 0)::bigint as cost_microcents,
        coalesce(r.cents, 0)::bigint as revenue_cents,
        coalesce(r.payments, 0)::int as payments
      from users u
      left join cost c on c.user_id = u.id
      left join revenue r on r.user_id = u.id
      left join subscribed s on s.user_id = u.id
      where u.created_at < ${month.to}
    )`;
}

export type AdminSummarySection = 'overview' | 'users' | 'models' | 'pricing' | 'performance';

export async function loadAdminSummary(
  month: SummaryMonth,
  section?: AdminSummarySection,
): Promise<AdminSummary> {
  const economics = userEconomicsQuery(month);
  const [segmentResult, topUserResult, modelResult, knobs, performance] =
    await Promise.all([
      (!section || section === 'overview') ? db.execute(sql`${economics}
        select paying,
          count(*)::int as users,
          count(*) filter (where requests > 0)::int as "activeUsers",
          coalesce(sum(requests), 0)::int as requests,
          coalesce(sum(cost_microcents), 0)::float8 as "costMicrocents",
          coalesce(sum(revenue_cents), 0)::float8 as "revenueCents",
          coalesce(sum(payments), 0)::int as payments
        from economics group by paying`) : Promise.resolve([]),
      (!section || section === 'users') ? db.execute(sql`${economics}
        select email, paying, requests,
          cost_microcents::float8 as "costMicrocents",
          revenue_cents::float8 as "revenueCents",
          payments
        from economics
        where requests > 0 or revenue_cents > 0
        order by cost_microcents desc, revenue_cents desc
        limit ${TOP_USERS_LIMIT}`) : Promise.resolve([]),
      (!section || section === 'models') ? db.execute(sql`${economics}
        select u.model, u.provider, e.paying,
          count(*)::int as requests,
          coalesce(sum(u.upstream_cost_microcents), 0)::float8 as "costMicrocents"
        from usage u join economics e on e.id = u.user_id
        where u.created_at >= ${month.from} and u.created_at < ${month.to}
        group by u.model, u.provider, e.paying
        order by 5 desc`) : Promise.resolve([]),
      (!section || section === 'pricing') ? loadPricingKnobs(section === 'pricing') : Promise.resolve(null),
      (!section || section === 'performance') ? loadModelPerformance(month) : Promise.resolve([]),
    ]);

  const segments = rowsOf<SegmentRow>(segmentResult);
  const topUsers = rowsOf<{
    email: string;
    paying: boolean;
    requests: number;
    costMicrocents: number;
    revenueCents: number;
    payments: number;
  }>(topUserResult);
  const models = rowsOf<{
    model: string;
    provider: string | null;
    paying: boolean;
    requests: number;
    costMicrocents: number;
  }>(modelResult);

  return {
    month: month.key,
    total: segmentTotals(segments),
    paying: segmentTotals(segments.filter((row) => row.paying)),
    free: segmentTotals(segments.filter((row) => !row.paying)),
    models: models.map((row) => ({
      model: row.model,
      provider: row.provider,
      paying: row.paying,
      requests: row.requests,
      costUsd: microcentsToUsd(row.costMicrocents),
    })),
    topUsers: topUsers.map((row) => {
      const costUsd = microcentsToUsd(row.costMicrocents);
      const revenueUsd = row.revenueCents / 100;
      return {
        email: row.email,
        paying: row.paying,
        requests: row.requests,
        costUsd,
        revenueUsd,
        netUsd:
          revenueUsd - estimateStripeFeesUsd(revenueUsd, row.payments) - costUsd,
      };
    }),
    knobs,
    performance,
  };
}

/** USD-only; non-USD or unreadable prices show as unknown. */
async function loadPricingKnobs(strict = false): Promise<PricingKnobs | null> {
  try {
    const catalog = billingCatalog();
    const items = [...catalog.subscriptions, ...catalog.topUps];
    const prices = await Promise.all(
      items.map(async function retrievePrice(item) {
        const price = await stripeClient().prices.retrieve(item.priceId);
        return [
          item.priceId,
          price.currency === 'usd' && price.unit_amount != null
            ? price.unit_amount / 100
            : null,
        ] as const;
      }),
    );
    const pricesUsd = new Map<string, number>();
    for (const [priceId, usd] of prices) if (usd != null) pricesUsd.set(priceId, usd);
    return pricingKnobs(catalog, pricesUsd);
  } catch (error) {
    if (strict) throw error;
    return null;
  }
}
