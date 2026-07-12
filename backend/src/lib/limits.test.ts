import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { estimateInputTokensFromBody, estimatePromptCredits, MAX_INPUT_TOKENS } from './limits';
import type { ModelCost } from './pricing';

const CHARS_PER_TOKEN = 4;

const makeCost = (input: number): ModelCost => ({
  provider: 'test',
  modelId: 'test-model',
  inputUsdPerMtok: input,
  outputUsdPerMtok: 0,
});

describe('estimateInputTokensFromBody', () => {
  test('empty string returns 0', () => {
    assert.strictEqual(estimateInputTokensFromBody(''), 0);
  });

  test('invalid JSON falls back to ceil(length/4)', () => {
    assert.strictEqual(estimateInputTokensFromBody('not json'), Math.ceil('not json'.length / CHARS_PER_TOKEN));
    assert.strictEqual(estimateInputTokensFromBody('abcd'), Math.ceil(4 / CHARS_PER_TOKEN));
    assert.strictEqual(estimateInputTokensFromBody('abcde'), Math.ceil(5 / CHARS_PER_TOKEN));
  });

  test('valid flat JSON sums string-field chars divided by 4', () => {
    const body = JSON.stringify({ prompt: 'hello world', system: 'be helpful' });
    const promptLength = 'hello world'.length;
    const systemLength = 'be helpful'.length;
    assert.strictEqual(
      estimateInputTokensFromBody(body),
      Math.ceil((promptLength + systemLength) / CHARS_PER_TOKEN),
    );
  });

  test('ignores non-string values', () => {
    const body = JSON.stringify({ prompt: 'hi', count: 42, flag: true, arr: [1, 2] });
    assert.strictEqual(
      estimateInputTokensFromBody(body),
      Math.ceil('hi'.length / CHARS_PER_TOKEN),
    );
  });

  test('recurses into nested objects', () => {
    const body = JSON.stringify({
      outer: 'hello',
      inner: { nested: 'world', deep: { deeper: 'test' } },
    });
    const totalChars = 'hello'.length + 'world'.length + 'test'.length;
    assert.strictEqual(
      estimateInputTokensFromBody(body),
      Math.ceil(totalChars / CHARS_PER_TOKEN),
    );
  });

  test('recurses into arrays', () => {
    const body = JSON.stringify({
      messages: [
        { role: 'user', content: 'hello' },
        { role: 'assistant', content: 'world' },
      ],
    });
    const totalChars = 'user'.length + 'hello'.length + 'assistant'.length + 'world'.length;
    assert.strictEqual(
      estimateInputTokensFromBody(body),
      Math.ceil(totalChars / CHARS_PER_TOKEN),
    );
  });

  test('deeply nested arrays and objects', () => {
    const body = JSON.stringify({
      messages: [
        { content: [{ type: 'text', text: 'a' }, { type: 'text', text: 'bc' }] },
      ],
    });
    const totalChars = 'text'.length + 'a'.length + 'text'.length + 'bc'.length;
    assert.strictEqual(
      estimateInputTokensFromBody(body),
      Math.ceil(totalChars / CHARS_PER_TOKEN),
    );
  });
});

describe('estimatePromptCredits', () => {
  test('returns credits for input tokens using cost chain', () => {
    const cost = makeCost(10);
    const credits = estimatePromptCredits(500_000, cost);
    assert.strictEqual(credits, 2500);
  });

  test('returns minimum 1 credit for tiny input', () => {
    const cost = makeCost(1);
    const credits = estimatePromptCredits(1, cost);
    assert.strictEqual(credits, 1);
  });

  test('zero input tokens returns 1 credit (ceil floor)', () => {
    const credits = estimatePromptCredits(0, makeCost(10));
    assert.strictEqual(credits, 1);
  });
});

describe('MAX_INPUT_TOKENS', () => {
  test('defaults to 100_000', () => {
    assert.strictEqual(MAX_INPUT_TOKENS, 100_000);
  });
});
