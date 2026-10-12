import type { APIRoute } from 'astro';
import { eq } from 'drizzle-orm';
import { z } from 'zod';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import { requireSameOrigin } from '../../../lib/csrf';
import { getBillingCheckoutStatus, BillingRequestError, refreshBillingCheckoutStatus } from '../../../lib/billing';
import { getSessionFromCookies } from '../../../lib/workos';
import { safeJsonBody } from '../../../lib/validation';

const lookupSchema = z.object({
  attemptKey: z.string().uuid().optional(),
  session_id: z.string().min(1).max(255).optional(),
  retryOperationId: z.string().uuid().optional(),
});

async function authenticatedUser(request: Request) {
  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return null;
  const [user] = await db.select().from(users).where(eq(users.workosUserId, session.user.id)).limit(1);
  return user ?? null;
}

function conflictResponse(error: unknown) {
  if (!(error instanceof BillingRequestError) || error.type !== 'idempotency_conflict') return null;
  return Response.json({ error: { type: error.type, message: error.message } }, { status: 409 });
}

export const GET: APIRoute = async ({ request }) => {
  const user = await authenticatedUser(request);
  if (!user) return new Response('Unauthorized', { status: 401 });
  const url = new URL(request.url);
  const parsed = lookupSchema.safeParse({
    attemptKey: url.searchParams.get('attemptKey') ?? undefined,
    session_id: url.searchParams.get('session_id') ?? undefined,
    retryOperationId: url.searchParams.get('retryOperationId') ?? undefined,
  });
  if (!parsed.success)
    return Response.json(
      {
        error: { type: 'invalid_request', message: 'Invalid checkout lookup.' },
      },
      { status: 400 },
    );
  try {
    return Response.json(
      await getBillingCheckoutStatus({
        userId: user.id,
        attemptKey: parsed.data.attemptKey,
        sessionId: parsed.data.session_id,
        retryOperationId: parsed.data.retryOperationId,
      }),
    );
  } catch (error) {
    return (
      conflictResponse(error) ??
      Response.json(
        {
          error: {
            type: 'server_error',
            message: 'Unable to read checkout status.',
          },
        },
        { status: 500 },
      )
    );
  }
};

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const user = await authenticatedUser(request);
  if (!user) return new Response('Unauthorized', { status: 401 });
  const parsed = await safeJsonBody(request, lookupSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  try {
    return Response.json(
      await refreshBillingCheckoutStatus({
        userId: user.id,
        attemptKey: parsed.data.attemptKey,
        sessionId: parsed.data.session_id,
        retryOperationId: parsed.data.retryOperationId,
      }),
    );
  } catch (error) {
    return (
      conflictResponse(error) ??
      Response.json(
        {
          error: {
            type: 'server_error',
            message: 'Unable to refresh checkout status.',
          },
        },
        { status: 500 },
      )
    );
  }
};
