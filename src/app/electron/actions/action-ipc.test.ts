import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: { app: { isPackaged: false, getPath: () => '/tmp' } },
});

const { createActionIpc } = await import('./action-ipc');
const { createExtensionActionResults } = await import(
  './extension-action-results'
);
const { createRootActionExecutor } = await import('./root-action-executor');
const { createPreviewRunObserver } = await import(
  '../ai/ai-preview-run-observer'
);

const commandAction = { kind: 'extension-command', id: 'summarize' };
const errorView = { id: 'extension-action-error' };

function actionIpc(
  executeAction: (action: unknown) => Promise<unknown>,
  observeActionRun?: (action: unknown) => unknown,
) {
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
    observeActionRun,
  } as unknown as Parameters<typeof createActionIpc>[0]);
}

function recordingObserver() {
  const calls: string[] = [];
  const observed: unknown[] = [];
  return {
    calls,
    observed,
    observeActionRun: (action: unknown) => {
      observed.push(action);
      return {
        succeeded: () => calls.push('succeeded'),
        failed: (error: unknown) =>
          calls.push(`failed:${(error as Error).message}`),
      };
    },
  };
}

test('a failing command still renders the error view and reports the error to the observer once', async () => {
  const recorder = recordingObserver();
  const { executeActionForIpc } = actionIpc(
    () => Promise.reject(new TypeError('text.trim is not a function')),
    recorder.observeActionRun,
  );

  const result = await executeActionForIpc(commandAction);

  assert.deepEqual(result, { view: errorView });
  assert.deepEqual(recorder.calls, ['failed:text.trim is not a function']);
  assert.deepEqual(recorder.observed, [commandAction]);
});

test('a real extension root handler failure reaches the observer and retains its error view', async () => {
  const failure = new TypeError('text.trim is not a function');
  const entry = { extension: { id: 'summary' }, command: { id: 'summarize' } };
  const extension = createExtensionActionResults({
    executeViewAction: async () => undefined,
    createExtensionContext: () => ({}),
    setDiagnosticExtensionTarget: () => {},
    extensionActionHandlers: new Map([
      [
        'handler',
        {
          entry,
          handler: () => {
            throw failure;
          },
        },
      ],
    ]),
    extensionErrorView: () => errorView,
    extensionDraftStore: { resolveByKey: () => {} } as any,
    schedulePersistExtensionDrafts: () => {},
    normalizeViewAction: (action) => action,
    normalizeExtensionView: (view) => view,
    normalizeViewPatch: (patch) => patch,
    normalizeViewItems: (items) => items,
  });
  const action = {
    kind: 'extension-action',
    id: 'summarize',
    extensionFile: 'summary.ts',
    rootAction: { type: 'runExtensionAction', handlerId: 'handler' },
  };
  const executor = createRootActionExecutor({
    paletteWindow: { hidePalette: () => {} },
    recordRecent: () => {},
    currentActionForStoredShortcut: () => action,
    openSystemKeyboardSettings: () => {},
    executeExtensionRootItem: extension.executeExtensionRootItem,
    presentActionResultFeedback: (result) => result,
  });
  const observer = createPreviewRunObserver();
  observer.expectRun('summary.ts', 'summarize');
  const { executeActionForIpc } = actionIpc(
    executor.executeAction,
    observer.observeRun,
  );
  const originalAction = {
    kind: 'extension-command',
    id: 'summarize',
    extensionFile: 'summary.ts',
  };

  assert.deepEqual(await executeActionForIpc(originalAction), {
    view: errorView,
  });
  assert.match(
    ((await observer.outcomeFor('summary.ts', 0)) as { message: string })
      .message,
    /text\.trim is not a function/,
  );
  observer.expectRun('summary.ts', 'summarize');
  action.rootAction.handlerId = 'missing';
  assert.equal(
    (await executeActionForIpc(originalAction)).view.title,
    'Action unavailable',
  );
  const unavailable = await observer.outcomeFor('summary.ts', 0);
  assert.equal(unavailable.status, 'failed');
  if (unavailable.status === 'failed')
    assert.match(unavailable.message, /Action unavailable/);
});

test('a succeeding command is reported to the observer once and executes once', async () => {
  const recorder = recordingObserver();
  const execute = mock.fn(() => Promise.resolve({ view: { id: 'summary' } }));
  const { executeActionForIpc } = actionIpc(execute, recorder.observeActionRun);

  const result = await executeActionForIpc(commandAction);

  assert.deepEqual(result, { view: { id: 'summary' } });
  assert.deepEqual(recorder.calls, ['succeeded']);
  assert.equal(execute.mock.callCount(), 1);
});

test('actions run unchanged when nothing observes them', async () => {
  const { executeActionForIpc } = actionIpc(() =>
    Promise.resolve({ view: { id: 'summary' } }),
  );

  assert.deepEqual(await executeActionForIpc(commandAction), {
    view: { id: 'summary' },
  });
});

test('an untrusted action is rejected before it is observed', async () => {
  const recorder = recordingObserver();
  const { executeActionForIpc } = createActionIpc({
    resolveRootActionForIpc: () => {
      throw new Error('Untrusted action');
    },
    executeAction: () => Promise.reject(new Error('must not run')),
    actionFailedFeedbackView: () => ({ id: 'action-failed' }),
    observeActionRun: recorder.observeActionRun,
  } as unknown as Parameters<typeof createActionIpc>[0]);

  const result = await executeActionForIpc(commandAction);

  assert.deepEqual(result, { view: { id: 'action-failed' } });
  assert.deepEqual(recorder.observed, []);
});
