import assert from 'node:assert/strict';
import { test } from 'node:test';
import { streamSimple, type Model } from 'pi-ai-legacy/compat';

test('released Pi 0.80.10 uses Responses with Nevermind model descriptors', async () => {
  const originalFetch = globalThis.fetch;
  let requestUrl = '';
  let requestBody: any;
  const item = {
    type: 'function_call',
    id: 'fc_fixture',
    call_id: 'call_fixture',
    name: 'lookup',
    arguments: '{}',
    status: 'completed',
  };
  const events = [
    {
      type: 'response.output_item.added',
      output_index: 0,
      item: { ...item, arguments: '' },
    },
    {
      type: 'response.function_call_arguments.delta',
      output_index: 0,
      delta: '{}',
    },
    { type: 'response.output_item.done', output_index: 0, item },
    {
      type: 'response.completed',
      response: {
        id: 'resp_fixture',
        status: 'completed',
        output: [item],
        usage: { input_tokens: 3, output_tokens: 4, total_tokens: 7 },
      },
    },
  ];
  globalThis.fetch = async (input, init) => {
    requestUrl = input instanceof Request ? input.url : String(input);
    requestBody = JSON.parse(String(init?.body));
    return new Response(
      events.map((event) => `data: ${JSON.stringify(event)}\n\n`).join(''),
      {
        headers: { 'content-type': 'text/event-stream' },
      },
    );
  };
  try {
    const model: Model<'openai-responses'> = {
      id: 'gpt-6.1-sol',
      name: 'GPT-6.1 Sol',
      api: 'openai-responses',
      provider: 'nevermind',
      baseUrl: 'https://example.invalid/api/v1',
      reasoning: true,
      input: ['text', 'image'],
      contextWindow: 100000,
      maxTokens: 1024,
      cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
    };
    const result = await streamSimple(
      model,
      {
        messages: [
          { role: 'user', content: 'Use lookup.', timestamp: Date.now() },
        ],
        tools: [
          {
            name: 'lookup',
            description: 'Look up a result',
            parameters: { type: 'object', properties: {} } as any,
          },
        ],
      },
      { apiKey: 'fixture-token', reasoning: 'low' },
    ).result();
    assert.equal(requestUrl, 'https://example.invalid/api/v1/responses');
    assert.equal(requestBody.store, false);
    assert.equal(requestBody.tools[0].type, 'function');
    assert.equal(result.stopReason, 'toolUse');
    assert.ok(
      result.content.some(
        (block) => block.type === 'toolCall' && block.name === 'lookup',
      ),
    );
  } finally {
    globalThis.fetch = originalFetch;
  }
});
