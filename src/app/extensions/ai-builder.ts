import {
  calculate,
  getUrlFromQuery,
  parseRateExpression,
} from '../electron/search-utils';
import { extensionContext } from './_context';

const AI_BUILDER_EXTENSION_ID = 'nevermind.ai-builder';

export function createAiBuilderExtension() {
  function chatsSubtitle() {
    const count = Object.keys(extensionContext.userState.aiChats || {}).length;
    return `${count} AI ${count === 1 ? 'chat' : 'chats'}`;
  }
  function chatItems(ctx, query = '') {
    return Object.values(extensionContext.userState.aiChats || {})
      .map((item: any) => ({
        id: `ai-chat:${item.id}`,
        title:
          item.kind === 'conversation'
            ? `Continue conversation: ${item.title || item.query}`
            : `Continue AI chat: ${item.title || item.query}`,
        subtitle:
          item.kind === 'conversation'
            ? 'AI conversation'
            : item.status === 'ready'
              ? 'AI builder chat'
              : 'Continue AI builder chat',
        icon: item.kind === 'conversation' ? 'message-circle' : 'sparkles',
        score: 13,
        lastUsed: Math.max(item.updatedAt || 0, item.createdAt || 0),
        primaryAction: ctx.aiBuilder.openChat(item.id),
        appearance: { foreground: 'yellow' },
      }))
      .filter((item) => !query || extensionContext.rankAction(item, query));
  }
  return {
    id: AI_BUILDER_EXTENSION_ID,
    title: 'AI Builder',
    capabilities: ['ai', 'extensions.ownership'] as const,
    actions(ctx) {
      return [
        ctx.action({
          id: 'cleanup-expired-conversations',
          title: 'Clean Up Expired AI Conversations',
          placement: ['hidden'],
          mode: 'background',
          triggers: [
            { type: 'startup', delayMs: 1000 },
            { type: 'interval', every: 60 * 60 * 1000 },
          ],
          async run(innerCtx) {
            await innerCtx.aiBuilder?.cleanupExpiredConversations();
          },
        }),
      ];
    },
    commands: [
      {
        id: 'ai-chats',
        actionId: 'ai-chats',
        title: 'AI Chats',
        get subtitle() {
          return chatsSubtitle();
        },
        icon: 'sparkles',
        score: 16,
        run: () => extensionContext.aiChatsView(),
      },
    ],
    rootItems(ctx) {
      return chatItems(ctx).slice(0, 4);
    },
    searchItems(ctx, query) {
      const q = String(query || '').trim();
      const items: any[] = chatItems(ctx, q);
      if (
        q &&
        !getUrlFromQuery(q) &&
        calculate(q) === null &&
        !parseRateExpression(q)
      )
        items.push({
          id: `ai:${q}`,
          title: `Automate "${q}"`,
          subtitle: 'Build a reusable command with AI',
          query: q,
          icon: 'bolt',
          score: 40,
          appearance: { foreground: 'yellow', background: 'accent' },
          primaryAction: ctx.aiBuilder.startChat({
            prompt: q,
            title: `Automate "${q}"`,
          }),
        });
      return items
        .filter((item) => extensionContext.rankAction(item, q))
        .slice(0, 5);
    },
  };
}
