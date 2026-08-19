import type { APIRoute } from 'astro';
import { z } from 'zod';
import { and, eq, gt, isNull } from 'drizzle-orm';
import { getSessionFromCookies } from '../../../../lib/workos';
import { db } from '../../../../db/client';
import { users, deviceCodes } from '../../../../db/schema';
import { safeJsonBody } from '../../../../lib/validation';
import { requireSameOrigin } from '../../../../lib/csrf';

const approveSchema = z.object({
  code: z.string().min(1),
  label: z.string().max(120).optional(),
});

export const POST: APIRoute = async ({ request }) => {
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;
  const mediaType = request.headers.get('content-type')?.split(';', 1)[0]?.trim().toLowerCase();
  if (mediaType !== 'application/json') {
    return new Response('Unsupported Media Type', { status: 415 });
  }
  const session = await getSessionFromCookies(request.headers.get('cookie'));
  if (!session) return new Response('Unauthorized', { status: 401 });
  const [user] = await db.select().from(users).where(eq(users.workosUserId, session.user.id)).limit(1);
  if (!user) return new Response('Unauthorized', { status: 401 });

  const parsed = await safeJsonBody(request, approveSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const code = parsed.data.code.trim();
  if (!code) return new Response('Missing code', { status: 400 });

  const updates: { userId: string; approvedAt: Date; deviceLabel?: string } = { userId: user.id, approvedAt: new Date() };
  const trimmedLabel = (parsed.data.label ?? '').trim();
  if (trimmedLabel) updates.deviceLabel = trimmedLabel;

  const updated = await db
    .update(deviceCodes)
    .set(updates)
    .where(and(eq(deviceCodes.code, code), isNull(deviceCodes.approvedAt), gt(deviceCodes.expiresAt, new Date())))
    .returning({ code: deviceCodes.code });
  if (updated.length === 0) return new Response('Invalid or expired code', { status: 400 });

  return Response.json({ ok: true });
};
