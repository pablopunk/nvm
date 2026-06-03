import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { db } from '../../../db/client';
import { creditLedger } from '../../../db/schema';
import { recordAudit } from '../../../lib/audit';

export const POST: APIRoute = async ({ request }) => {
  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const body = (await request.json().catch(() => ({}))) as { userId?: string; delta?: number; kind?: 'free' | 'paid'; reason?: string };
  if (!body.userId || !Number.isFinite(body.delta) || !body.delta) return new Response('Missing userId or delta', { status: 400 });
  const kind = body.kind === 'free' ? 'free' : 'paid';
  const delta = Math.trunc(body.delta!);
  await db.insert(creditLedger).values({
    userId: body.userId,
    delta,
    kind,
    reason: body.reason || 'admin_grant',
  });
  await recordAudit({
    actorUserId: actor.id,
    action: 'credits.granted',
    targetType: 'user',
    targetId: body.userId,
    meta: { delta, kind, reason: body.reason ?? null },
  });
  return Response.json({ ok: true });
};
