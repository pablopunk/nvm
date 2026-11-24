import { listCatalogCandidates } from '../admin-model-options';
import type { RouteKind } from '../pricing';
import { getModelRoute, type ModelRoute, tieredModelRouteSlot } from '../settings';
import { capabilitiesForRouteKinds, type ModelCapability } from './criteria';
import { activeModelCapabilityStore, type CapabilityVerdict, needsReprobe } from './store';
import { probeAndSaveCapabilities } from './verify';

const ROUTE_KINDS: RouteKind[] = ['fast', 'smart'];
const PROBE_CONCURRENCY = 4;

export type ProbeWork = { provider: string; modelId: string; capabilities: ModelCapability[]; lastProbedAt: number; configured: boolean };

export type CapabilityRefreshReport = {
  candidates: number;
  probedModels: number;
  probes: number;
  passed: number;
  failed: number;
  temporaryFailures: number;
  remaining: number;
};

const routeKey = (route: ModelRoute) => `${route.provider}\u0000${route.modelId}`;

async function configuredRouteKeys(): Promise<Set<string>> {
  const keys = new Set<string>();
  for (const tier of ['free', 'pro'] as const)
    for (const role of ROUTE_KINDS) {
      try {
        keys.add(routeKey(await getModelRoute(tieredModelRouteSlot(tier, role))));
      } catch {
        // An unconfigured slot has no route to prioritise.
      }
    }
  return keys;
}

async function kindsByCandidate(): Promise<Map<string, { route: ModelRoute; kinds: Set<RouteKind> }>> {
  const byKey = new Map<string, { route: ModelRoute; kinds: Set<RouteKind> }>();
  for (const kind of ROUTE_KINDS)
    for (const { provider, modelIds } of await listCatalogCandidates(kind))
      for (const modelId of modelIds) {
        const route = { provider, modelId };
        const entry = byKey.get(routeKey(route)) ?? { route, kinds: new Set<RouteKind>() };
        entry.kinds.add(kind);
        byKey.set(routeKey(route), entry);
      }
  return byKey;
}

async function verdictsByRoute(providers: Iterable<string>): Promise<Map<string, Map<ModelCapability, CapabilityVerdict>>> {
  const byRoute = new Map<string, Map<ModelCapability, CapabilityVerdict>>();
  for (const provider of providers)
    for (const verdict of await activeModelCapabilityStore().loadProvider(provider)) {
      const key = routeKey(verdict);
      const capabilities = byRoute.get(key) ?? new Map<ModelCapability, CapabilityVerdict>();
      capabilities.set(verdict.capability, verdict);
      byRoute.set(key, capabilities);
    }
  return byRoute;
}

function oldestFirst(left: ProbeWork, right: ProbeWork): number {
  return left.lastProbedAt - right.lastProbedAt;
}

function alternateProviders(work: ProbeWork[]): ProbeWork[] {
  const queues = new Map<string, ProbeWork[]>();
  for (const item of [...work].sort(oldestFirst)) queues.set(item.provider, [...(queues.get(item.provider) ?? []), item]);
  const alternated: ProbeWork[] = [];
  while ([...queues.values()].some((queue) => queue.length > 0))
    for (const queue of queues.values()) {
      const next = queue.shift();
      if (next) alternated.push(next);
    }
  return alternated;
}

export function orderProbeWork(work: ProbeWork[]): ProbeWork[] {
  return [
    ...alternateProviders(work.filter((item) => item.configured)),
    ...alternateProviders(work.filter((item) => !item.configured)),
  ];
}

export async function planCapabilityProbes(now = Date.now()): Promise<{ candidates: number; work: ProbeWork[] }> {
  const candidates = await kindsByCandidate();
  const configured = await configuredRouteKeys();
  const verdicts = await verdictsByRoute(new Set([...candidates.values()].map(({ route }) => route.provider)));
  const work: ProbeWork[] = [];
  for (const [key, { route, kinds }] of candidates) {
    const known = verdicts.get(key);
    const capabilities = capabilitiesForRouteKinds(kinds).filter((capability) => needsReprobe(known?.get(capability), now));
    if (capabilities.length === 0) continue;
    const lastProbedAt = Math.min(...capabilities.map((capability) => known?.get(capability)?.probedAt ?? 0));
    work.push({ ...route, capabilities, lastProbedAt, configured: configured.has(key) });
  }
  return { candidates: candidates.size, work: orderProbeWork(work) };
}

export async function refreshModelCapabilities(deadline: number, now = Date.now()): Promise<CapabilityRefreshReport> {
  const { candidates, work } = await planCapabilityProbes(now);
  const report: CapabilityRefreshReport = {
    candidates,
    probedModels: 0,
    probes: 0,
    passed: 0,
    failed: 0,
    temporaryFailures: 0,
    remaining: work.length,
  };
  const queue = [...work];
  async function probeUntilDeadline() {
    for (let next = queue.shift(); next && Date.now() < deadline; next = queue.shift()) {
      const { conclusive, temporaryFailures } = await probeAndSaveCapabilities(next, next.capabilities, now);
      report.probedModels += 1;
      report.probes += conclusive.length + temporaryFailures.length;
      report.passed += conclusive.filter((verdict) => verdict.passed).length;
      report.failed += conclusive.filter((verdict) => !verdict.passed).length;
      report.temporaryFailures += temporaryFailures.length;
      report.remaining -= 1;
    }
  }
  await Promise.all(Array.from({ length: PROBE_CONCURRENCY }, probeUntilDeadline));
  return report;
}
