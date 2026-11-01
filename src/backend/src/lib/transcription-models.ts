import reviewedPricing from './transcription-pricing-reviewed.json';
import {
  durableOpenRouterCatalog,
  openRouterCatalogData,
} from './openrouter-catalog-snapshots';
import { staleAdmissionsHalted } from './pricing-overrun-cap';
import { log } from './log';

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
  /** Unit and rate are operator-reviewed rather than supplied by the catalog. */
  reviewed?: true;
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

// Reviewed price units are data, separate from catalog parsing.
const durationPricingUnits = new Map(reviewedPricing.models.map((entry) => [entry.modelId, entry]));

function durationPricing(
  model: OpenRouterModel,
): TranscriptionDurationPricing | null {
  const unit = typeof model.id === 'string' ? durationPricingUnits.get(model.id) : undefined;
  if (!unit) return null;
  const rate = decimalRate(model.pricing?.prompt);
  const outputRate = decimalRate(model.pricing?.completion);
  if (rate == null || outputRate !== 0) return null;
  if (unit.observedRate !== undefined && rate !== unit.observedRate) {
    log.warn('transcription_catalog_rate_requires_review', { model: model.id, observed_rate: rate, reviewed_rate: unit.observedRate });
    return null;
  }
  return {
    unit: 'second',
    usdPerSecond: rate / unit.seconds,
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

function validatedPricing(modelId: string, pricing?: TranscriptionDurationPricing): TranscriptionDurationPricing | null {
  if (!pricing) return null;
  // Snapshots can have been written by older code; recheck reviewed rates at read time.
  const unit = durationPricingUnits.get(modelId);
  if (!unit || pricing.unit !== 'second' || pricing.source !== 'openrouter_transcription_catalog' ||
      !Number.isFinite(pricing.usdPerSecond) || pricing.usdPerSecond < 0 ||
      (unit.observedRate !== undefined && pricing.usdPerSecond !== unit.observedRate / unit.seconds)) return null;
  return unit.observedRate === undefined ? pricing : { ...pricing, reviewed: true };
}

/** Return usable prices from one snapshot, with one stale-admission check. */
export async function listPricedTranscriptionModels(): Promise<TranscriptionModel[]> {
  const catalog = await transcriptionCatalog.admissible();
  if (!catalog || (catalog.validity === 'stale' && await staleAdmissionsHalted())) return [];
  return catalog.value.flatMap((model) => {
    const pricing = validatedPricing(model.modelId, model.durationPricing);
    if (!pricing) return [];
    return [{ ...model, durationPricing: catalog.validity === 'stale' ? { ...pricing, stale: true as const } : pricing }];
  });
}

/** Duration pricing for a model, marked stale past the current window. Never fetches. */
export async function lookupTranscriptionDurationPricing(
  provider: string,
  modelId: string,
): Promise<TranscriptionDurationPricing | null> {
  if (provider !== 'openrouter') return null;
  return (await listPricedTranscriptionModels()).find((model) => model.modelId === modelId)?.durationPricing ?? null;
}

export function resetTranscriptionModelCacheForTests() {
  transcriptionCatalog.reset();
}
