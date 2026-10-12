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
import { listModelsForProvider, lookupModelCost } from '../../../lib/pricing';
import {
  DEFAULT_THINKING_LEVEL,
  getModelRoute,
  isProviderEnabled,
  listKnownProviders,
  ModelNotConfiguredError,
  type ModelRouteSlot,
  modelRouteToRef,
  parseModelRouteRef,
  parseModelRouteSlot,
  setModelRoute,
  THINKING_LEVELS,
  type ThinkingLevel,
} from '../../../lib/settings';
import { safeJsonBody } from '../../../lib/validation';

const putModelSchema = z.object({
  tier: z.enum(['paid', 'free']).optional(),
  purpose: z
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
  model: z.string().optional(),
  modelRef: z.string().optional(),
  thinkingLevel: z
    .string()
    .optional()
    .refine(
      (value) =>
        value === undefined || THINKING_LEVELS.includes(value as ThinkingLevel),
      'Invalid thinking level',
    ),
});

async function listModelRefs() {
  const providers = listKnownProviders();
  const groups = await Promise.all(
    providers.map(async (provider) => {
      const models = await listModelsForProvider(provider);
      return models.map((modelId) => ({
        provider,
        modelId,
        ref: modelRouteToRef({ provider, modelId }),
      }));
    }),
  );
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
  return parseModelRouteSlot(value) ?? 'paid';
}

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  return Response.json({
    paid: await safeRoute('paid'),
    free: await safeRoute('free'),
    smart: await safeRoute('smart'),
    fast: await safeRoute('fast'),
    proSmart: await safeRoute('pro-smart'),
    proFast: await safeRoute('pro-fast'),
    freeSmart: await safeRoute('free-smart'),
    freeFast: await safeRoute('free-fast'),
    models: await listModelRefs(),
  });
};

export const PUT: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;

  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });

  const parsed = await safeJsonBody(request, putModelSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const body = parsed.data;

  const slot = modelRouteSlotFromBody(body.purpose ?? body.tier);
  const route = parseModelRouteRef(body.modelRef ?? body.model ?? '');
  if (!route)
    return Response.json(
      {
        error: {
          type: 'invalid_model',
          message: 'Missing or invalid modelRef.',
        },
      },
      { status: 400 },
    );

  const cost = await lookupModelCost(route.provider, route.modelId);
  if (!cost)
    return Response.json(
      {
        error: {
          type: 'pricing_unavailable',
          message: 'The selected model has no known pricing.',
        },
      },
      { status: 400 },
    );
  if (!(await isProviderEnabled(route.provider)))
    return Response.json(
      {
        error: {
          type: 'provider_disabled',
          message: 'The selected provider is disabled.',
        },
      },
      { status: 400 },
    );

  const thinkingLevel =
    (body.thinkingLevel as ThinkingLevel | undefined) ??
    (await safeRoute(slot))?.thinkingLevel ??
    DEFAULT_THINKING_LEVEL;
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
    payload: { slot, route, thinkingLevel },
    execute: async (tx) => {
      await setModelRoute(slot, { ...route, thinkingLevel }, tx);
      await recordAuditStrict(tx, {
        actorUserId: actor.id,
        action: 'model.changed',
        targetType: 'model',
        targetId: modelRouteToRef(route),
        meta: {
          slot,
          provider: route.provider,
          thinkingLevel,
          ...(slot === 'free' || slot === 'paid' ? { tier: slot } : {}),
          idempotencyKey,
        },
      });
      return {
        body: {
          ok: true,
          slot,
          modelRef: modelRouteToRef(route),
          thinkingLevel,
        },
      };
    },
  });
  return adminMutationResponse(result);
};
