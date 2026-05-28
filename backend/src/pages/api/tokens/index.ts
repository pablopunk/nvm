import type { APIRoute } from 'astro';
import { eq } from 'drizzle-orm';
import { getSessionFromCookies } from '../../../lib/workos';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import { createApiToken, listApiTokens } from '../../../lib/tokens';

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
  const user = await getUser(request);
  if (!user) return new Response('Unauthorized', { status: 401 });
  const body = (await request.json().catch(() => ({}))) as { name?: string };
  const name = (body.name ?? '').trim() || 'Untitled token';
  const created = await createApiToken(user.id, name);
  return Response.json(created, { status: 201 });
};
