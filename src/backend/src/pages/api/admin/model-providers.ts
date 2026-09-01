import type { APIRoute } from 'astro';
import { z } from 'zod';
import { requireAdmin } from '../../../lib/admin';
import { requireSameOrigin } from '../../../lib/csrf';
import {
  getConfiguredModelProviderRoutes,
  getModelRoute,
  setModelProviderChain,
  listAllProviders,
  parseModelRouteSlot,
  type ModelRouteSlot,
} from '../../../lib/settings';
import { listModelsForProvider, lookupModelPricing } from '../../../lib/pricing';
import { providerSupportsFormat, selectApiForModel } from '../../../lib/upstream';
import { recordAudit } from '../../../lib/audit';
import { safeJsonBody } from '../../../lib/validation';

function slotFromBody(value: unknown): ModelRouteSlot {
  return parseModelRouteSlot(value) ?? 'paid';
}

const putProvidersSchema = z.object({
  slot: z.enum(['paid', 'free', 'smart', 'fast', 'pro-smart', 'pro-fast', 'free-smart', 'free-fast']).optional(),
  modelId: z.string().optional(),
  providerIds: z.array(z.string()).optional(),
  routes: z.array(z.object({ providerId: z.string().min(1), modelId: z.string().min(1) })).optional(),
});

export const GET: APIRoute = async ({ request, url }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const slot = slotFromBody(url.searchParams.get('slot'));
  const modelId = url.searchParams.get('modelId');
  if (!modelId) return new Response('Missing modelId', { status: 400 });

  const [chain, providers, primaryRoute] = await Promise.all([
    getConfiguredModelProviderRoutes(slot, modelId),
    listAllProviders(),
    getModelRoute(slot),
  ]);
  const format = selectApiForModel('configured', modelId);
  const providerOptions = await Promise.all(providers.map(async (provider) => {
    const compatibleModels = providerSupportsFormat(provider.id, format)
      ? (await listModelsForProvider(provider.id)).filter((candidateModelId) => selectApiForModel(provider.id, candidateModelId) === format)
      : [];
    return {
      id: provider.id,
      displayName: provider.displayName,
      enabled: provider.enabled === 'true',
      compatibleModels,
    };
  }));
  const chainWithPricing = await Promise.all(chain.map(async (route) => ({
    ...route,
    pricing: await lookupModelPricing(route.providerId, route.modelId),
  })));
  return Response.json({
    slot,
    modelId,
    chain: chain.map((route) => route.providerId),
    routes: chainWithPricing,
    providers: providerOptions,
    primaryPricing: await lookupModelPricing(primaryRoute.provider, primaryRoute.modelId),
  });
};

export const PUT: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;

  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });

  const parsed = await safeJsonBody(request, putProvidersSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const body = parsed.data;

  const slot = slotFromBody(body.slot);
  if (!body.modelId) return new Response('Missing modelId', { status: 400 });
  const routes = body.routes ?? body.providerIds?.map((providerId) => ({ providerId, modelId: body.modelId! }));
  if (!routes) return new Response('Missing routes', { status: 400 });
  const providers = await listAllProviders();
  const knownProviders = new Set(providers.map((provider) => provider.id));
  if (routes.some((route) => !knownProviders.has(route.providerId))) {
    return new Response('Unknown provider', { status: 400 });
  }
  if (new Set(routes.map((route) => route.providerId)).size !== routes.length) {
    return new Response('Duplicate provider', { status: 400 });
  }
  const enabledProviders = new Set(providers.filter((provider) => provider.enabled === 'true').map((provider) => provider.id));
  if (routes.some((route) => !enabledProviders.has(route.providerId))) {
    return new Response('Disabled provider', { status: 400 });
  }
  const format = selectApiForModel('configured', body.modelId);
  if (routes.some((route) => !providerSupportsFormat(route.providerId, format) || selectApiForModel(route.providerId, route.modelId) !== format)) {
    return new Response('Provider model uses an incompatible API format', { status: 400 });
  }
  const pricing = await Promise.all(routes.map((route) => lookupModelPricing(route.providerId, route.modelId)));
  if (pricing.some((value) => !value)) return new Response('Missing provider model pricing', { status: 400 });

  await setModelProviderChain(slot, body.modelId, routes);

  await recordAudit({
    actorUserId: actor.id,
    action: 'provider.changed',
    targetType: 'model_provider_chain',
    targetId: `${slot}/${body.modelId}`,
    meta: { slot, modelId: body.modelId, routes },
  });

  return Response.json({ ok: true });
};
