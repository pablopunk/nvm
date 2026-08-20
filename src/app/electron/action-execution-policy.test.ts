import assert from 'node:assert/strict';
import test from 'node:test';
import {
  type ActionExecutionRecord,
  actionFromExecutionRecord,
  createActionExecutionCapabilities,
  currentActionExecutionRecord,
  mutableViewActionFields,
  nativeActionFromExecutionRecord,
} from './action-execution-policy';

const NOW = 10_000;
const TTL_MS = 1000;
const CAPABILITY_SECRET = Buffer.from(
  'never-mind-action-capability-test',
  'utf8',
);
const ACTIONS_BEYOND_RECORD_BUDGET = 2001;
const UNTRUSTED_ERROR = /Untrusted/;

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

  assert.throws(() => resolve({ kind: 'extension-action' }), UNTRUSTED_ERROR);
  assert.throws(
    () => resolve({ kind: 'extension-action', executionId: 'unknown' }),
    UNTRUSTED_ERROR,
  );
  assert.throws(
    () =>
      resolve(
        { kind: 'extension-action', executionId: 'trusted-token' },
        viewRecords,
      ),
    UNTRUSTED_ERROR,
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

test('resolves renderer native app wrappers through the root action token', () => {
  const stored = {
    id: 'extension-root:nevermind.apps:app:safari',
    kind: 'extension-root-item',
    rootAction: {
      type: 'openPath',
      title: 'Open Safari',
      path: '/Applications/Safari.app',
    },
  };

  assert.deepEqual(
    nativeActionFromExecutionRecord(
      {
        type: 'nativeAction',
        title: 'Safari',
        nativeAction: {
          ...stored,
          executionId: 'trusted-token',
        },
        traceId: 'trace-id',
      },
      records(stored),
      { now: NOW, ttlMs: TTL_MS },
    ),
    {
      type: 'nativeAction',
      title: 'Safari',
      nativeAction: { ...stored, traceId: 'trace-id' },
      traceId: 'trace-id',
    },
  );
});

test('resolves a current signed capability without an execution record', () => {
  const capabilities = createActionExecutionCapabilities(CAPABILITY_SECRET);
  const action = { type: 'copyText', title: 'Copy', text: 'trusted' };
  const executionId = capabilities.issue(action, {
    scope: 'view',
    owner: 'clipboard',
    ownerVersion: 'current',
    mutableFields: mutableViewActionFields(action),
  });

  assert.deepEqual(
    capabilities.resolve(
      { ...action, executionId, traceId: 'trace-id' },
      {
        scope: 'view',
        ownerIsCurrent: (owner, version) =>
          owner === 'clipboard' && version === 'current',
      },
    ),
    { ...action, traceId: 'trace-id' },
  );
  assert.equal(
    capabilities.resolve(
      { ...action, text: 'forged', executionId },
      { scope: 'view', ownerIsCurrent: () => true },
    ),
    null,
  );
});

test('signed capabilities reject changed actions, scopes, and owners', () => {
  const capabilities = createActionExecutionCapabilities(CAPABILITY_SECRET);
  const action = { type: 'shellExec', command: 'trusted' };
  const executionId = capabilities.issue(action, {
    scope: 'view',
    owner: 'extension',
    ownerVersion: 'first',
  });
  const currentOwner = () => true;

  assert.equal(
    capabilities.resolve(
      { ...action, command: 'forged', executionId },
      { scope: 'view', ownerIsCurrent: currentOwner },
    ),
    null,
  );
  assert.equal(
    capabilities.resolve(
      { ...action, executionId },
      { scope: 'root', ownerIsCurrent: currentOwner },
    ),
    null,
  );
  assert.equal(
    capabilities.resolve(
      { ...action, executionId },
      { scope: 'view', ownerIsCurrent: () => false },
    ),
    null,
  );
});

test('signed root capabilities preserve the execution time limit', () => {
  const capabilities = createActionExecutionCapabilities(CAPABILITY_SECRET);
  const action = { kind: 'extension-root-item' };
  const executionId = capabilities.issue(action, {
    scope: 'root',
    owner: 'extension:test',
    ownerVersion: 'current',
    issuedAt: NOW,
  });

  assert.equal(
    capabilities.resolve(
      { ...action, executionId },
      {
        scope: 'root',
        ownerIsCurrent: () => true,
        maxAgeMs: TTL_MS,
        now: NOW + TTL_MS + 1,
      },
    ),
    null,
  );
});

test('signed view capabilities allow only declared renderer input', () => {
  const capabilities = createActionExecutionCapabilities(CAPABILITY_SECRET);
  const action = { type: 'runExtensionAction', handlerId: 'handler' };
  const executionId = capabilities.issue(action, {
    scope: 'view',
    owner: 'extension',
    ownerVersion: 'current',
    mutableFields: mutableViewActionFields(action),
  });
  const options = { scope: 'view' as const, ownerIsCurrent: () => true };

  assert.deepEqual(
    capabilities.resolve(
      { ...action, executionId, formValues: { name: 'Pablo' } },
      options,
    ),
    { ...action, formValues: { name: 'Pablo' } },
  );
  assert.equal(
    capabilities.resolve(
      { ...action, executionId, command: 'forged' },
      options,
    ),
    null,
  );
});

test('signed capabilities survive action counts beyond the record budget', () => {
  const capabilities = createActionExecutionCapabilities(CAPABILITY_SECRET);
  const firstAction = { type: 'copyText', text: 'first' };
  const firstExecutionId = capabilities.issue(firstAction, {
    scope: 'view',
    owner: 'host',
    ownerVersion: 'current',
  });
  for (let index = 0; index < ACTIONS_BEYOND_RECORD_BUDGET; index += 1) {
    capabilities.issue(
      { type: 'copyText', text: `unrelated ${index}` },
      { scope: 'view', owner: 'host', ownerVersion: 'current' },
    );
  }

  assert.deepEqual(
    capabilities.resolve(
      { ...firstAction, executionId: firstExecutionId },
      { scope: 'view', ownerIsCurrent: () => true },
    ),
    firstAction,
  );
});
