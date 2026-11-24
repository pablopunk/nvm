import { listCatalogCandidates } from '../admin-model-options';
import type { RouteKind } from '../pricing';
import { getModelRoute, type ModelRoute, tieredModelRouteSlot } from '../settings';
import { capabilitiesForRouteKinds, type ModelCapability } from './criteria';
import { activeModelCapabilityStore, type CapabilityVerdict, needsReprobe } from './store';
import { probeAndSaveCapabilities } from './verify';

const ROUTE_KINDS: RouteKind[] = ['fast', 'smart'];
const PROBE_CONCURRENCY = 4;

type ProbeWork = { provider: string; modelId: string; capabilities: ModelCapability[]; lastProbedAt: number; configured: boolean };

export type CapabilityRefreshReport = {
  candidates: number;
  probedModels: number;
  probes: number;
  passed: number;
  failed: number;
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

function compareProbeWork(left: ProbeWork, right: ProbeWork): number {
  if (left.configured !== right.configured) return left.configured ? -1 : 1;
  return left.lastProbedAt - right.lastProbedAt;
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
  return { candidates: candidates.size, work: work.sort(compareProbeWork) };
}

export async function refreshModelCapabilities(deadline: number, now = Date.now()): Promise<CapabilityRefreshReport> {
  const { candidates, work } = await planCapabilityProbes(now);
  const report: CapabilityRefreshReport = { candidates, probedModels: 0, probes: 0, passed: 0, failed: 0, remaining: work.length };
  const queue = [...work];
  async function probeUntilDeadline() {
    for (let next = queue.shift(); next && Date.now() < deadline; next = queue.shift()) {
      const verdicts = await probeAndSaveCapabilities(next, next.capabilities, now);
      report.probedModels += 1;
      report.probes += verdicts.length;
      report.passed += verdicts.filter((verdict) => verdict.passed).length;
      report.failed += verdicts.filter((verdict) => !verdict.passed).length;
      report.remaining -= 1;
    }
  }
  await Promise.all(Array.from({ length: PROBE_CONCURRENCY }, probeUntilDeadline));
  return report;
}
