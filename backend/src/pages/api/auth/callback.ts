import type { APIRoute } from 'astro';
import {
  COOKIE_PASSWORD,
  SESSION_COOKIE,
  WORKOS_CLIENT_ID,
  workos,
} from '../../../lib/workos';
import {
  createUserFromInviteIntent,
  DisposableEmailError,
  getUserByWorkosId,
  InviteRequiredError,
  upsertUserWithFreeGrant,
} from '../../../lib/users';
import {
  clientIp,
  rateLimitIp,
  tooManyRequests,
} from '../../../lib/ratelimit';
import {
  consumeGatewayState,
  createPreviewSessionGrant,
} from '../../../lib/preview-auth';
import {
  clearInviteIntentCookie,
  readInviteIntentCookie,
} from '../../../lib/waitlist';
import { getSignupsEnabled, SignupsPolicyError } from '../../../lib/settings';
import { log, redactAuthUrl } from '../../../lib/log';
import { productionAuthConfigured } from '../../../lib/auth-config';
import {
  appendAuthCorrelationClear,
  authCorrelationMatches,
} from '../../../lib/auth-correlation';

type CallbackDependencies = {
  authenticateWithCode: (
    input: Parameters<
      typeof workos.userManagement.authenticateWithCode
    >[0],
  ) => ReturnType<typeof workos.userManagement.authenticateWithCode>;
  consumeState: typeof consumeGatewayState;
  createPreviewGrant: typeof createPreviewSessionGrant;
  findUser: typeof getUserByWorkosId;
  signupsEnabled: typeof getSignupsEnabled;
  createInvitedUser: typeof createUserFromInviteIntent;
  upsertUser: typeof upsertUserWithFreeGrant;
};

const defaultDependencies: CallbackDependencies = {
  authenticateWithCode: (input) =>
    workos.userManagement.authenticateWithCode(input),
  consumeState: consumeGatewayState,
  createPreviewGrant: createPreviewSessionGrant,
  findUser: getUserByWorkosId,
  signupsEnabled: getSignupsEnabled,
  createInvitedUser: createUserFromInviteIntent,
  upsertUser: upsertUserWithFreeGrant,
};

function requestId(request: Request) {
  return (
    request.headers.get('x-vercel-id') ??
    request.headers.get('x-request-id') ??
    undefined
  );
}

function correlationRejectedResponse(
  requestIdentifier: string | undefined,
  url: URL,
  reason: string,
) {
  log.warn('auth_callback_state_rejected', {
    request_id: requestIdentifier,
    route: redactAuthUrl(url),
    reason,
  });
  return new Response('Sign-in expired; please restart.', { status: 400 });
}

export function createAuthCallbackRoute(
  overrides: Partial<CallbackDependencies> = {},
): APIRoute {
  const dependencies = { ...defaultDependencies, ...overrides };

  const handleCallback: APIRoute = async ({ url, request }) => {
    const requestIdentifier = requestId(request);
    const decision = await rateLimitIp('auth', clientIp(request), 30, '1 m');
    if (!decision.ok) return tooManyRequests(decision);
    const code = url.searchParams.get('code');
    if (!code) return new Response('Missing code', { status: 400 });
    if (!productionAuthConfigured()) {
      return new Response('Authentication is temporarily unavailable', {
        status: 503,
      });
    }

    const stateToken = url.searchParams.get('state');
    log.info('auth_callback_started', {
      request_id: requestIdentifier,
      route: redactAuthUrl(url),
      has_code: true,
      has_state: Boolean(stateToken),
    });
    if (!authCorrelationMatches(request.headers.get('cookie'), stateToken)) {
      return correlationRejectedResponse(
        requestIdentifier,
        url,
        'browser_correlation_mismatch',
      );
    }
    const state = await dependencies.consumeState(stateToken);
    if (!state) {
      return correlationRejectedResponse(
        requestIdentifier,
        url,
        'invalid_or_expired_state',
      );
    }

    let user: { id: string; email: string };
    let sealedSession: string | undefined;
    try {
      const authenticated = await dependencies.authenticateWithCode({
        clientId: WORKOS_CLIENT_ID,
        code,
        ...(state.flow === 'production'
          ? {
              session: {
                sealSession: true,
                cookiePassword: COOKIE_PASSWORD,
              },
            }
          : {}),
      });
      user = authenticated.user;
      sealedSession = authenticated.sealedSession;
    } catch (error) {
      log.error('auth_callback_workos_failed', {
        request_id: requestIdentifier,
        route: redactAuthUrl(url),
        flow: state.flow,
        error_name: error instanceof Error ? error.name : 'unknown',
      });
      return new Response('Authentication provider unavailable; please restart.', {
        status: 502,
      });
    }
    if (state.flow === 'production' && !sealedSession) {
      log.error('auth_callback_session_missing', {
        request_id: requestIdentifier,
        route: redactAuthUrl(url),
        flow: state.flow,
      });
      return new Response('Authentication session unavailable; please restart.', {
        status: 502,
      });
    }

    const intent = readInviteIntentCookie(request.headers.get('cookie'));
    try {
      if (state.flow === 'preview_gateway') {
        const grant = await dependencies.createPreviewGrant(
          { origin: state.exactOrigin, returnTo: '/' },
          { id: user.id, email: user.email },
        );
        if (!grant) {
          log.error('auth_callback_preview_grant_failed', {
            request_id: requestIdentifier,
            route: redactAuthUrl(url),
            flow: state.flow,
          });
          return new Response(
            'Preview authentication is temporarily unavailable.',
            { status: 503 },
          );
        }
        return new Response(null, {
          status: 302,
          headers: {
            Location: `${state.exactOrigin}/api/auth/preview-exchange?grant=${encodeURIComponent(grant)}`,
          },
        });
      }

      const existing = await dependencies.findUser(user.id);
      if (!existing) {
        let signupsEnabled: boolean;
        try {
          signupsEnabled = await dependencies.signupsEnabled();
        } catch (error) {
          if (error instanceof SignupsPolicyError) {
            return new Response('Sign-up policy is temporarily unavailable.', {
              status: 503,
            });
          }
          throw error;
        }
        if (!signupsEnabled && intent) {
          await dependencies.createInvitedUser({
            intentId: intent.id,
            nonce: intent.nonce,
            workosUserId: user.id,
            email: user.email,
          });
        } else if (!signupsEnabled) {
          throw new InviteRequiredError();
        } else {
          await dependencies.upsertUser({
            workosUserId: user.id,
            email: user.email,
          });
        }
      }
    } catch (error) {
      if (error instanceof DisposableEmailError) {
        return new Response(
          'Sign-up blocked: disposable email addresses are not allowed.',
          { status: 403 },
        );
      }
      if (error instanceof InviteRequiredError) {
        log.info('auth_callback_invite_required', {
          request_id: requestIdentifier,
          route: redactAuthUrl(url),
          flow: state.flow,
        });
        const headers = new Headers({ Location: '/?invite=required' });
        headers.append(
          'Set-Cookie',
          clearInviteIntentCookie(url.protocol === 'https:'),
        );
        return new Response(null, { status: 303, headers });
      }
      log.error('auth_callback_provisioning_failed', {
        request_id: requestIdentifier,
        route: redactAuthUrl(url),
        flow: state.flow,
        error_name: error instanceof Error ? error.name : 'unknown',
      });
      throw error;
    }

    const isHttps = url.protocol === 'https:';
    const headers = new Headers({ Location: state.safeRelativeReturnPath });
    headers.append(
      'Set-Cookie',
      `${SESSION_COOKIE}=${encodeURIComponent(sealedSession!)}; Path=/; HttpOnly; ${isHttps ? 'Secure; ' : ''}SameSite=Lax; Max-Age=${60 * 60 * 24 * 30}`,
    );
    log.info('auth_callback_succeeded', {
      request_id: requestIdentifier,
      route: redactAuthUrl(url),
      flow: state.flow,
    });
    return new Response(null, { status: 302, headers });
  };

  return async (context) => {
    const isSecure = context.url.protocol === 'https:';
    try {
      return appendAuthCorrelationClear(
        await handleCallback(context),
        isSecure,
      );
    } catch (error) {
      log.error('auth_callback_failed', {
        request_id: requestId(context.request),
        route: redactAuthUrl(context.url),
        error_name: error instanceof Error ? error.name : 'unknown',
      });
      return appendAuthCorrelationClear(
        new Response('Authentication failed; please restart.', { status: 500 }),
        isSecure,
      );
    }
  };
}

export const GET = createAuthCallbackRoute();
