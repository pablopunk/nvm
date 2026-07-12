import type { APIRoute } from 'astro';
import { workos, WORKOS_CLIENT_ID } from '../../../lib/workos';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../lib/ratelimit';
import { env } from '../../../lib/env';
import { safeRelativeRedirectPath } from '../../../lib/safe-redirect';
import { encodePreviewState, previewTargetFromRequest } from '../../../lib/preview-auth';

export const GET: APIRoute = async ({ url, request, redirect }) => {
  const decision = await rateLimitIp('auth', clientIp(request), 30, '1 m');
  if (!decision.ok) return tooManyRequests(decision);
  const previewTarget = previewTargetFromRequest(url, url.searchParams.get('return_to'));
  const returnTo = previewTarget
    ? encodePreviewState(previewTarget)
    : safeRelativeRedirectPath(url.searchParams.get('return_to'));
  const authorizationUrl = workos.userManagement.getAuthorizationUrl({
    provider: 'authkit',
    clientId: WORKOS_CLIENT_ID,
    redirectUri: env('WORKOS_REDIRECT_URI') as string,
    state: returnTo,
  });
  return redirect(authorizationUrl);
};
