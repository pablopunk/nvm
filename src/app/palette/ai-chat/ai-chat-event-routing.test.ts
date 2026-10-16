import assert from 'node:assert/strict';
import test from 'node:test';
import { AiChatEventOwners } from './ai-chat-event-routing';

test('legacy events are not assigned merely because one chat happens to be mounted', () => {
  const owners = new AiChatEventOwners();
  assert.equal(owners.sessionFor({ type: 'delta' }), undefined);
  owners.begin('a', 'trace-a');
  assert.equal(owners.sessionFor({ type: 'delta' }), 'a');
  owners.begin('b', 'trace-b');
  assert.equal(owners.sessionFor({ type: 'delta' }), undefined);
  assert.equal(owners.sessionFor({ type: 'delta', traceId: 'trace-a' }), 'a');
  assert.equal(owners.sessionFor({ type: 'delta', chatId: 'b' }), 'b');
});

test('terminal events and session release clear initiating ownership', () => {
  const owners = new AiChatEventOwners();
  owners.begin('a', 'trace');
  owners.finish({ type: 'done', traceId: 'trace' });
  assert.equal(
    owners.sessionFor({ type: 'delta', traceId: 'trace' }),
    undefined,
  );
  owners.begin('b', 'second');
  owners.release('b');
  assert.equal(owners.sessionFor({ type: 'delta' }), undefined);
});
