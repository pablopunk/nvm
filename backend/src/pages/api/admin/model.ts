import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import {
  getModelRoute,
  listKnownProviders,
  modelRouteToRef,
  parseModelRouteRef,
  setModelRoute,
  ModelNotConfiguredError,
  type ModelRouteSlot,
} from '../../../lib/settings';
import { listModelsForProvider, lookupModelCost } from '../../../lib/pricing';
import { recordAudit } from '../../../lib/audit';

async function listModelRefs() {
  const providers = listKnownProviders();
  const groups = await Promise.all(providers.map(async (provider) => {
    const models = await listModelsForProvider(provider);
    return models.map((modelId) => ({ provider, modelId, ref: modelRouteToRef({ provider, modelId }) }));
  }));
  return groups.flat();
}

async function safeRoute(slot: ModelRouteSlot) {
  try {
    const route = await getModelRoute(slot);
    return { ...route, ref: modelRouteToRef(route) };
  } catch (err) {
    if (err instanceof ModelNotConfiguredError) return null;
    throw err;
  }
}

function modelRouteSlotFromBody(value: unknown): ModelRouteSlot {
  return value === 'free' || value === 'smart' || value === 'fast' ? value : 'paid';
}

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  return Response.json({
    paid: await safeRoute('paid'),
    free: await safeRoute('free'),
    smart: await safeRoute('smart'),
    fast: await safeRoute('fast'),
    models: await listModelRefs(),
  });
};

export const PUT: APIRoute = async ({ request }) => {
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const body = (await request.json().catch(() => ({}))) as { tier?: ModelRouteSlot; purpose?: ModelRouteSlot; model?: string; modelRef?: string };
  const slot = modelRouteSlotFromBody(body.purpose ?? body.tier);
  const route = parseModelRouteRef(body.modelRef ?? body.model ?? '');
  if (!route) return new Response('Missing or invalid modelRef', { status: 400 });

  const cost = await lookupModelCost(route.provider, route.modelId);
  if (!cost) return new Response(`No pricing for ${route.provider}/${route.modelId}`, { status: 400 });

  await setModelRoute(slot, route);
  await recordAudit({
    actorUserId: actor.id,
    action: 'model.changed',
    targetType: 'model',
    targetId: modelRouteToRef(route),
    meta: { slot, provider: route.provider, ...(slot === 'free' || slot === 'paid' ? { tier: slot } : {}) },
  });
  return Response.json({ ok: true });
};
