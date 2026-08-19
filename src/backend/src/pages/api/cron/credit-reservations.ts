import type { APIRoute } from 'astro';
import { isAuthorizedCron } from '../../../lib/cron-auth';
import { reconcileStaleReservations } from '../../../lib/credit-reservations';

export const GET: APIRoute = async ({ request, url }) => {
  if (!isAuthorizedCron(request)) return new Response('Unauthorized', { status: 401 });
  const limit = Number(url.searchParams.get('limit') ?? '100');
  return Response.json(await reconcileStaleReservations(Number.isFinite(limit) ? limit : 100));
};
