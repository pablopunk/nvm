import type { APIRoute } from 'astro';
import { eq } from 'drizzle-orm';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import { BillingConfigError, createBillingPortal } from '../../../lib/billing';
import { getSessionFromCookies } from '../../../lib/workos';

export const POST: APIRoute = async ({ request }) => {
  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });

  const [user] = await db.select().from(users).where(eq(users.workosUserId, session.user.id)).limit(1);
  if (!user) return new Response('Unknown user', { status: 404 });

  try {
    const portal = await createBillingPortal({ user });
    return Response.json({ url: portal.url });
  } catch (error) {
    if (error instanceof BillingConfigError) {
      return Response.json({ error: { type: 'billing_not_configured', message: error.message } }, { status: 503 });
    }
    throw error;
  }
};
