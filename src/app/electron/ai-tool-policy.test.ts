import assert from 'node:assert/strict';
import test from 'node:test';
import {
  aiPromptUsesDirectModel,
  CONVERSATION_AI_TOOLS,
  missingConversationAiTools,
} from './ai-tool-policy';

test('conversation AI exposes only read-only and web access tools', () => {
  assert.deepEqual(CONVERSATION_AI_TOOLS, [
    'read',
    'grep',
    'find',
    'ls',
    'web_search',
    'fetch_content',
  ]);
  for (const forbidden of [
    'bash',
    'edit',
    'write',
    'source_check',
    'get_search_content',
  ])
    assert.equal(CONVERSATION_AI_TOOLS.includes(forbidden as never), false);
});

test('conversation AI uses an agent session instead of the tool-free direct model', () => {
  assert.equal(aiPromptUsesDirectModel({}), true);
  assert.equal(
    aiPromptUsesDirectModel({ sessionId: 'extension-session' }),
    false,
  );
  assert.equal(aiPromptUsesDirectModel({ toolMode: 'conversation' }), false);
});

test('conversation AI detects an incomplete packaged tool set', () => {
  assert.deepEqual(
    missingConversationAiTools(['read', 'grep', 'find', 'ls', 'web_search']),
    ['fetch_content'],
  );
  assert.deepEqual(missingConversationAiTools([...CONVERSATION_AI_TOOLS]), []);
});
