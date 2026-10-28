import { log } from './log';
import { getUpstreamProbeSpec, type UpstreamProbeSpec } from './upstream';

export const PROVIDER_PROBE_TIMEOUT_MS = 4000;
export const PROVIDER_CATALOG_MAX_BYTES = 8 * 1024 * 1024;

export type ProviderReadiness = { provider: string; reachable: boolean; ready: boolean };

type CatalogReply = { status: number; catalogValid: boolean };

function isSuccessfulStatus(status: number): boolean {
  return status >= 200 && status < 300;
}

function isCatalogList(body: unknown, listKey: UpstreamProbeSpec['catalogListKey']): boolean {
  return typeof body === 'object' && body !== null && Array.isArray((body as Record<string, unknown>)[listKey]);
}

async function readBoundedCatalog(res: Response): Promise<unknown> {
  const reader = res.body?.getReader();
  if (!reader) return null;
  const chunks: Uint8Array[] = [];
  let received = 0;
  for (;;) {
    const next = await reader.read();
    if (next.done) break;
    received += next.value.byteLength;
    if (received > PROVIDER_CATALOG_MAX_BYTES) {
      await reader.cancel();
      return null;
    }
    chunks.push(next.value);
  }
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'));
  } catch {
    return null;
  }
}

async function withProbeTimeout<T>(probe: (signal: AbortSignal) => Promise<T>): Promise<T | null> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), PROVIDER_PROBE_TIMEOUT_MS);
  try {
    return await probe(controller.signal);
  } catch (error) {
    log.error('health_upstream_failed', { error });
    return null;
  } finally {
    clearTimeout(timer);
  }
}

async function fetchStatus(url: string, headers: Record<string, string>): Promise<number | null> {
  return withProbeTimeout(async (signal) => {
    const res = await fetch(url, { method: 'GET', headers, signal });
    await res.body?.cancel().catch(() => undefined);
    return res.status;
  });
}

async function fetchCatalog(url: string, spec: UpstreamProbeSpec): Promise<CatalogReply | null> {
  return withProbeTimeout(async (signal) => {
    const res = await fetch(url, { method: 'GET', headers: spec.headers, signal });
    if (!isSuccessfulStatus(res.status)) {
      await res.body?.cancel().catch(() => undefined);
      return { status: res.status, catalogValid: false };
    }
    const body = await readBoundedCatalog(res);
    return { status: res.status, catalogValid: isCatalogList(body, spec.catalogListKey) };
  });
}

// Liveness keeps the released rule: any answer below 500 is reachable, and 401, 403,
// 404, and 429 still count. Readiness needs a 2xx catalog with its list and, when the
// catalog answers anonymously, a 2xx from the authenticated verification endpoint.
export async function probeProviderReadiness(provider: string): Promise<ProviderReadiness> {
  let spec: UpstreamProbeSpec;
  try {
    spec = getUpstreamProbeSpec(provider);
  } catch (error) {
    log.warn('health_upstream_config_missing', { provider, error });
    return { provider, reachable: false, ready: false };
  }
  const keyUrl = spec.verification.kind === 'key' ? `${spec.baseUrl}${spec.verification.path}` : null;
  const [catalog, keyStatus] = await Promise.all([
    fetchCatalog(`${spec.baseUrl}${spec.catalogPath}`, spec),
    keyUrl ? fetchStatus(keyUrl, spec.headers) : Promise.resolve(null),
  ]);
  const catalogReachable = catalog !== null && catalog.status < 500;
  const catalogReady = catalog !== null && isSuccessfulStatus(catalog.status) && catalog.catalogValid;
  const reachable = catalogReachable && (keyUrl === null || (keyStatus !== null && keyStatus < 500));
  const ready = catalogReady && (keyUrl === null || (keyStatus !== null && isSuccessfulStatus(keyStatus)));
  if (!ready)
    log.warn('health_upstream_not_ready', {
      provider,
      catalog_status: catalog?.status ?? null,
      catalog_valid: catalog?.catalogValid ?? false,
      key_status: keyStatus,
    });
  return { provider, reachable, ready };
}
