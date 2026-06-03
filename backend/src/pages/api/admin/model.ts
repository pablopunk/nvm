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
import { recordAudit } from '../../../lib/audit';

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
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const body = (await request.json().catch(() => ({}))) as { tier?: 'paid' | 'free'; model?: string };
  if (!body.model) return new Response('Missing model', { status: 400 });
  const provider = await getActiveProvider();
  const cost = await lookupModelCost(provider, body.model);
  if (!cost) return new Response(`No pricing for ${provider}/${body.model}`, { status: 400 });
  const tier: 'free' | 'paid' = body.tier === 'free' ? 'free' : 'paid';
  if (tier === 'free') await setFreeModelId(body.model);
  else await setActiveModelId(body.model);
  await recordAudit({
    actorUserId: actor.id,
    action: 'model.changed',
    targetType: 'model',
    targetId: body.model,
    meta: { tier, provider },
  });
  return Response.json({ ok: true });
};
