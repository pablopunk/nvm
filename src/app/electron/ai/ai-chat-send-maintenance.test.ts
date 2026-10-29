import assert from 'node:assert/strict';
import test, { mock } from 'node:test';
import type { PendingAiChatSend } from './ai-chat-lifecycle';
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

test('suspension blocks new sends and pending builder work remains tracked until it settles', async () => {
  let finish!: () => void;
  const response = new Promise<void>((resolve) => {
    finish = resolve;
  });
  const pending = new Map<string, PendingAiChatSend>();
  const service = createAiChatSend({
    userStateStore: { get: () => ({ aiChats: { chat: { kind: 'builder' } } }) },
    pendingAiChatSends: pending,
    activeConversationRequests: new Map(),
    activeConversationSessionIds: new Set(),
    activeBuilderSessionIds: new Set(),
    pendingAiChatModelChanges: new Set(),
    getNevermindAi: () => ({ send: () => response }),
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
  const send = service.sendAiChatMessage('hello', 'chat');
  await new Promise<void>((resolve) => setImmediate(resolve));
  const active = pending.get('chat');
  assert.ok(active);
  service.beginAiChatSendSuspension();
  await assert.rejects(
    service.sendAiChatMessage('again', 'chat'),
    /unavailable/,
  );
  let settled = false;
  void active.settled.then(() => {
    settled = true;
  });
  const teardown = createAiChatSessionTeardown({
    userStateStore: { get: () => ({}) },
    pendingAiChatSends: pending,
    activeConversationRequests: new Map(),
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
