import type { APIRoute } from 'astro';
import { and, eq } from 'drizzle-orm';
import { z } from 'zod';
import { db } from '../../../db/client';
import { auditLog, routeBreakers } from '../../../db/schema';
import { requireAdmin } from '../../../lib/admin';
import { requireSameOrigin } from '../../../lib/csrf';
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
  const target = parsed.data;
  await db.transaction(async (tx) => {
    await tx
      .delete(routeBreakers)
      .where(
        and(
          eq(routeBreakers.routeSlot, target.routeSlot),
          eq(routeBreakers.provider, target.provider),
          eq(routeBreakers.modelId, target.modelId),
        ),
      );
    await tx.insert(auditLog).values({
      actorUserId: actor.id,
      action: 'route_breaker.reenabled',
      targetType: 'route',
      targetId: `${target.routeSlot}:${target.provider}:${target.modelId}`,
      meta: target,
    });
  });
  return Response.json({ ok: true });
};
