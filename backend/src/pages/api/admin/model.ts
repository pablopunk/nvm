import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { getActiveModelId, setActiveModelId } from '../../../lib/settings';
import { listModels, MODELS } from '../../../lib/pricing';

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  return Response.json({ active: await getActiveModelId(), models: listModels() });
};

export const PUT: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const body = (await request.json().catch(() => ({}))) as { model?: string };
  if (!body.model || !MODELS[body.model]) return new Response('Unknown model', { status: 400 });
  await setActiveModelId(body.model);
  return Response.json({ active: body.model });
};
