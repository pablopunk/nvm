import type { APIRoute } from 'astro';
import { getCurrentUser } from '../../../lib/admin';
import { dashboardRead } from '../../../lib/dashboard-read';
import { ensureMonthlyFreeCredits, getBalances } from '../../../lib/users';

export const GET: APIRoute = async ({ request }) => {
  const user = await getCurrentUser(request);
  if (!user) return new Response('Unauthorized', { status: 401 });
  await ensureMonthlyFreeCredits(user.id);
  const balances = await dashboardRead(() => getBalances(user.id));
  return Response.json(balances, { headers: { 'Cache-Control': 'private, no-store' } });
};
