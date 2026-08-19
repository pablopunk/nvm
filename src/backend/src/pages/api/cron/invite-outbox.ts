import type { APIRoute } from 'astro';
import { isAuthorizedCron } from '../../../lib/cron-auth';
import { sendOutboxBatch } from '../../../lib/email';

export const GET: APIRoute = async ({ request }) => {
  if (!isAuthorizedCron(request)) return new Response('Unauthorized', { status: 401 });
  return Response.json(await sendOutboxBatch(10));
};
