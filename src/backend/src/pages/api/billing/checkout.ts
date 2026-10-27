import type { APIRoute } from 'astro';
import { z } from 'zod';
import { eq } from 'drizzle-orm';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import {
  BillingConfigError,
  BillingEligibilityError,
  BillingRequestError,
  createBillingCheckout,
  rejectCrossOriginBillingPost,
} from '../../../lib/billing';
import { getSessionFromCookies } from '../../../lib/workos';
import { safeJsonBody } from '../../../lib/validation';

const checkoutSchema = z.object({
  kind: z.enum(['subscription', 'top_up']),
  priceId: z.string().optional(),
  tier: z.string().optional(),
});

function checkoutError(status: number, type: string, message: string) {
  return Response.json({ status: 'not_created', error: { type, message } }, { status });
}

export const POST: APIRoute = async ({ request }) => {
  const crossOriginResponse = rejectCrossOriginBillingPost(request);
  if (crossOriginResponse) return crossOriginResponse;

  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });

  const [user] = await db.select().from(users).where(eq(users.workosUserId, session.user.id)).limit(1);
  if (!user) return new Response('Unknown user', { status: 404 });

  const parsed = await safeJsonBody(request, checkoutSchema);
  if (!parsed.ok) return Response.json({ status: 'not_created', error: parsed.error.error }, { status: 400 });

  try {
    return Response.json(await createBillingCheckout({ user, ...parsed.data }));
  } catch (error) {
    if (error instanceof BillingConfigError) return checkoutError(503, 'billing_not_configured', error.message);
    if (error instanceof BillingRequestError) return checkoutError(400, error.type, error.message);
    if (error instanceof BillingEligibilityError)
      return checkoutError(error.type === 'already_subscribed' ? 409 : 403, error.type, error.message);
    throw error;
  }
};
