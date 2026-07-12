import type { APIRoute } from 'astro';
import { z } from 'zod';
import { requireAdmin } from '../../../lib/admin';
import { requireSameOrigin } from '../../../lib/csrf';
import { db } from '../../../db/client';
import { creditLedger } from '../../../db/schema';
import { recordAudit } from '../../../lib/audit';
import { safeJsonBody } from '../../../lib/validation';

const creditsSchema = z.object({
  userId: z.string().uuid(),
  delta: z.number().int(),
  kind: z.enum(['free', 'paid']),
  reason: z.string().min(1),
});

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;

  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });

  const parsed = await safeJsonBody(request, creditsSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });

  await db.insert(creditLedger).values({
    userId: parsed.data.userId,
    delta: parsed.data.delta,
    kind: parsed.data.kind,
    reason: parsed.data.reason,
  });
  await recordAudit({
    actorUserId: actor.id,
    action: 'credits.granted',
    targetType: 'user',
    targetId: parsed.data.userId,
    meta: { delta: parsed.data.delta, kind: parsed.data.kind, reason: parsed.data.reason },
  });
  return Response.json({ ok: true });
};
