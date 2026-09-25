const TRUNCATION_NOTICE = '\n... earlier content truncated ...';

export const AI_CHAT_TRANSCRIPT_MESSAGE_LIMIT = 20_000;
export const AI_CHAT_TRANSCRIPT_TOTAL_LIMIT = 80_000;

export function truncateAiChatTranscriptContent(content: string): string {
  const text = String(content || '');
  if (text.length <= AI_CHAT_TRANSCRIPT_MESSAGE_LIMIT) {
    return text;
  }
  return `${text.slice(0, AI_CHAT_TRANSCRIPT_MESSAGE_LIMIT)}${TRUNCATION_NOTICE}`;
}

export function joinAiChatTranscript(
  entries: string[],
  totalLimit: number = AI_CHAT_TRANSCRIPT_TOTAL_LIMIT,
): string {
  const joined = entries.join('\n\n');
  if (joined.length <= totalLimit) {
    return joined;
  }
  const truncated = joined.slice(joined.length - totalLimit);
  return `... earlier transcript truncated ...\n\n${truncated}`;
}
