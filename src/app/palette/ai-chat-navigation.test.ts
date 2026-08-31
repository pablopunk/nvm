import assert from 'node:assert/strict';
import test from 'node:test';
import { restoredAiChatView } from './ai-chat-navigation';
import type { CommandView } from './model';

function view(title: string, aiChat = false): CommandView {
  return { type: 'list', title, ...(aiChat ? { aiChat: true } : {}) };
}

test('restores AI state for the view revealed by back navigation', () => {
  const stackedChat = view('Stacked chat', true);
  const historyChat = view('History chat', true);

  assert.equal(restoredAiChatView([historyChat], [stackedChat]), historyChat);
  assert.equal(restoredAiChatView([], [stackedChat]), stackedChat);
  assert.equal(restoredAiChatView([view('List')], [stackedChat]), null);
  assert.equal(restoredAiChatView([], []), null);
});
