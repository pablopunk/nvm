import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { estimateInputTokensFromBody, estimatePromptCredits, estimateRequestCredits, requestedMaxOutputTokens, MAX_INPUT_TOKENS } from './limits';
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

  test('estimates image payloads without counting base64 as text', () => {
    const base64 = 'a'.repeat(600_000);
    const textTokens = Math.ceil('user'.length / CHARS_PER_TOKEN);
    const expected = textTokens + 1_600;
    const payloads = [
      {
        messages: [
          {
            role: 'user',
            content: [
              {
                type: 'image_url',
                image_url: { url: `data:image/png;base64,${base64}` },
              },
            ],
          },
        ],
      },
      {
        messages: [
          {
            role: 'user',
            content: [
              {
                type: 'image',
                source: {
                  type: 'base64',
                  media_type: 'image/png',
                  data: base64,
                },
              },
            ],
          },
        ],
      },
      {
        contents: [
          {
            role: 'user',
            parts: [{ inlineData: { mimeType: 'image/png', data: base64 } }],
          },
        ],
      },
      {
        messages: [
          {
            role: 'user',
            content: [
              {
                type: 'input_image',
                detail: 'auto',
                image_url: `data:image/png;base64,${base64}`,
              },
            ],
          },
        ],
      },
      {
        messages: [
          {
            role: 'user',
            content: [
              {
                type: 'image_url',
                image_url: `data:image/png;base64,${base64}`,
              },
            ],
          },
        ],
      },
    ];

    for (const payload of payloads)
      assert.equal(estimateInputTokensFromBody(JSON.stringify(payload)), expected);
  });

  test('continues counting unrelated data fields as text', () => {
    const data = 'a'.repeat(600_000);
    assert.equal(
      estimateInputTokensFromBody(JSON.stringify({ data })),
      Math.ceil(data.length / CHARS_PER_TOKEN),
    );
  });

  test('does not exempt malformed image-like wrappers', () => {
    const data = 'a'.repeat(600_000);
    const malformed = [
      {
        type: 'image_url',
        image_url: { url: `data:image/png;base64,${data}` },
        hiddenText: data,
      },
      {
        type: 'image',
        source: { type: 'base64', media_type: 'text/plain', data },
      },
      {
        inlineData: { mimeType: 'image/png', data: `${data}!` },
      },
      {
        type: 'input_image',
        detail: data,
        image_url: `data:image/png;base64,${data}`,
      },
      {
        type: 'image',
        source: { type: 'base64', media_type: 'image/png', data },
        cache_control: { type: 'ephemeral', ttl: data },
      },
    ];

    for (const payload of malformed)
      assert.ok(estimateInputTokensFromBody(JSON.stringify(payload)) > 100_000);
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

test('reservation estimate includes bounded output cost', () => {
  const cost = { ...makeCost(10), outputUsdPerMtok: 20 };
  assert.equal(estimateRequestCredits(100_000, 100_000, cost), 1500);
});

test('reads output caps for OpenAI, Anthropic, and Google requests', () => {
  assert.equal(requestedMaxOutputTokens({ max_tokens: 12 }), 12);
  assert.equal(requestedMaxOutputTokens({ max_completion_tokens: 13 }), 13);
  assert.equal(requestedMaxOutputTokens({ generationConfig: { maxOutputTokens: 14 } }), 14);
  assert.equal(requestedMaxOutputTokens({ max_tokens: -1 }), undefined);
  assert.equal(requestedMaxOutputTokens({}), undefined);
});

describe('MAX_INPUT_TOKENS', () => {
  test('defaults to 100_000', () => {
    assert.strictEqual(MAX_INPUT_TOKENS, 100_000);
  });
});
