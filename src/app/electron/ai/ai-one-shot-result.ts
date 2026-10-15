interface AssistantMessage {
  role?: unknown;
  content?: unknown;
}

interface AssistantStreamEvent {
  type: string;
  delta?: unknown;
  message?: AssistantMessage;
  error?: { errorMessage?: string; stopReason?: string };
}

function assistantText(content: unknown) {
  if (typeof content === 'string') {
    return content;
  }
  if (!Array.isArray(content)) {
    return '';
  }
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
    if (message?.role === 'assistant') {
      return assistantText(message.content);
    }
  }
  return streamedDeltas.join('');
}

export async function streamedOneShotAssistantText(
  events: AsyncIterable<AssistantStreamEvent> | Iterable<AssistantStreamEvent>,
  onDelta: (delta: string) => void,
) {
  const deltas: string[] = [];
  let finalText: string | null = null;
  for await (const event of events) {
    if (event.type === 'text_delta' && typeof event.delta === 'string') {
      deltas.push(event.delta);
      onDelta(event.delta);
    } else if (event.type === 'done' && event.message) {
      finalText = finalOneShotAssistantText([event.message], deltas);
    } else if (event.type === 'error' && event.error) {
      const error = new Error(event.error.errorMessage || 'AI request failed');
      if (event.error.stopReason === 'aborted') {
        error.name = 'AbortError';
      }
      throw error;
    }
  }
  return finalText ?? deltas.join('');
}
