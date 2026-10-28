import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import type { ProviderReadiness } from './upstream-probe';
import {
  createUpstreamReadinessChecker,
  summarizeUpstreamReadiness,
  type UpstreamReadiness,
} from './upstream-readiness';
import type { LiveRouteVerdict } from './upstream-route-verdicts';

const READY = { reachable: true, ready: true };
const LIVE_NOT_READY = { reachable: true, ready: false };
const DOWN = { reachable: false, ready: false };

function verdict(route: string, providers: string[], overrides: Partial<LiveRouteVerdict> = {}): LiveRouteVerdict {
  return {
    route,
    candidates: providers.map((provider) => ({ provider, modelId: `${provider}-model` })),
    unserviceable: null,
    unverified: false,
    ...overrides,
  };
}

function probeMap(byProvider: Record<string, { reachable: boolean; ready: boolean }>): Map<string, ProviderReadiness> {
  return new Map(Object.entries(byProvider).map(([provider, flags]) => [provider, { provider, ...flags }]));
}

function summarize(verdicts: LiveRouteVerdict[], byProvider: Record<string, { reachable: boolean; ready: boolean }>) {
  return summarizeUpstreamReadiness(verdicts, probeMap(byProvider));
}

describe('summarizeUpstreamReadiness', () => {
  test('a fully ready deployment reports liveness and readiness', () => {
    assert.deepEqual(summarize([verdict('pro-smart', ['openrouter'])], { openrouter: READY }), {
      upstream: true,
      upstreamReady: true,
    });
  });

  test('liveness needs one reachable candidate, so a failover carries a route whose primary is down', () => {
    assert.deepEqual(
      summarize([verdict('pro-smart', ['openrouter', 'opencode_zen'])], {
        openrouter: DOWN,
        opencode_zen: READY,
      }),
      { upstream: true, upstreamReady: false },
    );
  });

  test('readiness requires every candidate the request path may call, including a failover that is live only', () => {
    assert.deepEqual(
      summarize([verdict('pro-smart', ['openrouter', 'opencode_zen'])], {
        openrouter: READY,
        opencode_zen: LIVE_NOT_READY,
      }),
      { upstream: true, upstreamReady: false },
    );
  });

  test('an unserviceable route fails liveness and readiness even when the other routes are ready', () => {
    const verdicts = [
      verdict('pro-smart', ['openrouter']),
      verdict('pro-fast', [], { unserviceable: 'provider_unkeyed' }),
    ];
    assert.deepEqual(summarize(verdicts, { openrouter: READY }), { upstream: false, upstreamReady: false });
  });

  test('an unverified route stays live but cannot be proven ready', () => {
    assert.deepEqual(
      summarize([verdict('audio_model_route', ['openrouter'], { unverified: true })], { openrouter: READY }),
      { upstream: true, upstreamReady: false },
    );
  });

  test('a single unreachable candidate fails liveness', () => {
    assert.deepEqual(summarize([verdict('pro-smart', ['openrouter'])], { openrouter: DOWN }), {
      upstream: false,
      upstreamReady: false,
    });
  });

  test('a candidate with no probe result is treated as unreachable', () => {
    assert.deepEqual(summarize([verdict('pro-smart', ['openrouter'])], {}), {
      upstream: false,
      upstreamReady: false,
    });
  });

  test('no live route at all keeps liveness vacuous and never proves readiness', () => {
    assert.deepEqual(summarize([], {}), { upstream: true, upstreamReady: false });
  });

  test('a missing optional audio route fails readiness but never liveness, so chat stays live', () => {
    const verdicts = [
      verdict('pro-smart', ['openrouter']),
      verdict('audio_model_route', [], { unserviceable: 'route_unconfigured' }),
    ];
    assert.deepEqual(summarize(verdicts, { openrouter: READY }), { upstream: true, upstreamReady: false });
  });

  test('a disabled optional audio route fails readiness, as the endpoint refuses it, and keeps liveness', () => {
    const verdicts = [
      verdict('pro-smart', ['openrouter']),
      verdict('audio_model_route', [], { unserviceable: 'provider_disabled' }),
    ];
    assert.deepEqual(summarize(verdicts, { openrouter: READY }), { upstream: true, upstreamReady: false });
  });

  test('an optional decision route whose provider is down fails readiness but not liveness', () => {
    const verdicts = [
      verdict('pro-smart', ['openrouter']),
      verdict('decision_model_route', ['decisions_provider']),
    ];
    assert.deepEqual(summarize(verdicts, { openrouter: READY, decisions_provider: DOWN }), {
      upstream: true,
      upstreamReady: false,
    });
  });

  test('a missing chat route fails liveness, because chat itself cannot serve that slot', () => {
    const verdicts = [
      verdict('pro-smart', ['openrouter']),
      verdict('pro-fast', [], { unserviceable: 'route_unconfigured' }),
    ];
    assert.deepEqual(summarize(verdicts, { openrouter: READY }), { upstream: false, upstreamReady: false });
  });
});

function fakeClock(start = 1_000_000) {
  let current = start;
  return {
    now: () => current,
    advance: (ms: number) => {
      current += ms;
    },
  };
}

function holdEvaluation() {
  let release!: () => void;
  const gate = new Promise<void>((resolve) => {
    release = resolve;
  });
  return { gate, release };
}

const READY_RESULT: UpstreamReadiness = { upstream: true, upstreamReady: true };
const UNVERIFIED_RESULT: UpstreamReadiness = { upstream: true, upstreamReady: false };

function readyProbe(provider: string): Promise<ProviderReadiness> {
  return Promise.resolve({ provider, reachable: true, ready: true });
}

describe('createUpstreamReadinessChecker', () => {
  test('concurrent callers share a single evaluation', async () => {
    const clock = fakeClock();
    let evaluations = 0;
    let release!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const checker = createUpstreamReadinessChecker({
      resolveVerdicts: async () => {
        evaluations += 1;
        await gate;
        return [verdict('pro-smart', ['openrouter'])];
      },
      probeProvider: readyProbe,
      now: clock.now,
    });
    const pending = [checker.check(), checker.check(), checker.check()];
    release();
    assert.deepEqual(await Promise.all(pending), [READY_RESULT, READY_RESULT, READY_RESULT]);
    assert.equal(evaluations, 1);
  });

  test('a ready result is reused for thirty seconds and then re-evaluated', async () => {
    const clock = fakeClock();
    let evaluations = 0;
    const checker = createUpstreamReadinessChecker({
      resolveVerdicts: async () => {
        evaluations += 1;
        return [verdict('pro-smart', ['openrouter'])];
      },
      probeProvider: readyProbe,
      now: clock.now,
    });
    await checker.check();
    clock.advance(29_999);
    await checker.check();
    assert.equal(evaluations, 1);
    clock.advance(1);
    await checker.check();
    assert.equal(evaluations, 2);
  });

  test('an unready result expires after five seconds, so a transient outage is not held for long', async () => {
    const clock = fakeClock();
    let evaluations = 0;
    const checker = createUpstreamReadinessChecker({
      resolveVerdicts: async () => {
        evaluations += 1;
        return [verdict('audio_model_route', ['openrouter'], { unverified: true })];
      },
      probeProvider: readyProbe,
      now: clock.now,
    });
    assert.deepEqual(await checker.check(), UNVERIFIED_RESULT);
    clock.advance(4_999);
    await checker.check();
    assert.equal(evaluations, 1);
    clock.advance(1);
    await checker.check();
    assert.equal(evaluations, 2);
  });

  test('each provider is probed once per evaluation even when several routes share it', async () => {
    const probed: string[] = [];
    const checker = createUpstreamReadinessChecker({
      resolveVerdicts: async () => [
        verdict('pro-smart', ['openrouter']),
        verdict('pro-fast', ['openrouter']),
        verdict('audio_model_route', ['openrouter']),
      ],
      probeProvider: async (provider) => {
        probed.push(provider);
        return { provider, reachable: true, ready: true };
      },
      now: fakeClock().now,
    });
    assert.deepEqual(await checker.check(), READY_RESULT);
    assert.deepEqual(probed, ['openrouter']);
  });

  test('a thrown evaluation reports unready and is retried after the short TTL', async () => {
    const clock = fakeClock();
    let evaluations = 0;
    const checker = createUpstreamReadinessChecker({
      resolveVerdicts: async () => {
        evaluations += 1;
        if (evaluations === 1) throw new Error('route lookup crashed');
        return [verdict('pro-smart', ['openrouter'])];
      },
      probeProvider: readyProbe,
      now: clock.now,
    });
    assert.deepEqual(await checker.check(), { upstream: false, upstreamReady: false });
    clock.advance(5_000);
    assert.deepEqual(await checker.check(), READY_RESULT);
    assert.equal(evaluations, 2);
  });

  test('a cold evaluation that outlasts the health wait reports unknown readiness, then the completed answer is shared', async () => {
    const clock = fakeClock();
    const hold = holdEvaluation();
    let evaluations = 0;
    const checker = createUpstreamReadinessChecker({
      resolveVerdicts: async () => {
        evaluations += 1;
        await hold.gate;
        return [verdict('pro-smart', ['openrouter'])];
      },
      probeProvider: readyProbe,
      now: clock.now,
      healthWaitMs: 10,
    });
    assert.deepEqual(await checker.check(), { upstream: false, upstreamReady: false });
    const joined = checker.check();
    hold.release();
    assert.deepEqual(await joined, READY_RESULT);
    assert.deepEqual(await checker.check(), READY_RESULT);
    assert.equal(evaluations, 1);
  });

  test('a slow evaluation past the cache TTL is joined, never duplicated, and carries liveness only for a bounded window', async () => {
    const clock = fakeClock();
    const hold = holdEvaluation();
    let evaluations = 0;
    let held = false;
    const checker = createUpstreamReadinessChecker({
      resolveVerdicts: async () => {
        evaluations += 1;
        if (held) await hold.gate;
        return [verdict('pro-smart', ['openrouter'])];
      },
      probeProvider: readyProbe,
      now: clock.now,
      healthWaitMs: 10,
    });
    assert.deepEqual(await checker.check(), READY_RESULT);
    held = true;
    clock.advance(30_000);
    assert.deepEqual(await checker.check(), { upstream: true, upstreamReady: false });
    assert.deepEqual(await checker.check(), { upstream: true, upstreamReady: false });
    assert.equal(evaluations, 2);
    clock.advance(30_001);
    assert.deepEqual(await checker.check(), { upstream: false, upstreamReady: false });
    assert.equal(evaluations, 2);
    hold.release();
    assert.deepEqual(await checker.check(), READY_RESULT);
    assert.equal(evaluations, 2);
  });

  test('a timed-out wait never caches its unknown answer, so the next completed evaluation is what callers see', async () => {
    const clock = fakeClock();
    const hold = holdEvaluation();
    let evaluations = 0;
    const checker = createUpstreamReadinessChecker({
      resolveVerdicts: async () => {
        evaluations += 1;
        await hold.gate;
        return [verdict('pro-smart', ['openrouter'], { unverified: true })];
      },
      probeProvider: readyProbe,
      now: clock.now,
      healthWaitMs: 10,
    });
    assert.deepEqual(await checker.check(), { upstream: false, upstreamReady: false });
    hold.release();
    assert.deepEqual(await checker.check(), UNVERIFIED_RESULT);
    clock.advance(4_999);
    assert.deepEqual(await checker.check(), UNVERIFIED_RESULT);
    assert.equal(evaluations, 1);
  });

  test('reset drops the cached result immediately', async () => {
    let evaluations = 0;
    const checker = createUpstreamReadinessChecker({
      resolveVerdicts: async () => {
        evaluations += 1;
        return [verdict('pro-smart', ['openrouter'])];
      },
      probeProvider: readyProbe,
      now: fakeClock().now,
    });
    await checker.check();
    checker.reset();
    await checker.check();
    assert.equal(evaluations, 2);
  });
});
