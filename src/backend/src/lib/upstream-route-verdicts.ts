import { decisionAdapterFor } from './decisions/decision-adapter';
import { decisionOutputIsUnbilled } from './decisions/decision-billing';
import { lookupDecisionModelCost } from './decisions/decision-models';
import { backendKillSwitchEnabled } from './compatibility';
import { log } from './log';
import { lookupModelCost } from './pricing';
import { isRouteTripped } from './route-breakers';
import {
  type AudioModelRoute,
  type DecisionModelRoute,
  type ExtensionAiModelRole,
  getAudioModelRoute,
  getDecisionModelRoute,
  getModelProviderRoutes,
  getModelRoute,
  isProviderEnabled,
  ModelNotConfiguredError,
  type ModelRouteConfig,
  type ModelTier,
  tieredModelRouteSlot,
  type TieredModelRouteSlot,
} from './settings';
import { lookupTranscriptionDurationPricing } from './transcription-models';
import { getUpstreamConfig, providerSupportsFormat, selectApiForModel } from './upstream';

const AUDIO_ROUTE = 'audio_model_route';
// Breakers are keyed by the slot the request path uses, which for audio is `audio`.
const AUDIO_ROUTE_SLOT = 'audio';
const DECISION_ROUTE = 'decision_model_route';
const AUDIO_PROVIDER = 'openrouter';
const LIVE_TIERS: ModelTier[] = ['free', 'pro'];
const LIVE_ROLES: ExtensionAiModelRole[] = ['smart', 'fast'];
// Optional routes serve features beside chat. Chat never depends on them, so their
// refusals fail readiness but never liveness.
const OPTIONAL_ROUTES: ReadonlySet<string> = new Set([AUDIO_ROUTE, DECISION_ROUTE]);

export type LiveRouteCandidate = { provider: string; modelId: string };

export type UnserviceableReason =
  | 'route_unconfigured'
  | 'route_lookup_failed'
  | 'provider_unsupported'
  | 'provider_unkeyed'
  | 'provider_disabled'
  | 'model_unavailable'
  | 'route_tripped'
  | 'no_eligible_candidate';

// One live route as the request path resolves it. `candidates` are the providers the
// request path may call, in order. `unserviceable` is a refusal the request path makes
// on every attempt. `unverified` means a lookup failed transiently, so readiness cannot
// be confirmed even though the route still serves.
export type LiveRouteVerdict = {
  route: string;
  candidates: LiveRouteCandidate[];
  unserviceable: UnserviceableReason | null;
  unverified: boolean;
};

type Lookup<T> = { status: 'found'; value: T } | { status: 'missing' } | { status: 'failed' };

type BreakerState = 'tripped' | 'clear' | 'unknown';

export function isOptionalRoute(route: string): boolean {
  return OPTIONAL_ROUTES.has(route);
}

export function liveTieredRouteSlots(): TieredModelRouteSlot[] {
  return LIVE_TIERS.flatMap((tier) => LIVE_ROLES.map((role) => tieredModelRouteSlot(tier, role)));
}

async function lookupOutcome<T>(lookup: () => Promise<T | null>): Promise<Lookup<T>> {
  try {
    const value = await lookup();
    return value ? { status: 'found', value } : { status: 'missing' };
  } catch (error) {
    log.warn('health_route_lookup_failed', { error });
    return { status: 'failed' };
  }
}

// A breaker that cannot be read is unknown, not clear and not tripped: the route stays a
// candidate and is marked unverified, so readiness cannot pass on it.
async function breakerStatus(routeSlot: string, provider: string, modelId: string): Promise<BreakerState> {
  try {
    return (await isRouteTripped(routeSlot, provider, modelId)) ? 'tripped' : 'clear';
  } catch (error) {
    log.warn('health_route_breaker_lookup_failed', { route: routeSlot, error });
    return 'unknown';
  }
}

function refusedRoute(route: string, reason: UnserviceableReason): LiveRouteVerdict {
  return { route, candidates: [], unserviceable: reason, unverified: false };
}

function routeLookupFailure(error: unknown): UnserviceableReason {
  return error instanceof ModelNotConfiguredError ? 'route_unconfigured' : 'route_lookup_failed';
}

function hasUpstreamKey(provider: string): boolean {
  try {
    getUpstreamConfig(provider);
    return true;
  } catch {
    return false;
  }
}

function aiProxyEnabled(): boolean {
  return !backendKillSwitchEnabled('ai_proxy');
}

// A failed chain lookup is logged and the proxy continues on the primary alone, so the
// route is still live. Readiness cannot prove the chain, so the failure is marked unverified.
async function failoverLinks(
  slot: TieredModelRouteSlot,
  modelId: string,
): Promise<{ links: LiveRouteCandidate[]; unverified: boolean }> {
  if (backendKillSwitchEnabled('ai_failover')) return { links: [], unverified: false };
  try {
    const routes = await getModelProviderRoutes(slot, modelId);
    return {
      links: routes.map((route) => ({ provider: route.providerId, modelId: route.modelId })),
      unverified: false,
    };
  } catch (error) {
    log.warn('health_failover_lookup_failed', { route: slot, error });
    return { links: [], unverified: true };
  }
}

// The proxy refuses before any failover attempt when the primary has no key or no
// pricing. A tripped breaker, a primary whose wire format the provider lacks, or a
// failover link the proxy would skip for format, key, or pricing, is not a candidate.
async function resolveTieredRoute(slot: TieredModelRouteSlot): Promise<LiveRouteVerdict> {
  let primary: ModelRouteConfig;
  try {
    primary = await getModelRoute(slot);
  } catch (error) {
    return refusedRoute(slot, routeLookupFailure(error));
  }
  if (!hasUpstreamKey(primary.provider)) return refusedRoute(slot, 'provider_unkeyed');
  const primaryCost = await lookupOutcome(() => lookupModelCost(primary.provider, primary.modelId));
  if (primaryCost.status === 'missing') return refusedRoute(slot, 'model_unavailable');

  const apiFormat = await selectApiForModel(primary.provider, primary.modelId);
  const failover = await failoverLinks(slot, primary.modelId);
  const links = [
    { provider: primary.provider, modelId: primary.modelId },
    ...failover.links.filter((link) => link.provider !== primary.provider || link.modelId !== primary.modelId),
  ];
  const candidates: LiveRouteCandidate[] = [];
  let unverified = failover.unverified || primaryCost.status === 'failed';
  for (const [index, link] of links.entries()) {
    const breaker = await breakerStatus(slot, link.provider, link.modelId);
    if (breaker === 'tripped') continue;
    if (!providerSupportsFormat(link.provider, apiFormat)) continue;
    if ((await selectApiForModel(link.provider, link.modelId)) !== apiFormat) continue;
    if (!hasUpstreamKey(link.provider)) continue;
    const cost =
      index === 0
        ? primaryCost
        : await lookupOutcome(() => lookupModelCost(link.provider, link.modelId));
    if (cost.status === 'missing') continue;
    if (cost.status === 'failed' || breaker === 'unknown') unverified = true;
    candidates.push(link);
  }
  if (candidates.length === 0) return refusedRoute(slot, 'no_eligible_candidate');
  return { route: slot, candidates, unserviceable: null, unverified };
}

// The audio endpoint refuses any provider other than OpenRouter, and it refuses while the
// OpenRouter provider is disabled. Those refusals are reported, so readiness cannot pass
// over a route the request path rejects. The audio kill switch is the one exception: it
// is an explicit emergency shutdown and leaves the route out of the set.
async function resolveAudioRoute(): Promise<LiveRouteVerdict | null> {
  if (backendKillSwitchEnabled('audio_transcription')) return null;
  let route: AudioModelRoute;
  try {
    route = await getAudioModelRoute();
  } catch (error) {
    return refusedRoute(AUDIO_ROUTE, routeLookupFailure(error));
  }
  if (route.provider !== AUDIO_PROVIDER) return refusedRoute(AUDIO_ROUTE, 'provider_unsupported');
  let enabled: boolean;
  try {
    enabled = await isProviderEnabled(AUDIO_PROVIDER);
  } catch {
    return refusedRoute(AUDIO_ROUTE, 'route_lookup_failed');
  }
  if (!enabled) return refusedRoute(AUDIO_ROUTE, 'provider_disabled');
  const breaker = await breakerStatus(AUDIO_ROUTE_SLOT, AUDIO_PROVIDER, route.modelId);
  if (breaker === 'tripped') return refusedRoute(AUDIO_ROUTE, 'route_tripped');
  if (!hasUpstreamKey(AUDIO_PROVIDER)) return refusedRoute(AUDIO_ROUTE, 'provider_unkeyed');
  const pricing = await lookupOutcome(() => lookupTranscriptionDurationPricing(AUDIO_PROVIDER, route.modelId));
  if (pricing.status === 'missing') return refusedRoute(AUDIO_ROUTE, 'model_unavailable');
  return {
    route: AUDIO_ROUTE,
    candidates: [{ provider: AUDIO_PROVIDER, modelId: route.modelId }],
    unserviceable: null,
    unverified: pricing.status === 'failed' || breaker === 'unknown',
  };
}

async function resolveDecisionRoute(): Promise<LiveRouteVerdict | null> {
  if (!aiProxyEnabled()) return null;
  let route: DecisionModelRoute;
  try {
    route = await getDecisionModelRoute();
  } catch (error) {
    return refusedRoute(DECISION_ROUTE, routeLookupFailure(error));
  }
  if (!decisionAdapterFor(route.provider)) return refusedRoute(DECISION_ROUTE, 'provider_unsupported');
  if (!hasUpstreamKey(route.provider)) return refusedRoute(DECISION_ROUTE, 'provider_unkeyed');
  const cost = await lookupOutcome(() => lookupDecisionModelCost(route.provider, route.modelId));
  if (cost.status === 'missing' || (cost.status === 'found' && !decisionOutputIsUnbilled(cost.value)))
    return refusedRoute(DECISION_ROUTE, 'model_unavailable');
  return {
    route: DECISION_ROUTE,
    candidates: [{ provider: route.provider, modelId: route.modelId }],
    unserviceable: null,
    unverified: cost.status === 'failed',
  };
}

// An unexpected error in one route's resolution is a refusal of that route alone. The
// other routes still resolve, and readiness fails for it. Liveness fails only when the
// route is a chat route, as any other chat refusal does.
async function guardedRoute(
  route: string,
  resolve: () => Promise<LiveRouteVerdict | null>,
): Promise<LiveRouteVerdict | null> {
  try {
    return await resolve();
  } catch (error) {
    log.error('health_route_resolution_failed', { route, error });
    return refusedRoute(route, 'route_lookup_failed');
  }
}

// Every route that real traffic can reach. A kill switch on the request path removes
// that route from the set, because the request never reaches a provider.
export async function resolveLiveRouteVerdicts(): Promise<LiveRouteVerdict[]> {
  const tiered = aiProxyEnabled()
    ? await Promise.all(liveTieredRouteSlots().map((slot) => guardedRoute(slot, () => resolveTieredRoute(slot))))
    : [];
  const [audio, decision] = await Promise.all([
    guardedRoute(AUDIO_ROUTE, resolveAudioRoute),
    guardedRoute(DECISION_ROUTE, resolveDecisionRoute),
  ]);
  return [...tiered, audio, decision].filter((verdict): verdict is LiveRouteVerdict => verdict !== null);
}
