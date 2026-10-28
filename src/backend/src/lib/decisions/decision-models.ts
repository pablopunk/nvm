import type { ModelCost } from '../cost';
import { getUpstreamBaseUrl } from '../upstream';

export const DECISION_CATALOG_TTL_MS = 15 * 60 * 1000;
const STALE_CATALOG_TTL_MS = 24 * 60 * 60 * 1000;

export type DecisionModel = {
  provider: 'openrouter';
  modelId: string;
  name: string;
  acceptsImages: boolean;
  cost?: ModelCost | null;
  contextLength?: number;
  imageTokenPriced?: boolean;
};

type OpenRouterModel = {
  id?: unknown;
  name?: unknown;
  pricing?: {
    prompt?: unknown;
    completion?: unknown;
    image?: unknown;
    request?: unknown;
  };
  context_length?: unknown;
  architecture?: {
    input_modalities?: unknown;
    output_modalities?: unknown;
  };
};

let cache: { models: DecisionModel[]; fetchedAt: number } | undefined;
let inflight: Promise<DecisionModel[]> | undefined;

function hasModality(value: unknown, modality: string): boolean {
  return Array.isArray(value) && value.includes(modality);
}

export function decisionModelFromCatalog(
  model: OpenRouterModel,
): DecisionModel | null {
  if (
    typeof model.id !== 'string' ||
    !hasModality(model.architecture?.output_modalities, 'decisions')
  )
    return null;
  return {
    provider: 'openrouter',
    modelId: model.id,
    name: typeof model.name === 'string' ? model.name : model.id,
    acceptsImages: hasModality(model.architecture?.input_modalities, 'image'),
    cost: decisionCatalogCost(model),
    ...(typeof model.context_length === 'number' &&
    Number.isSafeInteger(model.context_length) &&
    model.context_length > 0
      ? { contextLength: model.context_length }
      : {}),
    imageTokenPriced:
      (isZeroPrice(model.pricing?.image) ||
        (model.id === 'typesafe/jev-1.14' &&
          model.pricing?.image === undefined)) &&
      (model.pricing?.request === undefined ||
        isZeroPrice(model.pricing.request)),
  };
}

function isZeroPrice(value: unknown): boolean {
  return (
    typeof value === 'string' && value.trim() !== '' && Number(value) === 0
  );
}

/** New routes must support images and have the billing bounds used by execution. */
export function isSelectableDecisionModel(model: DecisionModel): boolean {
  return Boolean(
    model.acceptsImages &&
      model.contextLength &&
      model.imageTokenPriced &&
      model.cost &&
      model.cost.outputUsdPerMtok === 0,
  );
}

function decisionCatalogCost(model: OpenRouterModel): ModelCost | null {
  const input = model.pricing?.prompt;
  const output = model.pricing?.completion;
  if (
    typeof model.id !== 'string' ||
    typeof input !== 'string' ||
    typeof output !== 'string' ||
    !input.trim() ||
    !output.trim()
  )
    return null;
  const inputRate = Number(input);
  const outputRate = Number(output);
  if (
    !Number.isFinite(inputRate) ||
    inputRate < 0 ||
    !Number.isFinite(outputRate) ||
    outputRate < 0
  )
    return null;
  return {
    provider: 'openrouter',
    modelId: model.id,
    inputUsdPerMtok: inputRate * 1_000_000,
    outputUsdPerMtok: outputRate * 1_000_000,
  };
}

export async function lookupDecisionModelCost(
  provider: string,
  modelId: string,
): Promise<ModelCost | null> {
  return (await findEligibleDecisionModel(provider, modelId))?.cost ?? null;
}

async function fetchOpenRouterDecisionModels(): Promise<DecisionModel[]> {
  const baseUrl = getUpstreamBaseUrl('openrouter');
  const response = await fetch(
    `${baseUrl}/models?output_modalities=decisions`,
    {
      signal: AbortSignal.timeout(8000),
    },
  );
  if (!response.ok)
    throw new Error(`OpenRouter decision catalog returned ${response.status}`);
  const body = (await response.json()) as { data?: OpenRouterModel[] };
  if (!Array.isArray(body?.data))
    throw new Error('OpenRouter decision catalog is malformed');
  return body.data
    .map(decisionModelFromCatalog)
    .filter((model): model is DecisionModel => Boolean(model))
    .sort((left, right) => left.name.localeCompare(right.name));
}

function refreshDecisionModels(): Promise<DecisionModel[]> {
  if (!inflight) {
    inflight = fetchOpenRouterDecisionModels()
      .then((models) => {
        cache = { models, fetchedAt: Date.now() };
        return models;
      })
      .finally(() => {
        inflight = undefined;
      });
  }
  return inflight;
}

export async function listDecisionModels(
  options: { fresh?: boolean } = {},
): Promise<DecisionModel[]> {
  const now = Date.now();
  if (
    !options.fresh &&
    cache &&
    now - cache.fetchedAt < DECISION_CATALOG_TTL_MS
  )
    return cache.models;
  try {
    return await refreshDecisionModels();
  } catch (error) {
    if (!options.fresh && cache && now - cache.fetchedAt < STALE_CATALOG_TTL_MS)
      return cache.models;
    throw error;
  }
}

export async function findEligibleDecisionModel(
  provider: string,
  modelId: string,
  options: { fresh?: boolean } = {},
): Promise<DecisionModel | null> {
  if (provider !== 'openrouter') return null;
  const models = await listDecisionModels(options);
  return (
    models.find(
      (model) => model.provider === provider && model.modelId === modelId,
    ) ?? null
  );
}

export function resetDecisionModelCacheForTests() {
  cache = undefined;
  inflight = undefined;
}
