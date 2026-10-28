import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import {
  decisionErrorResponse,
  type DecisionErrorType,
} from './decision-errors';

function fixture(name: string) {
  return JSON.parse(
    readFileSync(
      new URL(
        `../../fixtures/contracts/desktop-v1/${name}.json`,
        import.meta.url,
      ),
      'utf8',
    ),
  );
}

const ERROR_FIXTURES: Array<[string, DecisionErrorType]> = [
  ['decision-unsupported-image-route-error', 'unsupported_image_route'],
  ['decision-request-invalid-error', 'invalid_decision_request'],
  ['decision-model-unavailable-error', 'decision_model_unavailable'],
  [
    'decision-provider-response-invalid-error',
    'decision_provider_response_invalid',
  ],
];

test('decision error fixtures are emitted with their exact machine-readable envelope', async () => {
  for (const [name, type] of ERROR_FIXTURES) {
    const expected = fixture(name);
    assert.equal(expected.error.type, type);
    const emitted = decisionErrorResponse(400, type, expected.error.message);
    assert.deepEqual(await emitted.json(), expected);
    assert.equal(emitted.headers.get('content-type'), 'application/json');
  }
});

test('decision success fixture keeps the desktop answer contract', () => {
  const body = fixture('decision-success');
  assert.deepEqual(Object.keys(body).sort(), [
    'answers',
    'id',
    'model',
    'usage',
  ]);
  assert.deepEqual(Object.keys(body.usage).sort(), [
    'inputTokens',
    'outputTokens',
  ]);
  assert.equal(body.answers.is_bug.type, 'predicate');
  assert.equal(typeof body.answers.is_bug.probability, 'number');
  assert.equal(body.answers.area.type, 'choice');
  assert.equal(body.answers.urgency.type, 'score');
  assert.equal('probability' in body.answers.urgency, false);
});
