import assert from 'node:assert/strict';
import { test } from 'node:test';
import { modelProbeRequest } from './[providerId]';

test('builds an OpenAI-compatible streaming model probe', () => {
  const probe = modelProbeRequest('openrouter', 'deepseek/deepseek-chat', 'https://openrouter.example/api/v1', 'secret');

  assert.equal(probe.url, 'https://openrouter.example/api/v1/chat/completions');
  assert.equal(probe.headers.get('authorization'), 'Bearer secret');
  assert.deepEqual(probe.body, {
    model: 'deepseek/deepseek-chat',
    max_tokens: 1,
    stream: true,
    messages: [{ role: 'user', content: 'Reply OK' }],
  });
});

test('builds an Anthropic streaming model probe', () => {
  const probe = modelProbeRequest('anthropic', 'claude-sonnet-4-6', 'https://api.anthropic.com', 'secret');

  assert.equal(probe.url, 'https://api.anthropic.com/v1/messages');
  assert.equal(probe.headers.get('x-api-key'), 'secret');
  assert.equal(probe.headers.get('anthropic-version'), '2023-06-01');
});

test('builds a Google streaming model probe', () => {
  const probe = modelProbeRequest('opencode_zen', 'gemini-3-flash', 'https://opencode.example/v1', 'secret');

  assert.equal(probe.url, 'https://opencode.example/v1/models/gemini-3-flash:streamGenerateContent?alt=sse');
  assert.equal(probe.headers.get('x-goog-api-key'), 'secret');
});
