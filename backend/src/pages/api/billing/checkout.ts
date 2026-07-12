import type { APIRoute } from 'astro';
import { z } from 'zod';
import { eq } from 'drizzle-orm';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import { BillingConfigError, BillingEligibilityError, BillingRequestError, createBillingCheckout, rejectCrossOriginBillingPost } from '../../../lib/billing';
import { getSessionFromCookies } from '../../../lib/workos';
import { safeJsonBody } from '../../../lib/validation';

const checkoutSchema = z.object({
  kind: z.enum(['subscription', 'top_up']),
  priceId: z.string().optional(),
  tier: z.string().optional(),
});

export const POST: APIRoute = async ({ request }) => {
  const crossOriginResponse = rejectCrossOriginBillingPost(request);
  if (crossOriginResponse) return crossOriginResponse;

  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });

  const [user] = await db.select().from(users).where(eq(users.workosUserId, session.user.id)).limit(1);
  if (!user) return new Response('Unknown user', { status: 404 });

  const parsed = await safeJsonBody(request, checkoutSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const body = parsed.data;
  const kind = body.kind;

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
  }
};
