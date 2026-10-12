import type Stripe from 'stripe';
import type { APIRoute } from 'astro';
import { desc, eq, inArray } from 'drizzle-orm';
import { z } from 'zod';
import { db } from '../../../../db/client';
import {
  billingOrders,
  billingRetryOperations,
  stripeEvents,
  users,
} from '../../../../db/schema';
import { requireAdmin } from '../../../../lib/admin';
import {
  adminMutationKeyFromRequest,
  adminMutationResponse,
  executeAdminMutation,
  lookupAdminMutationResult,
} from '../../../../lib/admin-mutations';
import {
  applyAdminRetryReconciliation,
  applyPreparedStripeEventReconciliation,
  BillingConfigError,
  BillingPendingError,
  applyAdminOrderReconciliation,
  loadAdminOrderReconciliationEvidence,
  loadAdminRetryReconciliationEvidence,
  prepareStripeEvent,
} from '../../../../lib/billing';
import { requireSameOrigin } from '../../../../lib/csrf';
import { safeJsonBody } from '../../../../lib/validation';

const reconciliationSchema = z
  .object({
    eventId: z.string().min(1).max(255).optional(),
    orderId: z.string().uuid().optional(),
    retryOperationId: z.string().uuid().optional(),
  })
  .strict()
  .refine(
    (value) =>
      [value.eventId, value.orderId, value.retryOperationId].filter(Boolean)
        .length === 1,
    {
      message: 'Provide exactly one of eventId, orderId, or retryOperationId.',
    },
  );

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  const [events, orders, retryOperations] = await Promise.all([
    db
      .select({
        eventId: stripeEvents.eventId,
        type: stripeEvents.type,
        reason: stripeEvents.reconciliationReason,
        createdAt: stripeEvents.createdAt,
      })
      .from(stripeEvents)
      .where(eq(stripeEvents.state, 'needs_reconciliation'))
      .orderBy(desc(stripeEvents.createdAt))
      .limit(100),
    db
      .select({
        id: billingOrders.id,
        userId: billingOrders.userId,
        email: users.email,
        kind: billingOrders.kind,
        tier: billingOrders.tier,
        credits: billingOrders.credits,
        amount: billingOrders.expectedTotalAmount,
        currency: billingOrders.currency,
        state: billingOrders.state,
        sessionId: billingOrders.sessionId,
        paymentIntentId: billingOrders.paymentIntentId,
        createdAt: billingOrders.createdAt,
      })
      .from(billingOrders)
      .innerJoin(users, eq(users.id, billingOrders.userId))
      .where(eq(billingOrders.state, 'needs_reconciliation'))
      .orderBy(desc(billingOrders.updatedAt))
      .limit(100),
    db
      .select({
        id: billingRetryOperations.id,
        userId: billingRetryOperations.userId,
        priorOrderId: billingRetryOperations.priorOrderId,
        replacementOrderId: billingRetryOperations.replacementOrderId,
        state: billingRetryOperations.state,
        evidence: billingRetryOperations.reconciliationEvidence,
        updatedAt: billingRetryOperations.updatedAt,
      })
      .from(billingRetryOperations)
      .where(
        inArray(billingRetryOperations.state, [
          'cancel_pending',
          'needs_reconciliation',
        ]),
      )
      .orderBy(desc(billingRetryOperations.updatedAt))
      .limit(100),
  ]);
  return Response.json({
    events,
    orders,
    retryOperations,
    source: 'Durable local reconciliation state; no Stripe reads were made.',
  });
};

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const parsed = await safeJsonBody(request, reconciliationSchema);
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

  if (parsed.data.eventId) {
    const [stored] = await db
      .select()
      .from(stripeEvents)
      .where(eq(stripeEvents.eventId, parsed.data.eventId))
      .limit(1);
    if (!stored)
      return Response.json(
        {
          error: {
            type: 'stripe_event_not_found',
            message: 'Stripe event not found.',
          },
        },
        { status: 404 },
      );
    if (stored.state !== 'needs_reconciliation') {
      const result = await executeAdminMutation({
        actorUserId: actor.id,
        idempotencyKey,
        payload: parsed.data,
        execute: async () => ({
          body: {
            status: 'already_resolved',
            eventId: stored.eventId,
            state: stored.state,
          },
        }),
      });
      return adminMutationResponse(result);
    }
    const event = stored.payload as Stripe.Event | null;
    if (!event || event.id !== stored.eventId || event.type !== stored.type) {
      return Response.json(
        {
          error: {
            type: 'stored_event_invalid',
            message:
              'The stored Stripe event payload is unavailable or inconsistent.',
          },
        },
        { status: 409 },
      );
    }
    let prepared;
    try {
      prepared = await prepareStripeEvent(event);
    } catch (error) {
      const configFailure = error instanceof BillingConfigError;
      return Response.json(
        {
          error: {
            type: configFailure
              ? 'stripe_unavailable'
              : 'stripe_retrieval_failed',
            message: configFailure
              ? error.message
              : 'Stripe evidence could not be retrieved. No billing state was changed.',
          },
        },
        { status: 503 },
      );
    }
    const result = await executeAdminMutation({
      actorUserId: actor.id,
      idempotencyKey,
      payload: parsed.data,
      execute: async (tx) =>
        applyPreparedStripeEventReconciliation(tx as any, {
          eventId: parsed.data.eventId!,
          actorUserId: actor.id,
          prepared,
        }),
    });
    return adminMutationResponse(result);
  }

  if (parsed.data.orderId) {
    let orderEvidence;
    try {
      orderEvidence = await loadAdminOrderReconciliationEvidence(
        parsed.data.orderId,
      );
    } catch (error) {
      const pending = error instanceof BillingPendingError;
      const configFailure = error instanceof BillingConfigError;
      return Response.json(
        {
          error: {
            type: pending
              ? (error.preCreation?.type ?? 'reconciliation_required')
              : configFailure
                ? 'stripe_unavailable'
                : 'stripe_retrieval_failed',
            message: pending
              ? error.message
              : configFailure
                ? error.message
                : 'Stripe evidence could not be retrieved. No billing state was changed.',
          },
        },
        { status: 503 },
      );
    }
    if (!orderEvidence)
      return Response.json(
        {
          error: {
            type: 'billing_order_not_found',
            message: 'Billing order not found.',
          },
        },
        { status: 404 },
      );
    const result = await executeAdminMutation({
      actorUserId: actor.id,
      idempotencyKey,
      payload: parsed.data,
      execute: async (tx) =>
        applyAdminOrderReconciliation(tx as any, {
          actorUserId: actor.id,
          evidence: orderEvidence,
        }),
    });
    return adminMutationResponse(result);
  }

  let evidence;
  try {
    evidence = await loadAdminRetryReconciliationEvidence(
      parsed.data.retryOperationId!,
    );
  } catch (error) {
    const pending = error instanceof BillingPendingError;
    const configFailure = error instanceof BillingConfigError;
    return Response.json(
      {
        error: {
          type: pending
            ? (error.preCreation?.type ?? 'reconciliation_required')
            : configFailure
              ? 'stripe_unavailable'
              : 'stripe_retrieval_failed',
          message: pending
            ? error.message
            : configFailure
              ? error.message
              : 'Stripe evidence could not be retrieved. No billing state was changed.',
        },
      },
      { status: 503 },
    );
  }
  if (!evidence)
    return Response.json(
      {
        error: {
          type: 'retry_operation_not_found',
          message: 'Retry operation not found.',
        },
      },
      { status: 404 },
    );
  const result = await executeAdminMutation({
    actorUserId: actor.id,
    idempotencyKey,
    payload: parsed.data,
    execute: async (tx) =>
      applyAdminRetryReconciliation(tx as any, {
        actorUserId: actor.id,
        evidence,
      }),
  });
  return adminMutationResponse(result);
};
