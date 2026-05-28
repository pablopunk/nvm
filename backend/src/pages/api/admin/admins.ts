import type { APIRoute } from 'astro';
import { eq } from 'drizzle-orm';
import { requireAdmin } from '../../../lib/admin';
import { db } from '../../../db/client';
import { users } from '../../../db/schema';

export const POST: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  const body = (await request.json().catch(() => ({}))) as { email?: string; userId?: string; role?: 'admin' | 'user' };
  const role = body.role === 'user' ? 'user' : 'admin';
  if (body.userId) {
    await db.update(users).set({ role }).where(eq(users.id, body.userId));
    return Response.json({ ok: true });
  }
  if (body.email) {
    const result = await db.update(users).set({ role }).where(eq(users.email, body.email)).returning({ id: users.id });
    if (!result.length) return new Response('User not found', { status: 404 });
    return Response.json({ ok: true });
  }
  return new Response('Missing email or userId', { status: 400 });
};
