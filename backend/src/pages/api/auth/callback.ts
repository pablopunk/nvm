import type { APIRoute } from 'astro';
import { workos, WORKOS_CLIENT_ID, COOKIE_PASSWORD, SESSION_COOKIE } from '../../../lib/workos';
import { upsertUserWithFreeGrant, createUserFromInviteIntent, getUserByWorkosId, DisposableEmailError, InviteRequiredError } from '../../../lib/users';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../lib/ratelimit';
import { consumeGatewayState, createPreviewSessionGrant } from '../../../lib/preview-auth';
import { readInviteIntentCookie, clearInviteIntentCookie } from '../../../lib/waitlist';
import { getSignupsEnabled, SignupsPolicyError } from '../../../lib/settings';
import { log, redactAuthUrl } from '../../../lib/log';

type AuthenticatedIdentity = { user: { id: string; email: string }; sealedSession?: string };
type CallbackDependencies = {
  rateLimit: typeof rateLimitIp;
  consumeState: typeof consumeGatewayState;
  authenticate: (input: Parameters<typeof workos.userManagement.authenticateWithCode>[0]) => Promise<AuthenticatedIdentity>;
  readInviteIntent: typeof readInviteIntentCookie;
  clearInviteIntent: typeof clearInviteIntentCookie;
  getExistingUser: (workosUserId: string) => Promise<unknown | null>;
  getSignupsEnabled: typeof getSignupsEnabled;
  createUserFromInvite: typeof createUserFromInviteIntent;
  upsertUser: typeof upsertUserWithFreeGrant;
  createPreviewGrant: typeof createPreviewSessionGrant;
  logger: Pick<typeof log, 'info' | 'warn' | 'error'>;
};

export function createCallbackHandler(overrides: Partial<CallbackDependencies> = {}): APIRoute {
  const dependencies: CallbackDependencies = {
    rateLimit: rateLimitIp,
    consumeState: consumeGatewayState,
    authenticate: (input) => workos.userManagement.authenticateWithCode(input) as Promise<AuthenticatedIdentity>,
    readInviteIntent: readInviteIntentCookie,
    clearInviteIntent: clearInviteIntentCookie,
    getExistingUser: getUserByWorkosId,
    getSignupsEnabled,
    createUserFromInvite: createUserFromInviteIntent,
    upsertUser: upsertUserWithFreeGrant,
    createPreviewGrant: createPreviewSessionGrant,
    logger: log,
    ...overrides,
  };

  return async ({ url, request }) => {
  const requestId = request.headers.get('x-vercel-id') ?? request.headers.get('x-request-id') ?? undefined;
  const decision = await dependencies.rateLimit('auth', clientIp(request), 30, '1 m');
  if (!decision.ok) return tooManyRequests(decision);
  const code = url.searchParams.get('code');
  if (!code) return new Response('Missing code', { status: 400 });
  dependencies.logger.info('auth_callback_started', { request_id: requestId, route: redactAuthUrl(url), has_code: true, has_state: Boolean(url.searchParams.get('state')) });
  const state = await dependencies.consumeState(url.searchParams.get('state'));
  if (!state) {
    dependencies.logger.warn('auth_callback_state_rejected', { request_id: requestId, route: redactAuthUrl(url), reason: 'invalid_or_expired_state' });
    return new Response('Sign-in expired; please restart.', { status: 400 });
  }

  let user: { id: string; email: string };
  let sealedSession: string | undefined;
  try {
    const authenticated = await dependencies.authenticate({
      clientId: WORKOS_CLIENT_ID,
      code,
      ...(state.flow === 'production' ? { session: { sealSession: true, cookiePassword: COOKIE_PASSWORD } } : {}),
    });
    user = authenticated.user;
    sealedSession = authenticated.sealedSession;
  } catch (error) {
    dependencies.logger.error('auth_callback_workos_failed', { request_id: requestId, route: redactAuthUrl(url), flow: state.flow, error_name: error instanceof Error ? error.name : 'unknown' });
    return new Response('Authentication provider unavailable; please restart.', { status: 502 });
  }
  if (state.flow === 'production' && !sealedSession) {
    dependencies.logger.error('auth_callback_session_missing', { request_id: requestId, route: redactAuthUrl(url), flow: state.flow });
    return new Response('Authentication session unavailable; please restart.', { status: 502 });
  }

  const intent = dependencies.readInviteIntent(request.headers.get('cookie'));
  try {
    if (state.flow === 'preview_gateway') {
      const grant = await dependencies.createPreviewGrant({ origin: state.exactOrigin, returnTo: '/' }, { id: user.id, email: user.email });
      if (!grant) {
        dependencies.logger.error('auth_callback_preview_grant_failed', { request_id: requestId, route: redactAuthUrl(url), flow: state.flow });
        return new Response('Preview authentication is temporarily unavailable.', { status: 503 });
      }
      const headers = new Headers({ Location: `${state.exactOrigin}/api/auth/preview-exchange?grant=${encodeURIComponent(grant)}` });
      return new Response(null, { status: 302, headers });
    }
    const existing = await dependencies.getExistingUser(user.id);
    if (existing) { /* Existing users retain access when the gate is enabled. */ }
    else {
      let signupsEnabled: boolean;
      try {
        signupsEnabled = await dependencies.getSignupsEnabled();
      } catch (err) {
        if (err instanceof SignupsPolicyError) return new Response('Sign-up policy is temporarily unavailable.', { status: 503 });
        throw err;
      }
      if (!signupsEnabled && intent) await dependencies.createUserFromInvite({ intentId: intent.id, nonce: intent.nonce, workosUserId: user.id, email: user.email });
      else if (!signupsEnabled) throw new InviteRequiredError();
      else await dependencies.upsertUser({ workosUserId: user.id, email: user.email });
    }
  } catch (err) {
    if (err instanceof DisposableEmailError) {
      return new Response('Sign-up blocked: disposable email addresses are not allowed.', { status: 403 });
    }
    if (err instanceof InviteRequiredError) {
      dependencies.logger.info('auth_callback_invite_required', { request_id: requestId, route: redactAuthUrl(url), flow: state.flow });
      const headers = new Headers({ Location: '/?invite=required' });
      headers.append('Set-Cookie', dependencies.clearInviteIntent(url.protocol === 'https:'));
      return new Response(null, { status: 303, headers });
    }
    dependencies.logger.error('auth_callback_provisioning_failed', { request_id: requestId, route: redactAuthUrl(url), flow: state.flow, error_name: err instanceof Error ? err.name : 'unknown' });
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
  dependencies.logger.info('auth_callback_succeeded', { request_id: requestId, route: redactAuthUrl(url), flow: state.flow });
  return new Response(null, { status: 302, headers });
  };
}

export const GET = createCallbackHandler();
