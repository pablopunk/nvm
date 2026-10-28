import assert from 'node:assert/strict';
import { afterEach, beforeEach, test } from 'node:test';
import { fakeRouteDb, type RouteFixture } from '../../test-support/route-db';
import {
  installUpstreamStub,
  OPENCODE_MODELS_URL,
  OPENROUTER_ACCEPTED_REPLIES,
  OPENROUTER_DECISION_URL,
  OPENROUTER_KEY_URL,
  OPENROUTER_MODELS_URL,
  OPENROUTER_TRANSCRIPTION_URL,
  type UpstreamReplies,
  useUpstreamTestEnv,
} from '../../test-support/upstream-stub';

process.env.DATABASE_URL ??= 'postgres://health-test.invalid/nvm';

const OPENROUTER_MODEL = 'deepseek/deepseek-v4-flash-0731';
const OPENCODE_MODEL = 'gpt-5.6-luna';
const LIVE_PROBES = [OPENROUTER_DECISION_URL, OPENROUTER_KEY_URL, OPENROUTER_MODELS_URL, OPENROUTER_TRANSCRIPTION_URL].sort();

let restoreEnv: () => void;

beforeEach(() => {
  restoreEnv = useUpstreamTestEnv();
});

afterEach(() => {
  restoreEnv();
});

function route(provider: string, modelId: string): string {
  return JSON.stringify({ provider, modelId, thinkingLevel: 'off' });
}

// Mirrors production: every tiered slot, audio, and decision on OpenRouter.
const OPENROUTER_LIVE_SETTINGS = {
  pro_smart_model_route: route('openrouter', OPENROUTER_MODEL),
  pro_fast_model_route: route('openrouter', OPENROUTER_MODEL),
  free_smart_model_route: route('openrouter', OPENROUTER_MODEL),
  free_fast_model_route: route('openrouter', OPENROUTER_MODEL),
  audio_model_route: route('openrouter', 'nvidia/parakeet-tdt-0.6b-v3'),
  decision_model_route: route('openrouter', 'typesafe/jev-1.14'),
};

async function healthFor(fixture: RouteFixture, replies: UpstreamReplies = OPENROUTER_ACCEPTED_REPLIES) {
  const [{ GET }, { withTestDb }] = await Promise.all([import('./health'), import('../../db/client')]);
  const requested = installUpstreamStub(replies);
  const response = await withTestDb(fakeRouteDb(fixture), () => GET({} as Parameters<typeof GET>[0]));
  const body = (await response.json()) as Record<string, unknown>;
  return { status: response.status, body, requested: [...requested].sort() };
}

function readinessOf(body: Record<string, unknown>) {
  return {
    ok: body.ok,
    db: body.db,
    upstream: body.upstream,
    upstreamReady: body.upstreamReady,
  };
}

test('an openrouter-only deployment is live and ready without OPENCODE_API_KEY and never probes opencode', async () => {
  const response = await healthFor({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(response.status, 200);
  assert.deepEqual(readinessOf(response.body), { ok: true, db: true, upstream: true, upstreamReady: true });
  assert.deepEqual(response.requested, LIVE_PROBES);
});

test('a rejected openrouter key keeps the deployment live but not ready', async () => {
  const response = await healthFor(
    { settings: OPENROUTER_LIVE_SETTINGS },
    { ...OPENROUTER_ACCEPTED_REPLIES, [OPENROUTER_KEY_URL]: { status: 401, body: '{"error":"invalid"}' } },
  );
  assert.equal(response.status, 200);
  assert.deepEqual(readinessOf(response.body), { ok: true, db: true, upstream: true, upstreamReady: false });
});

test('a 5xx from the openrouter key check fails liveness and readiness', async () => {
  const response = await healthFor(
    { settings: OPENROUTER_LIVE_SETTINGS },
    { ...OPENROUTER_ACCEPTED_REPLIES, [OPENROUTER_KEY_URL]: { status: 503, body: '{}' } },
  );
  assert.equal(response.status, 503);
  assert.deepEqual(readinessOf(response.body), { ok: false, db: true, upstream: false, upstreamReady: false });
});

test('a 2xx html page from the catalog is live but not a model catalog', async () => {
  const response = await healthFor(
    { settings: OPENROUTER_LIVE_SETTINGS },
    {
      ...OPENROUTER_ACCEPTED_REPLIES,
      [OPENROUTER_MODELS_URL]: { status: 200, contentType: 'text/html', body: '<!doctype html><html></html>' },
    },
  );
  assert.equal(response.status, 200);
  assert.equal(response.body.upstream, true);
  assert.equal(response.body.upstreamReady, false);
});

test('an unresponsive live provider is unreachable, so the deployment is 503', async () => {
  const response = await healthFor(
    { settings: OPENROUTER_LIVE_SETTINGS },
    { ...OPENROUTER_ACCEPTED_REPLIES, [OPENROUTER_MODELS_URL]: 'timeout' },
  );
  assert.equal(response.status, 503);
  assert.deepEqual(readinessOf(response.body), { ok: false, db: true, upstream: false, upstreamReady: false });
});

test('a keyless failover link is skipped as the proxy skips it and never fails the deployment', async () => {
  const response = await healthFor({
    settings: OPENROUTER_LIVE_SETTINGS,
    chain: [{ routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'opencode_zen', providerModelId: OPENCODE_MODEL }],
  });
  assert.equal(response.status, 200);
  assert.deepEqual(readinessOf(response.body), { ok: true, db: true, upstream: true, upstreamReady: true });
  assert.equal(
    response.requested.some((url) => url.startsWith('https://opencode.test')),
    false,
  );
});

test('a keyed failover link that the proxy would try must be ready, but it cannot make the route dead', async () => {
  process.env.OPENCODE_API_KEY = 'test-opencode-key';
  const response = await healthFor(
    {
      settings: OPENROUTER_LIVE_SETTINGS,
      chain: [{ routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'opencode_zen', providerModelId: OPENCODE_MODEL }],
    },
    {
      ...OPENROUTER_ACCEPTED_REPLIES,
      [OPENCODE_MODELS_URL]: { status: 401, body: '{"error":"rejected"}' },
    },
  );
  assert.equal(response.status, 200);
  assert.deepEqual(readinessOf(response.body), { ok: true, db: true, upstream: true, upstreamReady: false });
});

test('a legacy primary without a stored route resolves to opencode_zen and is refused without its key', async () => {
  const response = await healthFor({
    settings: {
      active_model: OPENCODE_MODEL,
      free_model: OPENCODE_MODEL,
      audio_model_route: OPENROUTER_LIVE_SETTINGS.audio_model_route,
      decision_model_route: OPENROUTER_LIVE_SETTINGS.decision_model_route,
    },
  });
  assert.equal(response.status, 503);
  assert.equal(response.body.upstreamReady, false);
  assert.equal(
    response.requested.some((url) => url.startsWith('https://opencode.test')),
    false,
  );
});

test('a partially configured deployment fails closed while its configured routes still answer', async () => {
  const response = await healthFor({
    settings: { pro_smart_model_route: route('openrouter', OPENROUTER_MODEL) },
  });
  assert.equal(response.status, 503);
  assert.deepEqual(readinessOf(response.body), { ok: false, db: true, upstream: false, upstreamReady: false });
});

test('absent route settings fail closed without probing any provider', async () => {
  const response = await healthFor({ settings: {} });
  assert.equal(response.status, 503);
  assert.deepEqual(readinessOf(response.body), { ok: false, db: true, upstream: false, upstreamReady: false });
  assert.deepEqual(response.requested, []);
});

test('a route lookup failure is 503 while the database check still reports live', async () => {
  const response = await healthFor({ failLookups: new Error('connection reset') });
  assert.equal(response.status, 503);
  assert.deepEqual(readinessOf(response.body), { ok: false, db: true, upstream: false, upstreamReady: false });
  assert.deepEqual(response.requested, []);
});

test('repeat checks within the TTL reuse the provider probes, while the database is checked on every call', async () => {
  const first = { count: 0 };
  const healthy = await healthFor({ settings: OPENROUTER_LIVE_SETTINGS, executeCalls: first });
  assert.equal(healthy.status, 200);
  assert.equal(first.count, 1);
  assert.ok(healthy.requested.length > 0);

  const degradedDatabase = await healthFor({
    settings: OPENROUTER_LIVE_SETTINGS,
    failExecute: new Error('database unavailable'),
  });
  assert.equal(degradedDatabase.status, 503);
  assert.deepEqual(readinessOf(degradedDatabase.body), { ok: false, db: false, upstream: true, upstreamReady: true });
  assert.deepEqual(degradedDatabase.requested, []);
});

test('the ai_proxy kill switch removes the AI routes, leaving audio probed and ready', async () => {
  process.env.NEVERMIND_KILL_SWITCHES = 'ai_proxy';
  const response = await healthFor({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(response.status, 200);
  assert.deepEqual(readinessOf(response.body), { ok: true, db: true, upstream: true, upstreamReady: true });
  assert.deepEqual(
    response.requested,
    [OPENROUTER_KEY_URL, OPENROUTER_MODELS_URL, OPENROUTER_TRANSCRIPTION_URL].sort(),
  );
});

test('with every live route switched off the deployment is live but never proven ready', async () => {
  process.env.NEVERMIND_KILL_SWITCHES = 'ai_proxy,audio_transcription';
  const response = await healthFor({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(response.status, 200);
  assert.deepEqual(readinessOf(response.body), { ok: true, db: true, upstream: true, upstreamReady: false });
  assert.deepEqual(response.requested, []);
});

test('an audio model missing from its catalog stays live for chat but fails readiness closed', async () => {
  const response = await healthFor(
    { settings: OPENROUTER_LIVE_SETTINGS },
    { ...OPENROUTER_ACCEPTED_REPLIES, [OPENROUTER_TRANSCRIPTION_URL]: { status: 200, body: '{"data":[]}' } },
  );
  assert.equal(response.status, 200);
  assert.deepEqual(readinessOf(response.body), { ok: true, db: true, upstream: true, upstreamReady: false });
});

test('a deployment without an audio route is live and never ready, and never probes audio', async () => {
  const settings: Record<string, string> = { ...OPENROUTER_LIVE_SETTINGS };
  delete settings.audio_model_route;
  const response = await healthFor({ settings });
  assert.equal(response.status, 200);
  assert.deepEqual(readinessOf(response.body), { ok: true, db: true, upstream: true, upstreamReady: false });
  assert.equal(response.requested.includes(OPENROUTER_TRANSCRIPTION_URL), false);
});

test('a deployment without a decision route is live and never ready while chat still answers', async () => {
  const settings: Record<string, string> = { ...OPENROUTER_LIVE_SETTINGS };
  delete settings.decision_model_route;
  const response = await healthFor({ settings });
  assert.equal(response.status, 200);
  assert.deepEqual(readinessOf(response.body), { ok: true, db: true, upstream: true, upstreamReady: false });
});

test('a disabled OpenRouter provider refuses audio, so the deployment is live but never ready', async () => {
  const response = await healthFor({
    settings: OPENROUTER_LIVE_SETTINGS,
    providers: { openrouter: { enabled: false } },
  });
  assert.equal(response.status, 200);
  assert.deepEqual(readinessOf(response.body), { ok: true, db: true, upstream: true, upstreamReady: false });
});
