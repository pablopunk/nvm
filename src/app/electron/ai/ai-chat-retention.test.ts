import assert from 'node:assert/strict';
import test from 'node:test';
import {
  AI_CONVERSATION_RETENTION_MS,
  expiredConversationAiChatIds,
} from './ai-chat-retention';

const now = 2_000_000_000_000;

test('expires conversations after one day of inactivity', () => {
  const chats = {
    expired: {
      kind: 'conversation',
      createdAt: now - AI_CONVERSATION_RETENTION_MS * 2,
      updatedAt: now - AI_CONVERSATION_RETENTION_MS,
    },
    recent: {
      kind: 'conversation',
      createdAt: now - AI_CONVERSATION_RETENTION_MS * 2,
      updatedAt: now - AI_CONVERSATION_RETENTION_MS + 1,
    },
  };

  assert.deepEqual(expiredConversationAiChatIds(chats, now), ['expired']);
});

test('uses creation time when a conversation has no update time', () => {
  assert.deepEqual(
    expiredConversationAiChatIds(
      {
        expired: {
          kind: 'conversation',
          createdAt: now - AI_CONVERSATION_RETENTION_MS,
        },
      },
      now,
    ),
    ['expired'],
  );
});

test('never expires extension-builder or legacy chats automatically', () => {
  const old = now - AI_CONVERSATION_RETENTION_MS * 10;
  const chats = {
    extension: {
      kind: 'builder',
      createdAt: old,
      updatedAt: old,
      contextExtensionFile: 'timer.ts',
    },
    detachedBuilder: { kind: 'builder', createdAt: old, updatedAt: old },
    legacyExtension: { createdAt: old, updatedAt: old },
    unknownAge: { kind: 'conversation' },
  };

  assert.deepEqual(expiredConversationAiChatIds(chats, now), []);
});
