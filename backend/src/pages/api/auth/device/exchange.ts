import type { APIRoute } from 'astro';
import { and, eq, gt, isNull, isNotNull } from 'drizzle-orm';
import { db } from '../../../../db/client';
import { users, deviceCodes } from '../../../../db/schema';
import { createApiToken } from '../../../../lib/tokens';

export const POST: APIRoute = async ({ request }) => {
  const body = (await request.json().catch(() => ({}))) as { code?: string };
  const code = (body.code ?? '').trim();
  if (!code) return new Response('Missing code', { status: 400 });

  const [row] = await db
    .select()
    .from(deviceCodes)
    .where(and(eq(deviceCodes.code, code), gt(deviceCodes.expiresAt, new Date())))
    .limit(1);
  if (!row) return Response.json({ status: 'expired' }, { status: 410 });
  if (row.consumedAt) return Response.json({ status: 'consumed' }, { status: 410 });
  if (!row.approvedAt || !row.userId) return Response.json({ status: 'pending' });

  const updated = await db
    .update(deviceCodes)
    .set({ consumedAt: new Date() })
    .where(and(eq(deviceCodes.code, code), isNull(deviceCodes.consumedAt), isNotNull(deviceCodes.approvedAt)))
    .returning({ code: deviceCodes.code });
  if (updated.length === 0) return Response.json({ status: 'consumed' }, { status: 410 });

  const [user] = await db.select().from(users).where(eq(users.id, row.userId)).limit(1);
  if (!user) return new Response('User not found', { status: 404 });

  const created = await createApiToken(user.id, row.deviceLabel);
  return Response.json({ status: 'ok', token: created.token, user: { email: user.email, role: user.role } });
};
