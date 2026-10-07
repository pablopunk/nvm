import type { APIRoute } from 'astro';
import { z } from 'zod';
import { eq } from 'drizzle-orm';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import {
  BillingConfigError,
  BillingEligibilityError,
  BillingPendingError,
  BillingRequestError,
  createBillingCheckout,
  rejectCrossOriginBillingPost,
} from '../../../lib/billing';
import { acquireCheckoutLock, releaseCheckoutLock } from '../../../lib/ratelimit';
import { getSessionFromCookies } from '../../../lib/workos';
import { safeJsonBody } from '../../../lib/validation';

const checkoutSchema = z.object({
  kind: z.enum(['subscription', 'top_up']),
  priceId: z.string().optional(),
  tier: z.string().optional(),
});

const attemptKeySchema = z.string().uuid();

export const POST: APIRoute = async ({ request }) => {
  const crossOriginResponse = rejectCrossOriginBillingPost(request);
  if (crossOriginResponse) return crossOriginResponse;

  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });

  const [user] = await db.select().from(users).where(eq(users.workosUserId, session.user.id)).limit(1);
  if (!user) return new Response('Unknown user', { status: 404 });

  const parsed = await safeJsonBody(request, checkoutSchema);
  if (!parsed.ok) return Response.json({ status: 'not_created', error: parsed.error.error }, { status: 400 });
  const body = parsed.data;
  const kind = body.kind;
  const suppliedAttemptKey = request.headers.get('idempotency-key');
  const attemptKey = suppliedAttemptKey ? attemptKeySchema.safeParse(suppliedAttemptKey) : null;
  if (attemptKey && !attemptKey.success) {
    return Response.json(
      {
        status: 'not_created',
        error: {
          type: 'invalid_idempotency_key',
          message: 'Idempotency-Key must be a UUID.',
        },
      },
      { status: 400 },
    );
  }

  if (!suppliedAttemptKey && kind === 'subscription') {
    const lock = await acquireCheckoutLock(user.id);
    if (!lock.ok) {
      return Response.json(
        {
          error: {
            type: 'checkout_in_flight',
            message: 'A checkout session is already in progress. Please wait and try again.',
          },
        },
        { status: 409 },
      );
    }
  }

  try {
    const checkout = await createBillingCheckout({
      user,
      kind,
      priceId: body.priceId,
      tier: body.tier,
      attemptKey: attemptKey?.data,
      legacyCaller: !suppliedAttemptKey,
    });
    return checkout.status === 'url' ? Response.json({ url: checkout.url }) : Response.json({ status: checkout.status }, { status: 202 });
  } catch (error) {
    if (error instanceof BillingPendingError) {
      const statusCode = error.status === 'processing' ? 202 : error.status === 'reconciliation_required' ? 409 : 200;
      return Response.json(
        {
          status: error.status,
          ...(error.attemptKey ? { attemptKey: error.attemptKey } : {}),
        },
        { status: statusCode },
      );
    }
    if (error instanceof BillingConfigError) {
      return Response.json(
        {
          status: 'not_created',
          error: { type: 'billing_not_configured', message: error.message },
        },
        { status: 503 },
      );
    }
    if (error instanceof BillingRequestError) {
      const status = error.type === 'idempotency_conflict' || error.type === 'checkout_in_flight' ? 409 : 400;
      return Response.json(
        {
          ...(error.type === 'idempotency_conflict' ? {} : { status: 'not_created' }),
          error: { type: error.type, message: error.message },
        },
        { status },
      );
    }
    if (error instanceof BillingEligibilityError) {
      return Response.json(
        {
          status: 'not_created',
          error: { type: error.type, message: error.message },
        },
        { status: error.type === 'already_subscribed' ? 409 : 403 },
      );
    }
    throw error;
  } finally {
    if (!suppliedAttemptKey && kind === 'subscription') {
      await releaseCheckoutLock(user.id);
    }
  }
};
