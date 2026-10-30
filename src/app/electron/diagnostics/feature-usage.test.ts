import assert from 'node:assert/strict';
import test from 'node:test';
import {
  actionFeatureLabel,
  buildChannel,
  createFeatureUsageDeduper,
  featureUsage,
  hashOwnerId,
  isRoutineBackgroundSuccess,
} from './feature-usage';

test('only operations a person starts count as feature usage', () => {
  assert.deepEqual(
    featureUsage({
      operation: 'command.execute',
      outcome: 'success',
      label: 'extension-command:nevermind.clipboard:history',
      component: 'clipboard',
    }),
    {
      feature: 'command.run',
      variant: 'extension-command:nevermind.clipboard:history',
    },
  );
  assert.deepEqual(
    featureUsage({
      operation: 'ai.stream',
      outcome: 'failed',
      component: 'ai',
    }),
    { feature: 'ai.chat', variant: 'ai' },
  );
  for (const operation of [
    'extension.load',
    'update.check',
    'update.download',
    'job.run',
    'backend.request',
    'view.load',
    'support.report',
  ] as const)
    assert.equal(featureUsage({ operation, outcome: 'success' }), undefined);
  assert.equal(
    featureUsage({ operation: 'command.execute', outcome: 'cancelled' }),
    undefined,
  );
});

test('user extensions collapse to one bucket and never leak their ids or text', () => {
  assert.equal(
    actionFeatureLabel(
      {
        kind: 'extension-command',
        extensionId: 'acme-secret-project',
        commandId: 'my private command',
      },
      false,
    ),
    'user-extension',
  );
  assert.equal(
    actionFeatureLabel({ kind: 'app', extensionId: undefined }, false),
    'app',
  );
  assert.equal(
    actionFeatureLabel(
      {
        kind: 'extension-command',
        extensionId: 'nevermind.clipboard',
        commandId: 'history',
      },
      true,
    ),
    'extension-command:nevermind.clipboard:history',
  );
  // Built-in ids still must look like identifiers, never paths or sentences.
  assert.equal(
    actionFeatureLabel(
      { kind: 'extension-command', commandId: '/Users/me/secret file' },
      true,
    ),
    'extension-command',
  );
  // Root items copy ids from data (clipboard entries, apps, files): never sent.
  assert.equal(
    actionFeatureLabel(
      {
        kind: 'extension-root-item',
        extensionId: 'nevermind.clipboard',
        commandId: 'clipboard-item-3f9a',
      },
      true,
    ),
    'extension-root-item:nevermind.clipboard',
  );
  assert.equal(actionFeatureLabel(null, false), 'unknown');
});

test('a feature counts once per journey but again in another', () => {
  const firstUse = createFeatureUsageDeduper(2);
  assert.equal(firstUse('journey-1', 'auth.signin'), true);
  assert.equal(firstUse('journey-1', 'auth.signin'), false);
  assert.equal(firstUse('journey-1', 'command.run'), true);
  assert.equal(firstUse('journey-2', 'auth.signin'), true);
});

test('owner ids are one-way, stable and fixed width', () => {
  const hash = hashOwnerId('user_123');
  assert.match(hash, /^[0-9a-f]{64}$/);
  assert.equal(hash, hashOwnerId('user_123'));
  assert.notEqual(hash, hashOwnerId('user_124'));
  assert.ok(!hash.includes('user_123'));
});

test('calendar versions are prerelease builds and 0.x versions are stable', () => {
  assert.equal(buildChannel('2026.1030.911'), 'prerelease');
  assert.equal(buildChannel('0.25.0'), 'stable');
  assert.equal(buildChannel('1.2.3'), 'stable');
});

test('only successful background job ticks are routine', () => {
  assert.equal(
    isRoutineBackgroundSuccess({ operation: 'job.run', outcome: 'success' }),
    true,
  );
  assert.equal(
    isRoutineBackgroundSuccess({ operation: 'job.run', outcome: 'failed' }),
    false,
  );
  assert.equal(
    isRoutineBackgroundSuccess({
      operation: 'command.execute',
      outcome: 'success',
    }),
    false,
  );
});
