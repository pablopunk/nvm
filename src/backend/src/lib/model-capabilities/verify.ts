import type { RouteKind } from '../pricing';
import { CAPABILITIES_FOR_ROUTE_KIND, type ModelCapability } from './criteria';
import type { ProbeTarget } from './probe-request';
import { probeCapability } from './probes';
import {
  activeModelCapabilityStore,
  type CapabilityVerdict,
  isVerdictFresh,
  needsReprobe,
} from './store';

export async function probeAndSaveCapabilities(
  target: ProbeTarget,
  capabilities: readonly ModelCapability[],
  now = Date.now(),
): Promise<CapabilityVerdict[]> {
  const verdicts = await Promise.all(
    capabilities.map(async (capability): Promise<CapabilityVerdict> => {
      const outcome = await probeCapability(target, capability);
      return { ...target, capability, passed: outcome.passed, detail: outcome.detail, probedAt: now };
    }),
  );
  await activeModelCapabilityStore().save(verdicts);
  return verdicts;
}

export type RouteKindVerification = {
  fits: boolean;
  failed: Array<{ capability: ModelCapability; detail: string | null }>;
};

export async function verifyModelForRouteKind(
  target: ProbeTarget,
  kind: RouteKind,
  now = Date.now(),
): Promise<RouteKindVerification> {
  const required = CAPABILITIES_FOR_ROUTE_KIND[kind];
  const known = new Map(
    (await activeModelCapabilityStore().loadModel(target.provider, target.modelId)).map((verdict) => [
      verdict.capability,
      verdict,
    ]),
  );
  const stale = required.filter((capability) => needsReprobe(known.get(capability), now));
  for (const verdict of await probeAndSaveCapabilities(target, stale, now)) known.set(verdict.capability, verdict);
  const failed = required
    .map((capability) => ({ capability, verdict: known.get(capability) }))
    .filter(({ verdict }) => !verdict?.passed || !isVerdictFresh(verdict, now))
    .map(({ capability, verdict }) => ({ capability, detail: verdict?.detail ?? null }));
  return { fits: failed.length === 0, failed };
}
