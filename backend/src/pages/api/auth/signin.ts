import type { APIRoute } from 'astro';
import { workos, WORKOS_CLIENT_ID } from '../../../lib/workos';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../lib/ratelimit';

export const GET: APIRoute = async ({ url, request, redirect }) => {
  const decision = await rateLimitIp('auth', clientIp(request), 30, '1 m');
  if (!decision.ok) return tooManyRequests(decision);
  const returnTo = url.searchParams.get('return_to') ?? '/dashboard';
  const authorizationUrl = workos.userManagement.getAuthorizationUrl({
    provider: 'authkit',
    clientId: WORKOS_CLIENT_ID,
    redirectUri: import.meta.env.WORKOS_REDIRECT_URI,
    state: returnTo,
  });
  return redirect(authorizationUrl);
};
