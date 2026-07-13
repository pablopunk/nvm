import type { APIRoute } from 'astro';
import { consumePreviewSessionGrant } from '../../../lib/preview-auth';
import { createPreviewSessionToken, PREVIEW_SESSION_COOKIE } from '../../../lib/workos';
import { createUserFromInviteIntent, getUserByWorkosId, InviteRequiredError, upsertUserWithFreeGrant } from '../../../lib/users';
import { getSignupsEnabled, SignupsPolicyError } from '../../../lib/settings';
import { readInviteIntentCookie, clearInviteIntentCookie } from '../../../lib/waitlist';
import { previewAuthConfigured } from '../../../lib/auth-config';
import { log, redactAuthUrl } from '../../../lib/log';

export function previewExchangeSecurityHeaders() {
  return new Headers({
    'Referrer-Policy': 'no-referrer',
    'Cache-Control': 'no-store, private',
    'X-Robots-Tag': 'noindex',
  });
}

export const GET: APIRoute = async ({ url, request }) => {
  log.debug('preview_exchange_request', { route: redactAuthUrl(url) });
  if (process.env.VERCEL_ENV !== 'preview') return new Response('Not found', { status: 404 });
  if (!previewAuthConfigured()) return new Response('Preview authentication is unavailable', { status: 503 });
  if ([...url.searchParams.keys()].some((key) => key !== 'grant')) return new Response('Invalid preview session grant', { status: 400, headers: previewExchangeSecurityHeaders() });
  const grant = url.searchParams.get('grant');
  if (!grant) return new Response('Missing preview session grant', { status: 400, headers: previewExchangeSecurityHeaders() });

  const session = await consumePreviewSessionGrant(grant, url.origin);
  if (!session) return new Response('Invalid or expired preview session grant', { status: 400, headers: previewExchangeSecurityHeaders() });

  try {
    const existing = await getUserByWorkosId(session.identity.id);
    if (!existing) {
      const signupsEnabled = await getSignupsEnabled();
      const intent = readInviteIntentCookie(request.headers.get('cookie'));
      if (!signupsEnabled && intent) await createUserFromInviteIntent({ intentId: intent.id, nonce: intent.nonce, workosUserId: session.identity.id, email: session.identity.email });
      else if (!signupsEnabled) throw new InviteRequiredError();
      else await upsertUserWithFreeGrant({ workosUserId: session.identity.id, email: session.identity.email });
    }
  } catch (error) {
    if (error instanceof InviteRequiredError) {
      const headers = new Headers({ Location: '/?invite=required' });
      headers.set('Set-Cookie', clearInviteIntentCookie(true));
      for (const [key, value] of previewExchangeSecurityHeaders()) headers.set(key, value);
      return new Response(null, { status: 303, headers });
    }
    if (error instanceof SignupsPolicyError) return new Response('Sign-up policy is temporarily unavailable.', { status: 503 });
    throw error;
  }
  const previewToken = await createPreviewSessionToken(session.identity);
  if (!previewToken) return new Response('Preview authentication is unavailable', { status: 503 });

  const headers = previewExchangeSecurityHeaders();
  headers.append(
    'Set-Cookie',
    `${PREVIEW_SESSION_COOKIE}=${encodeURIComponent(previewToken)}; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=${60 * 60 * 24 * 30}`,
  );
  headers.set('Location', session.returnTo);
  return new Response(null, { status: 303, headers });
};
