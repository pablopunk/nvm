import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { db } from '../../../db/client';
import { creditLedger } from '../../../db/schema';

export const POST: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const body = (await request.json().catch(() => ({}))) as { userId?: string; delta?: number; kind?: 'free' | 'paid'; reason?: string };
  if (!body.userId || !Number.isFinite(body.delta) || !body.delta) return new Response('Missing userId or delta', { status: 400 });
  const kind = body.kind === 'free' ? 'free' : 'paid';
  await db.insert(creditLedger).values({
    userId: body.userId,
    delta: Math.trunc(body.delta!),
    kind,
    reason: body.reason || 'admin_grant',
  });
  return Response.json({ ok: true });
};
