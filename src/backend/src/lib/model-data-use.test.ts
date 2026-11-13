import assert from 'node:assert/strict';
import { test } from 'node:test';
import { modelProviderMayTrainOnPrompts } from './model-data-use';

test('flags contributor-tier model refs as training on prompts', () => {
  assert.equal(modelProviderMayTrainOnPrompts('openrouter/meta/muse-spark-1.3-contributor'), true);
  assert.equal(modelProviderMayTrainOnPrompts('opencode_zen/muse-spark-1.3-contributor-free'), true);
});

test('does not flag standard-tier model refs', () => {
  assert.equal(modelProviderMayTrainOnPrompts('openrouter/meta/muse-spark-1.3'), false);
  assert.equal(modelProviderMayTrainOnPrompts('openrouter/acme/contributors-edition'), false);
});
