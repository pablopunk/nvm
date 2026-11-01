import { usdToCredits } from './cost';
import { STALE_RESERVATION_MARGIN } from './pricing';
import {
  listTranscriptionModels,
  lookupTranscriptionDurationPricing,
} from './transcription-models';

// A 60-second desktop segment can round up to 61 billable seconds.
const MAX_SEGMENT_BILLABLE_SECONDS = 61;
const AUDIO_RESERVATION_CREDITS = 5;

export async function canSelectTranscriptionModel(provider: string, modelId: string) {
  const pricing = await lookupTranscriptionDurationPricing(provider, modelId);
  return pricing !== null && usdToCredits(
    pricing.usdPerSecond * MAX_SEGMENT_BILLABLE_SECONDS * STALE_RESERVATION_MARGIN,
  ) <= AUDIO_RESERVATION_CREDITS;
}

export async function listSelectableTranscriptionModels() {
  const models = await listTranscriptionModels();
  const eligible = await Promise.all(models.map((model) =>
    canSelectTranscriptionModel(model.provider, model.modelId),
  ));
  return models.filter((_, index) => eligible[index]);
}
