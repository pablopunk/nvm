import {
  BUNDLED_CATALOG_VERIFIED_AT,
  bundledEntryValidity,
  bundledModelsFor,
  ownBundledModel,
  setBundledClockForTests,
} from './pricing-bundled';
import {
  type AdmissibleSection,
  admissibleCatalogSection,
  type CatalogSdk,
  catalogSectionStatus,
  expireCatalogForTests,
  hasValidPricing,
  type ModelsDevModel,
  ownCatalogModel,
  peekAdmissibleCatalogSection,
  resetCatalogForTests,
  type ValidModelPricing,
} from './pricing-catalog';
import {
  currentOpenRouterPrice,
  expireOpenRouterPricesForTests,
  peekOpenRouterPrice,
  resetOpenRouterPricesForTests,
} from './pricing-openrouter';
import {
  expirePricingSnapshotStoreForTests,
  type PriceValidity,
  resetPricingSnapshotStoreForTests,
  SNAPSHOT_MAX_AGE_MS,
} from './pricing-snapshot-store';
import { staleAdmissionsHalted } from './pricing-overrun-cap';
import { pricingContractForProvider } from './provider-pricing-contract';

export type { CatalogSdk } from './pricing-catalog';

/**
 * A price validated outside the current window still admits for every tier, and its
 * reservations hold this multiple of the estimate. Settlement is unchanged, so the
 * margin only covers an upstream rate that moved while the snapshot aged. Free spend
 * is held at the same margin, so a stale free request cannot exceed its free hold.
 */
export const STALE_RESERVATION_MARGIN = 2;

/** The multiplier every reservation and estimate for this price holds over its computed cost. */
export function reservationMarginFor(cost: { stale?: boolean }): number {
  return cost.stale ? STALE_RESERVATION_MARGIN : 1;
}

export type ModelCost = {
  provider: string;
  modelId: string;
  inputUsdPerMtok: number;
  outputUsdPerMtok: number;
  cacheReadUsdPerMtok?: number;
  cacheWriteUsdPerMtok?: number;
  tiers?: Array<{
    thresholdTokens: number;
    inputUsdPerMtok?: number;
    outputUsdPerMtok?: number;
    cacheReadUsdPerMtok?: number;
    cacheWriteUsdPerMtok?: number;
  }>;
  /** Present only when the validated price is past the current window. */
  stale?: true;
};

/**
 * Why a model cannot be priced for admission. `unverified`: no catalog or price
 * has ever been validated. `stale`: the last validation is past the hard bound.
 * `unpriced`: the validated data lacks the model or a usable price.
 */
export type PricingUnavailableReason = 'unpriced' | 'stale' | 'unverified';
export type PricingSource = 'models.dev' | 'bundled' | 'openrouter_endpoints';
export type ModelCostResolution =
  | { ok: true; cost: ModelCost; source: PricingSource }
  | { ok: false; reason: PricingUnavailableReason };

function modelsDevKey(provider: string) {
  return pricingContractForProvider(provider).catalogKey;
}

type RuntimeModel = {
  model: ModelsDevModel;
  source: 'models.dev' | 'bundled';
  validity: PriceValidity;
};

/**
 * The one capability entry a provider's model is served from. Listing, descriptors,
 * prices, and image bounds all resolve through this, so they never disagree about
 * which entry describes a model. Precedence: a current catalog entry, then the exact
 * bundled entry while the bundle's verification date is inside the hard bound, then a
 * catalog entry still inside the hard bound.
 */
function pickRuntimeModel(
  catalogKey: string,
  modelId: string,
  section: AdmissibleSection | undefined,
): RuntimeModel | null {
  const catalogEntry = ownCatalogModel(section?.value, modelId);
  if (catalogEntry && section?.validity === 'current')
    return { model: catalogEntry, source: 'models.dev', validity: 'current' };
  const bundled = ownBundledModel(catalogKey, modelId);
  const bundledValidity = bundled ? bundledEntryValidity() : null;
  if (bundled && bundledValidity)
    return { model: bundled, source: 'bundled', validity: bundledValidity };
  if (catalogEntry && section)
    return { model: catalogEntry, source: 'models.dev', validity: section.validity };
  return null;
}

async function runtimeModel(
  provider: string,
  modelId: string,
): Promise<RuntimeModel | null> {
  const contract = pricingContractForProvider(provider);
  if (contract.priceDiscovery === 'none') return null;
  return pickRuntimeModel(
    contract.catalogKey,
    modelId,
    await admissibleCatalogSection(contract.catalogKey),
  );
}

function withValidity(cost: ModelCost, validity: PriceValidity): ModelCost {
  return validity === 'stale' ? { ...cost, stale: true } : cost;
}

function modelCostFromCatalog(
  provider: string,
  modelId: string,
  cost: ValidModelPricing,
  validity: PriceValidity,
): ModelCost {
  const tiers = (cost.tiers ?? []).flatMap((tier) =>
    tier.tier?.size == null
      ? []
      : [
          {
            thresholdTokens: tier.tier.size,
            ...(tier.input == null ? {} : { inputUsdPerMtok: tier.input }),
            ...(tier.output == null ? {} : { outputUsdPerMtok: tier.output }),
            ...(tier.cache_read == null
              ? {}
              : { cacheReadUsdPerMtok: tier.cache_read }),
            ...(tier.cache_write == null
              ? {}
              : { cacheWriteUsdPerMtok: tier.cache_write }),
          },
        ],
  );
  return withValidity(
    {
      provider,
      modelId,
      inputUsdPerMtok: cost.input,
      outputUsdPerMtok: cost.output,
      ...(cost.cache_read == null
        ? {}
        : { cacheReadUsdPerMtok: cost.cache_read }),
      ...(cost.cache_write == null
        ? {}
        : { cacheWriteUsdPerMtok: cost.cache_write }),
      ...(tiers.length ? { tiers } : {}),
    },
    validity,
  );
}

/**
 * An OpenRouter price admits only alongside the capability entry the request will
 * describe the model with, so a price is never charged to a model whose context and
 * input limits are unknown. The two halves age separately; the weaker one governs.
 */
async function resolveOpenRouterCost(
  modelId: string,
): Promise<ModelCostResolution> {
  const capability = await runtimeModel('openrouter', modelId);
  if (!capability)
    return { ok: false, reason: catalogUnavailableReason('openrouter') };
  const lookup = await currentOpenRouterPrice(modelId);
  if (lookup.status === 'unverified') return { ok: false, reason: 'unverified' };
  if (lookup.status === 'expired') return { ok: false, reason: 'stale' };
  if (!lookup.value) return { ok: false, reason: 'unpriced' };
  const validity: PriceValidity =
    capability.validity === 'stale' || lookup.status === 'stale'
      ? 'stale'
      : 'current';
  return {
    ok: true,
    cost: withValidity(lookup.value, validity),
    source: 'openrouter_endpoints',
  };
}

/** Names why a models.dev-priced model has no admissible price, from its section state. */
function catalogUnavailableReason(
  catalogKey: string,
): PricingUnavailableReason {
  const status = catalogSectionStatus(catalogKey);
  if (status === 'expired') return 'stale';
  if (status === 'unverified') return 'unverified';
  return 'unpriced';
}

/**
 * The single admission price for a model. A stale price is refused once the day's
 * absorbed settlement overrun reaches its cap, so stale holds cannot keep admitting
 * after the aggregate bound is spent. A current price is never refused by the cap.
 */
export async function resolveModelCost(
  provider: string,
  modelId: string,
): Promise<ModelCostResolution> {
  const resolution = await resolveSnapshotModelCost(provider, modelId);
  if (!resolution.ok || resolution.cost.stale !== true) return resolution;
  return (await staleAdmissionsHalted())
    ? { ok: false, reason: 'stale' }
    : resolution;
}

async function resolveSnapshotModelCost(
  provider: string,
  modelId: string,
): Promise<ModelCostResolution> {
  const contract = pricingContractForProvider(provider);
  if (contract.priceDiscovery === 'openrouter_endpoints')
    return resolveOpenRouterCost(modelId);
  if (contract.priceDiscovery === 'none')
    return { ok: false, reason: 'unverified' };
  const resolved = await runtimeModel(provider, modelId);
  if (!resolved)
    return { ok: false, reason: catalogUnavailableReason(contract.catalogKey) };
  const cost = resolved.model.cost;
  if (!hasValidPricing(cost)) return { ok: false, reason: 'unpriced' };
  return {
    ok: true,
    cost: modelCostFromCatalog(provider, modelId, cost, resolved.validity),
    source: resolved.source,
  };
}

export async function lookupModelCost(
  provider: string,
  modelId: string,
): Promise<ModelCost | null> {
  const resolution = await resolveModelCost(provider, modelId);
  return resolution.ok ? resolution.cost : null;
}

export type ModelPricing = Omit<
  ModelCost,
  'cacheReadUsdPerMtok' | 'cacheWriteUsdPerMtok' | 'tiers' | 'stale'
> & {
  cacheReadUsdPerMtok: number | null;
  cacheWriteUsdPerMtok: number | null;
  tiers: Array<{
    thresholdTokens: number;
    inputUsdPerMtok: number | null;
    outputUsdPerMtok: number | null;
    cacheReadUsdPerMtok: number | null;
    cacheWriteUsdPerMtok: number | null;
  }>;
  source: PricingSource;
  stale?: true;
};

export async function lookupModelPricing(
  provider: string,
  modelId: string,
): Promise<ModelPricing | null> {
  const resolution = await resolveModelCost(provider, modelId);
  if (!resolution.ok) return null;
  const cost = resolution.cost;
  return {
    provider,
    modelId,
    inputUsdPerMtok: cost.inputUsdPerMtok,
    outputUsdPerMtok: cost.outputUsdPerMtok,
    cacheReadUsdPerMtok: cost.cacheReadUsdPerMtok ?? null,
    cacheWriteUsdPerMtok: cost.cacheWriteUsdPerMtok ?? null,
    tiers: (cost.tiers ?? []).map((tier) => ({
      thresholdTokens: tier.thresholdTokens,
      inputUsdPerMtok: tier.inputUsdPerMtok ?? null,
      outputUsdPerMtok: tier.outputUsdPerMtok ?? null,
      cacheReadUsdPerMtok: tier.cacheReadUsdPerMtok ?? null,
      cacheWriteUsdPerMtok: tier.cacheWriteUsdPerMtok ?? null,
    })),
    source: resolution.source,
    ...(cost.stale ? { stale: true as const } : {}),
  };
}

export type ModelDescriptor = {
  id: string;
  name: string;
  contextWindow: number;
  maxInputTokens?: number;
  maxTokens: number;
  reasoning: boolean;
  input: string[];
};

const DEFAULT_DESCRIPTOR = {
  contextWindow: 200_000,
  maxTokens: 32_000,
  reasoning: false,
  input: ['text'],
};

export async function lookupModelDescriptor(
  provider: string,
  modelId: string,
): Promise<ModelDescriptor | null> {
  const resolved = await runtimeModel(provider, modelId);
  if (!resolved) return null;
  return descriptorFrom(modelId, resolved.model);
}

function descriptorFrom(modelId: string, m: ModelsDevModel): ModelDescriptor {
  return {
    id: modelId,
    name: m.name ?? modelId,
    contextWindow: m.limit?.context ?? DEFAULT_DESCRIPTOR.contextWindow,
    ...(m.limit?.input == null ? {} : { maxInputTokens: m.limit.input }),
    maxTokens: m.limit?.output ?? DEFAULT_DESCRIPTOR.maxTokens,
    reasoning: m.reasoning ?? DEFAULT_DESCRIPTOR.reasoning,
    input: m.modalities?.input ?? DEFAULT_DESCRIPTOR.input,
  };
}

/** Every model a provider admits to listing, each resolved exactly as a request resolves it. */
async function runtimeModelsFor(
  provider: string,
): Promise<Map<string, RuntimeModel>> {
  const catalogKey = modelsDevKey(provider);
  const section = await admissibleCatalogSection(catalogKey);
  const modelIds = new Set([
    ...Object.keys(bundledModelsFor(catalogKey)),
    ...Object.keys(section?.value.models ?? {}),
  ]);
  const models = new Map<string, RuntimeModel>();
  for (const modelId of modelIds) {
    const resolved = pickRuntimeModel(catalogKey, modelId, section);
    if (resolved) models.set(modelId, resolved);
  }
  return models;
}

export async function listModelsForProvider(
  provider: string,
): Promise<string[]> {
  return [...(await runtimeModelsFor(provider)).keys()].sort();
}

export type RouteKind = 'smart' | 'fast';

export function routeKindForSlot(slot: string): RouteKind {
  return slot.includes('fast') ? 'fast' : 'smart';
}

function supportsEveryTierFeature(model: ModelsDevModel): boolean {
  const input = model.modalities?.input ?? ['text'];
  const output = model.modalities?.output ?? ['text'];
  return (
    model.tool_call === true &&
    input.includes('text') &&
    input.includes('image') &&
    output.includes('text') &&
    hasValidPricing(model.cost) &&
    Number.isSafeInteger(model.limit?.context) &&
    (model.limit?.context ?? 0) > 0
  );
}

function fitsRouteKind(
  modelId: string,
  model: ModelsDevModel,
  kind: RouteKind,
): boolean {
  if (model.id !== modelId || !supportsEveryTierFeature(model)) return false;
  return kind === 'fast' || Boolean(model.reasoning);
}

export async function modelFitsRouteKind(
  provider: string,
  modelId: string,
  kind: RouteKind,
): Promise<boolean> {
  return (await listModelsForRouteKind(provider, kind)).includes(modelId);
}

export async function listModelsForRouteKind(
  provider: string,
  kind: RouteKind,
): Promise<string[]> {
  const models = await runtimeModelsFor(provider);
  return [...models.entries()]
    .filter(([modelId, resolved]) => fitsRouteKind(modelId, resolved.model, kind))
    .map(([modelId]) => modelId)
    .sort();
}

/** Synchronous capability entry for hot paths: the same precedence, over loaded sections only. */
function loadedCapabilityModel(
  provider: string,
  modelId: string,
): RuntimeModel | null {
  const catalogKey = modelsDevKey(provider);
  return pickRuntimeModel(
    catalogKey,
    modelId,
    peekAdmissibleCatalogSection(catalogKey),
  );
}

/** Synchronous price for image bounds: an endpoint price for OpenRouter, else the catalog. */
function loadedPriceFor(
  provider: string,
  modelId: string,
): Omit<ModelCost, 'provider' | 'modelId'> | undefined {
  const price = loadedFullPrice(provider, modelId);
  if (!price) return undefined;
  const { provider: _provider, modelId: _modelId, ...rates } = price;
  return rates;
}

function loadedFullPrice(
  provider: string,
  modelId: string,
): ModelCost | undefined {
  if (
    pricingContractForProvider(provider).priceDiscovery ===
    'openrouter_endpoints'
  ) {
    const endpoint = peekOpenRouterPrice(modelId);
    return endpoint ? withValidity(endpoint.value, endpoint.validity) : undefined;
  }
  const resolved = loadedCapabilityModel(provider, modelId);
  return resolved && hasValidPricing(resolved.model.cost)
    ? modelCostFromCatalog(
        provider,
        modelId,
        resolved.model.cost,
        resolved.validity,
      )
    : undefined;
}

export type ImageTokenLimits = {
  contextWindow: number;
  maxInputTokens: number;
  maxTokens: number;
  maximumCost: Omit<ModelCost, 'provider' | 'modelId'>;
};

/**
 * Best-effort image billing bounds from the loaded catalog: images are
 * reserved at the model's full input ceiling and current catalog prices.
 * Undefined when the catalog lacks image input, a context limit, or prices.
 */
export function loadedImageTokenLimits(
  provider: string,
  modelId: string,
): ImageTokenLimits | undefined {
  const model = loadedCapabilityModel(provider, modelId)?.model;
  const contextWindow = model?.limit?.context;
  if (!model?.modalities?.input?.includes('image') || !contextWindow)
    return undefined;
  const maximumCost = loadedPriceFor(provider, modelId);
  if (!maximumCost) return undefined;
  return {
    contextWindow,
    maxInputTokens: model.limit?.input ?? contextWindow,
    maxTokens: model.limit?.output ?? DEFAULT_DESCRIPTOR.maxTokens,
    maximumCost,
  };
}

export async function catalogSdkForModel(
  provider: string,
  modelId: string,
): Promise<CatalogSdk> {
  const catalogKey = modelsDevKey(provider);
  const section = await admissibleCatalogSection(catalogKey);
  const modelSdk = pickRuntimeModel(catalogKey, modelId, section)?.model.provider;
  return {
    npm: modelSdk?.npm ?? section?.value.npm,
    shape: modelSdk?.shape,
  };
}

/**
 * Clears every validated price and evaluates the bundle as of its own verification
 * date, so tests that use bundled entries stay deterministic after the bundle ages.
 */
export function resetPricingCacheForTests() {
  resetCatalogForTests();
  resetOpenRouterPricesForTests();
  resetPricingSnapshotStoreForTests();
  setBundledClockForTests(BUNDLED_CATALOG_VERIFIED_AT);
}

/** A fresh serverless instance: in-memory state gone, durable snapshots kept. */
export function simulateColdInstanceForTests() {
  resetCatalogForTests();
  resetOpenRouterPricesForTests();
}

/**
 * Ages every validated snapshot by `ageMs`. The default is past the hard bound, so
 * nothing admits; pass a smaller age to test the stale-but-admissible window.
 */
export function expirePricingCacheForTests(ageMs = SNAPSHOT_MAX_AGE_MS + 1) {
  const expiredAt = Date.now() - ageMs;
  expireCatalogForTests(expiredAt);
  expireOpenRouterPricesForTests(expiredAt);
  expirePricingSnapshotStoreForTests(expiredAt);
}
