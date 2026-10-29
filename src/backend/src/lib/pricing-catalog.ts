import { declaredCatalogKeys } from './provider-pricing-contract';
import {
  loadSnapshotRecord,
  type PriceValidity,
  saveSnapshotRecord,
  type SnapshotLookup,
  snapshotLookup,
  type ValidatedSnapshot,
} from './pricing-snapshot-store';

const MODELS_DEV_URL = 'https://models.dev/api.json';
const MODELS_DEV_TIMEOUT_MS = 8000;

/** The SDK models.dev declares for a provider or model; it determines the wire API. */
export type CatalogSdk = { npm?: string; shape?: string };

export type ModelsDevModel = {
  id: string;
  name?: string;
  reasoning?: boolean;
  tool_call?: boolean;
  attachment?: boolean;
  provider?: CatalogSdk;
  modalities?: { input?: string[]; output?: string[] };
  limit?: { context?: number; input?: number; output?: number };
  cost?: {
    input?: number;
    output?: number;
    cache_read?: number;
    cache_write?: number;
    tiers?: Array<{
      input?: number;
      output?: number;
      cache_read?: number;
      cache_write?: number;
      tier?: { type?: string; size?: number };
    }>;
  };
};

export type ModelsDevProvider = {
  npm?: string;
  models: Record<string, ModelsDevModel>;
};
export type ModelsDevApi = Record<string, ModelsDevProvider>;

export type ValidModelPricing = NonNullable<ModelsDevModel['cost']> & {
  input: number;
  output: number;
};

export function hasValidPricing(
  cost: ModelsDevModel['cost'],
): cost is ValidModelPricing {
  if (!cost || cost.input == null || cost.output == null) return false;
  const validRate = (rate: number | undefined) =>
    rate === undefined || (Number.isFinite(rate) && rate >= 0);
  if (
    ![cost.input, cost.output, cost.cache_read, cost.cache_write].every(
      validRate,
    )
  )
    return false;
  return (cost.tiers ?? []).every((tier) => {
    if (tier.tier?.size == null) return true;
    return (
      Number.isFinite(tier.tier.size) &&
      tier.tier.size >= 0 &&
      [tier.input, tier.output, tier.cache_read, tier.cache_write].every(
        validRate,
      )
    );
  });
}

/** Own-property lookup, so a model id such as `constructor` never reads the prototype. */
export function ownCatalogModel(
  provider: ModelsDevProvider | undefined,
  modelId: string,
): ModelsDevModel | undefined {
  return provider && Object.hasOwn(provider.models, modelId)
    ? provider.models[modelId]
    : undefined;
}

const sections = new Map<string, ValidatedSnapshot<ModelsDevProvider>>();
/** Re-read at most this often per section, so a missing row does not query on every request. */
const RESTORE_RETRY_MS = 30 * 1000;
const restoreAttemptedAt = new Map<string, number>();
let refreshInflight: Promise<void> | null = null;

/** A section a request may price and describe a model from, with its validity. */
export type AdmissibleSection = { value: ModelsDevProvider; validity: PriceValidity };

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** A catalog is accepted only when every provider section carries a models map. */
function isValidModelsDevCatalog(value: unknown): value is ModelsDevApi {
  return (
    isPlainRecord(value) &&
    Object.values(value).every(
      (provider) => isPlainRecord(provider) && isPlainRecord(provider.models),
    )
  );
}

function isModelsDevProvider(value: unknown): value is ModelsDevProvider {
  return (
    isPlainRecord(value) &&
    isPlainRecord(value.models) &&
    Object.values(value.models).every(isPlainRecord)
  );
}

function modelCount(provider: ModelsDevProvider | undefined): number {
  return Object.keys(provider?.models ?? {}).length;
}

function snapshotKey(catalogKey: string) {
  return `models_dev:${catalogKey}`;
}

async function fetchModelsDevCatalog(): Promise<ModelsDevApi> {
  const response = await fetch(MODELS_DEV_URL, {
    signal: AbortSignal.timeout(MODELS_DEV_TIMEOUT_MS),
  });
  if (!response.ok) throw new Error(`models.dev ${response.status}`);
  const parsed: unknown = await response.json();
  if (!isValidModelsDevCatalog(parsed))
    throw new Error('models.dev catalog failed shape validation');
  return parsed;
}

/**
 * Publishes one validated fetch. A section that comes back empty or missing does
 * not replace a non-empty section, which keeps its older timestamp and expires on
 * its own bound. An empty section with nothing before it is still a validated
 * answer: the provider lists no such model, so the model is unpriced.
 */
async function publishModelsDevCatalog() {
  const catalog = await fetchModelsDevCatalog();
  const validatedAt = Date.now();
  const results = await Promise.all(
    declaredCatalogKeys().map(async (catalogKey) => {
      const provider: ModelsDevProvider = catalog[catalogKey] ?? { models: {} };
      if (
        modelCount(provider) === 0 &&
        modelCount(sections.get(catalogKey)?.value) > 0
      )
        return true;
      sections.set(catalogKey, { value: provider, validatedAt });
      return saveSnapshotRecord({
        key: snapshotKey(catalogKey),
        validatedAt,
        payload: provider,
      });
    }),
  );
  if (!results.every(Boolean))
    throw new Error('models.dev snapshot was not stored durably');
}

/**
 * Fetches models.dev and publishes every declared section. Only the scheduled
 * refresh and admin-selected routes call this, never the inference request path.
 * Concurrent callers share one fetch; a failure rejects for every caller.
 */
export function refreshModelsDevCatalog(): Promise<void> {
  if (refreshInflight) return refreshInflight;
  refreshInflight = publishModelsDevCatalog().finally(() => {
    refreshInflight = null;
  });
  return refreshInflight;
}

/** Restores a durable section unless it is older than what this instance already holds. */
async function restoreCatalogSection(catalogKey: string) {
  const now = Date.now();
  const attemptedAt = restoreAttemptedAt.get(catalogKey);
  if (attemptedAt !== undefined && now - attemptedAt < RESTORE_RETRY_MS) return;
  restoreAttemptedAt.set(catalogKey, now);
  const record = await loadSnapshotRecord(snapshotKey(catalogKey));
  if (!record || !isModelsDevProvider(record.payload)) return;
  const current = sections.get(catalogKey);
  if (!current || current.validatedAt < record.validatedAt)
    sections.set(catalogKey, {
      value: record.payload,
      validatedAt: record.validatedAt,
    });
}

/**
 * The admissible section for a catalog key. A section that is not current is
 * restored from the durable snapshot first; the network is never reached here.
 */
export async function admissibleCatalogSection(
  catalogKey: string,
): Promise<AdmissibleSection | undefined> {
  if (!catalogKey) return undefined;
  if (snapshotLookup(sections.get(catalogKey)).status !== 'current')
    await restoreCatalogSection(catalogKey);
  return peekAdmissibleCatalogSection(catalogKey);
}

/** Synchronous view for hot paths: the section as last loaded, when it may admit. */
export function peekAdmissibleCatalogSection(
  catalogKey: string,
): AdmissibleSection | undefined {
  const lookup = snapshotLookup(sections.get(catalogKey));
  if (lookup.status === 'current')
    return { value: lookup.value, validity: 'current' };
  if (lookup.status === 'stale') return { value: lookup.value, validity: 'stale' };
  return undefined;
}

/** Whether the section is current, stale, expired, or never validated. */
export function catalogSectionStatus(
  catalogKey: string,
): SnapshotLookup<ModelsDevProvider>['status'] {
  return snapshotLookup(sections.get(catalogKey)).status;
}

export function expireCatalogForTests(expiredAt: number) {
  for (const [catalogKey, entry] of sections)
    sections.set(catalogKey, {
      ...entry,
      validatedAt: Math.min(entry.validatedAt, expiredAt),
    });
  restoreAttemptedAt.clear();
}

export function resetCatalogForTests() {
  sections.clear();
  restoreAttemptedAt.clear();
  refreshInflight = null;
}
