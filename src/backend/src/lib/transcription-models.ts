import { getUpstreamConfig } from './upstream';

export const TRANSCRIPTION_CATALOG_TTL_MS = 15 * 60 * 1000;
const STALE_CATALOG_TTL_MS = 24 * 60 * 60 * 1000;

export type TranscriptionModel = {
  provider: 'openrouter';
  modelId: string;
  name: string;
  durationPricing?: TranscriptionDurationPricing;
};

export type TranscriptionDurationPricing = {
  unit: 'second';
  usdPerSecond: number;
  source: 'openrouter_transcription_catalog';
};

type OpenRouterModel = {
  id?: unknown;
  name?: unknown;
  pricing?: { prompt?: unknown; completion?: unknown };
  architecture?: {
    input_modalities?: unknown;
    output_modalities?: unknown;
  };
};

let cache: { models: TranscriptionModel[]; fetchedAt: number } | undefined;
let inflight: Promise<TranscriptionModel[]> | undefined;

function hasModality(value: unknown, modality: string) {
  return Array.isArray(value) && value.includes(modality);
}

function decimalRate(value: unknown): number | null {
  if (typeof value !== 'string' || !/^\d+(?:\.\d+)?$/.test(value)) return null;
  const rate = Number(value);
  return Number.isFinite(rate) && rate >= 0 ? rate : null;
}

function durationPricing(
  model: OpenRouterModel,
): TranscriptionDurationPricing | null {
  // Parakeet's provider documents USD/minute; the STT catalog exposes
  // USD/second. Other STT models may bill tokens and must not inherit this unit.
  if (model.id !== 'nvidia/parakeet-tdt-0.6b-v3') return null;
  const usdPerSecond = decimalRate(model.pricing?.prompt);
  const outputRate = decimalRate(model.pricing?.completion);
  if (usdPerSecond == null || outputRate !== 0) return null;
  return {
    unit: 'second',
    usdPerSecond,
    source: 'openrouter_transcription_catalog',
  };
}

function transcriptionModel(model: OpenRouterModel): TranscriptionModel | null {
  if (
    typeof model.id !== 'string' ||
    !hasModality(model.architecture?.input_modalities, 'audio') ||
    !hasModality(model.architecture?.output_modalities, 'transcription')
  )
    return null;
  const pricing = durationPricing(model);
  return {
    provider: 'openrouter',
    modelId: model.id,
    name: typeof model.name === 'string' ? model.name : model.id,
    ...(pricing ? { durationPricing: pricing } : {}),
  };
}

async function fetchOpenRouterTranscriptionModels() {
  const { baseUrl, apiKey } = getUpstreamConfig('openrouter');
  const response = await fetch(
    `${baseUrl}/models?input_modalities=audio&output_modalities=transcription`,
    {
      headers: { Authorization: `Bearer ${apiKey}` },
      signal: AbortSignal.timeout(8_000),
    },
  );
  if (!response.ok)
    throw new Error(
      `OpenRouter transcription catalog returned ${response.status}`,
    );
  const body = (await response.json()) as { data?: OpenRouterModel[] };
  if (!Array.isArray(body?.data))
    throw new Error('OpenRouter transcription catalog is malformed');
  return body.data
    .map(transcriptionModel)
    .filter((model): model is TranscriptionModel => Boolean(model))
    .sort((left, right) => left.name.localeCompare(right.name));
}

function refreshTranscriptionModels(): Promise<TranscriptionModel[]> {
  if (!inflight) {
    inflight = fetchOpenRouterTranscriptionModels()
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

export async function listTranscriptionModels(
  options: { fresh?: boolean } = {},
) {
  const now = Date.now();
  if (
    !options.fresh &&
    cache &&
    now - cache.fetchedAt < TRANSCRIPTION_CATALOG_TTL_MS
  )
    return cache.models;
  try {
    return await refreshTranscriptionModels();
  } catch (error) {
    if (!options.fresh && cache && now - cache.fetchedAt < STALE_CATALOG_TTL_MS)
      return cache.models;
    throw error;
  }
}

export async function lookupTranscriptionDurationPricing(
  provider: string,
  modelId: string,
  options: { fresh?: boolean } = {},
): Promise<TranscriptionDurationPricing | null> {
  if (provider !== 'openrouter') return null;
  const now = Date.now();
  const models =
    !options.fresh &&
    cache &&
    now - cache.fetchedAt < TRANSCRIPTION_CATALOG_TTL_MS
      ? cache.models
      : await refreshTranscriptionModels();
  return (
    models.find((model) => model.modelId === modelId)?.durationPricing ?? null
  );
}

export async function isCompatibleTranscriptionModel(
  provider: string,
  modelId: string,
  options: { fresh?: boolean } = {},
) {
  if (provider !== 'openrouter') return false;
  const models = await listTranscriptionModels(options);
  return models.some(
    (model) => model.provider === provider && model.modelId === modelId,
  );
}

export function resetTranscriptionModelCacheForTests() {
  cache = undefined;
  inflight = undefined;
}
