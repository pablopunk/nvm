import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { afterEach, beforeEach, test } from 'node:test';
import { withTestDb } from '../db/client';
import { fakeRouteDb, type RouteFixture } from '../test-support/route-db';
import {
  installUpstreamStub,
  OPENROUTER_ACCEPTED_REPLIES,
  useUpstreamTestEnv,
} from '../test-support/upstream-stub';
import {
  expirePricingCacheForTests,
  resetPricingCacheForTests,
} from './pricing';
import { absorbedOverrunSettingKey } from './pricing-overrun-cap';
import {
  type PricingReadinessReport,
  type RoutePricingCheck,
  summarizePricingReadiness,
  verifyLivePricingReadiness,
} from './pricing-readiness';
import { SNAPSHOT_CURRENT_MAX_AGE_MS } from './pricing-snapshot-store';

process.env.DATABASE_URL ??= 'postgres://pricing-readiness-test.invalid/nvm';

const TIERED_ROUTES = ['free-smart', 'free-fast', 'pro-smart', 'pro-fast'];
const OPENROUTER_MODEL = 'deepseek/deepseek-v4-flash-0731';

let restoreEnv: () => void;

beforeEach(async () => {
  restoreEnv = useUpstreamTestEnv();
  await installUpstreamStub(OPENROUTER_ACCEPTED_REPLIES);
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

function verifyWith(fixture: RouteFixture): Promise<PricingReadinessReport> {
  return withTestDb(fakeRouteDb(fixture), () => verifyLivePricingReadiness());
}

function checkFor(report: PricingReadinessReport, slot: string, provider?: string): RoutePricingCheck {
  const found = report.checks.find(
    (check) => check.route === slot && (provider === undefined || check.provider === provider),
  );
  assert.ok(found, `expected a pricing check for ${slot}`);
  return found;
}

test('every configured route admits a current price once the refresh has published its snapshots', async () => {
  const report = await verifyWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(report.ready, true);
  assert.deepEqual(
    report.checks.map((check) => check.route),
    [...TIERED_ROUTES, 'audio_model_route', 'decision_model_route'],
  );
  for (const check of report.checks) assert.equal(check.refusal, null, check.route);
});

test('a deployment with no durable snapshots is not ready, so its deploy is held back', async () => {
  resetPricingCacheForTests();
  const report = await verifyWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(report.ready, false);
  for (const slot of TIERED_ROUTES) assert.equal(checkFor(report, slot).refusal, 'unverified', slot);
  assert.equal(checkFor(report, 'audio_model_route').refusal, 'model_unavailable');
  assert.equal(checkFor(report, 'decision_model_route').refusal, 'model_unavailable');
});

test('a stale price is refused by the gate, although the request path still admits it', async () => {
  expirePricingCacheForTests(SNAPSHOT_CURRENT_MAX_AGE_MS + 1);
  const report = await verifyWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.equal(report.ready, false);
  for (const slot of TIERED_ROUTES) assert.equal(checkFor(report, slot).refusal, 'stale', slot);
  assert.equal(checkFor(report, 'audio_model_route').refusal, 'stale');
  assert.equal(checkFor(report, 'decision_model_route').refusal, 'stale');
});

test('a reached overrun cap keeps every stale route reported unready, so the gate does not conceal the refusal', async () => {
  expirePricingCacheForTests(SNAPSHOT_CURRENT_MAX_AGE_MS + 1);
  const capReached = { ...OPENROUTER_LIVE_SETTINGS, [absorbedOverrunSettingKey(new Date())]: '500' };
  const report = await verifyWith({ settings: capReached });
  assert.equal(report.ready, false);
  for (const slot of TIERED_ROUTES) assert.equal(checkFor(report, slot).refusal, 'stale', slot);
  // Audio and decision lookups return no price once the cap is reached, so they report as
  // unavailable rather than stale. Either way the route is unready and the gate fails.
  assert.notEqual(checkFor(report, 'audio_model_route').refusal, null);
  assert.notEqual(checkFor(report, 'decision_model_route').refusal, null);
});

test('a reached overrun cap does not hold back a deployment whose prices are all current', async () => {
  const capReached = { ...OPENROUTER_LIVE_SETTINGS, [absorbedOverrunSettingKey(new Date())]: '500' };
  const report = await verifyWith({ settings: capReached });
  assert.equal(report.ready, true);
});

test('an unpriced failover link blocks readiness until it is removed or priced', async () => {
  const report = await verifyWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    chain: [
      { routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'opencode_zen', providerModelId: 'unpriced-model' },
    ],
  });
  assert.equal(report.ready, false);
  assert.equal(checkFor(report, 'pro-smart', 'opencode_zen').refusal, 'unpriced');
  assert.equal(checkFor(report, 'pro-smart', 'openrouter').refusal, null);
});

test('a failover link priced inside the bundle window is current and does not block', async () => {
  const report = await verifyWith({
    settings: OPENROUTER_LIVE_SETTINGS,
    chain: [
      { routeSlot: 'pro-smart', modelId: OPENROUTER_MODEL, providerId: 'opencode_zen', providerModelId: 'gpt-5.6-luna' },
    ],
  });
  assert.equal(report.ready, true);
  assert.equal(checkFor(report, 'pro-smart', 'opencode_zen').refusal, null);
});

test('the ai_proxy kill switch removes the tiered and decision routes, and keeps audio, as requests do', async () => {
  process.env.NEVERMIND_KILL_SWITCHES = 'ai_proxy';
  const report = await verifyWith({ settings: OPENROUTER_LIVE_SETTINGS });
  assert.deepEqual(
    report.checks.map((check) => check.route),
    ['audio_model_route'],
  );
  assert.equal(report.ready, true);
});

test('readiness is held only while every checked route admits a current price', () => {
  const current: RoutePricingCheck = { route: 'pro-smart', provider: 'openrouter', modelId: OPENROUTER_MODEL, refusal: null };
  assert.equal(summarizePricingReadiness([current]).ready, true);
  assert.equal(summarizePricingReadiness([current, { ...current, refusal: 'stale' }]).ready, false);
  assert.equal(summarizePricingReadiness([]).ready, true);
});

test('the deploy workflow seeds and verifies pricing after migrations and before any production deploy', () => {
  const workflow = fs.readFileSync(
    path.resolve(import.meta.dirname, '../../../../.github/workflows/database-migration.yml'),
    'utf8',
  );
  const migrate = workflow.indexOf('name: Apply migrations');
  const seed = workflow.indexOf('name: Seed and verify pricing snapshots');
  const deploy = workflow.indexOf('name: Deploy production');
  assert.ok(migrate >= 0 && seed > migrate, 'seed must follow migrations');
  assert.ok(deploy > seed, 'seed must precede the production deploy');
  assert.match(workflow.slice(seed, deploy), /db:seed-pricing/);
});
