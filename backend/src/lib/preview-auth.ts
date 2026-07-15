import { randomBytes, randomUUID } from 'node:crypto';
import { Redis } from '@upstash/redis';
import { SignJWT, jwtVerify } from 'jose';
import { env } from './env';
import { safeRelativeRedirectPath } from './safe-redirect';

const STATE_TTL = 10 * 60;
const GRANT_TTL = 60;
const STATE_NAMESPACE = 'nvm:gateway:state:v2';
const GRANT_NAMESPACE = 'nvm:preview:grant:v2';
const PREVIEW_HOST_RE = /^nvm-[a-z0-9-]+-pablo-varelas-projects-4f86af8b\.vercel\.app$/;
const TOKEN_PREFIX = 'v2.';

export const PREVIEW_GATEWAY_CAPABILITY = 'preview-auth-gateway-v2';
export type PreviewTarget = { origin: string; returnTo: string };
export type PreviewIdentity = { id: string; email: string };
export type GatewayState =
  | { v: 2; flow: 'production'; safeRelativeReturnPath: string; jti: string; exp: number }
  | { v: 2; flow: 'preview_gateway'; exactOrigin: string; deploymentId: string; jti: string; exp: number };

let testStore: Map<string, unknown> | null = null;
export function setPreviewAuthStoreForTests(store: Map<string, unknown> | null) { testStore = store; }

function keyMaterial(name: string): Uint8Array {
  const value = env(name);
  if (!value) throw new Error(`${name} is required`);
  return new TextEncoder().encode(value);
}

function previewHost(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === 'https:' && url.pathname === '/' && !url.search && !url.hash && !url.port && PREVIEW_HOST_RE.test(url.hostname);
  } catch { return false; }
}

export function previewTargetFromEnvironment(): PreviewTarget | null {
  if (env('VERCEL_ENV') !== 'preview' || !env('VERCEL_URL')) return null;
  const origin = `https://${env('VERCEL_URL')}`;
  return previewHost(origin) ? { origin, returnTo: '/' } : null;
}

export function previewTargetFromRequest(url: URL, returnTo: string | null): PreviewTarget | null {
  if (env('VERCEL_ENV') !== 'preview' || url.origin !== `https://${env('VERCEL_URL')}` || !previewHost(url.origin)) return null;
  return { origin: url.origin, returnTo: safeRelativeRedirectPath(returnTo) };
}

function stateKey(flow: GatewayState['flow'], jti: string) { return `${STATE_NAMESPACE}:${flow}:${jti}`; }
function startKey(jti: string) { return `${STATE_NAMESPACE}:preview_start:${jti}`; }
function grantKey(id: string) { return `${GRANT_NAMESPACE}:${id}`; }
function gatewayRedis() {
  const url = env('GATEWAY_STATE_REDIS_URL');
  const token = env('GATEWAY_STATE_REDIS_TOKEN');
  return url && token ? new Redis({ url, token }) : null;
}
function grantWriterRedis() {
  const url = env('UPSTASH_REDIS_REST_URL');
  const token = env('UPSTASH_REDIS_REST_TOKEN');
  return url && token ? new Redis({ url, token }) : null;
}

async function setNx(key: string, value: string, ttl: number, kind: 'state' | 'grant') {
  if (testStore) {
    if (testStore.has(key)) return false;
    testStore.set(key, value);
    return true;
  }
  const redis = kind === 'state' ? gatewayRedis() : grantWriterRedis();
  if (!redis) return false;
  try { return (await redis.set(key, value, { nx: true, ex: ttl })) === 'OK'; } catch { return false; }
}

async function getDel(key: string, kind: 'state' | 'grant') {
  if (testStore) {
    const value = testStore.get(key);
    testStore.delete(key);
    if (value === null || value === undefined) return null;
    return typeof value === 'string' ? value : JSON.stringify(value);
  }
  const redis = kind === 'state' ? gatewayRedis() : grantWriterRedis();
  if (!redis) return null;
  try {
    const value = await redis.getdel<unknown>(key);
    if (value === null || value === undefined) return null;
    return typeof value === 'string' ? value : JSON.stringify(value);
  } catch { return null; }
}

async function signed(payload: Record<string, unknown>, keyName: string, audience: string, ttl: number) {
  return `${TOKEN_PREFIX}${await new SignJWT(payload).setProtectedHeader({ alg: 'HS256' }).setAudience(audience).setIssuedAt().setExpirationTime(`${ttl}s`).sign(keyMaterial(keyName))}`;
}

async function verified<T extends Record<string, unknown>>(token: string | null, keyName: string, audience: string): Promise<T | null> {
  if (!token?.startsWith(TOKEN_PREFIX)) return null;
  try {
    const { payload } = await jwtVerify(token.slice(TOKEN_PREFIX.length), keyMaterial(keyName), { audience });
    return payload as T;
  } catch { return null; }
}

export async function createProductionState(returnTo: string | null): Promise<string | null> {
  const state: GatewayState = { v: 2, flow: 'production', safeRelativeReturnPath: safeRelativeRedirectPath(returnTo), jti: randomUUID(), exp: Math.floor(Date.now() / 1000) + STATE_TTL };
  if (!(await setNx(stateKey(state.flow, state.jti), JSON.stringify(state), STATE_TTL, 'state'))) return null;
  try { return await signed(state, 'GATEWAY_STATE_KEY', 'nvm-gateway-state-v2', STATE_TTL); } catch { return null; }
}

export async function createPreviewStartIntent(target: PreviewTarget): Promise<string | null> {
  if (!previewHost(target.origin) || target.returnTo !== '/') return null;
  try {
    return await signed({ v: 2, flow: 'preview_start', exactOrigin: target.origin, deploymentId: env('VERCEL_GIT_COMMIT_SHA') ?? env('VERCEL_URL'), jti: randomUUID() }, 'PREVIEW_START_KEY', 'nvm-preview-start-v2', 60);
  } catch { return null; }
}

export async function createPreviewGatewayState(intent: string): Promise<{ state: string; target: PreviewTarget } | null> {
  const payload = await verified<{ v: 2; flow: 'preview_start'; exactOrigin: string; deploymentId?: string; jti: string }>(intent, 'PREVIEW_START_KEY', 'nvm-preview-start-v2');
  if (!payload || payload.v !== 2 || payload.flow !== 'preview_start' || !payload.jti || !previewHost(payload.exactOrigin)) return null;
  const state: GatewayState = { v: 2, flow: 'preview_gateway', exactOrigin: payload.exactOrigin, deploymentId: payload.deploymentId ?? '', jti: randomUUID(), exp: Math.floor(Date.now() / 1000) + STATE_TTL };
  if (!(await setNx(startKey(payload.jti), '1', STATE_TTL, 'state'))) return null;
  if (!(await setNx(stateKey(state.flow, state.jti), JSON.stringify(state), STATE_TTL, 'state'))) return null;
  try { return { state: await signed(state, 'GATEWAY_STATE_KEY', 'nvm-gateway-state-v2', STATE_TTL), target: { origin: state.exactOrigin, returnTo: '/' } }; } catch { return null; }
}

export async function consumeGatewayState(token: string | null): Promise<GatewayState | null> {
  const payload = await verified<GatewayState & { startJti?: string }>(token, 'GATEWAY_STATE_KEY', 'nvm-gateway-state-v2');
  if (!payload || payload.v !== 2 || !payload.jti || (payload.flow !== 'production' && payload.flow !== 'preview_gateway')) return null;
  const standardClaims = new Set(['v', 'flow', 'jti', 'exp', 'aud', 'iat']);
  if (payload.flow === 'production' && (typeof payload.safeRelativeReturnPath !== 'string' || Object.keys(payload).some((key) => !standardClaims.has(key) && key !== 'safeRelativeReturnPath'))) return null;
  if (payload.flow === 'preview_gateway' && (!previewHost(payload.exactOrigin) || typeof payload.deploymentId !== 'string' || Object.keys(payload).some((key) => !standardClaims.has(key) && key !== 'exactOrigin' && key !== 'deploymentId'))) return null;
  const raw = await getDel(stateKey(payload.flow, payload.jti), 'state');
  if (!raw) return null;
  try {
    const stored = JSON.parse(raw) as GatewayState;
    if (stored.v !== 2 || stored.flow !== payload.flow || stored.jti !== payload.jti) return null;
    if (payload.flow === 'production') {
      const production = stored as Extract<GatewayState, { flow: 'production' }>;
      return production.safeRelativeReturnPath === payload.safeRelativeReturnPath ? payload : null;
    }
    const preview = stored as Extract<GatewayState, { flow: 'preview_gateway' }>;
    return preview.exactOrigin === payload.exactOrigin && preview.deploymentId === payload.deploymentId ? payload : null;
  } catch { return null; }
}

export async function createPreviewSessionGrant(target: PreviewTarget, identity: PreviewIdentity): Promise<string | null> {
  if (!previewHost(target.origin) || !identity.id || !identity.email) return null;
  const id = randomBytes(32).toString('base64url');
  const value = JSON.stringify({ v: 2, flow: 'preview_grant', origin: target.origin, returnTo: safeRelativeRedirectPath(target.returnTo), jti: randomUUID(), exp: Math.floor(Date.now() / 1000) + GRANT_TTL, identity });
  if (!(await setNx(grantKey(id), value, GRANT_TTL, 'grant'))) return null;
  return `${TOKEN_PREFIX}${id}`;
}

export async function consumePreviewSessionGrant(grant: string | null, requestOrigin: string): Promise<{ identity: PreviewIdentity; returnTo: string } | null> {
  if (!grant?.startsWith(TOKEN_PREFIX) || grant.length < 20 || !previewHost(requestOrigin)) return null;
  const raw = await getDel(grantKey(grant.slice(TOKEN_PREFIX.length)), 'grant');
  if (!raw) return null;
  try {
    const value = JSON.parse(raw) as { v: number; flow: string; origin: string; returnTo: string; exp: number; identity: PreviewIdentity };
    if (value.v !== 2 || value.flow !== 'preview_grant' || value.origin !== requestOrigin || value.exp <= Math.floor(Date.now() / 1000) || !value.identity?.id || !value.identity.email) return null;
    return { identity: value.identity, returnTo: safeRelativeRedirectPath(value.returnTo) };
  } catch { return null; }
}
