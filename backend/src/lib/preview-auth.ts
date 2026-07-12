import { createHash } from 'node:crypto';
import { EncryptJWT, jwtDecrypt } from 'jose';
import { env } from './env';
import { safeRelativeRedirectPath } from './safe-redirect';

const PREVIEW_STATE_PREFIX = 'preview:';
const PREVIEW_AUDIENCE = 'nvm-preview-session';
const PREVIEW_GRANT_TTL_SECONDS = 60;

type PreviewTarget = { origin: string; returnTo: string };

function isVercelPreviewOrigin(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === 'https:' && url.pathname === '/' && url.search === '' && url.hash === '' && url.hostname.endsWith('.vercel.app');
  } catch {
    return false;
  }
}

export function previewTargetFromRequest(url: URL, returnTo: string | null): PreviewTarget | null {
  if (!isVercelPreviewOrigin(url.origin)) return null;
  return { origin: url.origin, returnTo: safeRelativeRedirectPath(returnTo) };
}

export function encodePreviewState(target: PreviewTarget): string {
  return `${PREVIEW_STATE_PREFIX}${Buffer.from(JSON.stringify(target)).toString('base64url')}`;
}

export function decodePreviewState(value: string | null): PreviewTarget | null {
  if (!value?.startsWith(PREVIEW_STATE_PREFIX)) return null;
  try {
    const parsed = JSON.parse(Buffer.from(value.slice(PREVIEW_STATE_PREFIX.length), 'base64url').toString()) as Partial<PreviewTarget>;
    if (typeof parsed.origin !== 'string' || typeof parsed.returnTo !== 'string' || !isVercelPreviewOrigin(parsed.origin)) return null;
    return { origin: parsed.origin, returnTo: safeRelativeRedirectPath(parsed.returnTo) };
  } catch {
    return null;
  }
}

function grantKey() {
  const password = env('WORKOS_COOKIE_PASSWORD');
  if (!password) throw new Error('WORKOS_COOKIE_PASSWORD is required');
  return createHash('sha256').update(password).digest();
}

export async function createPreviewSessionGrant(target: PreviewTarget, sealedSession: string): Promise<string> {
  return new EncryptJWT({ origin: target.origin, returnTo: target.returnTo, sealedSession })
    .setProtectedHeader({ alg: 'dir', enc: 'A256GCM' })
    .setAudience(PREVIEW_AUDIENCE)
    .setIssuedAt()
    .setExpirationTime(`${PREVIEW_GRANT_TTL_SECONDS}s`)
    .encrypt(grantKey());
}

export async function consumePreviewSessionGrant(grant: string, requestOrigin: string): Promise<{ sealedSession: string; returnTo: string } | null> {
  try {
    const { payload } = await jwtDecrypt(grant, grantKey(), { audience: PREVIEW_AUDIENCE });
    if (payload.origin !== requestOrigin || !isVercelPreviewOrigin(requestOrigin)) return null;
    if (typeof payload.sealedSession !== 'string' || typeof payload.returnTo !== 'string') return null;
    return { sealedSession: payload.sealedSession, returnTo: safeRelativeRedirectPath(payload.returnTo) };
  } catch {
    return null;
  }
}
