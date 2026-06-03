import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import {
  getActiveModelId,
  getActiveProvider,
  getFreeModelId,
  setActiveModelId,
  setFreeModelId,
  ModelNotConfiguredError,
} from '../../../lib/settings';
import { listModelsForProvider, lookupModelCost } from '../../../lib/pricing';

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const provider = await getActiveProvider();
  const safe = async (fn: () => Promise<string>) => {
    try { return await fn(); } catch (err) { if (err instanceof ModelNotConfiguredError) return null; throw err; }
  };
  return Response.json({
    active: await safe(getActiveModelId),
    free: await safe(getFreeModelId),
    models: await listModelsForProvider(provider),
  });
};

export const PUT: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const body = (await request.json().catch(() => ({}))) as { tier?: 'paid' | 'free'; model?: string };
  if (!body.model) return new Response('Missing model', { status: 400 });
  const provider = await getActiveProvider();
  const cost = await lookupModelCost(provider, body.model);
  if (!cost) return new Response(`No pricing for ${provider}/${body.model}`, { status: 400 });
  if (body.tier === 'free') await setFreeModelId(body.model);
  else await setActiveModelId(body.model);
  return Response.json({ ok: true });
};
