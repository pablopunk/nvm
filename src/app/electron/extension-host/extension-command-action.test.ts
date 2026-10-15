import assert from 'node:assert/strict';
import test from 'node:test';
import { extensionCommandAction } from './extension-command-action';

test('preserves a declarative command action', () => {
  const quitAction = { type: 'quitApp' };
  const runAction = { type: 'runExtensionAction' };

  assert.equal(
    extensionCommandAction({ primaryAction: quitAction }, runAction),
    quitAction,
  );
});

test('uses the command runner when no declarative action exists', () => {
  const runAction = { type: 'runExtensionAction' };

  assert.equal(extensionCommandAction({}, runAction), runAction);
});
