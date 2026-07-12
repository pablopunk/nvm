import type { APIRoute } from 'astro';
import { z } from 'zod';
import { requireAdmin } from '../../../lib/admin';
import { requireSameOrigin } from '../../../lib/csrf';
import {
  getModelProviderChain,
  setModelProviderChain,
  getModelRoute,
  modelRouteToRef,
  type ModelRouteSlot,
} from '../../../lib/settings';
import { recordAudit } from '../../../lib/audit';
import { safeJsonBody } from '../../../lib/validation';

function slotFromBody(value: unknown): ModelRouteSlot {
  return value === 'free' || value === 'smart' || value === 'fast' ? value : 'paid';
}

const putProvidersSchema = z.object({
  slot: z.string().optional(),
  modelId: z.string().optional(),
  providerIds: z.array(z.string()).optional(),
});

export const GET: APIRoute = async ({ request, url }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const slot = slotFromBody(url.searchParams.get('slot'));
  const modelId = url.searchParams.get('modelId');
  if (!modelId) return new Response('Missing modelId', { status: 400 });

  const chain = await getModelProviderChain(slot, modelId);
  return Response.json({ slot, modelId, chain });
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
  if (!body.providerIds || !Array.isArray(body.providerIds)) return new Response('Missing providerIds', { status: 400 });

  await setModelProviderChain(slot, body.modelId, body.providerIds);

  await recordAudit({
    actorUserId: actor.id,
    action: 'provider.changed',
    targetType: 'model_provider_chain',
    targetId: `${slot}/${body.modelId}`,
    meta: { slot, modelId: body.modelId, providerIds: body.providerIds },
  });

  return Response.json({ ok: true });
};
