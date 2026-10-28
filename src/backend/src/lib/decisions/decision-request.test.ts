import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  DECISION_IMAGE_MAX_DECODED_BYTES,
  decodedBase64ByteLength,
  parseDecisionRequest,
} from './decision-request';

const PNG_1X1 = 'iVBORw0KGgo=';

function validRequest(overrides: Record<string, unknown> = {}) {
  return {
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
        max: 5,
        labels: { '1': 'low', '5': 'blocker' },
      },
    ],
    ...overrides,
  };
}

test('accepts predicate, choice and score questions with shared input', () => {
  const parsed = parseDecisionRequest(validRequest());
  assert.equal(parsed.ok, true);
  if (!parsed.ok) return;
  assert.deepEqual(
    parsed.request.questions.map((question) => question.type),
    ['predicate', 'choice', 'score'],
  );
  assert.deepEqual(parsed.request.images, []);
});

test('accepts inline PNG, JPEG and WebP images as shared inputs', () => {
  const parsed = parseDecisionRequest(
    validRequest({
      images: [
        { mediaType: 'image/png', data: PNG_1X1 },
        { mediaType: 'image/jpeg', data: PNG_1X1 },
        { mediaType: 'image/webp', data: PNG_1X1 },
      ],
    }),
  );
  assert.equal(parsed.ok, true);
});

test('rejects duplicate question ids', () => {
  const request = validRequest();
  request.questions[1] = { ...request.questions[1], id: 'is_bug' } as never;
  const parsed = parseDecisionRequest(request);
  assert.equal(parsed.ok, false);
  if (!parsed.ok) assert.match(parsed.message, /Question ids must be unique/);
});

test('rejects choice questions with duplicate or single options', () => {
  const duplicate = parseDecisionRequest({
    input: 'x',
    questions: [
      { id: 'a', type: 'choice', question: 'q', options: ['x', 'x'] },
    ],
  });
  const single = parseDecisionRequest({
    input: 'x',
    questions: [{ id: 'a', type: 'choice', question: 'q', options: ['x'] }],
  });
  assert.equal(duplicate.ok, false);
  assert.equal(single.ok, false);
});

test('rejects score scales outside the supported level range', () => {
  const tooWide = parseDecisionRequest({
    input: 'x',
    questions: [{ id: 'a', type: 'score', question: 'q', min: 0, max: 10 }],
  });
  const inverted = parseDecisionRequest({
    input: 'x',
    questions: [{ id: 'a', type: 'score', question: 'q', min: 5, max: 1 }],
  });
  const labelOutside = parseDecisionRequest({
    input: 'x',
    questions: [
      {
        id: 'a',
        type: 'score',
        question: 'q',
        min: 1,
        max: 3,
        labels: { '9': 'x' },
      },
    ],
  });
  assert.equal(tooWide.ok, false);
  assert.equal(inverted.ok, false);
  assert.equal(labelOutside.ok, false);
});

test('rejects unsupported image media types instead of dropping them', () => {
  const parsed = parseDecisionRequest(
    validRequest({ images: [{ mediaType: 'image/gif', data: PNG_1X1 }] }),
  );
  assert.equal(parsed.ok, false);
});

test('rejects more than four images', () => {
  const images = Array.from({ length: 5 }, () => ({
    mediaType: 'image/png',
    data: PNG_1X1,
  }));
  assert.equal(parseDecisionRequest(validRequest({ images })).ok, false);
});

test('rejects images above the decoded byte bound', () => {
  const oversizedData = 'A'.repeat(
    Math.ceil(((DECISION_IMAGE_MAX_DECODED_BYTES + 3) * 4) / 3),
  );
  const parsed = parseDecisionRequest(
    validRequest({ images: [{ mediaType: 'image/png', data: oversizedData }] }),
  );
  assert.equal(parsed.ok, false);
});

test('rejects unknown fields and non-base64 image data', () => {
  assert.equal(parseDecisionRequest(validRequest({ extra: true })).ok, false);
  assert.equal(
    parseDecisionRequest(
      validRequest({
        images: [
          { mediaType: 'image/png', data: 'data:image/png;base64,AAAA' },
        ],
      }),
    ).ok,
    false,
  );
});

test('computes decoded base64 byte length from padding', () => {
  assert.equal(decodedBase64ByteLength('AAAA'), 3);
  assert.equal(decodedBase64ByteLength('AAA='), 2);
  assert.equal(decodedBase64ByteLength('AA=='), 1);
});
