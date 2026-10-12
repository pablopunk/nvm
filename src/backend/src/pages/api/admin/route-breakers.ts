import type { APIRoute } from 'astro';
import { z } from 'zod';
import { db } from '../../../db/client';
import { routeBreakers } from '../../../db/schema';
import { requireAdmin } from '../../../lib/admin';
import { requireSameOrigin } from '../../../lib/csrf';
import { safeJsonBody } from '../../../lib/validation';
import { reenableRouteBreaker } from '../../../lib/route-breaker-admin';

const breakerSchema = z
  .object({
    routeSlot: z.string().min(1),
    provider: z.string().min(1),
    modelId: z.string().min(1),
  })
  .strict();

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  return Response.json({ breakers: await db.select().from(routeBreakers) });
};

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const parsed = await safeJsonBody(request, breakerSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  await reenableRouteBreaker({
    actorUserId: actor.id,
    ...parsed.data,
  });
  return Response.json({ ok: true });
};
