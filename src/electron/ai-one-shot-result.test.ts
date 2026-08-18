import assert from 'node:assert/strict';
import test from 'node:test';
import { finalOneShotAssistantText } from './ai-one-shot-result';

test('returns only the final assistant response after retries', () => {
  assert.equal(
    finalOneShotAssistantText(
      [
        { role: 'assistant', content: [{ type: 'text', text: 'Hello.' }] },
        { role: 'assistant', content: [{ type: 'text', text: 'Hello.' }] },
        { role: 'assistant', content: [{ type: 'text', text: 'Hello.' }] },
      ],
      ['Hello.', 'Hello.', 'Hello.'],
    ),
    'Hello.',
  );
});

test('falls back to streamed deltas when no final assistant message exists', () => {
  assert.equal(finalOneShotAssistantText([], ['Hel', 'lo.']), 'Hello.');
});
