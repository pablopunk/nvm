import type { APIRoute } from 'astro';
import { desc, eq, inArray, sql } from 'drizzle-orm';
import type Stripe from 'stripe';
import { z } from 'zod';
import { db } from '../../../../db/client';
import { billingPriceTerms } from '../../../../db/schema';
import { isBillingPriceApprover, requireAdmin } from '../../../../lib/admin';
import {
  adminMutationKeyFromRequest,
  adminMutationResponse,
  executeAdminMutation,
  lookupAdminMutationResult,
} from '../../../../lib/admin-mutations';
import { recordAuditStrict } from '../../../../lib/audit';
import {
  type BillingCatalogItem,
  BillingConfigError,
  billingCatalog,
  OWNER_APPROVED_PRICE_TERMS_SOURCE,
  stripeClient,
} from '../../../../lib/billing';
import { requireSameOrigin } from '../../../../lib/csrf';
import { safeJsonBody } from '../../../../lib/validation';

const candidateSchema = z
  .object({
    priceId: z.string().min(1).max(255),
    kind: z.enum(['subscription', 'top_up']),
    tier: z.string().min(1).max(64).nullable(),
    credits: z.number().int().positive().max(1_000_000_000),
    unitAmount: z.number().int().positive().max(Number.MAX_SAFE_INTEGER),
    currency: z.string().regex(/^[a-z]{3}$/),
    quantity: z.literal(1),
    interval: z.enum(['day', 'week', 'month', 'year']).nullable(),
    intervalCount: z.number().int().positive().nullable(),
  })
  .strict();

type CandidateTerms = z.infer<typeof candidateSchema>;

function configuredItems(): BillingCatalogItem[] {
  const catalog = billingCatalog();
  return [...catalog.subscriptions, ...catalog.topUps];
}

function candidateTerms(
  item: BillingCatalogItem,
  unitAmount: number,
  currency: string,
  interval: string | null,
  intervalCount: number | null,
): CandidateTerms {
  return {
    priceId: item.priceId,
    kind: item.kind,
    tier: item.kind === 'subscription' ? item.tier : null,
    credits: item.credits,
    unitAmount,
    currency: currency.toLowerCase(),
    quantity: 1,
    interval:
      item.kind === 'subscription'
        ? (interval as CandidateTerms['interval'])
        : null,
    intervalCount: item.kind === 'subscription' ? intervalCount : null,
  };
}

function priceCandidateMatches(
  price: Stripe.Price,
  item: BillingCatalogItem,
  expected?: CandidateTerms,
) {
  const recurring = item.kind === 'subscription';
  const unitAmount = price.unit_amount;
  const interval = price.recurring?.interval ?? null;
  const intervalCount = price.recurring?.interval_count ?? null;
  if (
    !price.active ||
    price.billing_scheme !== 'per_unit' ||
    price.tiers_mode !== null ||
    price.transform_quantity !== null ||
    price.recurring?.usage_type === 'metered' ||
    unitAmount == null ||
    !Number.isSafeInteger(unitAmount) ||
    unitAmount <= 0 ||
    !/^[a-z]{3}$/i.test(price.currency) ||
    (price.type === 'recurring') !== recurring ||
    (recurring && (!interval || intervalCount !== 1)) ||
    (!recurring && (interval !== null || intervalCount !== null))
  ) {
    return false;
  }
  const configuredDisplay = item.displayPrice;
  if (
    configuredDisplay &&
    (unitAmount !== configuredDisplay.amount ||
      price.currency.toLowerCase() !==
        configuredDisplay.currency.toLowerCase() ||
      (recurring && interval !== configuredDisplay.interval))
  ) {
    return false;
  }
  if (!expected) return Boolean(configuredDisplay);
  return (
    expected.priceId === item.priceId &&
    expected.kind === item.kind &&
    expected.tier === (item.kind === 'subscription' ? item.tier : null) &&
    expected.credits === item.credits &&
    expected.unitAmount === unitAmount &&
    expected.currency === price.currency.toLowerCase() &&
    expected.quantity === 1 &&
    expected.interval === (recurring ? interval : null) &&
    expected.intervalCount === (recurring ? intervalCount : null)
  );
}

function termsMatchRow(
  candidate: CandidateTerms,
  row: typeof billingPriceTerms.$inferSelect,
) {
  return (
    row.priceId === candidate.priceId &&
    row.kind === candidate.kind &&
    row.tier === candidate.tier &&
    row.credits === candidate.credits &&
    row.unitAmount === candidate.unitAmount &&
    row.currency === candidate.currency &&
    row.quantity === candidate.quantity &&
    row.interval === candidate.interval &&
    row.intervalCount === candidate.intervalCount
  );
}

async function retrieveCandidatePrice(priceId: string): Promise<Stripe.Price> {
  return stripeClient().prices.retrieve(
    priceId,
    {},
    { timeout: 4_000, maxNetworkRetries: 0 },
  );
}

export const GET: APIRoute = async ({ request }) => {
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  let stripe: ReturnType<typeof stripeClient>;
  try {
    stripe = stripeClient();
  } catch (error) {
    return Response.json(
      {
        error: {
          type: 'stripe_unavailable',
          message:
            error instanceof BillingConfigError
              ? error.message
              : 'Stripe is unavailable.',
        },
      },
      { status: 503 },
    );
  }

  const items = configuredItems().slice(0, 100);
  const priceIds = [...new Set(items.map((item) => item.priceId))];
  const existingTerms = priceIds.length
    ? await db
        .select()
        .from(billingPriceTerms)
        .where(inArray(billingPriceTerms.priceId, priceIds))
    : [];
  const candidates = await Promise.all(
    items.map(async (item) => {
      try {
        const price = await stripe.prices.retrieve(
          item.priceId,
          {},
          { timeout: 4_000, maxNetworkRetries: 0 },
        );
        const expectedDisplay = item.displayPrice;
        const expected = expectedDisplay
          ? candidateTerms(
              item,
              expectedDisplay.amount,
              expectedDisplay.currency,
              expectedDisplay.interval ?? null,
              item.kind === 'subscription' ? 1 : null,
            )
          : null;
        const snapshot = candidateTerms(
          item,
          price.unit_amount ?? 0,
          price.currency,
          price.recurring?.interval ?? null,
          price.recurring?.interval_count ?? null,
        );
        const approved = existingTerms.find(
          (row) =>
            row.source === OWNER_APPROVED_PRICE_TERMS_SOURCE &&
            termsMatchRow(snapshot, row),
        );
        const matches = priceCandidateMatches(
          price,
          item,
          expected ?? undefined,
        );
        return {
          ...snapshot,
          priceActive: price.active,
          eligible: matches && Boolean(expected),
          approved: Boolean(matches && approved),
          approvalVersion: approved?.version ?? null,
          configuredTerms: expected,
          reason: !expected
            ? 'Configure the immutable amount, currency, and subscription interval before approving this candidate.'
            : matches
              ? null
              : 'The retrieved Stripe Price does not match the configured candidate terms.',
        };
      } catch {
        return {
          priceId: item.priceId,
          kind: item.kind,
          tier: item.kind === 'subscription' ? item.tier : null,
          credits: item.credits,
          unitAmount: null,
          currency: null,
          quantity: 1,
          interval: null,
          intervalCount: null,
          priceActive: null,
          eligible: false,
          approved: false,
          approvalVersion: null,
          configuredTerms: null,
          reason:
            'Could not retrieve this configured Stripe Price. Retry the read.',
        };
      }
    }),
  );
  return Response.json({
    candidates,
    canApprove: isBillingPriceApprover(actor),
    source: 'Stripe Price GET; refresh is user-triggered.',
  });
};

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor || !isBillingPriceApprover(actor))
    return new Response('Forbidden', { status: 403 });
  const parsed = await safeJsonBody(request, candidateSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const idempotencyKey = adminMutationKeyFromRequest(request);
  if (!idempotencyKey) {
    return Response.json(
      {
        error: {
          type: 'idempotency_key_required',
          message: 'Provide a UUID Idempotency-Key.',
        },
      },
      { status: 400 },
    );
  }
  const replay = await lookupAdminMutationResult({
    actorUserId: actor.id,
    idempotencyKey,
    payload: parsed.data,
  });
  if (replay) return adminMutationResponse(replay);

  const item = configuredItems().find(
    (candidate) =>
      candidate.priceId === parsed.data.priceId &&
      candidate.kind === parsed.data.kind,
  );
  if (!item) {
    return Response.json(
      {
        error: {
          type: 'unknown_price_candidate',
          message:
            'This Price is not in the server-configured billing catalog.',
        },
      },
      { status: 404 },
    );
  }
  if (!item.displayPrice) {
    return Response.json(
      {
        error: {
          type: 'configured_price_terms_missing',
          message:
            'Configure the existing purchase amount, currency, and interval before approving these terms.',
        },
      },
      { status: 409 },
    );
  }
  let price: Stripe.Price;
  try {
    price = await retrieveCandidatePrice(parsed.data.priceId);
  } catch (error) {
    return Response.json(
      {
        error: {
          type: 'stripe_unavailable',
          message:
            error instanceof BillingConfigError
              ? error.message
              : 'Could not retrieve the configured Stripe Price.',
        },
      },
      { status: 503 },
    );
  }
  if (!priceCandidateMatches(price, item, parsed.data)) {
    return Response.json(
      {
        error: {
          type: 'price_candidate_mismatch',
          message:
            'The live Stripe Price or server-configured grant no longer matches this reviewed candidate. Refresh before approval.',
        },
      },
      { status: 409 },
    );
  }

  const result = await executeAdminMutation({
    actorUserId: actor.id,
    idempotencyKey,
    payload: parsed.data,
    execute: async (tx) => {
      await tx.execute(
        sql`SELECT pg_advisory_xact_lock(hashtext(${parsed.data.priceId}))`,
      );
      const versions = await tx
        .select()
        .from(billingPriceTerms)
        .where(eq(billingPriceTerms.priceId, parsed.data.priceId))
        .orderBy(desc(billingPriceTerms.version))
        .for('update');
      const existing = versions.find(
        (row) =>
          row.source === OWNER_APPROVED_PRICE_TERMS_SOURCE &&
          termsMatchRow(parsed.data, row),
      );
      if (existing)
        return { body: { ok: true, existing: true, terms: existing } };
      const version = (versions[0]?.version ?? 0) + 1;
      const [terms] = await tx
        .insert(billingPriceTerms)
        .values({
          ...parsed.data,
          version,
          source: OWNER_APPROVED_PRICE_TERMS_SOURCE,
          approvedAt: new Date(),
        })
        .returning();
      await recordAuditStrict(tx, {
        actorUserId: actor.id,
        action: 'billing.price_terms_approved',
        targetType: 'billing_price_terms',
        targetId: terms.id,
        meta: { ...parsed.data, version, idempotencyKey },
      });
      return { status: 201, body: { ok: true, existing: false, terms } };
    },
  });
  return adminMutationResponse(result);
};
