import type { APIRoute } from 'astro';
import { eq } from 'drizzle-orm';
import { z } from 'zod';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import { requireSameOrigin } from '../../../lib/csrf';
import { BillingRequestError, retryBillingCheckout } from '../../../lib/billing';
import { getSessionFromCookies } from '../../../lib/workos';
import { safeJsonBody } from '../../../lib/validation';

const retrySchema = z
  .object({
    attemptKey: z.string().uuid().optional(),
    retryOperationId: z.string().uuid().optional(),
    retryKey: z.string().uuid(),
  })
  .refine((value) => Boolean(value.attemptKey || value.retryOperationId), 'An attemptKey or retryOperationId is required.');

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });
  const [user] = await db.select().from(users).where(eq(users.workosUserId, session.user.id)).limit(1);
  if (!user) return new Response('Unknown user', { status: 404 });
  const parsed = await safeJsonBody(request, retrySchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  let result: Awaited<ReturnType<typeof retryBillingCheckout>>;
  try {
    result = await retryBillingCheckout({ userId: user.id, ...parsed.data });
  } catch (error) {
    if (error instanceof BillingRequestError)
      return Response.json({ error: { type: error.type, message: error.message } }, { status: 409 });
    throw error;
  }
  const status = result.status === 'processing' ? 202 : result.status === 'reconciliation_required' ? 409 : 200;
  return Response.json(result, { status });
};
