interface AiChatHistoryMessage {
  role?: unknown;
  content?: unknown;
}

const TOOL_CALL_NAME_PATTERN = /^[a-z][a-z0-9]*(?:_[a-z0-9]+)+$/;

function isAiToolCallHistoryMessage(message: AiChatHistoryMessage) {
  return (
    message.role === 'system' &&
    TOOL_CALL_NAME_PATTERN.test(String(message.content || ''))
  );
}

function withoutAiToolCallHistory<T extends AiChatHistoryMessage>(
  messages: readonly T[],
) {
  return messages.filter((message) => !isAiToolCallHistoryMessage(message));
}

export { isAiToolCallHistoryMessage, withoutAiToolCallHistory };
