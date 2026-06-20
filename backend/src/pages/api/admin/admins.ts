import type { APIRoute } from 'astro';
import { eq } from 'drizzle-orm';
import { requireAdmin } from '../../../lib/admin';
import { requireSameOrigin } from '../../../lib/csrf';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';
import { recordAudit } from '../../../lib/audit';

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;

  const actor = await requireAdmin(request);
  if (!actor) return new Response('Forbidden', { status: 403 });
  const body = (await request.json().catch(() => ({}))) as { email?: string; userId?: string; role?: 'admin' | 'user' };
  const role = body.role === 'user' ? 'user' : 'admin';
  let targetId: string | null = null;
  if (body.userId) {
    await db.update(users).set({ role }).where(eq(users.id, body.userId));
    targetId = body.userId;
  } else if (body.email) {
    const result = await db.update(users).set({ role }).where(eq(users.email, body.email)).returning({ id: users.id });
    if (!result.length) return new Response('User not found', { status: 404 });
    targetId = result[0]!.id;
  } else {
    return new Response('Missing email or userId', { status: 400 });
  }
  await recordAudit({
    actorUserId: actor.id,
    action: 'role.changed',
    targetType: 'user',
    targetId: targetId ?? undefined,
    meta: { role, email: body.email ?? null },
  });
  return Response.json({ ok: true });
};
