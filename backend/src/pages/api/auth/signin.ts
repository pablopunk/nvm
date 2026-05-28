import type { APIRoute } from 'astro';
import { workos, WORKOS_CLIENT_ID } from '../../../lib/workos';

export const GET: APIRoute = ({ url, redirect }) => {
  const returnTo = url.searchParams.get('return_to') ?? '/dashboard';
  const authorizationUrl = workos.userManagement.getAuthorizationUrl({
    provider: 'authkit',
    clientId: WORKOS_CLIENT_ID,
    redirectUri: import.meta.env.WORKOS_REDIRECT_URI,
    state: returnTo,
  });
  return redirect(authorizationUrl);
};
