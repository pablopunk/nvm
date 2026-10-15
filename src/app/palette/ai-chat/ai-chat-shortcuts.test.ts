import assert from 'node:assert/strict';
import test from 'node:test';
import { shouldStartConversationFromTab } from './ai-chat-shortcuts';

const rootTab = {
  key: 'Tab',
  query: 'What is the weather?',
  isChildOpen: false,
  altKey: false,
  ctrlKey: false,
  metaKey: false,
  shiftKey: false,
};

test('starts a conversation from an unmodified root Tab', () => {
  assert.equal(shouldStartConversationFromTab(rootTab), true);
});

test('does not replace normal Tab behavior in child views or for empty queries', () => {
  assert.equal(
    shouldStartConversationFromTab({ ...rootTab, isChildOpen: true }),
    false,
  );
  assert.equal(
    shouldStartConversationFromTab({ ...rootTab, query: '' }),
    false,
  );
  assert.equal(
    shouldStartConversationFromTab({ ...rootTab, query: '   ' }),
    false,
  );
});

test('ignores modified Tab shortcuts', () => {
  for (const modifier of ['altKey', 'ctrlKey', 'metaKey', 'shiftKey'] as const)
    assert.equal(
      shouldStartConversationFromTab({ ...rootTab, [modifier]: true }),
      false,
    );
});
