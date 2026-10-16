// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import type { ExtensionContext } from '../../resources/nevermind-extension-api';

export function nestedAiChatView(ctx: ExtensionContext) {
  return Object.assign(
    ctx.ui.chat({
      id: 'dev-ui-nested-ai-chat',
      title: 'Dev UI · Nested AI Chat',
      messages: [
        {
          role: 'assistant',
          content:
            'Type a draft and attach an image. Open Clipboard History with its global shortcut. Copy or Escape must restore the same draft, attachment, caret, and live stream. Do not send: this fixture uses deterministic local events.',
        },
      ],
    }),
    { aiChat: true, chatId: 'dev-ui-nested-ai-chat-session' },
  );
}

export async function delayedNestedToolView(ctx: ExtensionContext) {
  await new Promise((resolve) => setTimeout(resolve, 1500));
  return ctx.ui.list({
    id: 'dev-ui-delayed-tool',
    title: 'Dev UI · Delayed Tool',
    items: [
      {
        id: 'copy',
        title: 'Copy fixture text',
        primaryAction: {
          ...ctx.actions.copyText('Nested clipboard fixture'),
          dismissAfterRun: 'auto',
        },
      },
    ],
  });
}

export async function nestedToolWithoutView() {
  await new Promise((resolve) => setTimeout(resolve, 1500));
}

export async function failingNestedTool() {
  await new Promise((resolve) => setTimeout(resolve, 1500));
  throw new Error('Intentional nested tool fixture failure');
}
