import type { APIRoute } from 'astro';
import { workos, WORKOS_CLIENT_ID } from '../../../lib/workos';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../lib/ratelimit';
import { env } from '../../../lib/env';
import { createProductionState, createPreviewStartIntent, previewTargetFromEnvironment } from '../../../lib/preview-auth';
import {
  previewAuthConfigured,
  productionAuthConfigured,
} from '../../../lib/auth-config';

export const GET: APIRoute = async ({ url, request, redirect }) => {
  const decision = await rateLimitIp('auth', clientIp(request), 30, '1 m');
  if (!decision.ok) return tooManyRequests(decision);
  if (env('VERCEL_ENV') === 'preview') {
    const target = previewTargetFromEnvironment();
    const gatewayOrigin = env('PREVIEW_GATEWAY_ORIGIN');
    const intent = target && gatewayOrigin && previewAuthConfigured() ? await createPreviewStartIntent(target) : null;
    if (!intent || !/^https:\/\/nvm\.fyi$/.test(gatewayOrigin ?? '')) return new Response('Preview authentication is unavailable', { status: 503 });
    return redirect(`${gatewayOrigin}/api/auth/preview-start?intent=${encodeURIComponent(intent)}`);
  }
  if (!productionAuthConfigured()) {
    return new Response('Authentication is temporarily unavailable', {
      status: 503,
    });
  }
  const state = await createProductionState(url.searchParams.get('return_to'));
  if (!state) return new Response('Authentication is temporarily unavailable', { status: 503 });
  const authorizationUrl = workos.userManagement.getAuthorizationUrl({
    provider: 'authkit',
    clientId: WORKOS_CLIENT_ID,
    redirectUri: env('WORKOS_REDIRECT_URI') as string,
    state,
  });
  return redirect(authorizationUrl);
};
