import type { APIRoute } from 'astro';
import { z } from 'zod';
import { recordAudit } from '../../../lib/audit';
import { requireAdmin } from '../../../lib/admin';
import { requireSameOrigin } from '../../../lib/csrf';
import {
  getAudioModelRoute,
  isProviderEnabled,
  ModelNotConfiguredError,
  setAudioModelRoute,
} from '../../../lib/settings';
import {
  isCompatibleTranscriptionModel,
  listTranscriptionModels,
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
  if (
    !(await isCompatibleTranscriptionModel(route.provider, route.modelId, {
      fresh: true,
    }))
  )
    return new Response('Model does not support audio transcription', {
      status: 400,
    });
  await setAudioModelRoute(route);
  await recordAudit({
    actorUserId: actor.id,
    action: 'model.changed',
    targetType: 'audio_model',
    targetId: `${route.provider}/${route.modelId}`,
    meta: { slot: 'audio', provider: route.provider },
  });
  return Response.json({ ok: true });
};
