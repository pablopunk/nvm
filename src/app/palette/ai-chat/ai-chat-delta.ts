import type { AiChatMessage } from '../../shared/ai-chat-images';

export function appendAssistantDelta(
  messages: AiChatMessage[],
  text: string,
): AiChatMessage[] {
  const last = messages[messages.length - 1];
  if (last?.role === 'assistant')
    return [
      ...messages.slice(0, -1),
      { ...last, content: last.content + text },
    ];
  const content = text.trimStart();
  return content ? [...messages, { role: 'assistant', content }] : messages;
}
