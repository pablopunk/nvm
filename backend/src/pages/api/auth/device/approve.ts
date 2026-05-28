import type { APIRoute } from 'astro';
import { and, eq, gt, isNull } from 'drizzle-orm';
import { getSessionFromCookies } from '../../../../lib/workos';
import { db } from '../../../../db/client';
import { users, deviceCodes } from '../../../../db/schema';

export const POST: APIRoute = async ({ request }) => {
  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });
  const [user] = await db.select().from(users).where(eq(users.workosUserId, session.user.id)).limit(1);
  if (!user) return new Response('Unauthorized', { status: 401 });

  const body = (await request.json().catch(() => ({}))) as { code?: string; label?: string };
  const code = (body.code ?? '').trim();
  if (!code) return new Response('Missing code', { status: 400 });

  const updates: { userId: string; approvedAt: Date; deviceLabel?: string } = { userId: user.id, approvedAt: new Date() };
  const trimmedLabel = (body.label ?? '').trim();
  if (trimmedLabel) updates.deviceLabel = trimmedLabel;

  const updated = await db
    .update(deviceCodes)
    .set(updates)
    .where(and(eq(deviceCodes.code, code), isNull(deviceCodes.approvedAt), gt(deviceCodes.expiresAt, new Date())))
    .returning({ code: deviceCodes.code });
  if (updated.length === 0) return new Response('Invalid or expired code', { status: 400 });

  return Response.json({ ok: true });
};
