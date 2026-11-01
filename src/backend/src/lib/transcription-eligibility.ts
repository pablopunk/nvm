import { usdToCredits } from './cost';
import { AUDIO_RESERVATION_CREDITS } from './audio-reservation-policy';
import { STALE_RESERVATION_MARGIN } from './pricing';
import {
  listPricedTranscriptionModels,
  lookupTranscriptionDurationPricing,
  type TranscriptionDurationPricing,
} from './transcription-models';

// Picker guidance for a normal 60-second segment, rounded up with one second of slack.
// Admission still checks the actual recording; a delayed timer can make it longer.
const STANDARD_SEGMENT_BILLABLE_SECONDS = 61;

function fitsRecordingBudget(pricing: TranscriptionDurationPricing | null | undefined) {
  return Boolean(pricing) && usdToCredits(
    (pricing?.usdPerSecond ?? 0) * STANDARD_SEGMENT_BILLABLE_SECONDS * STALE_RESERVATION_MARGIN,
  ) <= AUDIO_RESERVATION_CREDITS;
}

export async function canSelectTranscriptionModel(provider: string, modelId: string) {
  return fitsRecordingBudget(await lookupTranscriptionDurationPricing(provider, modelId));
}

export async function listSelectableTranscriptionModels() {
  const models = await listPricedTranscriptionModels();
  return models.filter((model) => fitsRecordingBudget(model.durationPricing));
}
