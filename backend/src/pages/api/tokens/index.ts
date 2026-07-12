import type { APIRoute } from 'astro';
import { z } from 'zod';
import { eq } from 'drizzle-orm';
import { getSessionFromCookies } from '../../../lib/workos';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import { createApiToken, listApiTokens } from '../../../lib/tokens';
import { requireSameOrigin } from '../../../lib/csrf';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../lib/ratelimit';
import { safeJsonBody } from '../../../lib/validation';

const createTokenSchema = z.object({
  name: z.string().min(1).max(120).optional(),
});

async function getUser(request: Request) {
  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return null;
  const [user] = await db.select().from(users).where(eq(users.workosUserId, session.user.id)).limit(1);
  return user ?? null;
}

export const GET: APIRoute = async ({ request }) => {
  const user = await getUser(request);
  if (!user) return new Response('Unauthorized', { status: 401 });
  return Response.json({ tokens: await listApiTokens(user.id) });
};

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;

  const decision = await rateLimitIp('tokens', clientIp(request), 10, '1 m');
  if (!decision.ok) return tooManyRequests(decision);
  const user = await getUser(request);
  if (!user) return new Response('Unauthorized', { status: 401 });

  const parsed = await safeJsonBody(request, createTokenSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const name = (parsed.data.name ?? '').trim() || 'Untitled token';
  const created = await createApiToken(user.id, name);
  return Response.json(created, { status: 201 });
};
