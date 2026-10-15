import path from 'node:path';
import { feedbackView } from '../../palette/feedback';
import type { CommandAction } from '../../palette/model';
import {
  currentOperation,
  recordOperationFailure,
} from '../diagnostics/observability';
import type { createExtensionWindowManager } from '../extension-host/extension-window-manager';

export type ActionFeedbackDeps = {
  normalizeView: (view: any, entry: any) => any;
  setDiagnosticExtensionTarget: (extension: any) => void;
  aiChatView: (chat: any, options?: any) => Promise<any>;
  getOrCreateExtensionChat: (extensionFile: string, title?: string) => any;
  extensionActionHandlers: Map<string, any>;
  extensionWindowManager: Pick<
    ReturnType<typeof createExtensionWindowManager>,
    'showIndicator' | 'getStateForSender'
  >;
  paletteWindow: { readonly win: { readonly webContents: unknown } | null };
};

export function createActionFeedback(deps: ActionFeedbackDeps) {
  const {
    normalizeView,
    setDiagnosticExtensionTarget,
    aiChatView,
    getOrCreateExtensionChat,
    extensionActionHandlers,
    extensionWindowManager,
    paletteWindow,
  } = deps;

  function extensionErrorMessage(error) {
    if (!error) return 'Unknown error';
    if (error.stack) return error.stack;
    if (error.message) return error.message;
    return String(error);
  }

  function extensionErrorView(entry, error) {
    if (entry?.extension) setDiagnosticExtensionTarget(entry.extension);
    recordOperationFailure(error, 'invoke');
    const message = extensionErrorMessage(error);
    const title =
      (entry?.command?.title || entry?.extension?.title || 'Extension') +
      ' failed';
    const fixWithAi = extensionErrorAiAction(entry, message);
    const actions: CommandAction[] = [{ type: 'popView', title: 'Back' }];
    if (fixWithAi) actions.unshift(fixWithAi as CommandAction);
    return normalizeView(
      feedbackView({
        id: 'extension-action-error',
        title,
        message: 'The action could not be completed. Try again.',
        tone: 'error',
        actions,
      }),
      entry,
    );
  }

  function actionFailedFeedbackView() {
    return feedbackView({
      id: 'action-failed',
      title: 'Action failed',
      message: 'The action could not be completed. Try again.',
      tone: 'error',
    });
  }

  function extensionErrorAiAction(entry, message) {
    if (!entry?.extension) return null;
    const extensionFile = entry.extension.__filePath
      ? path.basename(entry.extension.__filePath)
      : null;
    if (!extensionFile) return null;
    const prompt = `This generated action failed. Please fix the extension.\n\nAction: ${entry.command?.title || entry.command?.id || 'unknown'}\nExtension: ${entry.extension.title || entry.extension.id}\nFile: ${extensionFile}\n\nError:\n\`\`\`\n${message}\n\`\`\``;
    return {
      type: 'runExtensionAction',
      title: 'Fix with AI',
      __handler: async () =>
        await aiChatView(
          getOrCreateExtensionChat(
            extensionFile,
            entry.extension.title || entry.command?.title,
          ),
          { initialPrompt: prompt },
        ),
    };
  }

  function actionFeedbackOwner(action: any) {
    if (action?.extensionId) return String(action.extensionId);
    if (action?.ownerExtensionId) return String(action.ownerExtensionId);
    if (action?.type === 'runExtensionAction')
      return (
        extensionActionHandlers.get(action.handlerId)?.entry?.extension?.id ||
        'nevermind.host'
      );
    return 'nevermind.host';
  }

  function presentActionResultFeedback(result: any, action: any) {
    if (!result?.toast) return result;
    const message = String(result.toast.message || '');
    const tone = String(result.toast.tone || 'default');
    if (tone === 'error' && currentOperation())
      recordOperationFailure(undefined, 'invoke', 'local');
    if (message)
      extensionWindowManager.showIndicator(
        {
          id: 'feedback',
          title: String(action?.title || 'Nevermind'),
          subtitle: message,
          ...(tone === 'error' || tone === 'success' ? { status: tone } : {}),
          durationMs: tone === 'error' ? 4000 : 2200,
        },
        actionFeedbackOwner(action),
      );
    const { toast: _toast, ...remaining } = result;
    return Object.keys(remaining).length ? remaining : undefined;
  }

  function presentActionErrorFeedback(error: unknown, action: any) {
    recordOperationFailure(error, 'invoke');
    return presentActionResultFeedback(
      {
        toast: {
          message:
            error instanceof Error
              ? error.message
              : 'The action could not be completed',
          tone: 'error',
        },
      },
      action,
    );
  }

  function showRendererIndicator(sender: unknown, input: any) {
    const paletteSender = paletteWindow.win?.webContents === sender;
    const extensionWindowState = paletteSender
      ? null
      : extensionWindowManager.getStateForSender(sender);
    if (!(paletteSender || extensionWindowState))
      throw new Error('Indicator sender is not an application window');
    const status = [
      'recording',
      'transcribing',
      'loading',
      'success',
      'error',
    ].includes(String(input?.status || ''))
      ? String(input.status)
      : undefined;
    extensionWindowManager.showIndicator(
      {
        id: 'feedback',
        title: String(input?.title || 'Nevermind').slice(0, 120),
        subtitle: String(input?.subtitle || '').slice(0, 500),
        ...(status ? { status } : {}),
        durationMs: Math.min(
          10_000,
          Math.max(1000, Number(input?.durationMs) || 2200),
        ),
      },
      paletteSender
        ? 'nevermind.host'
        : `nevermind.window.${String((extensionWindowState as any).id || 'unknown')}`,
    );
  }

  return {
    extensionErrorView,
    actionFailedFeedbackView,
    presentActionResultFeedback,
    presentActionErrorFeedback,
    showRendererIndicator,
  };
}
