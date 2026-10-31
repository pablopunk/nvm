import assert from 'node:assert/strict';
import test from 'node:test';
import type { AiChatMessage } from '../../shared/ai-chat-images';
import { appendAssistantDelta } from './ai-chat-delta';

function streamAnswer(deltas: string[]) {
  return deltas.reduce<AiChatMessage[]>(appendAssistantDelta, [
    { role: 'user', content: 'Four fruits as a list' },
  ]);
}

test('streamed whitespace-only deltas keep Markdown list items on separate lines', () => {
  const messages = streamAnswer(['- Apple', '\n', '- Banana', '\n', '- Mango']);

  assert.equal(
    messages[messages.length - 1].content,
    '- Apple\n- Banana\n- Mango',
  );
});

test('streamed answer equals the saved answer when deltas split list markers', () => {
  const saved = '- Apple\n- Banana\n- Orange\n- Mango';
  const messages = streamAnswer([
    '-',
    ' Apple',
    '\n',
    '-',
    ' ',
    'Banana',
    '\n- Orange\n',
    '- Mango',
  ]);

  assert.equal(messages[messages.length - 1].content, saved);
});

test('leading whitespace-only deltas do not create an empty assistant message', () => {
  assert.equal(streamAnswer([' \n ']).length, 1);
  assert.equal(streamAnswer([' \n ', 'Hi'])[1].content, 'Hi');
});
