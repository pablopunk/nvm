import assert from 'node:assert/strict';
import test from 'node:test';
import {
  actionFromExecutionRecord,
  currentActionExecutionRecord,
  type ActionExecutionRecord,
} from './action-execution-policy';

const NOW = 10_000;
const TTL_MS = 1_000;

function records(action: unknown, createdAt = NOW) {
  return new Map<string, ActionExecutionRecord>([
    ['trusted-token', { action: structuredClone(action), createdAt }],
  ]);
}

test('returns only the host-owned action and renderer trace metadata', () => {
  const stored = { type: 'shellExec', command: 'safe-command' };
  const resolved = actionFromExecutionRecord(
    {
      ...structuredClone(stored),
      command: 'forged-command',
      executionId: 'trusted-token',
      traceId: 'trace-id',
    },
    records(stored),
    { actionName: (action) => String(action.type), now: NOW, ttlMs: TTL_MS },
  );

  assert.deepEqual(resolved, { ...stored, traceId: 'trace-id' });
});

test('rejects absent, unknown, and wrong-store tokens', () => {
  const rootRecords = records({ kind: 'extension-action' });
  const viewRecords = new Map<string, ActionExecutionRecord>();
  const resolve = (action: unknown, store = rootRecords) =>
    actionFromExecutionRecord(action, store, {
      actionName: (input) => String(input.kind || input.type || 'unknown'),
      now: NOW,
      ttlMs: TTL_MS,
    });

  assert.throws(() => resolve({ kind: 'extension-action' }), /Untrusted/);
  assert.throws(
    () => resolve({ kind: 'extension-action', executionId: 'unknown' }),
    /Untrusted/,
  );
  assert.throws(
    () =>
      resolve(
        { kind: 'extension-action', executionId: 'trusted-token' },
        viewRecords,
      ),
    /Untrusted/,
  );
});

test('rejects and removes expired tokens', () => {
  const store = records({ type: 'openPath', path: '/safe' }, NOW - TTL_MS - 1);

  assert.equal(
    currentActionExecutionRecord('trusted-token', store, NOW, TTL_MS),
    null,
  );
  assert.equal(store.has('trusted-token'), false);
});

test('requires host records for privileged view action types', () => {
  const types = [
    'openPath',
    'revealPath',
    'quickLook',
    'pasteClipboard',
    'typeText',
    'createWindow',
    'toggleWindow',
    'showWindow',
    'hideWindow',
    'closeWindow',
    'shellExec',
    'shellScript',
    'lockScreen',
    'sleepSystem',
    'restartSystem',
    'toggleSetting',
  ];

  for (const type of types) {
    assert.throws(
      () =>
        actionFromExecutionRecord(
          { type },
          new Map<string, ActionExecutionRecord>(),
          { actionName: (action) => String(action.type), ttlMs: TTL_MS },
        ),
      new RegExp(`Untrusted ${type} action`),
    );
  }
});

test('does not execute forged nested root actions through a valid token', () => {
  const stored = {
    kind: 'extension-action',
    rootAction: { type: 'copyText', text: 'safe' },
  };

  assert.deepEqual(
    actionFromExecutionRecord(
      {
        ...stored,
        executionId: 'trusted-token',
        rootAction: { type: 'shellExec', command: 'forged-command' },
      },
      records(stored),
      {
        actionName: (action) => String(action.kind),
        now: NOW,
        ttlMs: TTL_MS,
      },
    ),
    stored,
  );
});
