import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createViewActionExecutor,
  type ViewActionHandlers,
} from './view-action-executor';

const HANDLER_BY_ACTION_TYPE = {
  nativeAction: 'handleNativeAction',
  openPath: 'handleOpenPath',
  revealPath: 'handleRevealPath',
  quickLook: 'handleQuickLook',
  openWith: 'handleOpenWith',
  openUrl: 'handleOpenUrl',
  copyText: 'handleCopyText',
  pasteText: 'handlePasteText',
  insertCharacter: 'handleInsertCharacter',
  pasteClipboard: 'handlePasteClipboard',
  typeText: 'handleTypeText',
  showWindow: 'handleShowHideCloseWindow',
  hideWindow: 'handleShowHideCloseWindow',
  closeWindow: 'handleShowHideCloseWindow',
  copyImage: 'handleCopyImage',
  removeClipboardHistory: 'handleRemoveClipboardHistory',
  trash: 'handleTrash',
  shellExec: 'handleShellExec',
  shellScript: 'handleShellScript',
  lockScreen: 'handleLockScreen',
  sleepSystem: 'handleSleepSystem',
  restartSystem: 'handleRestartSystem',
  openSystemSettings: 'handleOpenSystemSettings',
  openKeyboardSettings: 'handleOpenKeyboardSettings',
  quitApp: 'handleQuitApp',
  forceQuitApp: 'handleForceQuitApp',
  checkForUpdates: 'handleCheckForUpdates',
  downloadUpdate: 'handleDownloadUpdate',
  installUpdate: 'handleInstallUpdate',
  toggleSetting: 'handleToggleSetting',
  setActionShortcut: 'handleSetActionShortcut',
  setSettingShortcut: 'handleSetSettingShortcut',
  removeShortcut: 'handleRemoveShortcut',
  setActionAlias: 'handleSetActionAlias',
  removeActionAlias: 'handleRemoveActionAlias',
  duplicateCreatedAction: 'handleDuplicateCreatedAction',
  removeCreatedAction: 'handleRemoveCreatedAction',
  submitExtensionPr: 'handleSubmitExtensionPr',
  clearActionOverride: 'handleClearActionOverride',
  recordShortcut: 'handleRecordShortcut',
  runExtensionRegisteredAction: 'handleRunExtensionRegisteredAction',
  renameExtensionPrompt: 'handleRenameExtensionPrompt',
  renameExtension: 'handleRenameExtension',
};

const HANDLER_BY_ACTION_TYPE_WITH_LAUNCH_CONTEXT = {
  createWindow: 'handleCreateOrToggleWindow',
  toggleWindow: 'handleCreateOrToggleWindow',
  runExtensionAction: 'handleRunExtensionAction',
};

function recordingExecutor() {
  const calls: Array<{ handler: string; args: unknown[] }> = [];
  const handlers = new Proxy(
    {},
    {
      get: (_target, handler) =>
        function recordCall(...args: unknown[]) {
          calls.push({ handler: String(handler), args });
          return `${String(handler)} result`;
        },
    },
  ) as ViewActionHandlers;
  return { calls, ...createViewActionExecutor(handlers) };
}

test('routes each action type to its handler with the action only', async () => {
  for (const [type, handler] of Object.entries(HANDLER_BY_ACTION_TYPE)) {
    const { calls, executeViewAction } = recordingExecutor();
    const action = { type };

    assert.equal(
      await executeViewAction(action, { requestId: 'ignored' }),
      `${handler} result`,
    );
    assert.deepEqual(calls, [{ handler, args: [action] }], type);
  }
});

test('passes the launch context to handlers that recurse into view actions', async () => {
  for (const [type, handler] of Object.entries(
    HANDLER_BY_ACTION_TYPE_WITH_LAUNCH_CONTEXT,
  )) {
    const { calls, executeViewAction } = recordingExecutor();
    const action = { type };
    const launchContext = { draftConflictDepth: 1 };

    await executeViewAction(action, launchContext);

    assert.deepEqual(calls, [{ handler, args: [action, launchContext] }], type);
  }
});

test('answers navigation actions without handler dependencies', async () => {
  const { executeViewAction } = recordingExecutor();
  const view = { type: 'list', title: 'Items', items: [] };

  assert.deepEqual(await executeViewAction({ type: 'rootView', view }), {
    view,
    navigation: 'root',
  });
  assert.deepEqual(await executeViewAction({ type: 'pushView', view }), {
    view,
    navigation: 'push',
  });
  assert.deepEqual(await executeViewAction({ type: 'replaceView', view }), {
    view,
    navigation: 'replace',
  });
  assert.deepEqual(await executeViewAction({ type: 'popView' }), {
    navigation: 'pop',
  });
});

test('rejects unsupported and missing action types', async () => {
  const { executeViewAction } = recordingExecutor();

  await assert.rejects(
    executeViewAction({ type: 'unknownType' }),
    /Unsupported action type: unknownType/,
  );
  await assert.rejects(
    executeViewAction(undefined),
    /Unsupported action type: unknown/,
  );
});
