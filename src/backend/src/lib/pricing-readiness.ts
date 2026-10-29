import { backendKillSwitchEnabled } from './compatibility';
import { decisionAdapterFor } from './decisions/decision-adapter';
import { decisionOutputIsUnbilled } from './decisions/decision-billing';
import { lookupDecisionModelCost } from './decisions/decision-models';
import { log } from './log';
import {
  type ModelCost,
  type ModelCostResolution,
  type PricingUnavailableReason,
  resolveModelCost,
} from './pricing';
import {
  getAudioModelRoute,
  getDecisionModelRoute,
  getModelProviderRoutes,
  getModelRoute,
  ModelNotConfiguredError,
  type TieredModelRouteSlot,
} from './settings';
import { lookupTranscriptionDurationPricing } from './transcription-models';
import { liveTieredRouteSlots } from './upstream-route-verdicts';

const AUDIO_ROUTE = 'audio_model_route';
const DECISION_ROUTE = 'decision_model_route';
const AUDIO_PROVIDER = 'openrouter';

/**
 * Why a configured route would not admit a current price. The first three are the
 * request path's own price refusals; the rest mean the route could not be read or is
 * one the request path refuses before it reaches pricing.
 */
export type PricingRefusal =
  | PricingUnavailableReason
  | 'route_unconfigured'
  | 'route_lookup_failed'
  | 'provider_unsupported'
  | 'model_unavailable'
  | 'charged_decision_output';

export type RoutePricingCheck = {
  route: string;
  provider: string | null;
  modelId: string | null;
  /** Null when the route admits a current price. */
  refusal: PricingRefusal | null;
};

export type PricingReadinessReport = {
  ready: boolean;
  checks: RoutePricingCheck[];
};

/** Ready only when every configured route admits a price that is current, not merely admissible. */
export function summarizePricingReadiness(
  checks: RoutePricingCheck[],
): PricingReadinessReport {
  return { ready: checks.every((check) => check.refusal === null), checks };
}

function refusalForResolution(resolution: ModelCostResolution): PricingRefusal | null {
  if (!resolution.ok) return resolution.reason;
  return resolution.cost.stale ? 'stale' : null;
}

function refusalForDecisionCost(cost: ModelCost | null): PricingRefusal | null {
  if (!cost) return 'model_unavailable';
  if (cost.stale) return 'stale';
  return decisionOutputIsUnbilled(cost) ? null : 'charged_decision_output';
}

function refusedRoute(route: string, refusal: PricingRefusal): RoutePricingCheck {
  return { route, provider: null, modelId: null, refusal };
}

async function priceTieredCandidate(
  slot: TieredModelRouteSlot,
  provider: string,
  modelId: string,
): Promise<RoutePricingCheck> {
  return {
    route: slot,
    provider,
    modelId,
    refusal: refusalForResolution(await resolveModelCost(provider, modelId)),
  };
}

/**
 * Every configured candidate of a tiered route: the primary and each enabled failover
 * link. The request path may skip a link for a missing key or format, but readiness
 * prices every configured link, so an unpriced link blocks the gate until it is removed
 * or priced.
 */
async function tieredRouteChecks(slot: TieredModelRouteSlot): Promise<RoutePricingCheck[]> {
  const primary = await getModelRoute(slot);
  const links = backendKillSwitchEnabled('ai_failover')
    ? []
    : await getModelProviderRoutes(slot, primary.modelId);
  const candidates = [
    { provider: primary.provider, modelId: primary.modelId },
    ...links.map((link) => ({ provider: link.providerId, modelId: link.modelId })),
  ];
  const unique = candidates.filter(
    (candidate, index) =>
      candidates.findIndex(
        (other) => other.provider === candidate.provider && other.modelId === candidate.modelId,
      ) === index,
  );
  return Promise.all(
    unique.map((candidate) => priceTieredCandidate(slot, candidate.provider, candidate.modelId)),
  );
}

async function audioRouteChecks(): Promise<RoutePricingCheck[]> {
  if (backendKillSwitchEnabled('audio_transcription')) return [];
  const route = await getAudioModelRoute();
  if (route.provider !== AUDIO_PROVIDER)
    return [
      { route: AUDIO_ROUTE, provider: route.provider, modelId: route.modelId, refusal: 'provider_unsupported' },
    ];
  const pricing = await lookupTranscriptionDurationPricing(AUDIO_PROVIDER, route.modelId);
  const refusal: PricingRefusal | null = !pricing
    ? 'model_unavailable'
    : pricing.stale
      ? 'stale'
      : null;
  return [{ route: AUDIO_ROUTE, provider: AUDIO_PROVIDER, modelId: route.modelId, refusal }];
}

async function decisionRouteChecks(): Promise<RoutePricingCheck[]> {
  if (backendKillSwitchEnabled('ai_proxy')) return [];
  const route = await getDecisionModelRoute();
  const base = { route: DECISION_ROUTE, provider: route.provider, modelId: route.modelId };
  if (!decisionAdapterFor(route.provider))
    return [{ ...base, refusal: 'provider_unsupported' }];
  const cost = await lookupDecisionModelCost(route.provider, route.modelId);
  return [{ ...base, refusal: refusalForDecisionCost(cost) }];
}

async function guardedRouteChecks(
  route: string,
  resolve: () => Promise<RoutePricingCheck[]>,
): Promise<RoutePricingCheck[]> {
  try {
    return await resolve();
  } catch (error) {
    log.warn('pricing_readiness_route_failed', { route, error: String(error) });
    return [
      refusedRoute(
        route,
        error instanceof ModelNotConfiguredError ? 'route_unconfigured' : 'route_lookup_failed',
      ),
    ];
  }
}

/**
 * Verifies every configured live route against the same price resolution requests use.
 * Kill switches remove a route exactly as the request path does. Run it after a refresh,
 * and it reads only the durable snapshots the refresh left behind; it never fetches.
 */
export async function verifyLivePricingReadiness(): Promise<PricingReadinessReport> {
  const tiered = backendKillSwitchEnabled('ai_proxy')
    ? []
    : liveTieredRouteSlots().map((slot) =>
        guardedRouteChecks(slot, () => tieredRouteChecks(slot)),
      );
  const groups = await Promise.all([
    ...tiered,
    guardedRouteChecks(AUDIO_ROUTE, audioRouteChecks),
    guardedRouteChecks(DECISION_ROUTE, decisionRouteChecks),
  ]);
  return summarizePricingReadiness(groups.flat());
}
