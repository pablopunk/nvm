import { WorkOS } from '@workos-inc/node';
import { SignJWT, jwtVerify } from 'jose';
import { env } from './env';

export const WORKOS_CLIENT_ID = env('WORKOS_CLIENT_ID') as string;
export const workos = new WorkOS(env('WORKOS_API_KEY'), {
  clientId: WORKOS_CLIENT_ID,
});
export const COOKIE_PASSWORD = env('WORKOS_COOKIE_PASSWORD') as string;
export const SESSION_COOKIE = 'nvm_session';
export const PREVIEW_SESSION_COOKIE = 'nvm_preview_session';

export function authorizationUrlForState(state: string, redirectUri: string): string | null {
  const clientId = env('WORKOS_CLIENT_ID')?.trim();
  if (!clientId) return null;
  try {
    return workos.userManagement.getAuthorizationUrl({
      provider: 'authkit',
      clientId,
      redirectUri,
      state,
    });
  } catch {
    return null;
  }
}

function previewSessionKey() {
  const value = env('PREVIEW_SESSION_KEY');
  return value ? new TextEncoder().encode(value) : null;
}

export async function createPreviewSessionToken(identity: { id: string; email: string }) {
  const key = previewSessionKey();
  if (!key) return null;
  return new SignJWT({ email: identity.email, mode: 'preview' })
    .setProtectedHeader({ alg: 'HS256' })
    .setSubject(identity.id)
    .setAudience('nvm-preview-session-v2')
    .setIssuedAt()
    .setExpirationTime('30d')
    .sign(key);
}

async function getPreviewSession(cookie: string) {
  const key = previewSessionKey();
  if (!key) return null;
  try {
    const { payload } = await jwtVerify(cookie, key, { audience: 'nvm-preview-session-v2' });
    if (payload.mode !== 'preview' || typeof payload.sub !== 'string' || typeof payload.email !== 'string') return null;
    return { authenticated: true, user: { id: payload.sub, email: payload.email } };
  } catch { return null; }
}

export async function getSessionFromCookies(cookieHeader: string | null) {
  if (!cookieHeader) return null;
  const parts = cookieHeader.split(/;\s*/);
  const preview = parts.find((c) => c.startsWith(`${PREVIEW_SESSION_COOKIE}=`));
  const production = parts.find((c) => c.startsWith(`${SESSION_COOKIE}=`));
  if (env('VERCEL_ENV') === 'preview') {
    if (!preview || production) return null;
    return getPreviewSession(decodeURIComponent(preview.slice(PREVIEW_SESSION_COOKIE.length + 1)));
  }
  if (preview || !production) return null;
  const match = production;
  if (!match) return null;
  const sealed = decodeURIComponent(match.slice(SESSION_COOKIE.length + 1));
  try {
    const session = workos.userManagement.loadSealedSession({
      sessionData: sealed,
      cookiePassword: COOKIE_PASSWORD,
    });
    const auth = await session.authenticate();
    return auth.authenticated ? auth : null;
  } catch {
    return null;
  }
}
