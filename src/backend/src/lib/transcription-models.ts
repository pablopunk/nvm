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
