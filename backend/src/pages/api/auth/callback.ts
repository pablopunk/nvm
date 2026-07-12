import type { APIRoute } from 'astro';
import { workos, WORKOS_CLIENT_ID, COOKIE_PASSWORD, SESSION_COOKIE } from '../../../lib/workos';
import { upsertUserWithFreeGrant, DisposableEmailError } from '../../../lib/users';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../lib/ratelimit';
import { safeRelativeRedirectPath } from '../../../lib/safe-redirect';
import { createPreviewSessionGrant, decodePreviewState } from '../../../lib/preview-auth';

export const GET: APIRoute = async ({ url, request }) => {
  const decision = await rateLimitIp('auth', clientIp(request), 30, '1 m');
  if (!decision.ok) return tooManyRequests(decision);
  const code = url.searchParams.get('code');
  const previewTarget = await decodePreviewState(url.searchParams.get('state'));
  const returnTo = safeRelativeRedirectPath(url.searchParams.get('state'));
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
  if (previewTarget) {
    const grant = await createPreviewSessionGrant(previewTarget, sealedSession!);
    if (!grant) return new Response('Preview authentication is temporarily unavailable.', { status: 503 });
    const exchangeUrl = new URL('/api/auth/preview-exchange', previewTarget.origin);
    exchangeUrl.searchParams.set('grant', grant);
    headers.set('Location', exchangeUrl.toString());
  } else {
    headers.append(
      'Set-Cookie',
      `${SESSION_COOKIE}=${encodeURIComponent(sealedSession!)}; Path=/; HttpOnly; ${isHttps ? 'Secure; ' : ''}SameSite=Lax; Max-Age=${60 * 60 * 24 * 30}`,
    );
    headers.set('Location', returnTo);
  }
  return new Response(null, { status: 302, headers });
};
