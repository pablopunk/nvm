import type { APIRoute } from 'astro';
import { createPreviewGatewayState } from '../../../lib/preview-auth';
import { isProductionGatewayOrigin, resolveAuthRedirectConfiguration } from '../../../lib/auth-config';
import { workos, WORKOS_CLIENT_ID } from '../../../lib/workos';

export const GET: APIRoute = async ({ url, redirect }) => {
  if (!isProductionGatewayOrigin(url.origin)) return new Response('Not found', { status: 404 });
  let redirectUri: string;
  try {
    ({ redirectUri } = resolveAuthRedirectConfiguration());
  } catch {
    return new Response('Preview authentication is unavailable', { status: 503 });
  }
  const created = await createPreviewGatewayState(url.searchParams.get('intent') ?? '');
  if (!created) return new Response('Preview authentication is unavailable', { status: 503 });
  const authorizationUrl = workos.userManagement.getAuthorizationUrl({
    provider: 'authkit',
    clientId: WORKOS_CLIENT_ID,
    redirectUri,
    state: created.state,
  });
  return redirect(authorizationUrl);
};
