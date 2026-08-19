import assert from 'node:assert/strict';
import test from 'node:test';
import { aiChatEventMatchesActiveChat } from './use-ai-chat';

test('ai chat events without a chat id remain global for the active surface', () => {
  assert.equal(aiChatEventMatchesActiveChat({ type: 'start' }, 'chat-a'), true);
  assert.equal(aiChatEventMatchesActiveChat({ type: 'done' }, undefined), true);
});

test('ai chat events are isolated to the active extension-window chat id', () => {
  assert.equal(
    aiChatEventMatchesActiveChat(
      { type: 'delta', chatId: 'chat-a', text: 'A' },
      'chat-a',
    ),
    true,
  );
  assert.equal(
    aiChatEventMatchesActiveChat(
      { type: 'delta', chatId: 'chat-b', text: 'B' },
      'chat-a',
    ),
    false,
  );
  assert.equal(
    aiChatEventMatchesActiveChat(
      { type: 'delta', chatId: 'chat-b', text: 'B' },
      undefined,
    ),
    false,
  );
});
