import assert from 'node:assert/strict';
import { afterEach, beforeEach, test } from 'node:test';
import { probeCapability } from './probes';

const originalFetch = globalThis.fetch;
const target = { provider: 'opencode_zen', modelId: 'test-model' };
let sentBodies: any[] = [];

function streamOf(events: unknown[]): Response {
  const body = `${events.map((event) => `data: ${JSON.stringify(event)}\n\n`).join('')}data: [DONE]\n\n`;
  return new Response(body, { headers: { 'content-type': 'text/event-stream' } });
}

function answerWith(reply: (body: any) => Response) {
  globalThis.fetch = (async (_input: string | URL | Request, init?: RequestInit) => {
    const body = JSON.parse(String(init?.body));
    sentBodies.push(body);
    return reply(body);
  }) as typeof fetch;
}

const usage = (completion: number, reasoning = 0) => ({
  usage: { prompt_tokens: 10, completion_tokens: completion, completion_tokens_details: { reasoning_tokens: reasoning } },
});
const text = (content: string) => ({ choices: [{ delta: { content } }] });

beforeEach(() => {
  sentBodies = [];
  process.env.OPENCODE_API_KEY = 'probe-key';
  process.env.OPENCODE_BASE_URL = 'https://upstream.example/v1';
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete process.env.OPENCODE_API_KEY;
  delete process.env.OPENCODE_BASE_URL;
});

test('thinking off passes only when the model answers without reasoning', async () => {
  answerWith(() => streamOf([text('31'), usage(2)]));
  assert.equal((await probeCapability(target, 'thinking_off')).passed, true);
  assert.equal(sentBodies[0].reasoning_effort, 'none');
  assert.equal(sentBodies[0].stream, true);

  answerWith(() => streamOf([text('31'), usage(90, 84)]));
  const thoughtAnyway = await probeCapability(target, 'thinking_off');
  assert.equal(thoughtAnyway.passed, false);
  assert.match(thoughtAnyway.detail, /84 reasoning tokens/);
});

test('a refused thinking setting fails the thinking-off probe with the upstream message', async () => {
  answerWith(() =>
    Response.json({ error: { message: 'Reasoning is mandatory for this endpoint and cannot be disabled.' } }, { status: 400 }),
  );
  const refused = await probeCapability(target, 'thinking_off');
  assert.equal(refused.passed, false);
  assert.match(refused.detail, /HTTP 400: Reasoning is mandatory/);
});

test('reasoning passes on reasoning tokens or streamed reasoning text', async () => {
  answerWith(() => streamOf([text('31'), usage(60, 50)]));
  assert.equal((await probeCapability(target, 'reasoning')).passed, true);
  assert.equal(sentBodies[0].reasoning_effort, 'medium');

  answerWith(() => streamOf([{ choices: [{ delta: { reasoning: 'Let me think.' } }] }, text('31'), { usage: { prompt_tokens: 5, completion_tokens: 9 } }]));
  assert.equal((await probeCapability(target, 'reasoning')).passed, true);

  answerWith(() => streamOf([text('31'), usage(2)]));
  assert.equal((await probeCapability(target, 'reasoning')).passed, false);
});

test('tool calls pass when the streamed call multiplies 17 and 23', async () => {
  answerWith(() =>
    streamOf([
      { choices: [{ delta: { tool_calls: [{ index: 0, function: { name: 'multiply', arguments: '{"a":' } }] } }] },
      { choices: [{ delta: { tool_calls: [{ index: 0, function: { arguments: '23,"b":17}' } }] } }] },
      usage(12),
    ]),
  );
  assert.equal((await probeCapability(target, 'tool_calls')).passed, true);
  assert.equal(sentBodies[0].tool_choice, undefined);
  assert.equal(sentBodies[0].reasoning_effort, undefined);

  answerWith(() => streamOf([text('391'), usage(3)]));
  assert.equal((await probeCapability(target, 'tool_calls')).passed, false);
});

test('stream usage fails when the stream ends without token usage', async () => {
  answerWith(() => streamOf([text('ok'), usage(1)]));
  assert.equal((await probeCapability(target, 'stream_usage')).passed, true);
  assert.deepEqual(sentBodies[0].stream_options, { include_usage: true });

  answerWith(() => streamOf([text('ok')]));
  assert.equal((await probeCapability(target, 'stream_usage')).passed, false);
});

test('image input passes when the model names the red test image', async () => {
  answerWith(() => streamOf([text('Red'), usage(1)]));
  assert.equal((await probeCapability(target, 'image_input')).passed, true);
  assert.match(sentBodies[0].messages[0].content[1].image_url.url, /^data:image\/png;base64,/);

  answerWith(() => streamOf([text('I cannot see images.'), usage(5)]));
  assert.equal((await probeCapability(target, 'image_input')).passed, false);
});

test('overloaded or unreachable upstreams give an inconclusive outcome', async () => {
  answerWith(() => Response.json({ error: { message: 'The inference service is overloaded.' } }, { status: 429 }));
  const overloaded = await probeCapability(target, 'stream_usage');
  assert.equal(overloaded.passed, false);
  assert.equal(overloaded.conclusive, false);

  globalThis.fetch = (async () => {
    throw new TypeError('fetch failed');
  }) as typeof fetch;
  assert.equal((await probeCapability(target, 'stream_usage')).conclusive, false);

  answerWith(() => Response.json({ error: { message: 'Unsupported value' } }, { status: 400 }));
  assert.equal((await probeCapability(target, 'stream_usage')).conclusive, true);
});
