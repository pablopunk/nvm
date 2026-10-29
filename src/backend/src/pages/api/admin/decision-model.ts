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
  findEligibleDecisionModel,
  isSelectableDecisionModel,
  listDecisionModels,
  refreshDecisionCatalog,
} from '../../../lib/decisions/decision-models';
import {
  getDecisionModelRoute,
  isProviderEnabled,
  ModelNotConfiguredError,
  setDecisionModelRoute,
} from '../../../lib/settings';
import { safeJsonBody } from '../../../lib/validation';

const updateDecisionModelSchema = z
  .object({ provider: z.literal('openrouter'), modelId: z.string().min(1) })
  .strict();

async function currentDecisionModel() {
  try {
    return await getDecisionModelRoute();
  } catch (error) {
    if (error instanceof ModelNotConfiguredError) return null;
    throw error;
  }
}

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  return Response.json({
    current: await currentDecisionModel(),
    models: (await listDecisionModels()).filter(isSelectableDecisionModel),
  });
};

export const PUT: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const parsed = await safeJsonBody(request, updateDecisionModelSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const route = parsed.data;
  if (!(await isProviderEnabled(route.provider)))
    return new Response(`Provider ${route.provider} is disabled`, {
      status: 400,
    });
  // Admin-selected routes validate their catalog here, the one fetch outside the scheduled refresh.
  await refreshDecisionCatalog().catch(() => undefined);
  const eligible = await findEligibleDecisionModel(
    route.provider,
    route.modelId,
  ).catch(() => null);
  if (!eligible)
    return new Response('Model is not an eligible decision model', {
      status: 400,
    });
  if (!isSelectableDecisionModel(eligible))
    return new Response(
      'Decision model must support images and have verified billing bounds',
      { status: 400 },
    );
  const idempotencyKey = adminMutationKeyFromRequest(request);
  if (!idempotencyKey)
    return Response.json(
      {
        error: {
          type: 'idempotency_key_required',
          message: 'Provide a UUID Idempotency-Key.',
        },
      },
      { status: 400 },
    );
  const result = await executeAdminMutation({
    actorUserId: actor.id,
    idempotencyKey,
    payload: { ...route, acceptsImages: eligible.acceptsImages },
    execute: async (tx) => {
      await setDecisionModelRoute(route, tx);
      await recordAuditStrict(tx, {
        actorUserId: actor.id,
        action: 'model.changed',
        targetType: 'decision_model',
        targetId: `${route.provider}/${route.modelId}`,
        meta: {
          slot: 'decision',
          provider: route.provider,
          acceptsImages: eligible.acceptsImages,
          idempotencyKey,
        },
      });
      return {
        body: { ok: true, route, acceptsImages: eligible.acceptsImages },
      };
    },
  });
  return adminMutationResponse(result);
};
