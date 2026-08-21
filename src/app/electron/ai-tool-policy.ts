export type AiToolMode = 'none' | 'conversation';

export const CONVERSATION_AI_TOOLS = [
  'read',
  'grep',
  'find',
  'ls',
  'web_search',
  'fetch_content',
] as const;

export function aiPromptUsesDirectModel(options: {
  sessionId?: string;
  toolMode?: AiToolMode;
}) {
  return !options.sessionId && options.toolMode !== 'conversation';
}

export function missingConversationAiTools(activeToolNames: string[]) {
  const active = new Set(activeToolNames);
  return CONVERSATION_AI_TOOLS.filter((name) => !active.has(name));
}
