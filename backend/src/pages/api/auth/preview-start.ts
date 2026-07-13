import type { APIRoute } from 'astro';
import { createPreviewGatewayState } from '../../../lib/preview-auth';
import { env } from '../../../lib/env';
import { workos, WORKOS_CLIENT_ID } from '../../../lib/workos';

export const GET: APIRoute = async ({ url, redirect }) => {
  if (url.origin !== (env('PRODUCTION_ORIGIN') ?? 'https://nvm.fyi')) return new Response('Not found', { status: 404 });
  const created = await createPreviewGatewayState(url.searchParams.get('intent') ?? '');
  if (!created) return new Response('Preview authentication is unavailable', { status: 503 });
  const authorizationUrl = workos.userManagement.getAuthorizationUrl({
    provider: 'authkit',
    clientId: WORKOS_CLIENT_ID,
    redirectUri: env('WORKOS_REDIRECT_URI') as string,
    state: created.state,
  });
  return redirect(authorizationUrl);
};
