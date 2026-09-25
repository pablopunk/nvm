import assert from 'node:assert/strict';
import test from 'node:test';
import {
  AI_CHAT_TRANSCRIPT_MESSAGE_LIMIT,
  AI_CHAT_TRANSCRIPT_TOTAL_LIMIT,
  joinAiChatTranscript,
  truncateAiChatTranscriptContent,
} from './ai-chat-context';

const OVER_LIMIT_EXTRA_CHARS = 100;
const LARGE_ENTRY_CHARS = 10_000;
const TOTAL_LIMIT_SLACK = 50;
const MESSAGE_COUNT = 12;

test('AI chat transcript keeps short content intact', () => {
  assert.equal(truncateAiChatTranscriptContent('hello'), 'hello');
  assert.equal(joinAiChatTranscript(['a', 'b']), 'a\n\nb');
});

test('AI chat transcript truncates a pasted accessibility dump per message', () => {
  const dump = 'x'.repeat(
    AI_CHAT_TRANSCRIPT_MESSAGE_LIMIT + OVER_LIMIT_EXTRA_CHARS,
  );
  const truncated = truncateAiChatTranscriptContent(dump);
  assert.ok(truncated.length < dump.length);
  assert.ok(truncated.includes('truncated'));
});

test('AI chat transcript keeps the newest messages within total budget', () => {
  const entries = Array.from(
    { length: MESSAGE_COUNT },
    (_, index) => `message-${index}-${'y'.repeat(LARGE_ENTRY_CHARS)}`,
  );
  const joined = joinAiChatTranscript(entries);
  assert.ok(
    joined.length <= AI_CHAT_TRANSCRIPT_TOTAL_LIMIT + TOTAL_LIMIT_SLACK,
  );
  assert.ok(joined.includes('message-11'));
});
