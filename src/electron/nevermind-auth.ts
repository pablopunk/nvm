import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { app, safeStorage } from 'electron';
import * as logger from './logger';
import { nevermindDesktopHeaders } from './nevermind-api';
import { checkNevermindCompatibility } from './nevermind-compatibility';
import { openExternalUrl } from './url-utils';

const FILENAME = 'nevermind-auth.json';

type StoredAuth = {
  encryptedToken?: string;
  token?: string;
  email: string;
  role: string;
  baseUrl: string;
  connectedAt: string;
};
type AuthSnapshot = {
  token: string;
  email: string;
  role: string;
  baseUrl: string;
} | null;
type SignInResult =
  | { ok: true; auth: NonNullable<AuthSnapshot> }
  | { ok: false; error: string };

const PRODUCTION_BASE_URL = 'https://api.nvm.fyi';
const DEFAULT_BASE_URL =
  process.env.NEVERMIND_BASE_URL ||
  (process.env.ELECTRON_RENDERER_URL
    ? 'http://localhost:4321'
    : PRODUCTION_BASE_URL);

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
  const trimmed = baseUrl.replace(/\/$/, '');
  if (
    shouldUseProductionBaseUrl() &&
    (['https://nvm.fyi', 'https://www.nvm.fyi'].includes(trimmed) ||
      isLoopbackBaseUrl(trimmed))
  )
    return PRODUCTION_BASE_URL;
  return trimmed;
}

function authPath() {
  return path.join(app.getPath('userData'), FILENAME);
}

let cached: AuthSnapshot = null;
let loadPromise: Promise<AuthSnapshot> | null = null;
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

async function readFromDisk(): Promise<AuthSnapshot> {
  try {
    const raw = await fs.readFile(authPath(), 'utf8');
    const data = JSON.parse(raw) as StoredAuth;
    const token = decryptToken(data);
    if (!token) return null;
    const baseUrl = normalizedBaseUrl(data.baseUrl);
    if (baseUrl !== data.baseUrl)
      logger.warn('normalized stored Nevermind auth base URL', {
        from: data.baseUrl,
        to: baseUrl,
      });
    return { token, email: data.email, role: data.role, baseUrl };
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code !== 'ENOENT')
      logger.warn('Failed to read nevermind auth', err as Error);
    return null;
  }
}

async function load() {
  if (loadPromise) return loadPromise;
  loadPromise = readFromDisk().then((auth) => {
    cached = auth;
    return auth;
  });
  return loadPromise;
}

export async function getNevermindAuth(): Promise<AuthSnapshot> {
  return load();
}

async function persist({
  token,
  email,
  role,
  baseUrl,
}: {
  token: string;
  email: string;
  role: string;
  baseUrl: string;
}) {
  const payload: StoredAuth = {
    email,
    role,
    baseUrl,
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
  await fs.writeFile(authPath(), JSON.stringify(payload, null, 2), {
    mode: 0o600,
  });
  if (process.platform !== 'win32') await fs.chmod(authPath(), 0o600);
  cached = { token, email, role, baseUrl };
  loadPromise = Promise.resolve(cached);
  return cached;
}

export async function clearNevermindAuth() {
  await fs.rm(authPath(), { force: true });
  cached = null;
  loadPromise = Promise.resolve(null);
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

export async function signInToNevermind({
  baseUrl = DEFAULT_BASE_URL,
  label = defaultDeviceLabel(),
}: {
  baseUrl?: string;
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
