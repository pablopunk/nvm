import assert from 'node:assert/strict';
import test from 'node:test';
import { aiChatItem } from './ai-chat-records';

test('builder chat from a described request starts without a generic question', () => {
  const item = aiChatItem('draft:1', 'Convert temperatures as I type');

  assert.deepEqual(item.messages, []);
  assert.equal(item.query, 'Convert temperatures as I type');
});

test('builder chat without a request asks what the command should do', () => {
  const [opening] = aiChatItem('draft:2', '').messages;

  assert.equal(opening.role, 'assistant');
  assert.match(opening.content, /What should this command do\?/);
});
