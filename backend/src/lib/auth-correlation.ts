const AUTH_CORRELATION_COOKIE = 'nvm_auth_state';
const AUTH_CORRELATION_PATH = '/api/auth/callback';
const AUTH_CORRELATION_MAX_AGE_SECONDS = 10 * 60;

function cookieAttributes(isSecure: boolean, maxAge: number) {
  return [
    `Path=${AUTH_CORRELATION_PATH}`,
    'HttpOnly',
    isSecure ? 'Secure' : null,
    'SameSite=Lax',
    `Max-Age=${maxAge}`,
  ]
    .filter(Boolean)
    .join('; ');
}

export function authCorrelationCookie(state: string, isSecure: boolean) {
  return `${AUTH_CORRELATION_COOKIE}=${encodeURIComponent(state)}; ${cookieAttributes(isSecure, AUTH_CORRELATION_MAX_AGE_SECONDS)}`;
}

export function clearAuthCorrelationCookie(isSecure: boolean) {
  return `${AUTH_CORRELATION_COOKIE}=; ${cookieAttributes(isSecure, 0)}`;
}

export function readAuthCorrelationCookie(cookieHeader: string | null) {
  const encoded = cookieHeader
    ?.split(/;\s*/)
    .find((cookie) => cookie.startsWith(`${AUTH_CORRELATION_COOKIE}=`))
    ?.slice(AUTH_CORRELATION_COOKIE.length + 1);
  if (encoded === undefined) return null;
  try {
    return decodeURIComponent(encoded);
  } catch {
    return null;
  }
}

export function authCorrelationMatches(
  cookieHeader: string | null,
  state: string | null,
) {
  return Boolean(state && readAuthCorrelationCookie(cookieHeader) === state);
}

export function appendAuthCorrelationClear(
  response: Response,
  isSecure: boolean,
) {
  const headers = new Headers(response.headers);
  headers.append('Set-Cookie', clearAuthCorrelationCookie(isSecure));
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
}
