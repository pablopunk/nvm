import type { APIRoute } from 'astro';
import { z } from 'zod';
import { db } from '../../../db/client';
import { creditLedger } from '../../../db/schema';
import { requireAdmin } from '../../../lib/admin';
import { recordAudit } from '../../../lib/audit';
import { requireSameOrigin } from '../../../lib/csrf';
import { safeJsonBody } from '../../../lib/validation';

const creditsSchema = z.object({
  userId: z.string().uuid(),
  delta: z.number().int(),
  kind: z.enum(['free', 'paid']),
  reason: z.string().trim().min(1).optional(),
});

export function isCreditDeltaAllowedUntilPlan006(delta: number): boolean {
  return delta >= 0;
}

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;

  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });

  const parsed = await safeJsonBody(request, creditsSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  if (!isCreditDeltaAllowedUntilPlan006(parsed.data.delta))
    return Response.json(
      {
        error: {
          type: 'invalid_adjustment',
          message: 'Negative credit adjustments are not available.',
        },
      },
      { status: 400 },
    );
  const reason = parsed.data.reason ?? 'admin_adjustment';

  await db.insert(creditLedger).values({
    userId: parsed.data.userId,
    delta: parsed.data.delta,
    kind: parsed.data.kind,
    reason,
  });
  await recordAudit({
    actorUserId: actor.id,
    action: 'credits.granted',
    targetType: 'user',
    targetId: parsed.data.userId,
    meta: { delta: parsed.data.delta, kind: parsed.data.kind, reason },
  });
  return Response.json({ ok: true });
};
