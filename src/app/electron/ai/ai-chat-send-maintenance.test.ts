import assert from 'node:assert/strict';
import test, { mock } from 'node:test';
import type {
  ActiveConversationRequest,
  PendingAiChatSend,
} from './ai-chat-lifecycle';
import { createAiChatSessionTeardown } from './ai-chat-session-teardown';

mock.module('../diagnostics/debug-performance', {
  namedExports: {
    performanceTraces: {
      run: (_name: string, _data: unknown, task: () => unknown) => task(),
    },
  },
});
mock.module('../diagnostics/observability', {
  namedExports: { operationOutcome: () => {} },
});
const { createAiChatSend } = await import('./ai-chat-send');

for (const kind of ['builder', 'conversation'])
  test(`suspension and teardown wait for ${kind} sends to settle`, async () => {
    let finish!: () => void;
    const response = new Promise<void>((resolve) => {
      finish = resolve;
    });
    const pending = new Map<string, PendingAiChatSend>();
    const conversations = new Map<string, ActiveConversationRequest>();
    const service = createAiChatSend({
      userStateStore: { get: () => ({ aiChats: { chat: { kind } } }) },
      pendingAiChatSends: pending,
      activeConversationRequests: conversations,
      activeConversationSessionIds: new Set(),
      activeBuilderSessionIds: new Set(),
      pendingAiChatModelChanges: new Set(),
      getNevermindAi: () => ({
        send: () => response,
        session: () => ({
          stream: () => ({ result: response, abort: () => {} }),
          reset: async () => {},
        }),
      }),
      setActiveAiChatId: () => {},
      initNevermindAi: async () => {},
      promoteDraftAiChat: () => {},
      appendAiChatMessage: () => ({}),
      appendAiChatDelta: () => {},
      patchAiChatsItem: () => {},
      persistAiChatImages: async () => [],
      aiChatMessageForRenderer: (message) => message,
      aiChatPromptWithContext: (message) => message,
      conversationPromptWithContext: (message) => message,
      restoredConversationImages: async () => [],
      registerAiEventActionForRenderer: (event) => event,
      broadcastAiChatEvent: () => {},
      scheduleSaveState: () => {},
    });
    const send = service.sendAiChatMessage(
      'hello',
      'chat',
      undefined,
      undefined,
    );
    await new Promise<void>((resolve) => setImmediate(resolve));
    const active = pending.get('chat');
    assert.ok(active);
    service.beginAiChatSendSuspension();
    await assert.rejects(
      service.sendAiChatMessage('again', 'chat', undefined, undefined),
      /unavailable/,
    );
    let settled = false;
    void active.settled.then(() => {
      settled = true;
    });
    const teardown = createAiChatSessionTeardown({
      userStateStore: { get: () => ({}) },
      pendingAiChatSends: pending,
      activeConversationRequests: conversations,
      activeConversationSessionIds: new Set(),
      activeBuilderSessionIds: new Set(),
      getNevermindAi: () => ({ disposeAllSessions: async () => {} }),
      getActiveAiChatId: () => undefined,
      setActiveAiChatId: () => {},
      abortAiChat: async () => {},
      recordLearningReview: () => {},
    });
    let disposed = false;
    const disposal = teardown.disposeAiSessions().then(() => {
      disposed = true;
    });
    await new Promise<void>((resolve) => setImmediate(resolve));
    assert.equal(settled, false);
    assert.equal(disposed, false);
    finish();
    await send;
    await active.settled;
    await disposal;
    assert.equal(pending.size, 0);
    service.endAiChatSendSuspension();
  });
