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

const FALLBACK: Record<string, Record<string, { input: number; output: number }>> = {
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
  attachment?: boolean;
  modalities?: { input?: string[]; output?: string[] };
  limit?: { context?: number; output?: number };
  cost?: {
    input?: number;
    output?: number;
    cache_read?: number;
    cache_write?: number;
    tiers?: Array<{ input?: number; output?: number; cache_read?: number; cache_write?: number; tier?: { type?: string; size?: number } }>;
  };
};
type ModelsDevProvider = { models: Record<string, ModelsDevModel> };
type ModelsDevApi = Record<string, ModelsDevProvider>;

const BUNDLED_RUNTIME_MODELS: ModelsDevApi = {
  opencode: {
    models: {
      'gpt-5.6-luna': {
        id: 'gpt-5.6-luna',
        name: 'GPT-5.6 Luna',
        reasoning: true,
        modalities: { input: ['text', 'image', 'pdf'], output: ['text'] },
        limit: { context: 1_050_000, output: 128_000 },
        cost: { input: 0.2, output: 1.2 },
      },
      'deepseek-v4-flash': {
        id: 'deepseek-v4-flash',
        name: 'DeepSeek V4 Flash',
        reasoning: true,
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
        modalities: { input: ['text'], output: ['text'] },
        limit: { context: 1_310_720, output: 393_216 },
        cost: { input: 0.14, output: 0.28 },
      },
      'deepseek/deepseek-v4-pro-0813': {
        id: 'deepseek/deepseek-v4-pro-0813',
        name: 'DeepSeek V4 Pro 0813',
        reasoning: true,
        modalities: { input: ['text'], output: ['text'] },
        limit: { context: 1_048_576, output: 384_000 },
        cost: { input: 1.188, output: 3.564 },
      },
      'google/gemini-2.5-flash': {
        id: 'google/gemini-2.5-flash',
        name: 'Gemini 2.5 Flash',
        reasoning: true,
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

let cache: { data: ModelsDevApi; at: number; source: Exclude<PricingSource, 'bundled'> } | null = null;
let inflight: Promise<ModelsDevApi> | null = null;

async function fetchCatalog(): Promise<ModelsDevApi> {
  const cacheTtl = cache?.source === 'fallback' ? FALLBACK_CACHE_TTL_MS : CACHE_TTL_MS;
  if (cache && Date.now() - cache.at < cacheTtl) return cache.data;
  if (inflight) return inflight;
  inflight = (async () => {
    try {
      const res = await fetch(MODELS_DEV_URL, { signal: AbortSignal.timeout(8000) });
      if (!res.ok) throw new Error(`models.dev ${res.status}`);
      const data = (await res.json()) as ModelsDevApi;
      cache = { data, at: Date.now(), source: 'models.dev' };
      return data;
    } catch (err) {
      if (cache && cache.source === 'models.dev') {
        console.warn('[pricing] models.dev refresh failed, using stale catalog', err);
        cache = { ...cache, at: Date.now() };
        return cache.data;
      }
      console.warn('[pricing] models.dev fetch failed, using fallback', err);
      const data = Object.fromEntries(
        Object.entries(FALLBACK).map(([id, models]) => [
          id,
          {
            models: Object.fromEntries(
              Object.entries(models).map(([mid, cost]) => [mid, { id: mid, cost }]),
            ),
          },
        ]),
      );
      cache = { data, at: Date.now(), source: 'fallback' };
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

async function runtimeModel(provider: string, modelId: string): Promise<{ model: ModelsDevModel; source: PricingSource } | null> {
  const key = modelsDevKey(provider);
  const currentCache = cache;
  const cached = currentCache?.data[key]?.models?.[modelId];
  if (cached) {
    if (Date.now() - currentCache.at >= CACHE_TTL_MS) void fetchCatalog();
    return { model: cached, source: currentCache.source };
  }
  const bundled = bundledRuntimeModel(provider, modelId);
  if (bundled) {
    void fetchCatalog();
    return { model: bundled, source: 'bundled' };
  }
  const model = (await fetchCatalog())[key]?.models?.[modelId];
  return model && cache ? { model, source: cache.source } : null;
}

export async function lookupModelCost(provider: string, modelId: string): Promise<ModelCost | null> {
  const cost = (await runtimeModel(provider, modelId))?.model.cost;
  if (!cost || cost.input == null || cost.output == null) return null;
  const tiers = (cost.tiers ?? []).flatMap((tier) => tier.tier?.size == null ? [] : [{
    thresholdTokens: tier.tier.size,
    ...(tier.input == null ? {} : { inputUsdPerMtok: tier.input }),
    ...(tier.output == null ? {} : { outputUsdPerMtok: tier.output }),
    ...(tier.cache_read == null ? {} : { cacheReadUsdPerMtok: tier.cache_read }),
    ...(tier.cache_write == null ? {} : { cacheWriteUsdPerMtok: tier.cache_write }),
  }]);
  return {
    provider,
    modelId,
    inputUsdPerMtok: cost.input,
    outputUsdPerMtok: cost.output,
    ...(cost.cache_read == null ? {} : { cacheReadUsdPerMtok: cost.cache_read }),
    ...(cost.cache_write == null ? {} : { cacheWriteUsdPerMtok: cost.cache_write }),
    ...(tiers.length ? { tiers } : {}),
  };
}

export type ModelPricing = Omit<ModelCost, 'cacheReadUsdPerMtok' | 'cacheWriteUsdPerMtok' | 'tiers'> & {
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

export async function lookupModelPricing(provider: string, modelId: string): Promise<ModelPricing | null> {
  const resolved = await runtimeModel(provider, modelId);
  const cost = resolved?.model.cost;
  if (!resolved || !cost || cost.input == null || cost.output == null) return null;
  return {
    provider,
    modelId,
    inputUsdPerMtok: cost.input,
    outputUsdPerMtok: cost.output,
    cacheReadUsdPerMtok: cost.cache_read ?? null,
    cacheWriteUsdPerMtok: cost.cache_write ?? null,
    tiers: (cost.tiers ?? []).flatMap((tier) => tier.tier?.size == null ? [] : [{
      thresholdTokens: tier.tier.size,
      inputUsdPerMtok: tier.input ?? null,
      outputUsdPerMtok: tier.output ?? null,
      cacheReadUsdPerMtok: tier.cache_read ?? null,
      cacheWriteUsdPerMtok: tier.cache_write ?? null,
    }]),
    source: resolved.source,
  };
}

export type ModelDescriptor = {
  id: string;
  name: string;
  contextWindow: number;
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

export async function lookupModelDescriptor(provider: string, modelId: string): Promise<ModelDescriptor | null> {
  const resolved = await runtimeModel(provider, modelId);
  if (!resolved) return null;
  const m = resolved.model;
  return {
    id: modelId,
    name: m.name ?? modelId,
    contextWindow: m.limit?.context ?? DEFAULT_DESCRIPTOR.contextWindow,
    maxTokens: m.limit?.output ?? DEFAULT_DESCRIPTOR.maxTokens,
    reasoning: m.reasoning ?? DEFAULT_DESCRIPTOR.reasoning,
    input: m.modalities?.input ?? DEFAULT_DESCRIPTOR.input,
  };
}

export async function listModelsForProvider(provider: string): Promise<string[]> {
  const catalog = await fetchCatalog();
  const key = modelsDevKey(provider);
  const models = catalog[key]?.models ?? {};
  const bundled = BUNDLED_RUNTIME_MODELS[key]?.models ?? {};
  return [...new Set([...Object.keys(models), ...Object.keys(bundled)])].sort();
}

export function resetPricingCacheForTests() {
  cache = null;
  inflight = null;
}
