import assert from 'node:assert/strict';
import test from 'node:test';
import { AI_CHAT_IMAGE_LIMIT, canSendAiChatMessage } from './ai-chat-images';

test('AI chat can send text, images, or both', () => {
  assert.equal(canSendAiChatMessage('', 0), false);
  assert.equal(canSendAiChatMessage('  ', 0), false);
  assert.equal(canSendAiChatMessage('Question', 0), true);
  assert.equal(canSendAiChatMessage('', 1), true);
  assert.equal(canSendAiChatMessage('Question', AI_CHAT_IMAGE_LIMIT), true);
});
