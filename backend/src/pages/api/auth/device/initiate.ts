import type { APIRoute } from 'astro';
import { randomBytes } from 'node:crypto';
import { db } from '../../../../db/client';
import { deviceCodes } from '../../../../db/schema';

const TTL_MS = 5 * 60 * 1000;

export const POST: APIRoute = async ({ request, url }) => {
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
