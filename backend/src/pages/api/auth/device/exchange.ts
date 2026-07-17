import type { APIRoute } from 'astro';
import { z } from 'zod';
import { killSwitchResponse, backendKillSwitchEnabled, requestIdFromHeaders } from '../../../../lib/compatibility';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../../lib/ratelimit';
import { safeJsonBody } from '../../../../lib/validation';
import { exchangeApprovedDeviceCode } from '../../../../lib/device-auth';

const exchangeSchema = z.object({ code: z.string().min(1) });

export const POST: APIRoute = async ({ request }) => {
  const requestId = requestIdFromHeaders(request.headers);
  if (backendKillSwitchEnabled('auth_device')) return killSwitchResponse('auth_device', 'Device authorization is temporarily disabled.', requestId);
  const decision = await rateLimitIp('auth', clientIp(request), 60, '1 m');
  if (!decision.ok) return tooManyRequests(decision);

  const parsed = await safeJsonBody(request, exchangeSchema);
  if (!parsed.ok) return Response.json(parsed.error, { status: 400 });
  const code = parsed.data.code.trim();
  if (!code) return new Response('Missing code', { status: 400 });

  const result = await exchangeApprovedDeviceCode(code);
  if (result.status === 'expired' || result.status === 'consumed') {
    return Response.json({ status: result.status }, { status: 410 });
  }
  if (result.status === 'pending') return Response.json(result);
  if (result.status === 'missing_user') {
    return new Response('User not found', { status: 404 });
  }
  return Response.json(result);
};
