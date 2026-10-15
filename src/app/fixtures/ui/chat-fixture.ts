// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import type { ExtensionContext } from '../../resources/nevermind-extension-api';

type ChatMessage = { role: 'system' | 'user' | 'assistant'; content: string };
const CHAT_STORAGE_KEY = 'dev-ui-chat-messages';

export async function chatView(ctx: ExtensionContext) {
  const messages =
    (await ctx.storage.get<ChatMessage[]>(CHAT_STORAGE_KEY, [
      {
        role: 'system',
        content: 'Interactive chat fixture. No AI backend — just local echo.',
      },
      {
        role: 'user',
        content: 'Can generated extensions render chat bubbles?',
      },
      {
        role: 'assistant',
        content:
          'Yes — this is a host-rendered chat view. Type a message below to test the interactive fixture.',
      },
    ])) || [];
  const sendAction = ctx.actions.run(
    'Send Chat Message',
    async (innerCtx, action) => {
      const userMessage = String(action.formValues?.message || '').trim();
      if (!userMessage) return;
      const current =
        (await innerCtx.storage.get<ChatMessage[]>(CHAT_STORAGE_KEY, [])) || [];
      const updated: ChatMessage[] = [
        ...current,
        { role: 'user', content: userMessage },
        { role: 'assistant', content: `Echo: ${userMessage}` },
      ];
      await innerCtx.storage.set(CHAT_STORAGE_KEY, updated);
      return innerCtx.ui.chat({
        id: 'dev-ui-chat',
        title: 'Dev UI · Chat',
        messages: updated,
        submitAction: sendAction,
        placeholder: 'Type a message…',
      });
    },
  );
  return ctx.ui.chat({
    id: 'dev-ui-chat',
    title: 'Dev UI · Chat',
    messages,
    submitAction: sendAction,
    placeholder:
      'Draft text, open Clipboard History via its global shortcut, then Escape and paste…',
  });
}
