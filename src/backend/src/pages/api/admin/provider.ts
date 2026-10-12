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
  getModelRoute,
  listAllProviders,
  type ModelRouteSlot,
  updateProvider,
} from '../../../lib/settings';
import { safeJsonBody } from '../../../lib/validation';

const putProviderSchema = z
  .object({
    id: z.string().min(1).max(128),
    enabled: z.boolean().optional(),
    priority: z.number().int().min(0).max(100_000).optional(),
  })
  .strict()
  .refine((body) => body.enabled !== undefined || body.priority !== undefined, {
    message: 'Provide enabled or priority.',
  });

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  const all = await listAllProviders();
  return Response.json({ providers: all });
};

export const PUT: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;

  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });

  const parsed = await safeJsonBody(request, putProviderSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const body = parsed.data;

  if (!body.id)
    return Response.json(
      { error: { type: 'missing_provider', message: 'Missing provider id.' } },
      { status: 400 },
    );
  if (body.enabled === false) {
    const slots: ModelRouteSlot[] = [
      'pro-smart',
      'pro-fast',
      'free-smart',
      'free-fast',
    ];
    const routes = await Promise.all(
      slots.map(async (slot) => ({ slot, route: await getModelRoute(slot) })),
    );
    const active = routes.find(({ route }) => route.provider === body.id);
    if (active)
      return Response.json(
        {
          error: {
            type: 'provider_route_active',
            message: `Provider is active for ${active.slot}.`,
          },
        },
        { status: 409 },
      );
  }
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
    payload: { id: body.id, enabled: body.enabled, priority: body.priority },
    execute: async (tx) => {
      await updateProvider(
        body.id,
        { enabled: body.enabled, priority: body.priority },
        tx,
      );
      await recordAuditStrict(tx, {
        actorUserId: actor.id,
        action: 'provider.changed',
        targetType: 'provider',
        targetId: body.id,
        meta: {
          enabled: body.enabled,
          priority: body.priority,
          idempotencyKey,
        },
      });
      return { body: { ok: true, providerId: body.id } };
    },
  });
  return adminMutationResponse(result);
};
