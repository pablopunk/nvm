import {
  durableOpenRouterCatalog,
  openRouterCatalogData,
} from './openrouter-catalog-snapshots';
import { staleAdmissionsHalted } from './pricing-overrun-cap';

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
  /** Present only when the validated catalog is past the current window. */
  stale?: true;
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

function hasModality(value: unknown, modality: string) {
  return Array.isArray(value) && value.includes(modality);
}

function decimalRate(value: unknown): number | null {
  if (typeof value !== 'string' || !/^\d+(?:\.\d+)?$/.test(value)) return null;
  const rate = Number(value);
  return Number.isFinite(rate) && rate >= 0 ? rate : null;
}

// Units checked against OpenRouter's model pages and a short billed transcription.
// The rate itself always comes from the current admissible catalog snapshot.
const durationPricingUnits = new Map<string, number>([
  ['nvidia/nemotron-3.5-transcribe', 1],
  ['mistralai/voxtral-small-transcribe-3-0', 1],
  ['mistralai/voxtral-mini-transcribe-3-0', 1],
  ['assemblyai/universal-3-6-pro', 1],
  ['elevenlabs/scribe-v2', 1],
  ['elevenlabs/scribe-v2-medical', 1],
  ['fish-audio/transcribe-1-pro', 1],
  ['assemblyai/universal-3-5-pro', 1],
  ['meta/muse-voice-transcribe-1.0', 1],
  ['microsoft/mai-transcribe-2', 3600],
  ['nvidia/nemotron-3.5-asr-streaming-multilingual-0.6b', 1],
  ['mistralai/voxtral-small-24b-2507-stt', 1],
  ['mistralai/voxtral-mini-3b-2507', 1],
  ['qwen/qwen3-asr-1.7b', 1],
  ['qwen/qwen3-asr-0.6b', 1],
  ['openai/gpt-transcribe', 1],
  ['fish-audio/transcribe-1', 1],
  ['x-ai/grok-stt-1.0', 1],
  ['deepgram/nova-3', 1],
  ['microsoft/mai-transcribe-1.5', 3600],
  ['nvidia/parakeet-tdt-0.6b-v3', 1],
  ['mistralai/voxtral-mini-transcribe', 1],
  ['google/chirp-3', 1],
  ['openai/whisper-large-v3', 1],
  ['openai/whisper-large-v3-turbo', 1],
  ['openai/whisper-1', 1],
]);

function durationPricing(
  model: OpenRouterModel,
): TranscriptionDurationPricing | null {
  const divisor = typeof model.id === 'string' ? durationPricingUnits.get(model.id) : undefined;
  if (!divisor) return null;
  const rate = decimalRate(model.pricing?.prompt);
  const outputRate = decimalRate(model.pricing?.completion);
  if (rate == null || outputRate !== 0) return null;
  return {
    unit: 'second',
    usdPerSecond: rate / divisor,
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

function parseTranscriptionCatalog(body: unknown): TranscriptionModel[] | null {
  const data = openRouterCatalogData(body);
  if (!data) return null;
  return (data as OpenRouterModel[])
    .map(transcriptionModel)
    .filter((model): model is TranscriptionModel => Boolean(model))
    .sort((left, right) => left.name.localeCompare(right.name));
}

function isTranscriptionModelList(payload: unknown): payload is TranscriptionModel[] {
  return (
    Array.isArray(payload) &&
    payload.every(
      (model) =>
        typeof model === 'object' &&
        model !== null &&
        typeof (model as TranscriptionModel).modelId === 'string' &&
        typeof (model as TranscriptionModel).name === 'string',
    )
  );
}

const transcriptionCatalog = durableOpenRouterCatalog<TranscriptionModel>({
  snapshotKey: 'openrouter_transcription_models',
  path: '/models?input_modalities=audio&output_modalities=transcription',
  parseResponse: parseTranscriptionCatalog,
  isPayload: isTranscriptionModelList,
});

/** Validates the transcription catalog. Only the scheduled refresh and admin saves fetch it. */
export function refreshTranscriptionCatalog(): Promise<void> {
  return transcriptionCatalog.refresh();
}

/** Listed models come from the admissible catalog: stale still lists, unvalidated or expired lists nothing. */
export async function listTranscriptionModels(): Promise<TranscriptionModel[]> {
  return (await transcriptionCatalog.admissible())?.value ?? [];
}

/** Duration pricing for a model, marked stale past the current window. Never fetches. */
export async function lookupTranscriptionDurationPricing(
  provider: string,
  modelId: string,
): Promise<TranscriptionDurationPricing | null> {
  if (provider !== 'openrouter') return null;
  const catalog = await transcriptionCatalog.admissible();
  const pricing = catalog?.value.find((model) => model.modelId === modelId)
    ?.durationPricing;
  if (!pricing) return null;
  if (catalog?.validity !== 'stale') return pricing;
  return (await staleAdmissionsHalted()) ? null : { ...pricing, stale: true };
}

export function resetTranscriptionModelCacheForTests() {
  transcriptionCatalog.reset();
}
