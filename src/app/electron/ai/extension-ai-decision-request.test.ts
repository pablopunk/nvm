import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  buildDecisionWireRequest,
  DECISION_IMAGE_MAX_DECODED_BYTES,
  decisionRequestFailure,
  type DecisionImageInput,
  ExtensionAiDecisionInputError,
  parseDecisionWireResponse,
} from './extension-ai-decision-request';

const QUESTIONS = [
  { id: 'is_bug', type: 'predicate' as const, question: 'Is it a bug?' },
];
const PNG: DecisionImageInput = {
  type: 'image',
  data: 'iVBORw0KGgo=',
  mimeType: 'image/png',
};

function requestWith(images: DecisionImageInput[]) {
  return buildDecisionWireRequest({
    input: 'Shared input',
    contextText: '### Notes\nFrom a file',
    images,
    questions: QUESTIONS,
  });
}

test('joins typed input with attachment text and forwards every image', () => {
  const wire = requestWith([PNG]);
  assert.equal(wire.input, 'Shared input\n\n### Notes\nFrom a file');
  assert.deepEqual(wire.images, [{ mediaType: 'image/png', data: PNG.data }]);
  assert.deepEqual(wire.questions, QUESTIONS);
});

test('rejects more than four images instead of dropping the extra ones', () => {
  const images = Array.from({ length: 5 }, () => PNG);
  assert.throws(
    () => requestWith(images),
    (error: unknown) =>
      error instanceof ExtensionAiDecisionInputError &&
      error.code === 'decision_too_many_images',
  );
});

test('rejects image types the decision route cannot read', () => {
  assert.throws(
    () => requestWith([{ ...PNG, mimeType: 'image/gif' }]),
    (error: unknown) =>
      error instanceof ExtensionAiDecisionInputError &&
      error.code === 'decision_image_type_unsupported',
  );
});

test('rejects images above the 2 MiB decoded bound', () => {
  const data = 'A'.repeat(
    Math.ceil(((DECISION_IMAGE_MAX_DECODED_BYTES + 3) * 4) / 3),
  );
  assert.throws(
    () => requestWith([{ ...PNG, data }]),
    (error: unknown) =>
      error instanceof ExtensionAiDecisionInputError &&
      error.code === 'decision_image_too_large',
  );
});

test('accepts a typed answer map and keeps optional probabilities when present', () => {
  const result = parseDecisionWireResponse({
    id: 'req-1',
    model: 'typesafe/jev-1.13',
    answers: {
      is_bug: { type: 'predicate', value: true, probability: 0.9 },
      area: {
        type: 'choice',
        value: 'checkout',
        probabilities: { checkout: 0.8, login: 0.2 },
      },
      urgency: { type: 'score', value: 3, confidence: 0.6 },
    },
    usage: { inputTokens: 120, outputTokens: 12 },
  });
  assert.equal(result.answers.area.type, 'choice');
  assert.deepEqual(result.answers.urgency, {
    type: 'score',
    value: 3,
    confidence: 0.6,
  });
  assert.deepEqual(result.usage, { inputTokens: 120, outputTokens: 12 });
});

test('rejects responses whose answers do not carry their declared type', () => {
  assert.throws(() =>
    parseDecisionWireResponse({
      id: 'req-2',
      model: 'm',
      answers: { is_bug: { type: 'predicate', value: 'yes' } },
      usage: { inputTokens: 1, outputTokens: 1 },
    }),
  );
  assert.throws(() => parseDecisionWireResponse({ id: 'req-3', model: 'm' }));
});

test('surfaces the backend error type as the thrown code', () => {
  const error = decisionRequestFailure(400, {
    error: {
      type: 'unsupported_image_route',
      message: 'The configured decision model does not accept images.',
    },
  });
  assert.equal(error.code, 'unsupported_image_route');
  assert.equal(error.status, 400);
  assert.equal(
    error.message,
    'The configured decision model does not accept images.',
  );
  assert.equal(decisionRequestFailure(502, null).code, 'decision_failed');
});
