import type { APIRoute } from 'astro';
import { eq, desc } from 'drizzle-orm';
import { getSessionFromCookies } from '../../lib/workos';
import { db } from '../../db/client';
import { users, usage } from '../../db/schema';
import { ensureMonthlyFreeCredits, getBalance } from '../../lib/users';

export const GET: APIRoute = async ({ request }) => {
  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });

  const [user] = await db
    .select()
    .from(users)
    .where(eq(users.workosUserId, session.user.id))
    .limit(1);
  if (!user) return new Response('Unknown user', { status: 404 });
  await ensureMonthlyFreeCredits(user.id);

  const [balance, recentUsage] = await Promise.all([
    getBalance(user.id),
    db.select().from(usage).where(eq(usage.userId, user.id)).orderBy(desc(usage.createdAt)).limit(10),
  ]);

  return Response.json({
    email: user.email,
    plan: user.plan,
    balance,
    recentUsage,
  });
};
