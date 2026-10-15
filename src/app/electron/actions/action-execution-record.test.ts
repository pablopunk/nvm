import assert from 'node:assert/strict';
import test from 'node:test';
import { actionMatchesExecutionRecord } from './action-execution-record';

test('matches an action that only adds its execution token', () => {
  const stored = {
    type: 'pushView',
    title: 'Preview',
    view: {
      type: 'preview',
      actions: [{ type: 'openWith', executionId: 'nested-token' }],
    },
  };

  assert.equal(
    actionMatchesExecutionRecord(
      { ...structuredClone(stored), executionId: 'outer-token' },
      { action: stored },
    ),
    true,
  );
});

test('rejects a reused token when the action payload changed', () => {
  assert.equal(
    actionMatchesExecutionRecord(
      {
        type: 'openWith',
        path: '/tmp/other.png',
        executionId: 'open-with-token',
      },
      {
        action: { type: 'openWith', path: '/tmp/screenshot.png' },
      },
    ),
    false,
  );
});
