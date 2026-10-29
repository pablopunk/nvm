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
  getAudioModelRoute,
  isProviderEnabled,
  ModelNotConfiguredError,
  setAudioModelRoute,
} from '../../../lib/settings';
import {
  listTranscriptionModels,
  lookupTranscriptionDurationPricing,
  refreshTranscriptionCatalog,
} from '../../../lib/transcription-models';
import { safeJsonBody } from '../../../lib/validation';

const updateAudioModelSchema = z
  .object({ provider: z.literal('openrouter'), modelId: z.string().min(1) })
  .strict();

async function currentAudioModel() {
  try {
    return await getAudioModelRoute();
  } catch (error) {
    if (error instanceof ModelNotConfiguredError) return null;
    throw error;
  }
}

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  return Response.json({
    current: await currentAudioModel(),
    models: await listTranscriptionModels(),
  });
};

export const PUT: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const parsed = await safeJsonBody(request, updateAudioModelSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const route = parsed.data;
  if (!(await isProviderEnabled(route.provider)))
    return new Response(`Provider ${route.provider} is disabled`, {
      status: 400,
    });
  // Admin-selected routes validate their catalog here, the one fetch outside the scheduled refresh.
  await refreshTranscriptionCatalog().catch(() => undefined);
  const pricing = await lookupTranscriptionDurationPricing(
    route.provider,
    route.modelId,
  ).catch(() => null);
  if (!pricing)
    return new Response(
      'Model does not have verified audio transcription pricing',
      {
        status: 400,
      },
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
    payload: route,
    execute: async (tx) => {
      await setAudioModelRoute(route, tx);
      await recordAuditStrict(tx, {
        actorUserId: actor.id,
        action: 'model.changed',
        targetType: 'audio_model',
        targetId: `${route.provider}/${route.modelId}`,
        meta: { slot: 'audio', provider: route.provider, idempotencyKey },
      });
      return { body: { ok: true, route } };
    },
  });
  return adminMutationResponse(result);
};
