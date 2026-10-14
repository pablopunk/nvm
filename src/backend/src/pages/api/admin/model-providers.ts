import type { APIRoute } from 'astro';
import { z } from 'zod';
import { requireAdmin } from '../../../lib/admin';
import {
  adminMutationKeyFromRequest,
  adminMutationResponse,
  executeAdminMutation,
} from '../../../lib/admin-mutations';
import { recordAuditStrict } from '../../../lib/audit';
import { requireSameOrigin } from '../../../lib/csrf';
import {
  hasReviewedImageInputSupport,
  listReviewedImageModelsForProvider,
  lookupModelPricing,
} from '../../../lib/pricing';
import {
  getConfiguredModelProviderRoutes,
  getModelRoute,
  listAllProviders,
  type ModelRouteSlot,
  parseModelRouteSlot,
  setModelProviderChain,
} from '../../../lib/settings';
import {
  providerSupportsFormat,
  selectApiForModel,
} from '../../../lib/upstream';
import { safeJsonBody } from '../../../lib/validation';

function slotFromBody(value: unknown): ModelRouteSlot {
  return parseModelRouteSlot(value) ?? 'paid';
}

const putProvidersSchema = z
  .object({
    slot: z
      .enum([
        'paid',
        'free',
        'smart',
        'fast',
        'pro-smart',
        'pro-fast',
        'free-smart',
        'free-fast',
      ])
      .optional(),
    modelId: z.string().min(1).max(256),
    providerIds: z.array(z.string().min(1).max(128)).max(20).optional(),
    routes: z
      .array(
        z
          .object({
            providerId: z.string().min(1).max(128),
            modelId: z.string().min(1).max(256),
          })
          .strict(),
      )
      .max(20)
      .optional(),
  })
  .strict();

export const GET: APIRoute = async ({ request, url }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  const slot = slotFromBody(url.searchParams.get('slot'));
  const modelId = url.searchParams.get('modelId');
  if (!modelId) return new Response('Missing modelId', { status: 400 });

  const [chain, providers, primaryRoute] = await Promise.all([
    getConfiguredModelProviderRoutes(slot, modelId),
    listAllProviders(),
    getModelRoute(slot),
  ]);
  const format = selectApiForModel('configured', modelId);
  const providerOptions = await Promise.all(
    providers.map(async (provider) => {
      const compatibleModels = providerSupportsFormat(provider.id, format)
        ? (await listReviewedImageModelsForProvider(provider.id)).filter(
            (candidateModelId) =>
              selectApiForModel(provider.id, candidateModelId) === format,
          )
        : [];
      return {
        id: provider.id,
        displayName: provider.displayName,
        enabled: provider.enabled === 'true',
        compatibleModels,
      };
    }),
  );
  const chainWithPricing = await Promise.all(
    chain.map(async (route) => ({
      ...route,
      pricing: await lookupModelPricing(route.providerId, route.modelId),
    })),
  );
  return Response.json({
    slot,
    modelId,
    chain: chain.map((route) => route.providerId),
    routes: chainWithPricing,
    providers: providerOptions,
    primaryPricing: await lookupModelPricing(
      primaryRoute.provider,
      primaryRoute.modelId,
    ),
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
  if (!body.modelId)
    return Response.json(
      { error: { type: 'missing_model', message: 'Missing modelId.' } },
      { status: 400 },
    );
  const routes =
    body.routes ??
    body.providerIds?.map((providerId) => ({
      providerId,
      modelId: body.modelId!,
    }));
  if (!routes)
    return Response.json(
      { error: { type: 'missing_routes', message: 'Missing routes.' } },
      { status: 400 },
    );
  const providers = await listAllProviders();
  const knownProviders = new Set(providers.map((provider) => provider.id));
  if (routes.some((route) => !knownProviders.has(route.providerId))) {
    return Response.json(
      {
        error: {
          type: 'unknown_provider',
          message: 'The route includes an unknown provider.',
        },
      },
      { status: 400 },
    );
  }
  if (new Set(routes.map((route) => route.providerId)).size !== routes.length) {
    return Response.json(
      {
        error: {
          type: 'duplicate_provider',
          message: 'A provider may appear only once in the route.',
        },
      },
      { status: 400 },
    );
  }
  const enabledProviders = new Set(
    providers
      .filter((provider) => provider.enabled === 'true')
      .map((provider) => provider.id),
  );
  if (routes.some((route) => !enabledProviders.has(route.providerId))) {
    return Response.json(
      {
        error: {
          type: 'provider_disabled',
          message: 'A route provider is disabled.',
        },
      },
      { status: 400 },
    );
  }
  const format = selectApiForModel('configured', body.modelId);
  if (
    routes.some(
      (route) =>
        !providerSupportsFormat(route.providerId, format) ||
        selectApiForModel(route.providerId, route.modelId) !== format,
    )
  ) {
    return Response.json(
      {
        error: {
          type: 'incompatible_provider_model',
          message: 'A provider model uses an incompatible API format.',
        },
      },
      { status: 400 },
    );
  }
  const routesSupportImages = await Promise.all(
    routes.map((route) =>
      hasReviewedImageInputSupport(route.providerId, route.modelId),
    ),
  );
  if (routesSupportImages.some((supported) => !supported))
    return Response.json(
      {
        error: {
          type: 'unsupported_image_model',
          message:
            'Every provider route must have reviewed image-input and billing support.',
        },
      },
      { status: 400 },
    );
  const pricing = await Promise.all(
    routes.map((route) => lookupModelPricing(route.providerId, route.modelId)),
  );
  if (pricing.some((value) => !value))
    return Response.json(
      {
        error: {
          type: 'pricing_unavailable',
          message: 'A provider model has no known pricing.',
        },
      },
      { status: 400 },
    );
  const idempotencyKey = adminMutationKeyFromRequest(request);
  if (!idempotencyKey) {
    return Response.json(
      {
        error: {
          type: 'idempotency_key_required',
          message: 'Provide a UUID Idempotency-Key.',
        },
      },
      { status: 400 },
    );
  }

  const result = await executeAdminMutation({
    actorUserId: actor.id,
    idempotencyKey,
    payload: { slot, modelId: body.modelId, routes },
    execute: async (tx) => {
      await setModelProviderChain(slot, body.modelId, routes, tx);
      await recordAuditStrict(tx, {
        actorUserId: actor.id,
        action: 'provider.changed',
        targetType: 'model_provider_chain',
        targetId: `${slot}/${body.modelId}`,
        meta: { slot, modelId: body.modelId, routes, idempotencyKey },
      });
      return { body: { ok: true, slot, modelId: body.modelId, routes } };
    },
  });
  return adminMutationResponse(result);
};
