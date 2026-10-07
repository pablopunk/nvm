import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import {
  containsImagePayload,
  containsUnsupportedMultimodalPayload,
  estimateInputTokensFromBody,
  estimateInputTokensFromValue,
  estimatePromptCredits,
  estimateRequestCredits,
  MAX_INPUT_TOKENS,
  requestedMaxOutputTokens,
} from './limits';
import type { ModelCost } from './pricing';

const CHARS_PER_TOKEN = 4;

const makeCost = (input: number): ModelCost => ({
  provider: 'test',
  modelId: 'test-model',
  inputUsdPerMtok: input,
  outputUsdPerMtok: 0,
});

describe('estimateInputTokensFromBody', () => {
  test('image payloads are identified independently of the fixed token estimate', () => {
    const payload = {
      messages: [
        {
          content: [{ type: 'image_url', image_url: 'data:image/png;base64,YWJj' }],
        },
      ],
    };
    assert.equal(containsImagePayload(payload), true);
    assert.equal(containsImagePayload({ messages: [{ content: 'text only' }] }), false);
  });
  test('empty string returns 0', () => {
    assert.strictEqual(estimateInputTokensFromBody(''), 0);
  });

  test('invalid JSON falls back to ceil(length/4)', () => {
    assert.strictEqual(estimateInputTokensFromBody('not json'), Math.ceil('not json'.length / CHARS_PER_TOKEN));
    assert.strictEqual(estimateInputTokensFromBody('abcd'), Math.ceil(4 / CHARS_PER_TOKEN));
    assert.strictEqual(estimateInputTokensFromBody('abcde'), Math.ceil(5 / CHARS_PER_TOKEN));
  });

  test('valid flat JSON sums string-field chars divided by 4', () => {
    const body = JSON.stringify({
      prompt: 'hello world',
      system: 'be helpful',
    });
    const promptLength = 'hello world'.length;
    const systemLength = 'be helpful'.length;
    assert.strictEqual(estimateInputTokensFromBody(body), Math.ceil((promptLength + systemLength) / CHARS_PER_TOKEN));
  });

  test('estimates parsed request bodies without serializing them again', () => {
    const body = { messages: [{ role: 'user', content: 'hello world' }] };
    assert.equal(estimateInputTokensFromValue(body), estimateInputTokensFromBody(JSON.stringify(body)));
  });

  test('ignores non-string values', () => {
    const body = JSON.stringify({
      prompt: 'hi',
      count: 42,
      flag: true,
      arr: [1, 2],
    });
    assert.strictEqual(estimateInputTokensFromBody(body), Math.ceil('hi'.length / CHARS_PER_TOKEN));
  });

  test('recurses into nested objects', () => {
    const body = JSON.stringify({
      outer: 'hello',
      inner: { nested: 'world', deep: { deeper: 'test' } },
    });
    const totalChars = 'hello'.length + 'world'.length + 'test'.length;
    assert.strictEqual(estimateInputTokensFromBody(body), Math.ceil(totalChars / CHARS_PER_TOKEN));
  });

  test('recurses into arrays', () => {
    const body = JSON.stringify({
      messages: [
        { role: 'user', content: 'hello' },
        { role: 'assistant', content: 'world' },
      ],
    });
    const totalChars = 'user'.length + 'hello'.length + 'assistant'.length + 'world'.length;
    assert.strictEqual(estimateInputTokensFromBody(body), Math.ceil(totalChars / CHARS_PER_TOKEN));
  });

  test('deeply nested arrays and objects', () => {
    const body = JSON.stringify({
      messages: [
        {
          content: [
            { type: 'text', text: 'a' },
            { type: 'text', text: 'bc' },
          ],
        },
      ],
    });
    const totalChars = 'text'.length + 'a'.length + 'text'.length + 'bc'.length;
    assert.strictEqual(estimateInputTokensFromBody(body), Math.ceil(totalChars / CHARS_PER_TOKEN));
  });

  test('estimates image payloads without counting base64 as text', () => {
    const base64 = 'a'.repeat(600_000);
    const textTokens = Math.ceil('user'.length / CHARS_PER_TOKEN);
    const expected = textTokens + 1600;
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

    for (const payload of payloads) assert.equal(estimateInputTokensFromBody(JSON.stringify(payload)), expected);
  });

  test('continues counting unrelated data fields as text', () => {
    const data = 'a'.repeat(600_000);
    assert.equal(estimateInputTokensFromBody(JSON.stringify({ data })), Math.ceil(data.length / CHARS_PER_TOKEN));
  });

  test('fails closed on remote, malformed, and extended image-shaped inputs', () => {
    const data = 'a'.repeat(600_000);
    const imageShapes = [
      { type: 'image_url', image_url: 'https://images.example/picture.png' },
      { type: 'image_url', image_url: `data:image/png;base64,${data}`, detail: 'high', extra: data },
      { type: 'image', source: { type: 'base64', media_type: 'image/png', data, extra: true } },
      { inlineData: { mimeType: 'image/png', data, extra: true } },
      { type: 'input_image', image_url: { url: 'https://images.example/picture.png' }, detail: 'high' },
      { type: 'image_url', image_url: null, detail: 'auto' },
      { type: 'image', source: { type: 'base64', media_type: 'image/png', data: 'malformed' } },
    ];

    for (const payload of imageShapes) {
      assert.equal(containsImagePayload(payload), true);
      assert.ok(estimateInputTokensFromValue(payload) < 100_000, 'image bytes and URLs must not become text-token estimates');
    }
    assert.equal(containsImagePayload('{"type":"image_url","image_url":'), true, 'malformed raw JSON with image intent is detected');
  });

  test('detects non-image multimodal structures and modality data URIs', () => {
    const payloads = [
      { messages: [{ role: 'user', content: [{ type: 'input_audio', input_audio: { data: 'AAAA', format: 'wav' } }] }] },
      { messages: [{ role: 'user', content: [{ type: 'document', source: { type: 'base64', media_type: 'application/pdf', data: 'AAAA' } }] }] },
      { contents: [{ parts: [{ inlineData: { mimeType: 'audio/wav', data: 'AAAA' } }] }] },
      { contents: [{ parts: [{ inlineData: { mimeType: 'video/mp4', data: 'AAAA' } }] }] },
      { messages: [{ role: 'user', content: [{ type: 'video_url', video_url: { url: 'https://example.test/clip.mp4' } }] }] },
      { messages: [{ role: 'user', content: 'data:audio/wav;base64,AAAA' }] },
      { prompt: 'data:application/pdf;base64,AAAA' },
    ];
    for (const payload of payloads) assert.equal(containsUnsupportedMultimodalPayload(payload), true);
    assert.equal(containsImagePayload({ input_audio: { data: 'AAAA' } }), true, 'legacy broad detector alias remains compatible');
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

  test('maximum text input boundary is measurable without image byte estimates', () => {
    const atLimit = 'a'.repeat(MAX_INPUT_TOKENS * CHARS_PER_TOKEN);
    const overLimit = `${atLimit}aaaa`;
    assert.equal(estimateInputTokensFromValue({ prompt: atLimit }), MAX_INPUT_TOKENS);
    assert.equal(estimateInputTokensFromValue({ prompt: overLimit }), MAX_INPUT_TOKENS + 1);
  });
});
