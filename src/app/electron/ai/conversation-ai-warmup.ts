// biome-ignore-all lint: the AI runtime is dynamically typed at this boundary.
import { warn as logWarn } from '../platform/logger';
import { CONVERSATION_SYSTEM_PROMPT } from './ai-chat-prompts';

const CONVERSATION_AI_WARM_INTERVAL_MS = 30_000;

export type ConversationAiWarmupDeps = {
  getNevermindAi: () => any;
};

export function createConversationAiWarmup(deps: ConversationAiWarmupDeps) {
  const { getNevermindAi } = deps;

  let lastConversationAiWarmAt = 0;

  function warmConversationAiOnPaletteOpen() {
    if (
      !getNevermindAi() ||
      Date.now() - lastConversationAiWarmAt < CONVERSATION_AI_WARM_INTERVAL_MS
    )
      return;
    lastConversationAiWarmAt = Date.now();
    void getNevermindAi()
      .prepare({
        model: 'fast',
        system: CONVERSATION_SYSTEM_PROMPT,
        toolMode: 'conversation',
      })
      .catch((error) =>
        logWarn('ai.conversation.palette-warm.failed', error, {
          source: 'host',
          scope: 'ai',
        }),
      );
  }

  return { warmConversationAiOnPaletteOpen };
}
