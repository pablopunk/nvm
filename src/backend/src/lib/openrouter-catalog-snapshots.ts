import { log } from './log';
import {
  loadSnapshotRecord,
  type PriceValidity,
  registerDurableCacheForTests,
  saveSnapshotRecord,
  snapshotLookup,
  type ValidatedSnapshot,
} from './pricing-snapshot-store';
import { getUpstreamBaseUrl } from './upstream';

const CATALOG_TIMEOUT_MS = 8_000;
/** Re-read at most this often per catalog, so a missing row does not query on every request. */
const RESTORE_RETRY_MS = 30 * 1000;

export type OpenRouterCatalogSpec<T> = {
  /** Key of the catalog's durable row in `pricing_snapshots`. */
  snapshotKey: string;
  /** Path and query under the OpenRouter base URL, such as `/models?output_modalities=decisions`. */
  path: string;
  /** Reads one catalog response; null when the body is not a catalog. */
  parseResponse(body: unknown): T[] | null;
  /** Re-validates a restored payload, which is untrusted once it is read back. */
  isPayload(payload: unknown): payload is T[];
};

export type AdmissibleCatalog<T> = { value: T[]; validity: PriceValidity };

export type DurableOpenRouterCatalog<T> = {
  /** Fetches and publishes the catalog. Only the scheduled refresh and admin saves call this. */
  refresh(): Promise<void>;
  /** The catalog a request may use, restored from its durable row. Never reaches the network. */
  admissible(): Promise<AdmissibleCatalog<T> | undefined>;
  reset(): void;
  expire(expiredAt: number): void;
};

/** `data` of an OpenRouter catalog body, or null when the body is not one. */
export function openRouterCatalogData(body: unknown): unknown[] | null {
  if (typeof body !== 'object' || body === null) return null;
  const data = (body as { data?: unknown }).data;
  return Array.isArray(data) ? data : null;
}

/**
 * A validated OpenRouter catalog kept as one durable snapshot. A request restores the
 * last validated copy and admits it while it is current or stale; an unvalidated or
 * expired catalog admits nothing. A failed refresh keeps the last validated copy.
 */
export function durableOpenRouterCatalog<T>(
  spec: OpenRouterCatalogSpec<T>,
): DurableOpenRouterCatalog<T> {
  let entry: ValidatedSnapshot<T[]> | undefined;
  let restoreAttemptedAt: number | undefined;
  let inflight: Promise<void> | undefined;

  async function publish(): Promise<void> {
    const response = await fetch(`${getUpstreamBaseUrl('openrouter')}${spec.path}`, {
      signal: AbortSignal.timeout(CATALOG_TIMEOUT_MS),
    });
    if (!response.ok)
      throw new Error(`OpenRouter ${spec.snapshotKey} returned ${response.status}`);
    const value = spec.parseResponse(await response.json());
    if (!value) throw new Error(`OpenRouter ${spec.snapshotKey} is malformed`);
    const validatedAt = Date.now();
    entry = { value, validatedAt };
    const stored = await saveSnapshotRecord({
      key: spec.snapshotKey,
      validatedAt,
      payload: value,
    });
    if (!stored)
      throw new Error(`OpenRouter ${spec.snapshotKey} was not stored durably`);
  }

  function refresh(): Promise<void> {
    inflight ??= publish()
      .catch((error: unknown) => {
        log.warn('openrouter_catalog_refresh_failed', {
          catalog: spec.snapshotKey,
          error: String(error),
        });
        throw error;
      })
      .finally(() => {
        inflight = undefined;
      });
    return inflight;
  }

  async function restore(): Promise<void> {
    const now = Date.now();
    if (restoreAttemptedAt !== undefined && now - restoreAttemptedAt < RESTORE_RETRY_MS)
      return;
    restoreAttemptedAt = now;
    const record = await loadSnapshotRecord(spec.snapshotKey);
    if (!record || !spec.isPayload(record.payload)) return;
    if (!entry || entry.validatedAt < record.validatedAt)
      entry = { value: record.payload, validatedAt: record.validatedAt };
  }

  async function admissible(): Promise<AdmissibleCatalog<T> | undefined> {
    if (!entry || snapshotLookup(entry).status !== 'current') await restore();
    const lookup = snapshotLookup(entry);
    if (lookup.status === 'current' || lookup.status === 'stale')
      return { value: lookup.value, validity: lookup.status };
    return undefined;
  }

  function reset() {
    entry = undefined;
    restoreAttemptedAt = undefined;
    inflight = undefined;
  }

  function expire(expiredAt: number) {
    if (entry) entry = { ...entry, validatedAt: Math.min(entry.validatedAt, expiredAt) };
    restoreAttemptedAt = undefined;
  }

  const catalog = { refresh, admissible, reset, expire };
  registerDurableCacheForTests(catalog);
  return catalog;
}
