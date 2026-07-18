import type { APIRoute } from 'astro';
import { z } from 'zod';
import { randomBytes } from 'node:crypto';
import { db } from '../../../../db/client';
import { deviceCodes } from '../../../../db/schema';
import { killSwitchResponse, backendKillSwitchEnabled, requestIdFromHeaders } from '../../../../lib/compatibility';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../../lib/ratelimit';
import { safeJsonBody } from '../../../../lib/validation';
import { env } from '../../../../lib/env';
import { parsePublicOrigin } from '../../../../../../src/shared/public-origin';
import { previewOriginMatchesRequest } from '../../../../lib/preview-auth';

const TTL_MS = 5 * 60 * 1000;

const initiateSchema = z.object({ label: z.string().optional() });

export const POST: APIRoute = async ({ request, url }) => {
  const requestId = requestIdFromHeaders(request.headers);
  if (backendKillSwitchEnabled('auth_device')) return killSwitchResponse('auth_device', 'Device authorization is temporarily disabled.', requestId);
  const decision = await rateLimitIp('auth', clientIp(request), 20, '1 m');
  if (!decision.ok) return tooManyRequests(decision);

  const parsed = await safeJsonBody(request, initiateSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const deviceLabel = ((parsed.data.label ?? '').trim() || 'Desktop').slice(0, 120);

  let verifyOrigin: string;
  try {
    if (env('VERCEL_ENV') === 'preview') {
      if (!previewOriginMatchesRequest(url.origin)) throw new Error('Preview request origin does not match deployment');
      verifyOrigin = url.origin;
    } else {
      verifyOrigin = parsePublicOrigin(env('PRODUCTION_ORIGIN') ?? 'https://www.nvm.fyi', 'production_web');
    }
  } catch {
    return new Response('Public web origin is unavailable', { status: 503 });
  }

  const code = randomBytes(32).toString('base64url');
  const expiresAt = new Date(Date.now() + TTL_MS);
  await db.insert(deviceCodes).values({ code, deviceLabel, expiresAt });

  const verifyUrl = new URL('/auth/device', verifyOrigin);
  verifyUrl.searchParams.set('code', code);

  return Response.json({
    code,
    verifyUrl: verifyUrl.toString(),
    expiresAt: expiresAt.toISOString(),
    pollIntervalMs: 2000,
  });
};
