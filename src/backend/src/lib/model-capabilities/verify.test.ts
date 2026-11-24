import assert from 'node:assert/strict';
import { afterEach, beforeEach, test } from 'node:test';
import { CAPABILITIES_FOR_ROUTE_KIND, type ModelCapability } from './criteria';
import {
  CAPABILITY_REPROBE_AFTER_MS,
  CAPABILITY_VERDICT_MAX_AGE_MS,
  type ModelCapabilityStore,
  passedCapabilitiesByModel,
  resetModelCapabilityStoreForTests,
  useMemoryModelCapabilityStoreForTests,
} from './store';
import { verifyModelForRouteKind } from './verify';

const originalFetch = globalThis.fetch;
const target = { provider: 'opencode_zen', modelId: 'test-model' };
let store: ModelCapabilityStore;
let probeRequests = 0;

function passEveryProbe() {
  globalThis.fetch = (async (_input: string | URL | Request, init?: RequestInit) => {
    probeRequests += 1;
    const body = JSON.parse(String(init?.body));
    const events = body.tools
      ? [{ choices: [{ delta: { tool_calls: [{ index: 0, function: { name: 'multiply', arguments: '{"a":17,"b":23}' } }] } }] }]
      : [{ choices: [{ delta: { content: 'red 31' } }] }];
    const reasoning = body.reasoning_effort === 'medium' ? 40 : 0;
    events.push({ usage: { prompt_tokens: 5, completion_tokens: 50, completion_tokens_details: { reasoning_tokens: reasoning } } } as never);
    return new Response(`${events.map((event) => `data: ${JSON.stringify(event)}\n\n`).join('')}data: [DONE]\n\n`);
  }) as typeof fetch;
}

function verdict(capability: ModelCapability, passed: boolean, probedAt: number) {
  return { ...target, capability, passed, detail: passed ? 'ok' : 'refused', probedAt };
}

beforeEach(() => {
  probeRequests = 0;
  store = useMemoryModelCapabilityStoreForTests();
  process.env.OPENCODE_API_KEY = 'probe-key';
  process.env.OPENCODE_BASE_URL = 'https://upstream.example/v1';
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  resetModelCapabilityStoreForTests();
  delete process.env.OPENCODE_API_KEY;
  delete process.env.OPENCODE_BASE_URL;
});

test('an unprobed model is probed for its route kind and the verdicts are saved', async () => {
  passEveryProbe();
  const verification = await verifyModelForRouteKind(target, 'fast');
  assert.deepEqual(verification, { fits: true, failed: [] });
  assert.ok(probeRequests >= CAPABILITIES_FOR_ROUTE_KIND.fast.length);
  const passed = passedCapabilitiesByModel(await store.loadProvider('opencode_zen'));
  assert.deepEqual([...(passed.get('test-model') ?? [])].sort(), [...CAPABILITIES_FOR_ROUTE_KIND.fast].sort());
});

test('recent verdicts are reused and a recent failure keeps the model out', async () => {
  const now = Date.now();
  await store.save([
    verdict('thinking_off', false, now),
    verdict('quick_first_token', true, now),
    verdict('tool_calls', true, now),
    verdict('stream_usage', true, now),
  ]);
  passEveryProbe();
  const verification = await verifyModelForRouteKind(target, 'fast', now);
  assert.equal(probeRequests, 0);
  assert.equal(verification.fits, false);
  assert.deepEqual(verification.failed, [{ capability: 'thinking_off', detail: 'refused' }]);
});

test('verdicts older than the reprobe window are checked again', async () => {
  const now = Date.now();
  const old = now - CAPABILITY_REPROBE_AFTER_MS - 1;
  await store.save(CAPABILITIES_FOR_ROUTE_KIND.smart.map((capability) => verdict(capability, false, old)));
  passEveryProbe();
  const verification = await verifyModelForRouteKind(target, 'smart', now);
  assert.equal(probeRequests, CAPABILITIES_FOR_ROUTE_KIND.smart.length);
  assert.equal(verification.fits, true);
});

test('expired passes do not count toward the dropdown lists', () => {
  const now = Date.now();
  const passed = passedCapabilitiesByModel(
    [verdict('tool_calls', true, now - CAPABILITY_VERDICT_MAX_AGE_MS - 1), verdict('stream_usage', true, now)],
    now,
  );
  assert.deepEqual([...(passed.get('test-model') ?? [])], ['stream_usage']);
});

test('a temporary upstream failure blocks the save but is not stored as a verdict', async () => {
  globalThis.fetch = (async () => {
    probeRequests += 1;
    return Response.json({ error: { message: 'overloaded' } }, { status: 503 });
  }) as typeof fetch;
  const verification = await verifyModelForRouteKind(target, 'fast');
  assert.equal(verification.fits, false);
  assert.equal(verification.failed.length, CAPABILITIES_FOR_ROUTE_KIND.fast.length);
  assert.deepEqual(await store.loadModel(target.provider, target.modelId), []);
});

test('probe work starts with configured routes and then alternates providers', async () => {
  const { orderProbeWork } = await import('./refresh');
  const item = (provider: string, modelId: string, configured = false, lastProbedAt = 0) => ({
    provider,
    modelId,
    capabilities: CAPABILITIES_FOR_ROUTE_KIND.fast.slice(),
    lastProbedAt,
    configured,
  });
  const ordered = orderProbeWork([
    item('opencode_zen', 'a'),
    item('opencode_zen', 'b'),
    item('opencode_zen', 'c'),
    item('openrouter', 'x', false, 5),
    item('openrouter', 'y'),
    item('openrouter', 'configured', true),
  ]);
  assert.deepEqual(
    ordered.map((work) => work.modelId),
    ['configured', 'a', 'y', 'b', 'x', 'c'],
  );
});
