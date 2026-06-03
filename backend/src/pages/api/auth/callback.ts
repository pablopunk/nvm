import type { APIRoute } from 'astro';
import { workos, WORKOS_CLIENT_ID, COOKIE_PASSWORD, SESSION_COOKIE } from '../../../lib/workos';
import { upsertUserWithFreeGrant, DisposableEmailError } from '../../../lib/users';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../lib/ratelimit';

export const GET: APIRoute = async ({ url, request }) => {
  const decision = await rateLimitIp('auth', clientIp(request), 30, '1 m');
  if (!decision.ok) return tooManyRequests(decision);
  const code = url.searchParams.get('code');
  const returnTo = url.searchParams.get('state') ?? '/dashboard';
  if (!code) return new Response('Missing code', { status: 400 });

  const { user, sealedSession } = await workos.userManagement.authenticateWithCode({
    clientId: WORKOS_CLIENT_ID,
    code,
    session: { sealSession: true, cookiePassword: COOKIE_PASSWORD },
  });

  try {
    await upsertUserWithFreeGrant({ workosUserId: user.id, email: user.email });
  } catch (err) {
    if (err instanceof DisposableEmailError) {
      return new Response('Sign-up blocked: disposable email addresses are not allowed.', { status: 403 });
    }
    throw err;
  }

  const isHttps = url.protocol === 'https:';
  const headers = new Headers();
  headers.append(
    'Set-Cookie',
    `${SESSION_COOKIE}=${encodeURIComponent(sealedSession!)}; Path=/; HttpOnly; ${isHttps ? 'Secure; ' : ''}SameSite=Lax; Max-Age=${60 * 60 * 24 * 30}`,
  );
  headers.set('Location', returnTo);
  return new Response(null, { status: 302, headers });
};
