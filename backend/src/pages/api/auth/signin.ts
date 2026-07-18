import type { APIRoute } from 'astro';
import {
  createPreviewStartIntent,
  prepareProductionState,
  previewOriginMatchesRequest,
  previewTargetFromEnvironment,
} from '../../../lib/preview-auth';
import {
  AuthConfigurationError,
  previewAuthConfigured,
  resolveAuthRedirectConfiguration,
} from '../../../lib/auth-config';
import { authorizationUrlForState } from '../../../lib/workos';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../lib/ratelimit';
import { env } from '../../../lib/env';
import { authCorrelationCookie } from '../../../lib/auth-correlation';
import { log } from '../../../lib/log';

function unavailable(stage: string, error?: unknown) {
  log.error('auth_signin_failed', {
    route: '/api/auth/signin',
    stage,
    ...(error instanceof Error ? { error_name: error.name } : {}),
    ...(error instanceof AuthConfigurationError
      ? { reason: error.message }
      : {}),
  });
  return new Response('Authentication is temporarily unavailable', {
    status: 503,
  });
}

export const GET: APIRoute = async ({ url, request, redirect }) => {
  const decision = await rateLimitIp('auth', clientIp(request), 30, '1 m');
  if (!decision.ok) return tooManyRequests(decision);
  if (env('VERCEL_ENV') === 'preview') {
    if (!previewOriginMatchesRequest(url.origin)) {
      return new Response('Preview authentication is unavailable', { status: 503 });
    }
    const target = previewTargetFromEnvironment();
    const gatewayOrigin = env('PREVIEW_GATEWAY_ORIGIN');
    const intent =
      target && gatewayOrigin && previewAuthConfigured()
        ? await createPreviewStartIntent(target)
        : null;
    if (!intent || gatewayOrigin !== 'https://www.nvm.fyi') {
      return new Response('Preview authentication is unavailable', { status: 503 });
    }
    try {
      return redirect(
        `${gatewayOrigin}/api/auth/preview-start?intent=${encodeURIComponent(intent)}`,
      );
    } catch {
      return new Response('Preview authentication is unavailable', { status: 503 });
    }
  }

  let redirectUri: string;
  try {
    ({ redirectUri } = resolveAuthRedirectConfiguration());
  } catch (error) {
    return unavailable('configuration', error);
  }
  const pending = await prepareProductionState(url.searchParams.get('return_to'));
  if (!pending) return unavailable('state_signing');
  const authorizationUrl = authorizationUrlForState(pending.state, redirectUri);
  if (!authorizationUrl) return unavailable('authorization_url');
  let response: Response;
  try {
    response = redirect(authorizationUrl);
  } catch {
    return unavailable('redirect');
  }
  if (!(await pending.commit())) return unavailable('state_commit');
  response.headers.append(
    'Set-Cookie',
    authCorrelationCookie(pending.state, new URL(redirectUri).protocol === 'https:'),
  );
  return response;
};
