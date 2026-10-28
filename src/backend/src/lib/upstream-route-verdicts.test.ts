import assert from 'node:assert/strict';
import { afterEach, beforeEach, test } from 'node:test';
import { withTestDb } from '../db/client';
import { fakeRouteDb, type RouteFixture } from '../test-support/route-db';
import {
  installUpstreamStub,
  MODELS_DEV_URL,
  OPENROUTER_ACCEPTED_REPLIES,
  OPENROUTER_DECISION_URL,
  OPENROUTER_TRANSCRIPTION_URL,
  type UpstreamReplies,
  useUpstreamTestEnv,
} from '../test-support/upstream-stub';
import { type LiveRouteVerdict, resolveLiveRouteVerdicts } from './upstream-route-verdicts';

process.env.DATABASE_URL ??= 'postgres://route-verdicts-test.invalid/nvm';

const AUDIO = 'audio_model_route';
const DECISION = 'decision_model_route';
const TIERED = ['free-smart', 'free-fast', 'pro-smart', 'pro-fast'];
// Priced by the bundled catalog, so pricing never depends on a network answer.
const OPENROUTER_MODEL = 'deepseek/deepseek-v4-flash-0731';
const OPENCODE_MODEL = 'gpt-5.6-luna';
const OPENROUTER_CANDIDATE = { provider: 'openrouter', modelId: OPENROUTER_MODEL };

let restoreEnv: () => void;

beforeEach(() => {
  restoreEnv = useUpstreamTestEnv();
  installUpstreamStub(OPENROUTER_ACCEPTED_REPLIES);
});

afterEach(() => {
  restoreEnv();
});

function route(provider: string, modelId: string): string {
  return JSON.stringify({ provider, modelId, thinkingLevel: 'off' });
}

const OPENROUTER_LIVE_SETTINGS = {
  pro_smart_model_route: route('openrouter', OPENROUTER_MODEL),
  pro_fast_model_route: route('openrouter', OPENROUTER_MODEL),
  free_smart_model_route: route('openrouter', OPENROUTER_MODEL),
  free_fast_model_route: route('openrouter', OPENROUTER_MODEL),
  audio_model_route: route('openrouter', 'nvidia/parakeet-tdt-0.6b-v3'),
  decision_model_route: route('openrouter', 'typesafe/jev-1.14'),
};

function resolveWith(fixture: RouteFixture): Promise<LiveRouteVerdict[]> {
  return Promise.resolve(withTestDb(fakeRouteDb(fixture), () => resolveLiveRouteVerdicts()));
}

function verdictFor(verdicts: LiveRouteVerdict[], route: string): LiveRouteVerdict {
  const found = verdicts.find((verdict) => verdict.route === route);
  assert.ok(found, `expected a live verdict for ${route}`);
  return found;
}

test('an openrouter-only deployment serves every live route without any OpenCode key', async () => {
  const verdicts = await resolveWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.deepEqual(
    verdicts.map((verdict) => verdict.route),
    [...TIERED, AUDIO, DECISION],
  );
  for (const verdict of verdicts) {
    assert.equal(verdict.unserviceable, null, verdict.route);
    assert.equal(verdict.unverified, false, verdict.route);
  }
  for (const slot of TIERED) assert.deepEqual(verdictFor(verdicts, slot).candidates, [OPENROUTER_CANDIDATE], slot);
  assert.deepEqual(verdictFor(verdicts, AUDIO).candidates, [
    { provider: 'openrouter', modelId: 'nvidia/parakeet-tdt-0.6b-v3' },
  ]);
  assert.deepEqual(verdictFor(verdicts, DECISION).candidates, [
    { provider: 'openrouter', modelId: 'typesafe/jev-1.14' },
  ]);
});

test('a keyless failover link is skipped, as the proxy skips it, and never fails the route', async () => {
  const verdicts = await resolveWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    chain: [
      { routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'opencode_zen', providerModelId: OPENCODE_MODEL },
    ],
  });
  const proSmart = verdictFor(verdicts, 'pro-smart');
  assert.equal(proSmart.unserviceable, null);
  assert.equal(proSmart.unverified, false);
  assert.deepEqual(proSmart.candidates, [OPENROUTER_CANDIDATE]);
});

test('a keyed failover link the proxy would try becomes a candidate after the primary', async () => {
  process.env.OPENCODE_API_KEY = 'test-opencode-key';
  const verdicts = await resolveWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    chain: [
      { routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'opencode_zen', providerModelId: OPENCODE_MODEL },
    ],
  });
  assert.deepEqual(verdictFor(verdicts, 'pro-smart').candidates, [
    OPENROUTER_CANDIDATE,
    { provider: 'opencode_zen', modelId: OPENCODE_MODEL },
  ]);
  assert.deepEqual(verdictFor(verdicts, 'pro-fast').candidates, [OPENROUTER_CANDIDATE]);
});

test('a failover link the proxy skips for an unpriced model or an incompatible wire format is excluded', async () => {
  process.env.OPENCODE_API_KEY = 'test-opencode-key';
  process.env.ANTHROPIC_API_KEY = 'test-anthropic-key';
  installUpstreamStub({
    ...OPENROUTER_ACCEPTED_REPLIES,
    [MODELS_DEV_URL]: {
      status: 200,
      body: JSON.stringify({
        anthropic: {
          models: {
            'claude-sonnet-probe': {
              id: 'claude-sonnet-probe',
              cost: { input: 3, output: 15 },
              provider: { npm: '@ai-sdk/anthropic' },
            },
          },
        },
      }),
    },
  });
  const verdicts = await resolveWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    chain: [
      { routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'opencode_zen', providerModelId: 'unpriced-model' },
      { routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'anthropic', providerModelId: 'claude-sonnet-probe' },
    ],
  });
  const proSmart = verdictFor(verdicts, 'pro-smart');
  assert.equal(proSmart.unserviceable, null);
  assert.equal(proSmart.unverified, false);
  assert.deepEqual(proSmart.candidates, [OPENROUTER_CANDIDATE]);
});

test('the ai_failover kill switch ignores the failover chain entirely, as the proxy does', async () => {
  process.env.OPENCODE_API_KEY = 'test-opencode-key';
  process.env.NEVERMIND_KILL_SWITCHES = 'ai_failover';
  const verdicts = await resolveWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    chain: [
      { routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'opencode_zen', providerModelId: OPENCODE_MODEL },
    ],
  });
  assert.deepEqual(verdictFor(verdicts, 'pro-smart').candidates, [OPENROUTER_CANDIDATE]);
});

test('a failing failover lookup keeps each tiered route live on its primary but unverified', async () => {
  const verdicts = await resolveWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    failChain: new Error('chain query failed'),
  });
  for (const slot of TIERED) {
    const verdict = verdictFor(verdicts, slot);
    assert.equal(verdict.unserviceable, null, slot);
    assert.equal(verdict.unverified, true, slot);
    assert.deepEqual(verdict.candidates, [OPENROUTER_CANDIDATE], slot);
  }
  assert.equal(verdictFor(verdicts, AUDIO).unverified, false);
});

test('a failover link the breaker has tripped is skipped, as the proxy skips it', async () => {
  process.env.OPENCODE_API_KEY = 'test-opencode-key';
  const verdicts = await resolveWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    chain: [
      { routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'opencode_zen', providerModelId: OPENCODE_MODEL },
    ],
    breakers: [{ routeSlot: 'pro-smart', provider: 'opencode_zen', modelId: OPENCODE_MODEL }],
  });
  assert.deepEqual(verdictFor(verdicts, 'pro-smart').candidates, [OPENROUTER_CANDIDATE]);
});

test('a tripped primary falls to an eligible failover, and is refused when no candidate remains', async () => {
  process.env.OPENCODE_API_KEY = 'test-opencode-key';
  const verdicts = await resolveWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    chain: [
      { routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'opencode_zen', providerModelId: OPENCODE_MODEL },
    ],
    breakers: [
      { routeSlot: 'pro-smart', provider: 'openrouter', modelId: OPENROUTER_MODEL },
      { routeSlot: 'pro-fast', provider: 'openrouter', modelId: OPENROUTER_MODEL },
    ],
  });
  assert.deepEqual(verdictFor(verdicts, 'pro-smart').candidates, [
    { provider: 'opencode_zen', modelId: OPENCODE_MODEL },
  ]);
  assert.equal(verdictFor(verdicts, 'pro-fast').unserviceable, 'no_eligible_candidate');
});

test('a tripped audio route is refused, as the audio endpoint refuses it', async () => {
  const verdicts = await resolveWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    breakers: [{ routeSlot: 'audio', provider: 'openrouter', modelId: 'nvidia/parakeet-tdt-0.6b-v3' }],
  });
  assert.equal(verdictFor(verdicts, AUDIO).unserviceable, 'route_tripped');
});

test('a primary without a key refuses the route before any failover, as the proxy does', async () => {
  const verdicts = await resolveWith({
    settings: {
      ...OPENROUTER_LIVE_SETTINGS,
      pro_fast_model_route: route('opencode_zen', OPENCODE_MODEL),
    },
    chain: [
      { routeSlot: 'pro-fast', modelId: OPENCODE_MODEL, providerId: 'openrouter', providerModelId: OPENROUTER_MODEL },
    ],
  });
  const proFast = verdictFor(verdicts, 'pro-fast');
  assert.equal(proFast.unserviceable, 'provider_unkeyed');
  assert.deepEqual(proFast.candidates, []);
});

test('a primary without pricing refuses the route as unavailable', async () => {
  const verdicts = await resolveWith({
    settings: { ...OPENROUTER_LIVE_SETTINGS, pro_fast_model_route: route('openrouter', 'unpriced-model') },
  });
  assert.equal(verdictFor(verdicts, 'pro-fast').unserviceable, 'model_unavailable');
  assert.equal(verdictFor(verdicts, 'pro-smart').unserviceable, null);
});

test('a primary whose wire format the provider lacks falls through to a failover that speaks that format', async () => {
  process.env.OPENCODE_API_KEY = 'test-opencode-key';
  installUpstreamStub({
    ...OPENROUTER_ACCEPTED_REPLIES,
    [MODELS_DEV_URL]: {
      status: 200,
      body: JSON.stringify({
        openrouter: {
          models: {
            [OPENROUTER_MODEL]: { id: OPENROUTER_MODEL, cost: { input: 1, output: 2 }, provider: { npm: '@ai-sdk/anthropic' } },
          },
        },
        opencode: {
          models: {
            [OPENCODE_MODEL]: { id: OPENCODE_MODEL, cost: { input: 1, output: 2 }, provider: { npm: '@ai-sdk/anthropic' } },
          },
        },
      }),
    },
  });
  const verdicts = await resolveWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    chain: [
      { routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'opencode_zen', providerModelId: OPENCODE_MODEL },
    ],
  });
  const proSmart = verdictFor(verdicts, 'pro-smart');
  assert.equal(proSmart.unserviceable, null);
  assert.deepEqual(proSmart.candidates, [{ provider: 'opencode_zen', modelId: OPENCODE_MODEL }]);
});

test('a primary whose format no candidate supports is unserviceable, not silently live', async () => {
  installUpstreamStub({
    ...OPENROUTER_ACCEPTED_REPLIES,
    [MODELS_DEV_URL]: {
      status: 200,
      body: JSON.stringify({
        openrouter: {
          models: {
            [OPENROUTER_MODEL]: {
              id: OPENROUTER_MODEL,
              cost: { input: 1, output: 2 },
              provider: { npm: '@ai-sdk/anthropic' },
            },
          },
        },
      }),
    },
  });
  const verdicts = await resolveWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(verdictFor(verdicts, 'pro-smart').unserviceable, 'no_eligible_candidate');
});

test('a missing live slot is route_unconfigured while the other routes still resolve', async () => {
  const settings: Record<string, string> = { ...OPENROUTER_LIVE_SETTINGS };
  delete settings.pro_fast_model_route;
  const verdicts = await resolveWith({ settings });
  assert.equal(verdictFor(verdicts, 'pro-fast').unserviceable, 'route_unconfigured');
  assert.equal(verdictFor(verdicts, 'pro-smart').unserviceable, null);
});

test('a route lookup error refuses every live route as route_lookup_failed, never a default provider', async () => {
  const verdicts = await resolveWith({ failLookups: new Error('connection reset') });
  assert.deepEqual(
    verdicts.map((verdict) => [verdict.route, verdict.unserviceable]),
    [...TIERED, AUDIO, DECISION].map((route) => [route, 'route_lookup_failed']),
  );
});

test('a disabled OpenRouter provider refuses the audio route, as the endpoint does, and leaves chat serviceable', async () => {
  const verdicts = await resolveWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    providers: { openrouter: { enabled: false } },
  });
  assert.deepEqual(verdictFor(verdicts, AUDIO), {
    route: AUDIO,
    candidates: [],
    unserviceable: 'provider_disabled',
    unverified: false,
  });
  assert.equal(verdictFor(verdicts, 'pro-smart').unserviceable, null);
});

test('a missing audio route is route_unconfigured and a missing decision route is too, while chat still resolves', async () => {
  const settings: Record<string, string> = { ...OPENROUTER_LIVE_SETTINGS };
  delete settings[AUDIO];
  delete settings[DECISION];
  const verdicts = await resolveWith({ settings });
  assert.equal(verdictFor(verdicts, AUDIO).unserviceable, 'route_unconfigured');
  assert.equal(verdictFor(verdicts, DECISION).unserviceable, 'route_unconfigured');
  assert.equal(verdictFor(verdicts, 'pro-smart').unserviceable, null);
});

test('an unreadable breaker keeps the tiered route serviceable but unverified, never silently clear', async () => {
  const verdicts = await resolveWith({ settings: OPENROUTER_LIVE_SETTINGS, failBreakers: new Error('breaker read failed') });
  const proSmart = verdictFor(verdicts, 'pro-smart');
  assert.equal(proSmart.unserviceable, null);
  assert.equal(proSmart.unverified, true);
  assert.deepEqual(proSmart.candidates, [OPENROUTER_CANDIDATE]);
});

test('the audio_transcription kill switch removes the audio route from the live set', async () => {
  process.env.NEVERMIND_KILL_SWITCHES = 'audio_transcription';
  const verdicts = await resolveWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(
    verdicts.some((verdict) => verdict.route === AUDIO),
    false,
  );
  assert.equal(verdicts.length, TIERED.length + 1);
});

test('an audio route on a provider other than OpenRouter is unsupported', async () => {
  const verdicts = await resolveWith({
    settings: { ...OPENROUTER_LIVE_SETTINGS, audio_model_route: route('anthropic', 'claude-audio') },
  });
  assert.equal(verdictFor(verdicts, AUDIO).unserviceable, 'provider_unsupported');
});

test('an audio route without an OpenRouter key is refused with the key reason', async () => {
  delete process.env.OPENROUTER_API_KEY;
  const verdicts = await resolveWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(verdictFor(verdicts, AUDIO).unserviceable, 'provider_unkeyed');
});

test('an audio model missing from the transcription catalog is unavailable', async () => {
  installUpstreamStub({
    ...OPENROUTER_ACCEPTED_REPLIES,
    [OPENROUTER_TRANSCRIPTION_URL]: { status: 200, body: '{"data":[]}' },
  });
  const verdicts = await resolveWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(verdictFor(verdicts, AUDIO).unserviceable, 'model_unavailable');
});

test('a transcription catalog failure leaves audio live and unverified, never unserviceable', async () => {
  installUpstreamStub({
    ...OPENROUTER_ACCEPTED_REPLIES,
    [OPENROUTER_TRANSCRIPTION_URL]: { status: 503, body: '{}' },
  });
  const verdicts = await resolveWith({ settings: OPENROUTER_LIVE_SETTINGS });
  const audio = verdictFor(verdicts, AUDIO);
  assert.equal(audio.unserviceable, null);
  assert.equal(audio.unverified, true);
  assert.deepEqual(audio.candidates, [{ provider: 'openrouter', modelId: 'nvidia/parakeet-tdt-0.6b-v3' }]);
});

test('a decision route on a provider without an adapter is unsupported', async () => {
  const verdicts = await resolveWith({
    settings: { ...OPENROUTER_LIVE_SETTINGS, decision_model_route: route('anthropic', 'decision-model') },
  });
  assert.equal(verdictFor(verdicts, DECISION).unserviceable, 'provider_unsupported');
});

test('the ai_proxy kill switch removes every tiered and decision route and keeps audio', async () => {
  process.env.NEVERMIND_KILL_SWITCHES = 'ai_proxy';
  const verdicts = await resolveWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.deepEqual(
    verdicts.map((verdict) => verdict.route),
    [AUDIO],
  );
});

test('a decision model missing from the decision catalog is unavailable', async () => {
  installUpstreamStub({
    ...OPENROUTER_ACCEPTED_REPLIES,
    [OPENROUTER_DECISION_URL]: { status: 200, body: '{"data":[]}' },
  });
  const verdicts = await resolveWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(verdictFor(verdicts, DECISION).unserviceable, 'model_unavailable');
});

test('a decision model with a charged output has no bounded cost and is unavailable', async () => {
  const charged: UpstreamReplies = {
    ...OPENROUTER_ACCEPTED_REPLIES,
    [OPENROUTER_DECISION_URL]: {
      status: 200,
      body: JSON.stringify({
        data: [
          {
            id: 'typesafe/jev-1.14',
            context_length: 262144,
            pricing: { prompt: '0.000000042', completion: '0.000001', image: '0', request: '0' },
            architecture: { input_modalities: ['text', 'image'], output_modalities: ['decisions'] },
          },
        ],
      }),
    },
  };
  installUpstreamStub(charged);
  const verdicts = await resolveWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(verdictFor(verdicts, DECISION).unserviceable, 'model_unavailable');
});
