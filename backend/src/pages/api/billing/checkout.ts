import type { APIRoute } from 'astro';
import { eq } from 'drizzle-orm';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import { BillingConfigError, BillingEligibilityError, BillingRequestError, createBillingCheckout, rejectCrossOriginBillingPost, type BillingKind } from '../../../lib/billing';
import { acquireCheckoutLock, releaseCheckoutLock } from '../../../lib/ratelimit';
import { getSessionFromCookies } from '../../../lib/workos';

export const POST: APIRoute = async ({ request }) => {
  const crossOriginResponse = rejectCrossOriginBillingPost(request);
  if (crossOriginResponse) return crossOriginResponse;

  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });

  const [user] = await db.select().from(users).where(eq(users.workosUserId, session.user.id)).limit(1);
  if (!user) return new Response('Unknown user', { status: 404 });

  const body = (await request.json().catch(() => ({}))) as { kind?: BillingKind; priceId?: string; tier?: string };
  if (body.kind !== 'top_up' && body.kind !== 'subscription') {
    return Response.json({ error: { type: 'invalid_request', message: 'Invalid billing kind' } }, { status: 400 });
  }
  const kind = body.kind;

  if (kind === 'subscription') {
    const lock = await acquireCheckoutLock(user.id);
    if (!lock.ok) {
      return Response.json(
        { error: { type: 'checkout_in_flight', message: 'A checkout session is already in progress. Please wait and try again.' } },
        { status: 409 },
      );
    }
  }

  try {
    const checkout = await createBillingCheckout({ user, kind, priceId: body.priceId, tier: body.tier });
    return Response.json({ url: checkout.url });
  } catch (error) {
    if (error instanceof BillingConfigError) {
      return Response.json({ error: { type: 'billing_not_configured', message: error.message } }, { status: 503 });
    }
    if (error instanceof BillingRequestError) {
      return Response.json({ error: { type: error.type, message: error.message } }, { status: 400 });
    }
    if (error instanceof BillingEligibilityError) {
      return Response.json(
        { error: { type: error.type, message: error.message } },
        { status: error.type === 'already_subscribed' ? 409 : 403 },
      );
    }
    throw error;
  } finally {
    if (kind === 'subscription') {
      await releaseCheckoutLock(user.id);
    }
  }
};
