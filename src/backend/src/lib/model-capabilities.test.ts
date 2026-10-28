import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  isReviewedModelRoute,
  requiresResponsesForTools,
  supportsChatCompletionsTools,
  supportsModelThinkingLevel,
} from './model-capabilities';

test('only reviewed provider, model and role combinations are selectable', () => {
  for (const model of [
    'google/gemini-2.5-flash',
    'deepseek/deepseek-v4.1-flash-1017',
    'openai/gpt-6.1-terra',
  ]) {
    assert.equal(isReviewedModelRoute('openrouter', model, 'smart'), true);
    assert.equal(isReviewedModelRoute('openrouter', model, 'fast'), true);
  }
  assert.equal(
    isReviewedModelRoute('openrouter', 'openai/gpt-6-sol', 'fast'),
    true,
  );
  assert.equal(
    isReviewedModelRoute('openrouter', 'openai/gpt-6-sol', 'smart'),
    false,
  );
  assert.equal(
    isReviewedModelRoute('openrouter', 'openai/gpt-6.1-sol', 'smart'),
    true,
  );
  assert.equal(
    isReviewedModelRoute('openrouter', 'openai/gpt-6.1-sol', 'fast'),
    false,
  );
  assert.equal(isReviewedModelRoute('openai', 'gpt-6.1-sol', 'smart'), true);
  assert.equal(
    isReviewedModelRoute('opencode_zen', 'gpt-6.1-sol', 'smart'),
    false,
  );
  assert.equal(
    isReviewedModelRoute('openrouter', 'vendor/new-unreviewed-model', 'smart'),
    false,
  );
});

test('GPT-6.1 Sol requires Responses and a supported reasoning level', () => {
  for (const model of ['gpt-6.1-sol', 'openai/gpt-6.1-sol']) {
    assert.equal(requiresResponsesForTools(model), true);
    for (const level of ['off', 'minimal'])
      assert.equal(supportsModelThinkingLevel(model, level), false);
    for (const level of ['low', 'medium', 'high', 'xhigh', 'max'])
      assert.equal(supportsModelThinkingLevel(model, level), true);
  }
  assert.equal(supportsModelThinkingLevel('another-model', 'off'), true);
});

test('Chat Completions filters models with unsupported tool and reasoning combinations', () => {
  for (const model of ['openai/gpt-6-astra', 'openai/gpt-6.1-sol']) {
    assert.equal(supportsChatCompletionsTools(model, false), false);
    assert.equal(supportsChatCompletionsTools(model, true), false);
  }
  for (const model of [
    'openai/gpt-6-sol',
    'openai/gpt-6-luna',
    'gpt-5.6-luna',
  ]) {
    assert.equal(supportsChatCompletionsTools(model, false), false);
    assert.equal(supportsChatCompletionsTools(model, true), true);
  }
  assert.equal(supportsChatCompletionsTools('another-model', false), true);
});
