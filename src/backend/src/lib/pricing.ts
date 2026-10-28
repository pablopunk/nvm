import { requiresResponsesForTools } from './model-capabilities';

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
};

const MODELS_DEV_URL = 'https://models.dev/api.json';
const CACHE_TTL_MS = 60 * 60 * 1000;
const FALLBACK_CACHE_TTL_MS = 60 * 1000;

const PROVIDER_TO_MODELS_DEV: Record<string, string> = {
  opencode_zen: 'opencode',
  opencode: 'opencode',
  openrouter: 'openrouter',
  anthropic: 'anthropic',
  google: 'google',
  openai: 'openai',
};

const FALLBACK: Record<
  string,
  Record<string, { input: number; output: number }>
> = {
  opencode: {
    'claude-haiku-4-5': { input: 1, output: 5 },
    'claude-sonnet-4-6': { input: 3, output: 15 },
    'gemini-3.5-flash': { input: 1.5, output: 9 },
    'gemini-3-flash': { input: 0.3, output: 2.5 },
    'gemini-3.1-pro': { input: 12.5, output: 100 },
  },
  openrouter: {
    'google/gemini-2.5-flash': { input: 0.3, output: 2.5 },
  },
};

type ModelsDevModel = {
  id: string;
  name?: string;
  reasoning?: boolean;
  tool_call?: boolean;
  attachment?: boolean;
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
type ModelsDevProvider = { models: Record<string, ModelsDevModel> };
type ModelsDevApi = Record<string, ModelsDevProvider>;

type ValidModelPricing = NonNullable<ModelsDevModel['cost']> & {
  input: number;
  output: number;
};

function hasValidPricing(
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

const BUNDLED_RUNTIME_MODELS: ModelsDevApi = {
  opencode: {
    models: {
      'gpt-5.6-luna': {
        id: 'gpt-5.6-luna',
        name: 'GPT-5.6 Luna',
        reasoning: true,
        tool_call: true,
        modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
        limit: { context: 1_050_000, output: 128_000 },
        cost: { input: 0.2, output: 1.2 },
      },
      'deepseek-v4-flash': {
        id: 'deepseek-v4-flash',
        name: 'DeepSeek V4 Flash',
        reasoning: true,
        tool_call: true,
        modalities: { input: ['text'], output: ['text'] },
        limit: { context: 1_000_000, output: 384_000 },
        cost: { input: 0.14, output: 0.28 },
      },
    },
  },
  openrouter: {
    models: {
      'deepseek/deepseek-v4-flash-0731': {
        id: 'deepseek/deepseek-v4-flash-0731',
        name: 'DeepSeek V4 Flash 0731',
        reasoning: true,
        tool_call: true,
        modalities: { input: ['text'], output: ['text'] },
        limit: { context: 1_310_720, output: 393_216 },
        cost: { input: 0.14, output: 0.28 },
      },
      'deepseek/deepseek-v4-pro-0813': {
        id: 'deepseek/deepseek-v4-pro-0813',
        name: 'DeepSeek V4 Pro 0813',
        reasoning: true,
        tool_call: true,
        modalities: { input: ['text'], output: ['text'] },
        limit: { context: 1_048_576, output: 384_000 },
        cost: { input: 1.188, output: 3.564 },
      },
      'google/gemini-2.5-flash': {
        id: 'google/gemini-2.5-flash',
        name: 'Gemini 2.5 Flash',
        reasoning: true,
        tool_call: true,
        attachment: true,
        modalities: {
          input: ['text', 'image', 'audio', 'video', 'pdf'],
          output: ['text'],
        },
        limit: { context: 1_048_576, output: 65_535 },
        cost: { input: 0.3, output: 2.5 },
      },
    },
  },
};

type PricingSource = 'models.dev' | 'bundled' | 'fallback';

let cache: {
  data: ModelsDevApi;
  at: number;
  lastSuccessfulAt: number | null;
  source: Exclude<PricingSource, 'bundled'>;
} | null = null;
let inflight: Promise<ModelsDevApi> | null = null;

async function fetchCatalog(): Promise<ModelsDevApi> {
  const cacheTtl =
    cache?.source === 'fallback' ? FALLBACK_CACHE_TTL_MS : CACHE_TTL_MS;
  if (cache && Date.now() - cache.at < cacheTtl) return cache.data;
  if (inflight) return inflight;
  inflight = (async () => {
    try {
      const res = await fetch(MODELS_DEV_URL, {
        signal: AbortSignal.timeout(8000),
      });
      if (!res.ok) throw new Error(`models.dev ${res.status}`);
      const data = (await res.json()) as ModelsDevApi;
      const fetchedAt = Date.now();
      cache = {
        data,
        at: fetchedAt,
        lastSuccessfulAt: fetchedAt,
        source: 'models.dev',
      };
      return data;
    } catch (err) {
      if (cache && cache.source === 'models.dev') {
        console.warn(
          '[pricing] models.dev refresh failed, using stale catalog',
          err,
        );
        return cache.data;
      }
      console.warn('[pricing] models.dev fetch failed, using fallback', err);
      const data = Object.fromEntries(
        Object.entries(FALLBACK).map(([id, models]) => [
          id,
          {
            models: Object.fromEntries(
              Object.entries(models).map(([mid, cost]) => [
                mid,
                { id: mid, cost },
              ]),
            ),
          },
        ]),
      );
      cache = {
        data,
        at: Date.now(),
        lastSuccessfulAt: null,
        source: 'fallback',
      };
      return data;
    } finally {
      inflight = null;
    }
  })();
  return inflight;
}

function modelsDevKey(provider: string) {
  return PROVIDER_TO_MODELS_DEV[provider] ?? provider;
}

function bundledRuntimeModel(provider: string, modelId: string) {
  return BUNDLED_RUNTIME_MODELS[modelsDevKey(provider)]?.models?.[modelId];
}

async function runtimeModel(
  provider: string,
  modelId: string,
): Promise<{ model: ModelsDevModel; source: PricingSource } | null> {
  const key = modelsDevKey(provider);
  const currentCache = cache;
  const cached = currentCache?.data[key]?.models?.[modelId];
  if (
    cached &&
    currentCache?.lastSuccessfulAt != null &&
    Date.now() - currentCache.lastSuccessfulAt < CACHE_TTL_MS
  ) {
    return { model: cached, source: currentCache.source };
  }
  const bundled = bundledRuntimeModel(provider, modelId);
  if (bundled) {
    void fetchCatalog();
    return { model: bundled, source: 'bundled' };
  }
  const catalog = await fetchCatalog();
  const model = catalog[key]?.models?.[modelId];
  if (
    !model ||
    !cache ||
    cache.source !== 'models.dev' ||
    cache.lastSuccessfulAt == null ||
    Date.now() - cache.lastSuccessfulAt >= CACHE_TTL_MS
  )
    return null;
  return { model, source: cache.source };
}

function modelCostFromCatalog(
  provider: string,
  modelId: string,
  cost: ValidModelPricing,
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
  return {
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
  };
}

export async function lookupModelCost(
  provider: string,
  modelId: string,
): Promise<ModelCost | null> {
  const resolved = await runtimeModel(provider, modelId);
  if (resolved?.source === 'fallback') return null;
  const cost = resolved?.model.cost;
  if (!hasValidPricing(cost)) return null;
  return modelCostFromCatalog(provider, modelId, cost);
}

export type ModelPricing = Omit<
  ModelCost,
  'cacheReadUsdPerMtok' | 'cacheWriteUsdPerMtok' | 'tiers'
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
};

export async function lookupModelPricing(
  provider: string,
  modelId: string,
): Promise<ModelPricing | null> {
  const resolved = await runtimeModel(provider, modelId);
  if (resolved?.source === 'fallback') return null;
  const cost = resolved?.model.cost;
  if (!resolved || !hasValidPricing(cost)) return null;
  return {
    provider,
    modelId,
    inputUsdPerMtok: cost.input,
    outputUsdPerMtok: cost.output,
    cacheReadUsdPerMtok: cost.cache_read ?? null,
    cacheWriteUsdPerMtok: cost.cache_write ?? null,
    tiers: (cost.tiers ?? []).flatMap((tier) =>
      tier.tier?.size == null
        ? []
        : [
            {
              thresholdTokens: tier.tier.size,
              inputUsdPerMtok: tier.input ?? null,
              outputUsdPerMtok: tier.output ?? null,
              cacheReadUsdPerMtok: tier.cache_read ?? null,
              cacheWriteUsdPerMtok: tier.cache_write ?? null,
            },
          ],
    ),
    source: resolved.source,
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
  const m = resolved.model;
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

export async function listModelsForProvider(
  provider: string,
): Promise<string[]> {
  const catalog = await fetchCatalog();
  const key = modelsDevKey(provider);
  const models = catalog[key]?.models ?? {};
  const bundled = BUNDLED_RUNTIME_MODELS[key]?.models ?? {};
  return [...new Set([...Object.keys(models), ...Object.keys(bundled)])].sort();
}

export type RouteKind = 'smart' | 'fast';

export function routeKindForSlot(slot: string): RouteKind {
  return slot.includes('fast') ? 'fast' : 'smart';
}

/**
 * Both routes require tool calls and token prices. Smart routes also require
 * image input and reasoning compatible with tools; fast routes disable thinking.
 */
function fitsRouteKind(model: ModelsDevModel, kind: RouteKind): boolean {
  if (!hasValidPricing(model.cost)) return false;
  if (model.tool_call !== true) return false;
  if (requiresResponsesForTools(model.id)) return false;
  const input = model.modalities?.input ?? ['text'];
  const output = model.modalities?.output ?? ['text'];
  if (!input.includes('text') || !output.includes('text')) return false;
  return (
    kind === 'fast' ||
    (Boolean(model.reasoning) && input.includes('image'))
  );
}

export async function modelFitsRouteKind(
  provider: string,
  modelId: string,
  kind: RouteKind,
): Promise<boolean> {
  const resolved = await runtimeModel(provider, modelId);
  return Boolean(
    resolved && resolved.source !== 'fallback' && fitsRouteKind(resolved.model, kind),
  );
}

export async function listModelsForRouteKind(
  provider: string,
  kind: RouteKind,
): Promise<string[]> {
  await fetchCatalog();
  const key = modelsDevKey(provider);
  const models = {
    ...BUNDLED_RUNTIME_MODELS[key]?.models,
    ...(cache?.source === 'models.dev' ? cache.data[key]?.models : {}),
  };
  return Object.entries(models)
    .filter(([, model]) => fitsRouteKind(model, kind))
    .map(([modelId]) => modelId)
    .sort();
}

/** Synchronous view of the already-loaded catalog for the proxy hot path. */
function loadedCatalogModel(
  provider: string,
  modelId: string,
): ModelsDevModel | undefined {
  const fromCatalog =
    cache?.source === 'models.dev'
      ? cache.data[modelsDevKey(provider)]?.models?.[modelId]
      : undefined;
  return fromCatalog ?? bundledRuntimeModel(provider, modelId);
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
  const model = loadedCatalogModel(provider, modelId);
  const contextWindow = model?.limit?.context;
  if (
    !model?.modalities?.input?.includes('image') ||
    !contextWindow ||
    !hasValidPricing(model.cost)
  )
    return undefined;
  const { provider: _provider, modelId: _modelId, ...maximumCost } =
    modelCostFromCatalog(provider, modelId, model.cost);
  return {
    contextWindow,
    maxInputTokens: model.limit?.input ?? contextWindow,
    maxTokens: model.limit?.output ?? DEFAULT_DESCRIPTOR.maxTokens,
    maximumCost,
  };
}

export function resetPricingCacheForTests() {
  cache = null;
  inflight = null;
}

export function expirePricingCacheForTests() {
  if (!cache) return;
  cache.at = Date.now() - CACHE_TTL_MS - 1;
  cache.lastSuccessfulAt = Date.now() - CACHE_TTL_MS - 1;
}
