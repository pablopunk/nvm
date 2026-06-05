import { WorkOS } from '@workos-inc/node';
import { env } from './env';

export const WORKOS_CLIENT_ID = env('WORKOS_CLIENT_ID') as string;
export const workos = new WorkOS(env('WORKOS_API_KEY'), {
  clientId: WORKOS_CLIENT_ID,
});
export const COOKIE_PASSWORD = env('WORKOS_COOKIE_PASSWORD') as string;
export const SESSION_COOKIE = 'nvm_session';

export async function getSessionFromCookies(cookieHeader: string | null) {
  if (!cookieHeader) return null;
  const match = cookieHeader.split(/;\s*/).find((c) => c.startsWith(`${SESSION_COOKIE}=`));
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
