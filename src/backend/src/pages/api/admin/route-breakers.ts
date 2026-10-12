import type { APIRoute } from 'astro';
import { z } from 'zod';
import { db } from '../../../db/client';
import { routeBreakers } from '../../../db/schema';
import { requireAdmin } from '../../../lib/admin';
import {
  adminMutationKeyFromRequest,
  adminMutationResponse,
  executeAdminMutation,
} from '../../../lib/admin-mutations';
import { requireSameOrigin } from '../../../lib/csrf';
import { reenableRouteBreaker } from '../../../lib/route-breaker-admin';
import { safeJsonBody } from '../../../lib/validation';

const breakerSchema = z
  .object({
    routeSlot: z.string().min(1),
    provider: z.string().min(1),
    modelId: z.string().min(1),
  })
  .strict();

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  return Response.json({ breakers: await db.select().from(routeBreakers) });
};

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const parsed = await safeJsonBody(request, breakerSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
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
    payload: parsed.data,
    execute: async (tx) => {
      await reenableRouteBreaker({ actorUserId: actor.id, ...parsed.data }, tx);
      return { body: { ok: true } };
    },
  });
  return adminMutationResponse(result);
};
