import type { createClipboardActionHandlers } from './view-action-handlers/clipboard-actions';
import type { createCustomizationActionHandlers } from './view-action-handlers/customization-actions';
import type { createExtensionActionHandlers } from './view-action-handlers/extension-actions';
import type { createFileActionHandlers } from './view-action-handlers/file-actions';
import type { createNativeActionHandlers } from './view-action-handlers/native-action';
import {
  handlePopView,
  handlePromptAction,
  handlePushView,
  handleReplaceView,
  handleRootView,
} from './view-action-handlers/navigation-actions';
import type { createSettingActionHandlers } from './view-action-handlers/settings-actions';
import type { createShellActionHandlers } from './view-action-handlers/shell-actions';
import type { createSystemActionHandlers } from './view-action-handlers/system-actions';
import type { createWindowActionHandlers } from './view-action-handlers/window-actions';

export type ViewActionHandlers = ReturnType<typeof createNativeActionHandlers> &
  ReturnType<typeof createFileActionHandlers> &
  ReturnType<typeof createClipboardActionHandlers> &
  ReturnType<typeof createWindowActionHandlers> &
  ReturnType<typeof createShellActionHandlers> &
  ReturnType<typeof createSystemActionHandlers> &
  ReturnType<typeof createSettingActionHandlers> &
  ReturnType<typeof createCustomizationActionHandlers> &
  ReturnType<typeof createExtensionActionHandlers>;

export function createViewActionExecutor(handlers: ViewActionHandlers) {
  const {
    handleNativeAction,
    handleOpenPath,
    handleRevealPath,
    handleQuickLook,
    handleOpenWith,
    handleOpenUrl,
    handleCopyText,
    handlePasteText,
    handleInsertCharacter,
    handlePasteClipboard,
    handleTypeText,
    handleCreateOrToggleWindow,
    handleShowHideCloseWindow,
    handleCopyImage,
    handleRemoveClipboardHistory,
    handleTrash,
    handleShellExec,
    handleShellScript,
    handleLockScreen,
    handleSleepSystem,
    handleRestartSystem,
    handleOpenSystemSettings,
    handleOpenKeyboardSettings,
    handleQuitApp,
    handleForceQuitApp,
    handleCheckForUpdates,
    handleDownloadUpdate,
    handleInstallUpdate,
    handleToggleSetting,
    handleSetActionShortcut,
    handleSetSettingShortcut,
    handleRemoveShortcut,
    handleSetActionAlias,
    handleRemoveActionAlias,
    handleDuplicateCreatedAction,
    handleRemoveCreatedAction,
    handleSubmitExtensionPr,
    handleClearActionOverride,
    handleRecordShortcut,
    handleRunExtensionRegisteredAction,
    handleRenameExtensionPrompt,
    handleRenameExtension,
    handleRunExtensionAction,
  } = handlers;

  async function executeViewAction(action, launchContext?: any) {
    switch (action?.type) {
      case 'nativeAction':
        return handleNativeAction(action);
      case 'openPath':
        return handleOpenPath(action);
      case 'revealPath':
        return handleRevealPath(action);
      case 'quickLook':
        return handleQuickLook(action);
      case 'openWith':
        return handleOpenWith(action);
      case 'openUrl':
        return handleOpenUrl(action);
      case 'copyText':
        return handleCopyText(action);
      case 'pasteText':
        return handlePasteText(action);
      case 'insertCharacter':
        return handleInsertCharacter(action);
      case 'pasteClipboard':
        return handlePasteClipboard(action);
      case 'typeText':
        return handleTypeText(action);
      case 'createWindow':
      case 'toggleWindow':
        return handleCreateOrToggleWindow(action, launchContext);
      case 'showWindow':
      case 'hideWindow':
      case 'closeWindow':
        return handleShowHideCloseWindow(action);
      case 'copyImage':
        return handleCopyImage(action);
      case 'removeClipboardHistory':
        return handleRemoveClipboardHistory(action);
      case 'trash':
        return handleTrash(action);
      case 'rootView':
        return handleRootView(action);
      case 'pushView':
        return handlePushView(action);
      case 'replaceView':
        return handleReplaceView(action);
      case 'popView':
        return handlePopView(action);
      case 'shellExec':
        return handleShellExec(action);
      case 'shellScript':
        return handleShellScript(action);
      case 'lockScreen':
        return handleLockScreen(action);
      case 'sleepSystem':
        return handleSleepSystem(action);
      case 'restartSystem':
        return handleRestartSystem(action);
      case 'openSystemSettings':
        return handleOpenSystemSettings(action);
      case 'openKeyboardSettings':
        return handleOpenKeyboardSettings(action);
      case 'quitApp':
        return handleQuitApp(action);
      case 'forceQuitApp':
        return handleForceQuitApp(action);
      case 'checkForUpdates':
        return handleCheckForUpdates(action);
      case 'downloadUpdate':
        return handleDownloadUpdate(action);
      case 'installUpdate':
        return handleInstallUpdate(action);
      case 'toggleSetting':
        return handleToggleSetting(action);
      case 'setActionShortcut':
        return handleSetActionShortcut(action);
      case 'setSettingShortcut':
        return handleSetSettingShortcut(action);
      case 'removeShortcut':
        return handleRemoveShortcut(action);
      case 'setActionAlias':
        return handleSetActionAlias(action);
      case 'removeActionAlias':
        return handleRemoveActionAlias(action);
      case 'duplicateCreatedAction':
        return handleDuplicateCreatedAction(action);
      case 'removeCreatedAction':
        return handleRemoveCreatedAction(action);
      case 'submitExtensionPr':
        return handleSubmitExtensionPr(action);
      case 'clearActionOverride':
        return handleClearActionOverride(action);
      case 'recordShortcut':
        return handleRecordShortcut(action);
      case 'runExtensionRegisteredAction':
        return handleRunExtensionRegisteredAction(action);
      case 'renameExtensionPrompt':
        return handleRenameExtensionPrompt(action);
      case 'renameExtension':
        return handleRenameExtension(action);
      case 'promptAction':
        return handlePromptAction(action);
      case 'runExtensionAction':
        return handleRunExtensionAction(action, launchContext);
      default:
        throw new Error(
          `Unsupported action type: ${String(action?.type || 'unknown')}`,
        );
    }
  }

  return { executeViewAction };
}
