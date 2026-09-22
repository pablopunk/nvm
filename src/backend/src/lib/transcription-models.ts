import { getUpstreamConfig } from './upstream';

const CATALOG_TTL_MS = 15 * 60 * 1000;
const STALE_CATALOG_TTL_MS = 24 * 60 * 60 * 1000;

export type TranscriptionModel = {
  provider: 'openrouter';
  modelId: string;
  name: string;
};

type OpenRouterModel = {
  id?: unknown;
  name?: unknown;
  architecture?: {
    input_modalities?: unknown;
    output_modalities?: unknown;
  };
};

let cache:
  | { models: TranscriptionModel[]; fetchedAt: number }
  | undefined;
let inflight: Promise<TranscriptionModel[]> | undefined;

function hasModality(value: unknown, modality: string) {
  return Array.isArray(value) && value.includes(modality);
}

function transcriptionModel(model: OpenRouterModel): TranscriptionModel | null {
  if (
    typeof model.id !== 'string' ||
    !hasModality(model.architecture?.input_modalities, 'audio') ||
    !hasModality(model.architecture?.output_modalities, 'transcription')
  )
    return null;
  return {
    provider: 'openrouter',
    modelId: model.id,
    name: typeof model.name === 'string' ? model.name : model.id,
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
    throw new Error(`OpenRouter transcription catalog returned ${response.status}`);
  const body = (await response.json()) as { data?: OpenRouterModel[] };
  return (body.data ?? [])
    .map(transcriptionModel)
    .filter((model): model is TranscriptionModel => Boolean(model))
    .sort((left, right) => left.name.localeCompare(right.name));
}

export async function listTranscriptionModels(options: { fresh?: boolean } = {}) {
  const now = Date.now();
  if (
    !options.fresh &&
    cache &&
    now - cache.fetchedAt < CATALOG_TTL_MS
  )
    return cache.models;
  if (inflight) return inflight;
  inflight = fetchOpenRouterTranscriptionModels()
    .then((models) => {
      cache = { models, fetchedAt: Date.now() };
      return models;
    })
    .catch((error) => {
      if (cache && now - cache.fetchedAt < STALE_CATALOG_TTL_MS)
        return cache.models;
      throw error;
    })
    .finally(() => {
      inflight = undefined;
    });
  return inflight;
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
