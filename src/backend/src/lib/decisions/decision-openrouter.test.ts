import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  buildOpenRouterDecisionBody,
  openRouterDecisionsUrl,
  parseOpenRouterDecisionResponse,
} from './decision-openrouter';
import { DecisionProviderResponseError } from './decision-contract';
import { type DecisionRequest, parseDecisionRequest } from './decision-request';

function requestFixture(extra: Record<string, unknown> = {}): DecisionRequest {
  const parsed = parseDecisionRequest({
    input: 'Checkout shows a blank screen.',
    questions: [
      { id: 'is_bug', type: 'predicate', question: 'Is this a defect?' },
      {
        id: 'area',
        type: 'choice',
        question: 'Which area?',
        options: ['checkout', 'login'],
      },
      {
        id: 'urgency',
        type: 'score',
        question: 'How urgent?',
        min: 1,
        max: 3,
        labels: { '3': 'blocker' },
      },
    ],
    ...extra,
  });
  if (!parsed.ok) throw new Error(parsed.message);
  return parsed.request;
}

const PNG_DATA = 'iVBORw0KGgo=';

function responseFixture(overrides: Record<string, unknown> = {}) {
  return {
    id: 'gen-dec-1',
    model: 'typesafe/jev-1.13-2026-10-01',
    answers: {
      is_bug: { type: 'noul', noul: 0.9 },
      area: {
        type: 'choice',
        choice: 'checkout',
        probabilities: { checkout: 0.8, login: 0.2 },
        confidence: 0.75,
      },
      urgency: { type: 'score', score: 3 },
    },
    usage: { input_tokens: 120, output_tokens: 12, cost: 0.00001 },
    ...overrides,
  };
}

test('builds a text-only state string and typed questions for OpenRouter', () => {
  const body = buildOpenRouterDecisionBody(
    requestFixture(),
    'typesafe/jev-1.13',
  );
  assert.deepEqual(body, {
    model: 'typesafe/jev-1.13',
    state: 'Checkout shows a blank screen.',
    questions: {
      is_bug: { type: 'noul', instructions: 'Is this a defect?' },
      area: {
        type: 'choice',
        instructions: 'Which area?',
        criteria: { checkout: null, login: null },
      },
      urgency: {
        type: 'score',
        instructions: 'How urgent?',
        criteria: ['1', '2', '3 = blocker'],
      },
    },
    provider: { data_collection: 'deny' },
  });
});

test('moves shared images into state image_url parts without dropping them', () => {
  const request = requestFixture({
    images: [{ mediaType: 'image/png', data: PNG_DATA }],
  });
  const body = buildOpenRouterDecisionBody(request, 'vendor/vision');
  assert.deepEqual(body.state, [
    'Checkout shows a blank screen.',
    {
      type: 'image_url',
      image_url: {
        url: `data:image/png;base64,${PNG_DATA}`,
        detail: 'auto',
      },
    },
  ]);
});

test('derives the native decisions URL from the upstream origin', () => {
  assert.equal(
    openRouterDecisionsUrl('https://openrouter.ai/api/v1'),
    'https://openrouter.ai/api/alpha/decisions',
  );
  assert.equal(
    openRouterDecisionsUrl('http://127.0.0.1:4010/api/v1'),
    'http://127.0.0.1:4010/api/alpha/decisions',
  );
});

test('maps predicate, choice and score answers with optional probability and confidence', () => {
  const parsed = parseOpenRouterDecisionResponse(
    responseFixture(),
    requestFixture(),
  );
  assert.deepEqual(parsed.answers, {
    is_bug: { type: 'predicate', value: true, probability: 0.9 },
    area: {
      type: 'choice',
      value: 'checkout',
      probabilities: { checkout: 0.8, login: 0.2 },
      confidence: 0.75,
    },
    urgency: {
      type: 'score',
      value: 3,
      probabilities: undefined,
      confidence: undefined,
    },
  });
  assert.deepEqual(parsed.usage, { inputTokens: 120, outputTokens: 12 });
  assert.equal(parsed.providerCostUsd, 0.00001);
  assert.equal(parsed.providerModel, 'typesafe/jev-1.13-2026-10-01');
});

test('treats a predicate below one half as false while keeping its probability', () => {
  const body = responseFixture({
    answers: {
      ...responseFixture().answers,
      is_bug: { type: 'noul', noul: 0.2 },
    },
  });
  const parsed = parseOpenRouterDecisionResponse(body, requestFixture());
  assert.deepEqual(parsed.answers.is_bug, {
    type: 'predicate',
    value: false,
    probability: 0.2,
  });
});

test('leaves cost undefined when the upstream omits usage cost', () => {
  const body = responseFixture({
    usage: { input_tokens: 10, output_tokens: 1 },
  });
  const parsed = parseOpenRouterDecisionResponse(body, requestFixture());
  assert.equal(parsed.providerCostUsd, undefined);
});

test('rejects answers that are missing, unrequested, or out of contract', () => {
  const request = requestFixture();
  const missing = responseFixture();
  delete (missing.answers as Record<string, unknown>).urgency;
  const extra = responseFixture({
    answers: {
      ...responseFixture().answers,
      other: { type: 'noul', noul: 0.1 },
    },
  });
  const badChoice = responseFixture({
    answers: {
      ...responseFixture().answers,
      area: { type: 'choice', choice: 'billing' },
    },
  });
  const badScore = responseFixture({
    answers: {
      ...responseFixture().answers,
      urgency: { type: 'score', score: 9 },
    },
  });
  const badProbability = responseFixture({
    answers: {
      ...responseFixture().answers,
      is_bug: { type: 'noul', noul: 1.4 },
    },
  });
  const badUsage = responseFixture({
    usage: { input_tokens: -1, output_tokens: 1 },
  });
  for (const body of [
    missing,
    extra,
    badChoice,
    badScore,
    badProbability,
    badUsage,
  ]) {
    assert.throws(
      () => parseOpenRouterDecisionResponse(body, request),
      DecisionProviderResponseError,
    );
  }
});
