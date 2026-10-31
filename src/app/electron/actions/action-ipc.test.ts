import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: { app: { isPackaged: false, getPath: () => '/tmp' } },
});

const { createActionIpc } = await import('./action-ipc');

const commandAction = { kind: 'extension-command', id: 'summarize' };
const errorView = { id: 'extension-action-error' };

function actionIpc(executeAction: (action: unknown) => Promise<unknown>) {
  return createActionIpc({
    resolveRootActionForIpc: (action) => action,
    resolveViewActionForIpc: (action) => action,
    executeAction,
    extensionEntryForAction: () => ({ extension: {} }),
    extensionErrorView: () => errorView,
    actionFailedFeedbackView: () => ({ id: 'action-failed' }),
    normalizeHostViewResult: (result) => result,
    presentActionResultFeedback: (result) => result,
    spawnPendingViewLoaders: () => {},
  } as unknown as Parameters<typeof createActionIpc>[0]);
}

test('a failing command still renders the error view and reports the underlying error to the host caller', async () => {
  const failure = new TypeError('text.trim is not a function');
  const { executeActionForIpc } = actionIpc(() => Promise.reject(failure));
  const failures: unknown[] = [];

  const result = await executeActionForIpc(commandAction, {
    onFailure: (error) => failures.push(error),
  });

  assert.deepEqual(result, { view: errorView });
  assert.deepEqual(failures, [failure]);
});

test('a succeeding command does not report a failure', async () => {
  const { executeActionForIpc } = actionIpc(() =>
    Promise.resolve({ view: { id: 'summary' } }),
  );
  const failures: unknown[] = [];

  const result = await executeActionForIpc(commandAction, {
    onFailure: (error) => failures.push(error),
  });

  assert.deepEqual(result, { view: { id: 'summary' } });
  assert.deepEqual(failures, []);
});
