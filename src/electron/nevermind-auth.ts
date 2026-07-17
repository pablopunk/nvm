// biome-ignore-all lint: This module retains existing authentication persistence conventions.
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { app, safeStorage } from 'electron';
import * as logger from './logger';
import { nevermindDesktopHeaders } from './nevermind-api';
import { checkNevermindCompatibility } from './nevermind-compatibility';
import { openExternalUrl } from './url-utils';
import {
  migrateLegacyDesktopOrigin,
  parsePublicOrigin,
} from '../shared/public-origin';

const FILENAME = 'nevermind-auth.json';
const STORE_FILENAME = 'nevermind-auth-by-origin.json';

export type NevermindEnvironment = 'production' | 'pr_preview' | 'custom';

type StoredAuth = {
  encryptedToken?: string;
  token?: string;
  email: string;
  role: string;
  baseUrl: string;
  environment?: NevermindEnvironment;
  connectedAt: string;
};
type AuthStore = Record<string, StoredAuth>;
export type NevermindAuthSnapshot = {
  token: string;
  email: string;
  role: string;
  baseUrl: string;
  environment: NevermindEnvironment;
} | null;
type SignInResult =
  | { ok: true; auth: NonNullable<NevermindAuthSnapshot> }
  | { ok: false; error: string };

const PRODUCTION_BASE_URL = 'https://api.nvm.fyi';
export function resolveDefaultNevermindBaseUrl(
  environment: Partial<
    Pick<NodeJS.ProcessEnv, 'NEVERMIND_BASE_URL' | 'ELECTRON_RENDERER_URL'>
  > = process.env,
  isPackaged = app.isPackaged,
) {
  const candidate =
    environment.NEVERMIND_BASE_URL ||
    (!isPackaged && environment.ELECTRON_RENDERER_URL
      ? 'http://localhost:4321'
      : PRODUCTION_BASE_URL);
  const migrated = migrateLegacyDesktopOrigin(candidate);
  if (migrated) return migrated;
  try {
    return parsePublicOrigin(
      candidate,
      candidate.startsWith('http://localhost') ? 'local' : 'smoke',
    );
  } catch {
    return PRODUCTION_BASE_URL;
  }
}

const DEFAULT_BASE_URL = resolveDefaultNevermindBaseUrl();

function shouldUseProductionBaseUrl() {
  return app.isPackaged && !process.env.NEVERMIND_BASE_URL;
}

function isLoopbackBaseUrl(baseUrl: string) {
  try {
    const host = new URL(baseUrl).hostname;
    return host === 'localhost' || host === '127.0.0.1' || host === '::1';
  } catch {
    return false;
  }
}

function normalizedBaseUrl(baseUrl: string) {
  const trimmed = baseUrl.trim().replace(/\/$/, '');
  const migrated = migrateLegacyDesktopOrigin(trimmed);
  if (
    migrated &&
    (shouldUseProductionBaseUrl() || migrated === PRODUCTION_BASE_URL)
  )
    return migrated;
  if (shouldUseProductionBaseUrl() && isLoopbackBaseUrl(trimmed))
    return PRODUCTION_BASE_URL;
  try {
    return parsePublicOrigin(
      trimmed,
      isLoopbackBaseUrl(trimmed) ? 'local' : 'smoke',
    );
  } catch {
    logger.warn('rejected invalid Nevermind auth base URL', {
      baseUrl: trimmed,
    });
    return PRODUCTION_BASE_URL;
  }
}

export function nevermindEnvironmentForBaseUrl(
  baseUrl: string,
): NevermindEnvironment {
  return normalizedBaseUrl(baseUrl) === PRODUCTION_BASE_URL
    ? 'production'
    : 'custom';
}

function authPath() {
  return path.join(app.getPath('userData'), FILENAME);
}

function storePath() {
  return path.join(app.getPath('userData'), STORE_FILENAME);
}

let activeBaseUrl = normalizedBaseUrl(DEFAULT_BASE_URL);
let authPathOverride: string | null = null;

function currentAuthPath() {
  return authPathOverride || storePath();
}

function legacyAuthPath() {
  return authPathOverride
    ? path.join(path.dirname(authPathOverride), FILENAME)
    : authPath();
}

let cached: NevermindAuthSnapshot = null;
let loadPromise: Promise<NevermindAuthSnapshot> | null = null;
let activeSignIn: Promise<SignInResult> | null = null;

function decryptToken(data: StoredAuth): string | null {
  if (data.encryptedToken && safeStorage.isEncryptionAvailable()) {
    return safeStorage.decryptString(
      Buffer.from(data.encryptedToken, 'base64'),
    );
  }
  if (data.token) {
    if (!safeStorage.isEncryptionAvailable())
      logger.warn('safeStorage unavailable; reading plaintext Nevermind token');
    return Buffer.from(data.token, 'base64').toString('utf8');
  }
  if (data.encryptedToken) {
    logger.warn(
      'safeStorage unavailable; cannot decrypt stored Nevermind token',
    );
  }
  return null;
}

function authFromStored(data: StoredAuth): NevermindAuthSnapshot {
  const token = decryptToken(data);
  if (!token) return null;
  const baseUrl = normalizedBaseUrl(data.baseUrl);
  if (baseUrl !== data.baseUrl)
    logger.warn('normalized stored Nevermind auth base URL', {
      from: data.baseUrl,
      to: baseUrl,
    });
  return {
    token,
    email: data.email,
    role: data.role,
    baseUrl,
    environment: data.environment || nevermindEnvironmentForBaseUrl(baseUrl),
  };
}

function isLegacyAuth(data: unknown): data is StoredAuth {
  return Boolean(
    data &&
      typeof data === 'object' &&
      'baseUrl' in data &&
      typeof (data as StoredAuth).baseUrl === 'string',
  );
}

async function readStore(): Promise<AuthStore | null> {
  try {
    const raw = await fs.readFile(currentAuthPath(), 'utf8');
    const data = JSON.parse(raw) as AuthStore | StoredAuth;
    return isLegacyAuth(data) ? {} : data;
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === 'ENOENT') return {};
    throw err;
  }
}

async function migrateLegacyAuthFile(): Promise<NevermindAuthSnapshot> {
  try {
    const raw = await fs.readFile(legacyAuthPath(), 'utf8');
    const data = JSON.parse(raw) as AuthStore | StoredAuth;
    if (!isLegacyAuth(data)) return null;
    const baseUrl = normalizedBaseUrl(data.baseUrl);
    const store: AuthStore = { [baseUrl]: { ...data, baseUrl } };
    await fs.writeFile(currentAuthPath(), JSON.stringify(store, null, 2), {
      mode: 0o600,
    });
    if (process.platform !== 'win32') await fs.chmod(currentAuthPath(), 0o600);
    await fs.rename(legacyAuthPath(), `${legacyAuthPath()}.bak`);
    return authFromStored(store[baseUrl]);
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === 'ENOENT') return null;
    logger.warn('nevermind.auth.migration.failed', err as Error);
    return null;
  }
}

async function readFromDisk(): Promise<NevermindAuthSnapshot> {
  let store: AuthStore | null;
  try {
    store = await readStore();
  } catch (err) {
    logger.warn('Failed to read nevermind auth', err as Error);
    return null;
  }
  if (!store || Object.keys(store).length === 0 || !store[activeBaseUrl]) {
    const migrated = await migrateLegacyAuthFile();
    if (migrated && migrated.baseUrl === activeBaseUrl) return migrated;
  }
  const stored = store?.[activeBaseUrl];
  return stored ? authFromStored(stored) : null;
}

async function readOrMigrateStore(): Promise<AuthStore> {
  let store = (await readStore()) || {};
  if (Object.keys(store).length === 0) {
    await migrateLegacyAuthFile();
    store = (await readStore()) || {};
  }
  return store;
}

async function load() {
  if (loadPromise) return loadPromise;
  loadPromise = readFromDisk().then((auth) => {
    cached = auth;
    return auth;
  });
  return loadPromise;
}

export async function getNevermindAuth(): Promise<NevermindAuthSnapshot> {
  return load();
}

export function setActiveNevermindAuthBaseUrl(baseUrl: string | null) {
  activeBaseUrl = normalizedBaseUrl(baseUrl || DEFAULT_BASE_URL);
  cached = null;
  loadPromise = null;
}

async function persist({
  token,
  email,
  role,
  baseUrl,
  environment = nevermindEnvironmentForBaseUrl(baseUrl),
}: {
  token: string;
  email: string;
  role: string;
  baseUrl: string;
  environment?: NevermindEnvironment;
}) {
  const payload: StoredAuth = {
    email,
    role,
    baseUrl,
    environment,
    connectedAt: new Date().toISOString(),
  };
  if (safeStorage.isEncryptionAvailable()) {
    payload.encryptedToken = safeStorage
      .encryptString(token)
      .toString('base64');
  } else {
    logger.warn(
      'safeStorage unavailable; storing Nevermind token as plaintext',
    );
    payload.token = Buffer.from(token, 'utf8').toString('base64');
  }
  const store = await readOrMigrateStore();
  store[normalizedBaseUrl(baseUrl)] = payload;
  await fs.writeFile(currentAuthPath(), JSON.stringify(store, null, 2), {
    mode: 0o600,
  });
  if (process.platform !== 'win32') await fs.chmod(currentAuthPath(), 0o600);
  cached = { token, email, role, baseUrl, environment };
  loadPromise = Promise.resolve(cached);
  return cached;
}

export async function clearNevermindAuth() {
  const store = await readOrMigrateStore();
  delete store[activeBaseUrl];
  await fs.writeFile(currentAuthPath(), JSON.stringify(store, null, 2), {
    mode: 0o600,
  });
  if (process.platform !== 'win32') await fs.chmod(currentAuthPath(), 0o600);
  cached = null;
  loadPromise = Promise.resolve(null);
}

export function setNevermindAuthFilePathForTests(filePath: string | null) {
  authPathOverride = filePath;
}

export function clearNevermindAuthCacheForTests() {
  cached = null;
  loadPromise = null;
  activeBaseUrl = normalizedBaseUrl(DEFAULT_BASE_URL);
}

export async function signOutFromNevermind(): Promise<{ revoked: boolean }> {
  const current = await load();
  let revoked = false;
  if (current) {
    try {
      const res = await fetch(`${current.baseUrl}/api/tokens/current`, {
        method: 'DELETE',
        headers: nevermindDesktopHeaders({
          Authorization: `Bearer ${current.token}`,
          Origin: current.baseUrl,
        }),
      });
      revoked = res.ok || res.status === 401;
      if (!revoked) logger.warn(`token revoke returned ${res.status}`);
    } catch (err) {
      logger.warn('token revoke failed', err as Error);
    }
  }
  await clearNevermindAuth();
  return { revoked };
}

export class NevermindAuthRequiredError extends Error {
  constructor() {
    super('Sign in to Nevermind to use AI features.');
    this.name = 'NevermindAuthRequiredError';
  }
}

export function getDefaultNevermindBaseUrl() {
  return DEFAULT_BASE_URL;
}

function defaultDeviceLabel() {
  return `${os.hostname()} (${process.platform})`;
}

async function postJson(url: string, body: unknown) {
  return fetch(url, {
    method: 'POST',
    headers: nevermindDesktopHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(body),
  });
}

export function isSigningIn() {
  return Boolean(activeSignIn);
}

export async function consumeDeviceCode({
  code,
  baseUrl = DEFAULT_BASE_URL,
}: {
  code: string;
  baseUrl?: string;
}): Promise<SignInResult> {
  if (activeSignIn) return activeSignIn;
  const trimmedBase = normalizedBaseUrl(baseUrl);
  activeSignIn = (async (): Promise<SignInResult> => {
    try {
      await checkNevermindCompatibility(trimmedBase);
      const deadline = Date.now() + 300_000;
      const interval = 2000;
      while (Date.now() < deadline) {
        await new Promise((r) => setTimeout(r, interval));
        const res = await postJson(`${trimmedBase}/api/auth/device/exchange`, {
          code,
        });
        if (res.status === 410)
          return { ok: false, error: 'code expired or already used' };
        if (res.status === 503) {
          const body = await res.json().catch(() => ({}));
          if ((body as any)?.error?.type === 'service_unavailable') {
            return {
              ok: false,
              error:
                (body as any)?.error?.message ||
                'device auth is temporarily unavailable',
            };
          }
        }
        if (!res.ok) {
          logger.warn(`device exchange returned ${res.status}`);
          continue;
        }
        const data = (await res.json()) as {
          status: string;
          token?: string;
          user?: { email: string; role: string };
        };
        if (data.status === 'ok' && data.token && data.user) {
          const auth = await persist({
            token: data.token,
            email: data.user.email,
            role: data.user.role,
            baseUrl: trimmedBase,
          });
          return { ok: true, auth };
        }
      }
      return { ok: false, error: 'timed out waiting for approval' };
    } catch (err) {
      logger.error('consumeDeviceCode failed', err as Error);
      return { ok: false, error: (err as Error).message };
    } finally {
      activeSignIn = null;
    }
  })();
  return activeSignIn;
}

export async function signInToNevermind({
  baseUrl = DEFAULT_BASE_URL,
  environment = nevermindEnvironmentForBaseUrl(baseUrl),
  label = defaultDeviceLabel(),
}: {
  baseUrl?: string;
  environment?: NevermindEnvironment;
  label?: string;
} = {}): Promise<SignInResult> {
  if (activeSignIn) return activeSignIn;
  const trimmedBase = normalizedBaseUrl(baseUrl);
  activeSignIn = (async (): Promise<SignInResult> => {
    try {
      await checkNevermindCompatibility(trimmedBase);
      const initRes = await postJson(
        `${trimmedBase}/api/auth/device/initiate`,
        { label },
      );
      if (!initRes.ok)
        return { ok: false, error: `initiate failed: ${initRes.status}` };
      const { code, verifyUrl, expiresAt, pollIntervalMs } =
        (await initRes.json()) as {
          code: string;
          verifyUrl: string;
          expiresAt: string;
          pollIntervalMs?: number;
        };
      if (!(await openExternalUrl(verifyUrl)))
        return { ok: false, error: 'unsafe verification URL' };
      const deadline = new Date(expiresAt).getTime();
      const interval = Math.max(1000, pollIntervalMs ?? 2000);
      while (Date.now() < deadline) {
        await new Promise((r) => setTimeout(r, interval));
        const res = await postJson(`${trimmedBase}/api/auth/device/exchange`, {
          code,
        });
        if (res.status === 410)
          return { ok: false, error: 'code expired or already used' };
        if (!res.ok) {
          logger.warn(`device exchange returned ${res.status}`);
          continue;
        }
        const data = (await res.json()) as {
          status: string;
          token?: string;
          user?: { email: string; role: string };
        };
        if (data.status === 'ok' && data.token && data.user) {
          const auth = await persist({
            token: data.token,
            email: data.user.email,
            role: data.user.role,
            baseUrl: trimmedBase,
            environment,
          });
          return { ok: true, auth };
        }
      }
      return { ok: false, error: 'timed out waiting for approval' };
    } catch (err) {
      logger.error('signInToNevermind failed', err as Error);
      return { ok: false, error: (err as Error).message };
    } finally {
      activeSignIn = null;
    }
  })();
  return activeSignIn;
}
