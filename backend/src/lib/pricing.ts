export type ModelCost = {
  provider: string;
  modelId: string;
  inputUsdPerMtok: number;
  outputUsdPerMtok: number;
};

const MODELS_DEV_URL = 'https://models.dev/api.json';
const CACHE_TTL_MS = 60 * 60 * 1000;

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
};

type ModelsDevModel = {
  id: string;
  name?: string;
  reasoning?: boolean;
  attachment?: boolean;
  modalities?: { input?: string[]; output?: string[] };
  limit?: { context?: number; output?: number };
  cost?: { input?: number; output?: number };
};
type ModelsDevProvider = { models: Record<string, ModelsDevModel> };
type ModelsDevApi = Record<string, ModelsDevProvider>;

let cache: { data: ModelsDevApi; at: number } | null = null;
let inflight: Promise<ModelsDevApi> | null = null;

async function fetchCatalog(): Promise<ModelsDevApi> {
  if (cache && Date.now() - cache.at < CACHE_TTL_MS) return cache.data;
  if (inflight) return inflight;
  inflight = (async () => {
    try {
      const res = await fetch(MODELS_DEV_URL, { signal: AbortSignal.timeout(8000) });
      if (!res.ok) throw new Error(`models.dev ${res.status}`);
      const data = (await res.json()) as ModelsDevApi;
      cache = { data, at: Date.now() };
      return data;
    } catch (err) {
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
      cache = { data, at: Date.now() };
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

export async function lookupModelCost(provider: string, modelId: string): Promise<ModelCost | null> {
  const catalog = await fetchCatalog();
  const key = modelsDevKey(provider);
  const cost = catalog[key]?.models?.[modelId]?.cost;
  if (!cost || cost.input == null || cost.output == null) return null;
  return { provider, modelId, inputUsdPerMtok: cost.input, outputUsdPerMtok: cost.output };
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
  const catalog = await fetchCatalog();
  const m = catalog[modelsDevKey(provider)]?.models?.[modelId];
  if (!m) return null;
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
  return Object.keys(models).sort();
}

export function resetPricingCacheForTests() {
  cache = null;
  inflight = null;
}
