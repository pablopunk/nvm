import type { APIRoute } from 'astro';
import {
  isProductionGatewayOrigin,
  resolveAuthRedirectConfiguration,
} from '../../../lib/auth-config';
import { preparePreviewGatewayState } from '../../../lib/preview-auth';
import { authorizationUrlForState } from '../../../lib/workos';
import { authCorrelationCookie } from '../../../lib/auth-correlation';

export const GET: APIRoute = async ({ url, redirect }) => {
  if (!isProductionGatewayOrigin(url.origin)) {
    return new Response('Not found', { status: 404 });
  }
  let redirectUri: string;
  try {
    ({ redirectUri } = resolveAuthRedirectConfiguration());
  } catch {
    return new Response('Preview authentication is unavailable', { status: 503 });
  }
  const pending = await preparePreviewGatewayState(
    url.searchParams.get('intent') ?? '',
  );
  if (!pending) {
    return new Response('Preview authentication is unavailable', { status: 503 });
  }
  const authorizationUrl = authorizationUrlForState(pending.state, redirectUri);
  if (!authorizationUrl) {
    return new Response('Preview authentication is unavailable', { status: 503 });
  }
  let response: Response;
  try {
    response = redirect(authorizationUrl);
  } catch {
    return new Response('Preview authentication is unavailable', { status: 503 });
  }
  if (!(await pending.commit())) {
    return new Response('Preview authentication is unavailable', { status: 503 });
  }
  response.headers.append(
    'Set-Cookie',
    authCorrelationCookie(pending.state, new URL(redirectUri).protocol === 'https:'),
  );
  return response;
};
