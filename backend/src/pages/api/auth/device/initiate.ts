import type { APIRoute } from 'astro';
import { randomBytes } from 'node:crypto';
import { db } from '../../../../db/client';
import { deviceCodes } from '../../../../db/schema';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../../lib/ratelimit';

const TTL_MS = 5 * 60 * 1000;

export const POST: APIRoute = async ({ request, url }) => {
  const decision = await rateLimitIp('auth', clientIp(request), 20, '1 m');
  if (!decision.ok) return tooManyRequests(decision);
  const body = (await request.json().catch(() => ({}))) as { label?: string };
  const deviceLabel = (body.label ?? '').trim() || 'Desktop';

  const code = randomBytes(32).toString('base64url');
  const expiresAt = new Date(Date.now() + TTL_MS);
  await db.insert(deviceCodes).values({ code, deviceLabel, expiresAt });

  const verifyUrl = new URL('/auth/device', url);
  verifyUrl.searchParams.set('code', code);

  return Response.json({
    code,
    verifyUrl: verifyUrl.toString(),
    expiresAt: expiresAt.toISOString(),
    pollIntervalMs: 2000,
  });
};
