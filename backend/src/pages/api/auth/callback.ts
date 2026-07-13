import type { APIRoute } from 'astro';
import { workos, WORKOS_CLIENT_ID, COOKIE_PASSWORD, SESSION_COOKIE } from '../../../lib/workos';
import { upsertUserWithFreeGrant, createUserFromInviteIntent, getUserByWorkosId, DisposableEmailError, InviteRequiredError } from '../../../lib/users';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../lib/ratelimit';
import { consumeGatewayState, createPreviewSessionGrant } from '../../../lib/preview-auth';
import { readInviteIntentCookie, clearInviteIntentCookie } from '../../../lib/waitlist';
import { getSignupsEnabled, SignupsPolicyError } from '../../../lib/settings';

export const GET: APIRoute = async ({ url, request }) => {
  const decision = await rateLimitIp('auth', clientIp(request), 30, '1 m');
  if (!decision.ok) return tooManyRequests(decision);
  const code = url.searchParams.get('code');
  if (!code) return new Response('Missing code', { status: 400 });
  const state = await consumeGatewayState(url.searchParams.get('state'));
  if (!state) return new Response('Sign-in expired; please restart.', { status: 400 });

  const { user, sealedSession } = await workos.userManagement.authenticateWithCode({
    clientId: WORKOS_CLIENT_ID,
    code,
    ...(state.flow === 'production' ? { session: { sealSession: true, cookiePassword: COOKIE_PASSWORD } } : {}),
  });

  const intent = readInviteIntentCookie(request.headers.get('cookie'));
  try {
    if (state.flow === 'preview_gateway') {
      const grant = await createPreviewSessionGrant({ origin: state.exactOrigin, returnTo: '/' }, { id: user.id, email: user.email });
      if (!grant) return new Response('Preview authentication is temporarily unavailable.', { status: 503 });
      const headers = new Headers({ Location: `${state.exactOrigin}/api/auth/preview-exchange?grant=${encodeURIComponent(grant)}` });
      return new Response(null, { status: 302, headers });
    }
    const existing = await getUserByWorkosId(user.id);
    if (existing) { /* Existing users retain access when the gate is enabled. */ }
    else {
      let signupsEnabled: boolean;
      try {
        signupsEnabled = await getSignupsEnabled();
      } catch (err) {
        if (err instanceof SignupsPolicyError) return new Response('Sign-up policy is temporarily unavailable.', { status: 503 });
        throw err;
      }
      if (!signupsEnabled && intent) await createUserFromInviteIntent({ intentId: intent.id, nonce: intent.nonce, workosUserId: user.id, email: user.email });
      else if (!signupsEnabled) throw new InviteRequiredError();
      else await upsertUserWithFreeGrant({ workosUserId: user.id, email: user.email });
    }
  } catch (err) {
    if (err instanceof DisposableEmailError) {
      return new Response('Sign-up blocked: disposable email addresses are not allowed.', { status: 403 });
    }
    if (err instanceof InviteRequiredError) {
      const headers = new Headers({ Location: '/?invite=required' });
      headers.append('Set-Cookie', clearInviteIntentCookie(url.protocol === 'https:'));
      return new Response(null, { status: 303, headers });
    }
    throw err;
  }

  const isHttps = url.protocol === 'https:';
  const headers = new Headers();
  {
    headers.append(
      'Set-Cookie',
      `${SESSION_COOKIE}=${encodeURIComponent(sealedSession!)}; Path=/; HttpOnly; ${isHttps ? 'Secure; ' : ''}SameSite=Lax; Max-Age=${60 * 60 * 24 * 30}`,
    );
    headers.set('Location', state.safeRelativeReturnPath);
  }
  return new Response(null, { status: 302, headers });
};
