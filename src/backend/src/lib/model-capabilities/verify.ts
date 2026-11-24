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

export type ProbedCapabilities = { conclusive: CapabilityVerdict[]; temporaryFailures: CapabilityVerdict[] };

export async function probeAndSaveCapabilities(
  target: ProbeTarget,
  capabilities: readonly ModelCapability[],
  now = Date.now(),
): Promise<ProbedCapabilities> {
  const probed = await Promise.all(
    capabilities.map(async (capability) => {
      const outcome = await probeCapability(target, capability);
      const verdict: CapabilityVerdict = { ...target, capability, passed: outcome.passed, detail: outcome.detail, probedAt: now };
      return { verdict, conclusive: outcome.conclusive };
    }),
  );
  const conclusive = probed.filter((result) => result.conclusive).map((result) => result.verdict);
  await activeModelCapabilityStore().save(conclusive);
  return { conclusive, temporaryFailures: probed.filter((result) => !result.conclusive).map((result) => result.verdict) };
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
  const probed = await probeAndSaveCapabilities(target, stale, now);
  for (const verdict of [...probed.conclusive, ...probed.temporaryFailures]) known.set(verdict.capability, verdict);
  const failed = required
    .map((capability) => ({ capability, verdict: known.get(capability) }))
    .filter(({ verdict }) => !verdict?.passed || !isVerdictFresh(verdict, now))
    .map(({ capability, verdict }) => ({ capability, detail: verdict?.detail ?? null }));
  return { fits: failed.length === 0, failed };
}
