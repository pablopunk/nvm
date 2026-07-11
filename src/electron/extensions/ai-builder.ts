import {
  calculate,
  getUrlFromQuery,
  parseRateExpression,
} from '../search-utils';
import { extensionContext } from './_context';

const AI_BUILDER_EXTENSION_ID = 'nevermind.ai-builder';

export function createAiBuilderExtension() {
  function chatsSubtitle() {
    const count = Object.keys(extensionContext.userState.aiChats || {}).length;
    return `${count} builder ${count === 1 ? 'chat' : 'chats'}`;
  }
  function chatItems(ctx, query = '') {
    return Object.values(extensionContext.userState.aiChats || {})
      .map((item: any) => ({
        id: `ai-chat:${item.id}`,
        title: `Continue AI chat: ${item.title || item.query}`,
        subtitle:
          item.status === 'ready'
            ? 'AI builder chat'
            : 'Continue AI builder chat',
        icon: 'sparkles',
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
    permissions: ['ai', 'extensions.ownership'] as const,
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
          title: `Press Tab to automate "${q}"`,
          subtitle: 'Automate with AI',
          query: q,
          icon: 'bolt',
          score: 40,
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
