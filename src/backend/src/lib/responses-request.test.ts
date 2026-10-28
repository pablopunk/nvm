import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  rewriteResponsesRequest,
  validateResponsesRequest,
} from './responses-request';

const request = {
  model: 'placeholder',
  input: [{ role: 'user', content: 'hello' }],
  tools: [
    {
      type: 'function',
      name: 'lookup',
      parameters: { type: 'object', properties: {} },
    },
  ],
  stream: true,
  include: ['reasoning.encrypted_content'],
};

test('Responses accepts function tools and rejects hosted tools and server-owned state', () => {
  assert.equal(validateResponsesRequest(request), null);
  for (const value of [
    { ...request, tools: [{ type: 'web_search' }] },
    { ...request, tool_choice: { type: 'file_search' } },
    { ...request, previous_response_id: 'resp_other_user' },
    { ...request, conversation: 'conv_other_user' },
    { ...request, input: [{ type: 'item_reference', id: 'item_other_user' }] },
    { ...request, background: true },
    { ...request, include: ['web_search_call.action.sources'] },
    null,
  ])
    assert.ok(validateResponsesRequest(value));
});

test('Responses rewrite owns model, cost ceiling and storage', () => {
  const body = JSON.parse(
    rewriteResponsesRequest(
      JSON.stringify({
        ...request,
        store: true,
        service_tier: 'priority',
        max_output_tokens: 100000,
        prompt_cache_key: 'other',
      }),
      { activeModelId: 'gpt-6.1-sol' },
      200,
    ),
  );
  assert.equal(body.model, 'gpt-6.1-sol');
  assert.equal(body.max_output_tokens, 200);
  assert.equal(body.store, false);
  assert.equal(body.service_tier, undefined);
  assert.equal(body.prompt_cache_key, undefined);
  assert.deepEqual(body.tools, request.tools);
});

test('Responses forwards the client reasoning level unchanged', () => {
  for (const effort of ['none', 'minimal', 'high']) {
    const body = JSON.parse(
      rewriteResponsesRequest(
        JSON.stringify({ ...request, reasoning: { effort, summary: 'auto' } }),
        { activeModelId: 'any-model' },
        200,
      ),
    );
    assert.equal(body.reasoning.effort, effort);
    assert.equal(body.reasoning.summary, 'auto');
  }
});
