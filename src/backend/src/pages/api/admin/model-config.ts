import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { readAdminModelConfig } from '../../../lib/admin-model-config';

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  return Response.json(await readAdminModelConfig(), { headers: { 'Cache-Control': 'private, no-store' } });
};
