type AssistantMessage = {
  role?: unknown;
  content?: unknown;
};

function assistantText(content: unknown) {
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return '';
  return content
    .filter(
      (part): part is { type: 'text'; text: string } =>
        part?.type === 'text' && typeof part.text === 'string',
    )
    .map((part) => part.text)
    .join('');
}

export function finalOneShotAssistantText(
  messages: AssistantMessage[],
  streamedDeltas: string[],
) {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message?.role === 'assistant') return assistantText(message.content);
  }
  return streamedDeltas.join('');
}
