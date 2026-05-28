import type { APIRoute } from 'astro';
import { eq } from 'drizzle-orm';
import { getSessionFromCookies } from '../../../lib/workos';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import { revokeApiToken } from '../../../lib/tokens';

export const DELETE: APIRoute = async ({ request, params }) => {
  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });
  const [user] = await db.select().from(users).where(eq(users.workosUserId, session.user.id)).limit(1);
  if (!user) return new Response('Unknown user', { status: 404 });
  await revokeApiToken(user.id, params.id as string);
  return new Response(null, { status: 204 });
};

export const POST: APIRoute = async (ctx) => DELETE(ctx);
