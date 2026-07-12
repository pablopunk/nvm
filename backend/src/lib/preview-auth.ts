import { createHash, randomBytes } from 'node:crypto';
import { Redis } from '@upstash/redis';
import { EncryptJWT, SignJWT, jwtVerify, jwtDecrypt } from 'jose';
import { env } from './env';
import { safeRelativeRedirectPath } from './safe-redirect';

const PREVIEW_STATE_PREFIX = 'preview:';
const PREVIEW_STATE_AUDIENCE = 'nvm-preview-auth';
const PREVIEW_AUDIENCE = 'nvm-preview-session';
const PREVIEW_STATE_TTL_SECONDS = 60;
const PREVIEW_GRANT_TTL_SECONDS = 60;
const PREVIEW_HOST_RE = /^nvm-git-[a-z0-9-]+-pablo-varelas-projects-4f86af8b\.vercel\.app$/;

export type PreviewTarget = { origin: string; returnTo: string };

function isVercelPreviewOrigin(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === 'https:' && url.pathname === '/' && url.search === '' && url.hash === '' && PREVIEW_HOST_RE.test(url.hostname);
  } catch {
    return false;
  }
}

export function previewTargetFromRequest(url: URL, returnTo: string | null): PreviewTarget | null {
  if (!isVercelPreviewOrigin(url.origin)) return null;
  return { origin: url.origin, returnTo: safeRelativeRedirectPath(returnTo) };
}

function stateKey(id: string) {
  return `nvm:preview-auth:${id}`;
}

function grantStoreKey(id: string) {
  return `nvm:preview-grant:${id}`;
}

const redisUrl = env('UPSTASH_REDIS_REST_URL') ?? env('KV_REST_API_URL');
const redisToken = env('UPSTASH_REDIS_REST_TOKEN') ?? env('KV_REST_API_TOKEN');
const redis = redisUrl && redisToken ? new Redis({ url: redisUrl, token: redisToken }) : null;
let testStore: Map<string, string> | null = null;

export function setPreviewAuthStoreForTests(store: Map<string, string> | null) {
  testStore = store;
}

function stateKeyMaterial() {
  const password = env('WORKOS_COOKIE_PASSWORD');
  if (!password) throw new Error('WORKOS_COOKIE_PASSWORD is required');
  return createHash('sha256').update(password).digest();
}

async function putState(id: string, target: PreviewTarget) {
  const value = JSON.stringify({ origin: target.origin, returnTo: target.returnTo });
  if (testStore) {
    testStore.set(stateKey(id), value);
    return true;
  }
  if (!redis) return false;
  try {
    await redis.set(stateKey(id), value, { ex: PREVIEW_STATE_TTL_SECONDS });
    return true;
  } catch {
    return false;
  }
}

async function takeState(id: string): Promise<PreviewTarget | null> {
  let raw: string | null | undefined;
  try {
    raw = testStore ? testStore.get(stateKey(id)) : redis ? await redis.getdel<string>(stateKey(id)) : null;
    if (testStore) testStore.delete(stateKey(id));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<PreviewTarget>;
    if (typeof parsed.origin !== 'string' || typeof parsed.returnTo !== 'string' || !isVercelPreviewOrigin(parsed.origin)) return null;
    return { origin: parsed.origin, returnTo: safeRelativeRedirectPath(parsed.returnTo) };
  } catch {
    return null;
  }
}

async function putGrant(id: string) {
  if (testStore) {
    testStore.set(grantStoreKey(id), '1');
    return true;
  }
  if (!redis) return false;
  try {
    await redis.set(grantStoreKey(id), '1', { ex: PREVIEW_GRANT_TTL_SECONDS });
    return true;
  } catch {
    return false;
  }
}

async function takeGrant(id: string) {
  try {
    const raw = testStore ? testStore.get(grantStoreKey(id)) : redis ? await redis.getdel<string>(grantStoreKey(id)) : null;
    if (testStore) testStore.delete(grantStoreKey(id));
    return raw === '1';
  } catch {
    return false;
  }
}

export async function encodePreviewState(target: PreviewTarget): Promise<string | null> {
  if (!isVercelPreviewOrigin(target.origin)) return null;
  const id = randomBytes(32).toString('base64url');
  if (!(await putState(id, target))) return null;
  return `${PREVIEW_STATE_PREFIX}${await new SignJWT({ id })
    .setProtectedHeader({ alg: 'HS256' })
    .setAudience(PREVIEW_STATE_AUDIENCE)
    .setIssuedAt()
    .setExpirationTime(`${PREVIEW_STATE_TTL_SECONDS}s`)
    .sign(stateKeyMaterial())}`;
}

export async function decodePreviewState(value: string | null): Promise<PreviewTarget | null> {
  if (!value?.startsWith(PREVIEW_STATE_PREFIX)) return null;
  try {
    const { payload } = await jwtVerify(value.slice(PREVIEW_STATE_PREFIX.length), stateKeyMaterial(), { audience: PREVIEW_STATE_AUDIENCE });
    return typeof payload.id === 'string' ? takeState(payload.id) : null;
  } catch {
    return null;
  }
}

function grantKey() {
  return stateKeyMaterial();
}

export async function createPreviewSessionGrant(target: PreviewTarget, sealedSession: string): Promise<string | null> {
  const id = randomBytes(32).toString('base64url');
  if (!(await putGrant(id))) return null;
  return new EncryptJWT({ id, origin: target.origin, returnTo: target.returnTo, sealedSession })
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
    if (typeof payload.id !== 'string' || typeof payload.sealedSession !== 'string' || typeof payload.returnTo !== 'string') return null;
    if (!(await takeGrant(payload.id))) return null;
    return { sealedSession: payload.sealedSession, returnTo: safeRelativeRedirectPath(payload.returnTo) };
  } catch {
    return null;
  }
}
