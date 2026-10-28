import assert from 'node:assert/strict';
import test from 'node:test';
import { createExtensionActionResults } from './extension-action-results';

test('a returned action requiring confirmation is presented without executing it', async () => {
  let executions = 0;
  const results = createExtensionActionResults({
    executeViewAction: async () => {
      executions += 1;
    },
    createExtensionContext: () => ({}),
    setDiagnosticExtensionTarget: () => {},
    extensionActionHandlers: new Map(),
    extensionErrorView: () => ({}),
    extensionDraftStore: { resolveByKey: () => {} } as any,
    schedulePersistExtensionDrafts: () => {},
    normalizeViewAction: (action) => action,
    normalizeExtensionView: (view) => view,
    normalizeViewPatch: (patch) => patch,
    normalizeViewItems: (items) => items,
  });
  const action = {
    type: 'runExtensionAction',
    title: 'Delete',
    requiresConfirmation: true,
    confirmMessage: 'Move files to Trash?',
    confirmLabel: 'Move to Trash',
  };
  for (const result of [action, { action }]) {
    const response = await results.executeViewActionResult(result, {});
    assert.equal(executions, 0);
    assert.equal(response.navigation, 'push');
    assert.equal(
      response.view.items[1].primaryAction.requiresConfirmation,
      true,
    );
    assert.equal(response.view.items[1].title, 'Move to Trash');
  }
});
