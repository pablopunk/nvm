import type { APIRoute } from 'astro';
import { workos, WORKOS_CLIENT_ID, COOKIE_PASSWORD, SESSION_COOKIE } from '../../../lib/workos';
import { upsertUserWithFreeGrant } from '../../../lib/users';

export const GET: APIRoute = async ({ url }) => {
  const code = url.searchParams.get('code');
  const returnTo = url.searchParams.get('state') ?? '/dashboard';
  if (!code) return new Response('Missing code', { status: 400 });

  const { user, sealedSession } = await workos.userManagement.authenticateWithCode({
    clientId: WORKOS_CLIENT_ID,
    code,
    session: { sealSession: true, cookiePassword: COOKIE_PASSWORD },
  });

  await upsertUserWithFreeGrant({ workosUserId: user.id, email: user.email });

  const isHttps = url.protocol === 'https:';
  const headers = new Headers();
  headers.append(
    'Set-Cookie',
    `${SESSION_COOKIE}=${encodeURIComponent(sealedSession!)}; Path=/; HttpOnly; ${isHttps ? 'Secure; ' : ''}SameSite=Lax; Max-Age=${60 * 60 * 24 * 30}`,
  );
  headers.set('Location', returnTo);
  return new Response(null, { status: 302, headers });
};
