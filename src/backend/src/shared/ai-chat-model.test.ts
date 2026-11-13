import assert from 'node:assert/strict';
import test from 'node:test';
import {
  aiChatModelForChat,
  AUTOMATE_AI_CHAT_MODEL,
  DEFAULT_AI_CHAT_MODEL,
  normalizeAiChatModel,
} from './ai-chat-model';

test('new conversations default to the fast model', () => {
  assert.equal(DEFAULT_AI_CHAT_MODEL, 'fast');
  assert.equal(aiChatModelForChat({ kind: 'conversation' }), 'fast');
});

test('conversation model choices are preserved', () => {
  assert.equal(
    aiChatModelForChat({ kind: 'conversation', model: 'smart' }),
    'smart',
  );
  assert.equal(
    aiChatModelForChat({ kind: 'conversation', model: 'fast' }),
    'fast',
  );
});

test('builder chats always use the smart model', () => {
  assert.equal(AUTOMATE_AI_CHAT_MODEL, 'smart');
  assert.equal(aiChatModelForChat({ kind: 'builder', model: 'fast' }), 'smart');
  assert.equal(aiChatModelForChat({}), 'smart');
});

test('invalid model values use the default', () => {
  assert.equal(normalizeAiChatModel('unknown'), 'fast');
  assert.equal(normalizeAiChatModel(undefined, 'smart'), 'smart');
  assert.equal(
    aiChatModelForChat({ kind: 'conversation', model: 'unknown' }),
    'fast',
  );
});
