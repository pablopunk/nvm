import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { ModelCost } from '../cost';
import { computeUsdCost } from '../cost';
import type { DecisionAccount } from './decision-account';
import { decisionReservationCostUsd } from './decision-billing';
import { runDecision } from './decision-execution';
import type { DecisionModel } from './decision-models';
import { type DecisionRequest, parseDecisionRequest } from './decision-request';

const COST: ModelCost = {
  provider: 'openrouter',
  modelId: 'typesafe/jev-1.13',
  inputUsdPerMtok: 1,
  outputUsdPerMtok: 0,
};
const ACCOUNT: DecisionAccount = {
  user: { id: 'user-1', plan: 'pro' },
  kind: 'paid',
};
const ROUTE = { provider: 'openrouter', modelId: 'typesafe/jev-1.13' };
const UPSTREAM = { baseUrl: 'https://openrouter.ai/api/v1', apiKey: 'key' };
const PNG_DATA = 'iVBORw0KGgo=';

function requestFixture(images = false): DecisionRequest {
  const parsed = parseDecisionRequest({
    input: 'Checkout is broken.',
    ...(images ? { images: [{ mediaType: 'image/png', data: PNG_DATA }] } : {}),
    questions: [{ id: 'is_bug', type: 'predicate', question: 'Is it a bug?' }],
  });
  if (!parsed.ok) throw new Error(parsed.message);
  return parsed.request;
}

function providerResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

const VALID_BODY = {
  model: 'typesafe/jev-1.13',
  answers: { is_bug: { type: 'noul', noul: 0.8 } },
  usage: { input_tokens: 100, output_tokens: 5, cost: 0.0003 },
};

function fixtureDeps(options: {
  fetchResponse?: () => Response | Promise<Response>;
  reserveOk?: boolean;
  textOnlyModel?: boolean;
}) {
  const calls = {
    reserve: [] as unknown[],
    finalize: [] as unknown[],
    fetch: [] as Array<{ url: string; init: RequestInit }>,
  };
  const model: DecisionModel = {
    provider: 'openrouter',
    modelId: ROUTE.modelId,
    name: 'Jev',
    acceptsImages: options.textOnlyModel !== true,
    contextLength: 262_144,
    imageTokenPriced: true,
  };
  const deps = {
    reserve: (async (input: unknown) => {
      calls.reserve.push(input);
      return options.reserveOk === false
        ? { ok: false, reason: 'insufficient_credits', balance: 1, reserved: 1 }
        : { ok: true, reservation: {}, balance: 100, reserved: 1 };
    }) as never,
    finalize: (async (input: unknown) => {
      calls.finalize.push(input);
      return 'settled';
    }) as never,
    lookupCost: async () => COST,
    findModel: async () => model,
    upstream: () => UPSTREAM,
    fetchImpl: async (url: string, init: RequestInit) => {
      calls.fetch.push({ url, init });
      return options.fetchResponse
        ? options.fetchResponse()
        : providerResponse(VALID_BODY);
    },
  };
  return { deps, calls };
}

test('reserves the ceiling, calls the native endpoint once, and settles provider cost', async () => {
  const { deps, calls } = fixtureDeps({});
  const response = await runDecision({
    requestId: 'req-1',
    account: ACCOUNT,
    route: ROUTE,
    request: requestFixture(),
    deps,
  });
  assert.equal(response.status, 200);
  const body = await response.json();
  assert.equal(body.id, 'req-1');
  assert.deepEqual(body.answers.is_bug, {
    type: 'predicate',
    value: true,
    probability: 0.8,
  });
  assert.equal(calls.reserve.length, 1);
  assert.equal(calls.fetch.length, 1);
  assert.equal(calls.fetch[0].url, 'https://openrouter.ai/api/alpha/decisions');
  assert.equal(
    (calls.fetch[0].init.headers as Record<string, string>).authorization,
    'Bearer key',
  );
  assert.ok(calls.fetch[0].init.signal);
  assert.equal(calls.finalize.length, 1);
  const settlement = calls.finalize[0] as Record<string, unknown>;
  assert.equal(settlement.outcome, 'settle');
  assert.equal(settlement.expectedUserId, 'user-1');
  assert.deepEqual(settlement.tokens, { inputTokens: 100, outputTokens: 5 });
  assert.equal(settlement.providerCostUsd, 0.0003);
  assert.equal(settlement.costSource, 'provider_reported');
  assert.equal(settlement.modality, 'text');
  assert.equal(settlement.status, 200);
});

test('reserves the model context as the image input bound', async () => {
  const { deps, calls } = fixtureDeps({});
  const response = await runDecision({
    requestId: 'image-bound',
    account: ACCOUNT,
    route: ROUTE,
    request: requestFixture(true),
    deps,
  });
  assert.equal(response.status, 200);
  assert.equal(
    (calls.reserve[0] as { estimatedProviderCostUsd: number })
      .estimatedProviderCostUsd,
    262_144 / 1_000_000,
  );
});

test('refuses images on a text-only configured model before any charge', async () => {
  const { deps, calls } = fixtureDeps({ textOnlyModel: true });
  const response = await runDecision({
    requestId: 'req-2',
    account: ACCOUNT,
    route: ROUTE,
    request: requestFixture(true),
    deps,
  });
  assert.equal(response.status, 400);
  assert.equal((await response.json()).error.type, 'unsupported_image_route');
  assert.equal(calls.reserve.length, 0);
  assert.equal(calls.fetch.length, 0);
});

test('stops before calling the provider when credits cannot cover the reservation', async () => {
  const { deps, calls } = fixtureDeps({ reserveOk: false });
  const response = await runDecision({
    requestId: 'req-3',
    account: ACCOUNT,
    route: ROUTE,
    request: requestFixture(),
    deps,
  });
  assert.equal(response.status, 402);
  assert.equal(calls.fetch.length, 0);
  assert.equal(calls.finalize.length, 0);
});

test('releases the hold when the provider fails with a retryable status', async () => {
  const { deps, calls } = fixtureDeps({
    fetchResponse: () => providerResponse({ error: { code: 500 } }, 500),
  });
  const response = await runDecision({
    requestId: 'req-4',
    account: ACCOUNT,
    route: ROUTE,
    request: requestFixture(),
    deps,
  });
  assert.equal(response.status, 502);
  assert.equal(
    (await response.json()).error.type,
    'decision_provider_unavailable',
  );
  assert.deepEqual(calls.finalize, [
    { requestId: 'req-4', expectedUserId: 'user-1', outcome: 'release' },
  ]);
});

test('maps upstream rate limits and rejected requests to explicit client errors', async () => {
  const limited = fixtureDeps({
    fetchResponse: () => providerResponse({}, 429),
  });
  const rejected = fixtureDeps({
    fetchResponse: () => providerResponse({}, 422),
  });
  const limitedResponse = await runDecision({
    requestId: 'req-5',
    account: ACCOUNT,
    route: ROUTE,
    request: requestFixture(),
    deps: limited.deps,
  });
  const rejectedResponse = await runDecision({
    requestId: 'req-6',
    account: ACCOUNT,
    route: ROUTE,
    request: requestFixture(),
    deps: rejected.deps,
  });
  assert.equal(limitedResponse.status, 429);
  assert.equal(rejectedResponse.status, 400);
  assert.equal(
    (await rejectedResponse.json()).error.type,
    'decision_request_rejected',
  );
});

test('settles at the reservation ceiling when a 2xx answer cannot be trusted', async () => {
  const { deps, calls } = fixtureDeps({
    fetchResponse: () =>
      providerResponse({
        ...VALID_BODY,
        answers: { is_bug: { type: 'choice' } },
      }),
  });
  const response = await runDecision({
    requestId: 'req-7',
    account: ACCOUNT,
    route: ROUTE,
    request: requestFixture(),
    deps,
  });
  assert.equal(response.status, 502);
  assert.equal(
    (await response.json()).error.type,
    'decision_provider_response_invalid',
  );
  const settlement = calls.finalize[0] as {
    outcome: string;
    providerCostUsd: number;
    costSource: string;
  };
  assert.equal(settlement.outcome, 'settle');
  assert.equal(settlement.costSource, 'catalog_estimate');
  assert.equal(
    settlement.providerCostUsd,
    decisionReservationCostUsd(COST, requestFixture()),
  );
});

test('uses catalog token pricing when a valid answer omits the upstream cost', async () => {
  const { deps, calls } = fixtureDeps({
    fetchResponse: () =>
      providerResponse({
        ...VALID_BODY,
        usage: { input_tokens: 100, output_tokens: 5 },
      }),
  });
  await runDecision({
    requestId: 'req-8',
    account: ACCOUNT,
    route: ROUTE,
    request: requestFixture(),
    deps,
  });
  const settlement = calls.finalize[0] as {
    providerCostUsd: number;
    costSource: string;
  };
  assert.equal(settlement.costSource, 'catalog_estimate');
  assert.equal(settlement.providerCostUsd, computeUsdCost(COST, 100, 5));
});

test('settles a timed-out call at the ceiling because upstream may have billed it', async () => {
  const { deps, calls } = fixtureDeps({
    fetchResponse: () => {
      throw new DOMException('The operation timed out.', 'TimeoutError');
    },
  });
  const response = await runDecision({
    requestId: 'req-9',
    account: ACCOUNT,
    route: ROUTE,
    request: requestFixture(),
    deps,
  });
  assert.equal(response.status, 502);
  const settlement = calls.finalize[0] as {
    outcome: string;
    providerCostUsd: number;
    costSource: string;
  };
  assert.equal(settlement.outcome, 'settle');
  assert.equal(settlement.costSource, 'catalog_estimate');
  assert.equal(
    settlement.providerCostUsd,
    decisionReservationCostUsd(COST, requestFixture()),
  );
});

test('releases the hold when the connection fails before any upstream response', async () => {
  const { deps, calls } = fixtureDeps({
    fetchResponse: () => {
      throw new TypeError('fetch failed');
    },
  });
  const response = await runDecision({
    requestId: 'req-10',
    account: ACCOUNT,
    route: ROUTE,
    request: requestFixture(),
    deps,
  });
  assert.equal(response.status, 502);
  assert.deepEqual(calls.finalize, [
    { requestId: 'req-10', expectedUserId: 'user-1', outcome: 'release' },
  ]);
});
