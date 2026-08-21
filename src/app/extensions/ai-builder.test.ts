import assert from 'node:assert/strict';
import test from 'node:test';
import { initExtensionContext } from './_context';
import { createAiBuilderExtension } from './ai-builder';

function context() {
  return {
    action: (input: unknown) => input,
    aiBuilder: {
      openChat: (chatId: string) => ({ type: 'openChat', chatId }),
      startChat: (input: unknown) => ({ type: 'startChat', input }),
      cleanupExpiredConversations: () => Promise.resolve(0),
    },
  };
}

test('offers automation as an Enter action without mentioning Tab', () => {
  initExtensionContext({
    userState: { aiChats: {} },
    rankAction: () => true,
  });

  const items = createAiBuilderExtension().searchItems(
    context() as any,
    'what is the weather',
  );
  const automation = items.find((item) => item.id.startsWith('ai:'));

  assert.equal(automation?.title, 'Automate "what is the weather"');
  assert.equal(automation?.subtitle, 'Build a reusable command with AI');
  assert.deepEqual(automation?.appearance, {
    foreground: 'yellow',
    background: 'accent',
  });
  assert.deepEqual(automation?.primaryAction, {
    type: 'startChat',
    input: {
      prompt: 'what is the weather',
      title: 'Automate "what is the weather"',
    },
  });
});

test('labels conversational history separately from builder chats', () => {
  initExtensionContext({
    userState: {
      aiChats: {
        conversation: {
          id: 'conversation',
          kind: 'conversation',
          title: 'Weather',
          updatedAt: 2,
        },
        builder: {
          id: 'builder',
          kind: 'builder',
          title: 'Weather command',
          status: 'ready',
          updatedAt: 1,
        },
      },
    },
    rankAction: () => true,
  });

  const items = createAiBuilderExtension().rootItems(context() as any);

  assert.deepEqual(
    items.map((item) => [item.title, item.subtitle, item.icon]),
    [
      ['Continue conversation: Weather', 'AI conversation', 'message-circle'],
      ['Continue AI chat: Weather command', 'AI builder chat', 'sparkles'],
    ],
  );
});

test('cleans expired conversations through declarative background triggers', async () => {
  let cleanupRuns = 0;
  const ctx = context();
  ctx.aiBuilder.cleanupExpiredConversations = async () => ++cleanupRuns;
  const [cleanup] = createAiBuilderExtension().actions(ctx as any) as any[];

  assert.equal(cleanup.placement[0], 'hidden');
  assert.equal(cleanup.mode, 'background');
  assert.deepEqual(cleanup.triggers, [
    { type: 'startup', delayMs: 1000 },
    { type: 'interval', every: 60 * 60 * 1000 },
  ]);

  await cleanup.run(ctx);
  assert.equal(cleanupRuns, 1);
});
