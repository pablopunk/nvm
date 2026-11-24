import { AsyncLocalStorage } from 'node:async_hooks';
import { SESSION_COOKIE } from './session-cookie';

type RefreshScope = { refreshedSealedSession: string | null };

const refreshScope = new AsyncLocalStorage<RefreshScope>();

export function rememberRefreshedSession(sealedSession: string) {
  const scope = refreshScope.getStore();
  if (scope) scope.refreshedSealedSession = sealedSession;
}

export function refreshedSessionInThisRequest(): string | null {
  return refreshScope.getStore()?.refreshedSealedSession ?? null;
}

export async function sendRefreshedSessionCookie(
  isHttps: boolean,
  handleRequest: () => Promise<Response>,
): Promise<Response> {
  const scope: RefreshScope = { refreshedSealedSession: null };
  const response = await refreshScope.run(scope, handleRequest);
  if (!scope.refreshedSealedSession) return response;
  return withSetCookie(response, sessionCookieHeader(scope.refreshedSealedSession, isHttps));
}

export function sessionCookieHeader(sealedSession: string, isHttps: boolean) {
  return `${SESSION_COOKIE}=${encodeURIComponent(sealedSession)}; Path=/; HttpOnly; ${isHttps ? 'Secure; ' : ''}SameSite=Lax; Max-Age=${60 * 60 * 24 * 30}`;
}

function withSetCookie(response: Response, cookie: string) {
  try {
    response.headers.append('Set-Cookie', cookie);
    return response;
  } catch {
    const headers = new Headers(response.headers);
    headers.append('Set-Cookie', cookie);
    return new Response(response.body, { status: response.status, statusText: response.statusText, headers });
  }
}
