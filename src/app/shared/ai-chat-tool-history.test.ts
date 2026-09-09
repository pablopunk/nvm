import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isAiToolCallHistoryMessage,
  withoutAiToolCallHistory,
} from './ai-chat-tool-history';

test('identifies persisted tool-call rows without hiding system notices', () => {
  assert.equal(
    isAiToolCallHistoryMessage({
      role: 'system',
      content: 'list_capabilities',
    }),
    true,
  );
  assert.equal(
    isAiToolCallHistoryMessage({ role: 'system', content: 'Request failed' }),
    false,
  );
  assert.equal(
    isAiToolCallHistoryMessage({
      role: 'assistant',
      content: 'list_capabilities',
    }),
    false,
  );
});

test('removes tool-call rows from stored chat history', () => {
  const messages = [
    { role: 'user', content: 'Build an extension' },
    { role: 'system', content: 'read_extension_api' },
    { role: 'assistant', content: 'Done' },
  ];

  assert.deepEqual(withoutAiToolCallHistory(messages), [
    messages[0],
    messages[2],
  ]);
});
