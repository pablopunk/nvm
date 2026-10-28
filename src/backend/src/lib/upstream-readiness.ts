import { log } from './log';
import { probeProviderReadiness, type ProviderReadiness } from './upstream-probe';
import { isOptionalRoute, type LiveRouteVerdict, resolveLiveRouteVerdicts } from './upstream-route-verdicts';

export type UpstreamReadiness = { upstream: boolean; upstreamReady: boolean };

const READY_TTL_MS = 30_000;
const NOT_READY_TTL_MS = 5_000;
const HEALTH_WAIT_MS = 8_000;
const LIVENESS_CARRY_MS = 60_000;

// Liveness is the chat path: every tiered route must be servable and reachable. Optional
// audio and decision routes never flip it, so a missing or refused feature cannot 503 a
// deployment whose chat works. Readiness is strict: every route the product exposes must
// be serviceable and verified, and every candidate it may call must be ready. A route
// refused as optional still fails readiness. No live route at all leaves liveness vacuous
// and never proves readiness, so a deployment with every route switched off is not ready.
export function summarizeUpstreamReadiness(
  verdicts: LiveRouteVerdict[],
  probes: ReadonlyMap<string, ProviderReadiness>,
): UpstreamReadiness {
  const probeFor = (provider: string) => probes.get(provider) ?? { provider, reachable: false, ready: false };
  const serviceable = (verdict: LiveRouteVerdict) => verdict.unserviceable === null;
  const upstream = verdicts.every(
    (verdict) =>
      isOptionalRoute(verdict.route) ||
      (serviceable(verdict) && verdict.candidates.some((candidate) => probeFor(candidate.provider).reachable)),
  );
  const upstreamReady =
    verdicts.length > 0 &&
    verdicts.every(
      (verdict) =>
        serviceable(verdict) &&
        !verdict.unverified &&
        verdict.candidates.every((candidate) => probeFor(candidate.provider).ready),
    );
  return { upstream, upstreamReady };
}

type ReadinessCheckerDeps = {
  resolveVerdicts: () => Promise<LiveRouteVerdict[]>;
  probeProvider: (provider: string) => Promise<ProviderReadiness>;
  now: () => number;
  healthWaitMs?: number;
};

export type UpstreamReadinessChecker = {
  check: () => Promise<UpstreamReadiness>;
  reset: () => void;
};

// Route resolution and provider probes are the expensive part, so at most one evaluation
// runs per instance. Concurrent callers and callers after a timeout join that same
// evaluation, and it is never abandoned, so a slow catalog cannot start a second one.
// A caller waits at most `healthWaitMs`. When it times out it gets unknown readiness,
// with liveness carried from the last completed evaluation for a bounded window. Only a
// completed evaluation is cached, and the database check stays live in the route handler.
export function createUpstreamReadinessChecker(deps: ReadinessCheckerDeps): UpstreamReadinessChecker {
  const healthWaitMs = deps.healthWaitMs ?? HEALTH_WAIT_MS;
  let cached: { value: UpstreamReadiness; expiresAt: number } | null = null;
  let failedUntil = 0;
  let lastCompleted: { upstream: boolean; at: number } | null = null;
  let inflight: Promise<UpstreamReadiness> | null = null;

  function carriedLiveness(): boolean {
    return lastCompleted !== null && lastCompleted.upstream && deps.now() - lastCompleted.at <= LIVENESS_CARRY_MS;
  }

  function unknownReadiness(): UpstreamReadiness {
    return { upstream: carriedLiveness(), upstreamReady: false };
  }

  async function evaluate(): Promise<UpstreamReadiness> {
    const verdicts = await deps.resolveVerdicts();
    const providers = [
      ...new Set(
        verdicts.flatMap((verdict) =>
          verdict.unserviceable === null ? verdict.candidates.map((candidate) => candidate.provider) : [],
        ),
      ),
    ];
    const probes = await Promise.all(providers.map((provider) => deps.probeProvider(provider)));
    return summarizeUpstreamReadiness(verdicts, new Map(probes.map((probe) => [probe.provider, probe])));
  }

  function startEvaluation(): Promise<UpstreamReadiness> {
    return evaluate()
      .then(
        (value) => {
          cached = { value, expiresAt: deps.now() + (value.upstreamReady ? READY_TTL_MS : NOT_READY_TTL_MS) };
          lastCompleted = { upstream: value.upstream, at: deps.now() };
          return value;
        },
        (error: unknown) => {
          log.error('health_upstream_evaluation_failed', { error });
          failedUntil = deps.now() + NOT_READY_TTL_MS;
          return unknownReadiness();
        },
      )
      .finally(() => {
        inflight = null;
      });
  }

  function awaitWithinHealthWait(pending: Promise<UpstreamReadiness>): Promise<UpstreamReadiness> {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const deadline = new Promise<UpstreamReadiness>((resolve) => {
      timer = setTimeout(() => {
        log.warn('health_upstream_evaluation_pending', { wait_ms: healthWaitMs });
        resolve(unknownReadiness());
      }, healthWaitMs);
    });
    return Promise.race([pending, deadline]).finally(() => clearTimeout(timer));
  }

  return {
    check() {
      if (cached && deps.now() < cached.expiresAt) return Promise.resolve(cached.value);
      if (deps.now() < failedUntil) return Promise.resolve(unknownReadiness());
      inflight ??= startEvaluation();
      return awaitWithinHealthWait(inflight);
    },
    reset() {
      cached = null;
      failedUntil = 0;
      lastCompleted = null;
      inflight = null;
    },
  };
}

const liveChecker = createUpstreamReadinessChecker({
  resolveVerdicts: resolveLiveRouteVerdicts,
  probeProvider: probeProviderReadiness,
  now: Date.now,
});

export function probeUpstreamReadiness(): Promise<UpstreamReadiness> {
  return liveChecker.check();
}

export function resetUpstreamReadinessCacheForTests() {
  liveChecker.reset();
}
