import type { PricingUnavailableReason } from './pricing';

// The `type` stays `pricing_unavailable` for every reason so released clients keep
// their existing handling. `reason` is additive and lets newer clients explain it.
const PRICING_UNAVAILABLE_MESSAGES: Record<PricingUnavailableReason, string> = {
  unpriced:
    'The active AI model has no usable price in the current catalog. Choose another model or try again later.',
  stale:
    'The active AI model price has not been refreshed recently enough to admit this request. Try again later.',
  unverified:
    'Nevermind has not verified the active AI model price yet. Try again later.',
};

export function pricingUnavailableBody(reason: PricingUnavailableReason) {
  return {
    error: {
      type: 'pricing_unavailable',
      message: PRICING_UNAVAILABLE_MESSAGES[reason],
      reason,
    },
  };
}
