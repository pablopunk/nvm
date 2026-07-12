// biome-ignore-all lint: This compatibility cache retains established Electron persistence conventions.
import fs from 'node:fs/promises';
import path from 'node:path';
import { app } from 'electron';
import * as logger from './logger';
import { nevermindDesktopHeaders } from './nevermind-api';

export type NevermindCompatibilityManifest = {
  client?: {
    compatible?: boolean;
    unsupportedReason?: string | null;
  };
  desktop?: {
    minimumSupportedVersion?: string;
    latestVersion?: string | null;
    updateUrl?: string;
  };
  features?: Record<string, boolean>;
};

type CachedCompatibilityManifest = {
  baseUrl: string;
  fetchedAt: string;
  manifest: NevermindCompatibilityManifest;
};

type CompatibilityCacheFile = {
  manifests?: Record<string, CachedCompatibilityManifest>;
};

type CompatibilityListener = () => void;

const CACHE_FILENAME = 'nevermind-compatibility.json';
const COMPATIBILITY_FETCH_TIMEOUT_MS = 5_000;
const cachedManifests = new Map<string, CachedCompatibilityManifest>();
const listeners = new Set<CompatibilityListener>();
let cacheLoadPromise: Promise<void> | null = null;

export class NevermindFeatureUnavailableError extends Error {
  constructor(public feature: string) {
    super(`Nevermind backend feature is unavailable: ${feature}`);
    this.name = 'NevermindFeatureUnavailableError';
  }
}

export class NevermindCompatibilityError extends Error {
  updateUrl?: string;
  minimumSupportedVersion?: string;
  latestVersion?: string | null;
  unsupportedReason?: string | null;

  constructor(manifest: NevermindCompatibilityManifest) {
    const minimum = manifest.desktop?.minimumSupportedVersion;
    super(
      minimum
        ? `This Nevermind version is no longer supported. Please update to ${minimum} or newer.`
        : 'This Nevermind version is no longer supported. Please update Nevermind.',
    );
    this.name = 'NevermindCompatibilityError';
    this.updateUrl = manifest.desktop?.updateUrl;
    this.minimumSupportedVersion = minimum;
    this.latestVersion = manifest.desktop?.latestVersion;
    this.unsupportedReason = manifest.client?.unsupportedReason;
  }
}

export function onNevermindCompatibilityChanged(
  listener: CompatibilityListener,
) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function currentNevermindCompatibilityManifest(baseUrl?: string) {
  if (baseUrl)
    return cachedManifests.get(normalizeBaseUrl(baseUrl))?.manifest || null;
  return (
    [...cachedManifests.values()].sort(
      (left, right) => Date.parse(right.fetchedAt) - Date.parse(left.fetchedAt),
    )[0]?.manifest || null
  );
}

export async function getCachedNevermindCompatibilityManifest(baseUrl: string) {
  await loadCompatibilityCache();
  return currentNevermindCompatibilityManifest(baseUrl);
}

export function nevermindCompatibilityFeatureEnabled(
  feature: string,
  manifest = currentNevermindCompatibilityManifest(),
) {
  return manifest?.features?.[feature] === true;
}

export function requireNevermindCompatibilityFeature(
  feature: string,
  manifest = currentNevermindCompatibilityManifest(),
) {
  if (!nevermindCompatibilityFeatureEnabled(feature, manifest))
    throw new NevermindFeatureUnavailableError(feature);
}

export function warmNevermindCompatibilityCache(baseUrl: string) {
  void (async () => {
    await loadCompatibilityCache();
    notifyCompatibilityChanged();
    await fetchCompatibilityManifest(baseUrl);
  })().catch((error) =>
    logger.warn('nevermind.compatibility.warm.failed', error as Error),
  );
}

export async function invalidateNevermindCompatibilityCache(baseUrl?: string) {
  await loadCompatibilityCache();
  if (baseUrl) cachedManifests.delete(normalizeBaseUrl(baseUrl));
  else cachedManifests.clear();
  await saveCompatibilityCache();
  notifyCompatibilityChanged();
}

export async function checkNevermindCompatibility(baseUrl: string) {
  const cached = await getCachedNevermindCompatibilityManifest(baseUrl);
  const manifest = await fetchCompatibilityManifest(baseUrl).catch((error) => {
    logger.warn('nevermind.compatibility.fetch.failed', error as Error);
    return null;
  });
  const effectiveManifest = manifest || cached;
  if (!effectiveManifest) return null;
  if (effectiveManifest.client?.compatible === false)
    throw new NevermindCompatibilityError(effectiveManifest);
  return effectiveManifest;
}

async function fetchCompatibilityManifest(baseUrl: string) {
  const trimmed = normalizeBaseUrl(baseUrl);
  const controller = new AbortController();
  const timeout = setTimeout(
    () => controller.abort(),
    COMPATIBILITY_FETCH_TIMEOUT_MS,
  );
  let res: Response;
  try {
    res = await fetch(`${trimmed}/api/compatibility`, {
      headers: nevermindDesktopHeaders(),
      signal: controller.signal,
    });
  } finally {
    clearTimeout(timeout);
  }
  if (res.status === 404) return null;
  if (!res.ok) {
    logger.warn('nevermind.compatibility.unavailable', { status: res.status });
    return null;
  }
  const manifest = (await res
    .json()
    .catch(() => null)) as NevermindCompatibilityManifest | null;
  if (!manifest) return null;
  await cacheCompatibilityManifest(trimmed, manifest);
  return manifest;
}

async function cacheCompatibilityManifest(
  baseUrl: string,
  manifest: NevermindCompatibilityManifest,
) {
  await loadCompatibilityCache();
  cachedManifests.set(baseUrl, {
    baseUrl,
    fetchedAt: new Date().toISOString(),
    manifest,
  });
  await saveCompatibilityCache();
  notifyCompatibilityChanged();
}

async function loadCompatibilityCache() {
  if (cacheLoadPromise) return cacheLoadPromise;
  cacheLoadPromise = (async () => {
    try {
      const data = JSON.parse(
        await fs.readFile(compatibilityCachePath(), 'utf8'),
      ) as CompatibilityCacheFile;
      cachedManifests.clear();
      for (const [baseUrl, entry] of Object.entries(data.manifests || {})) {
        if (entry?.manifest)
          cachedManifests.set(normalizeBaseUrl(baseUrl), {
            ...entry,
            baseUrl: normalizeBaseUrl(entry.baseUrl || baseUrl),
          });
      }
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== 'ENOENT')
        logger.warn(
          'nevermind.compatibility.cache.read.failed',
          error as Error,
        );
    }
  })();
  return cacheLoadPromise;
}

async function saveCompatibilityCache() {
  try {
    await fs.writeFile(
      compatibilityCachePath(),
      JSON.stringify(
        { manifests: Object.fromEntries(cachedManifests) },
        null,
        2,
      ),
      { mode: 0o600 },
    );
  } catch (error) {
    logger.warn('nevermind.compatibility.cache.write.failed', error as Error);
  }
}

function compatibilityCachePath() {
  return path.join(app.getPath('userData'), CACHE_FILENAME);
}

function normalizeBaseUrl(baseUrl: string) {
  return String(baseUrl || '').replace(/\/$/, '');
}

function notifyCompatibilityChanged() {
  for (const listener of listeners) {
    try {
      listener();
    } catch {}
  }
}
