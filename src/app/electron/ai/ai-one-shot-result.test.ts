import assert from 'node:assert/strict';
import test from 'node:test';
import {
  finalOneShotAssistantText,
  streamedOneShotAssistantText,
} from './ai-one-shot-result';

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

test('forwards direct stream deltas and returns the final assistant text', async () => {
  const deltas: string[] = [];
  const result = await streamedOneShotAssistantText(
    (function* events() {
      yield { type: 'text_delta' as const, delta: 'Hel' };
      yield { type: 'text_delta' as const, delta: 'lo' };
      yield {
        type: 'done' as const,
        message: {
          role: 'assistant',
          content: [{ type: 'text', text: 'Hello' }],
        },
      };
    })(),
    (delta) => deltas.push(delta),
  );

  assert.equal(result, 'Hello');
  assert.deepEqual(deltas, ['Hel', 'lo']);
});

test('turns an aborted direct stream into an AbortError', async () => {
  const deltas: string[] = [];
  await assert.rejects(
    streamedOneShotAssistantText(
      (function* events() {
        yield {
          type: 'error' as const,
          error: { stopReason: 'aborted', errorMessage: 'Request aborted' },
        };
      })(),
      (delta) => deltas.push(delta),
    ),
    { name: 'AbortError', message: 'Request aborted' },
  );
  assert.deepEqual(deltas, []);
});
