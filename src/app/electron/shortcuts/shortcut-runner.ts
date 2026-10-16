import crypto from 'node:crypto';
import type { BrowserWindow } from 'electron';
import { extensionLoadingView } from '../../palette/model';
import { performanceTraces } from '../diagnostics/debug-performance';
import { runOperation } from '../diagnostics/observability';
import { error as logError } from '../platform/logger';
import { shortcutActionRunsWithoutView } from './shortcut-lifecycle';

export type ShortcutRunnerDeps = {
  paletteWindow: {
    readonly win: BrowserWindow | null;
    showPalette: (options: {
      skipShownEvent?: boolean;
      deferReveal?: boolean;
    }) => void;
  };
  paneInvocations: {
    register: (requestId: string, senderId: number) => void;
    invalidate: (senderId: number) => void;
  };
  paneInvocationWindows: WeakSet<object>;
  currentActionForStoredShortcut: (action: any) => any;
  isClipboardHistoryAction: (action: any) => boolean;
  clipboardHistoryView: () => any;
  recordRecent: (action: any) => void;
  runInBackground: (task: () => Promise<void>, traceId?: string) => void;
  executeAction: (action: any, options?: any) => Promise<any>;
  presentActionErrorFeedback: (error: unknown, action: any) => void;
  normalizeHostViewResult: (result: any) => any;
  actionFailedFeedbackView: () => any;
};

export function createShortcutRunner(deps: ShortcutRunnerDeps) {
  const {
    paletteWindow,
    paneInvocations,
    paneInvocationWindows,
    currentActionForStoredShortcut,
    isClipboardHistoryAction,
    clipboardHistoryView,
    recordRecent,
    runInBackground,
    executeAction,
    presentActionErrorFeedback,
    normalizeHostViewResult,
    actionFailedFeedbackView,
  } = deps;

  function executeShortcutAction(action) {
    return runOperation('command.execute', () => runShortcutAction(action), {
      freshJourney: true,
    });
  }

  async function runShortcutAction(action) {
    const traceId = crypto.randomUUID();
    const requestId = crypto.randomUUID();
    performanceTraces.event(
      'shortcut.invoked',
      {
        actionKind: action?.kind,
        extensionId: action?.extensionId,
      },
      { traceId },
    );
    const currentAction = currentActionForStoredShortcut(action);
    const instantView = isClipboardHistoryAction(currentAction)
      ? clipboardHistoryView()
      : null;
    if (instantView) recordRecent(currentAction);
    if (shortcutActionRunsWithoutView(currentAction)) {
      runInBackground(async () => {
        try {
          await executeAction({ ...currentAction, traceId });
        } catch (error) {
          presentActionErrorFeedback(error, currentAction);
        }
      }, traceId);
      return;
    }
    const wasVisible = Boolean(paletteWindow.win?.isVisible());
    if (!wasVisible)
      paletteWindow.showPalette({
        skipShownEvent: true,
        deferReveal: true,
      });
    const initialResult = normalizeHostViewResult({
      view:
        instantView ||
        extensionLoadingView(currentAction?.title || 'Opening...'),
      revealWhenReady: !wasVisible,
      asSibling: wasVisible,
      isPrimary: !wasVisible,
      traceId,
      requestId,
      phase: 'start',
    });
    const sender = paletteWindow.win?.webContents;
    if (!sender) return;
    paneInvocations.register(requestId, sender.id);
    const shortcutWindow = paletteWindow.win;
    if (shortcutWindow && !paneInvocationWindows.has(shortcutWindow)) {
      paneInvocationWindows.add(shortcutWindow);
      shortcutWindow.on('hide', () => paneInvocations.invalidate(sender.id));
      shortcutWindow.on('closed', () => paneInvocations.invalidate(sender.id));
    }
    paletteWindow.win?.webContents.send('action:view-open', initialResult);
    if (instantView) return;
    try {
      const result = normalizeHostViewResult(
        await executeAction(
          { ...currentAction, traceId },
          { keepPaletteOpen: true },
        ),
      );
      if (!sender.isDestroyed())
        sender.send('action:view-open', {
          view: result?.view,
          traceId,
          requestId,
          phase: 'resolve',
        });
    } catch (error) {
      logError('shortcut.view.failed', error);
      if (!sender.isDestroyed())
        sender.send('action:view-open', {
          view: actionFailedFeedbackView(),
          traceId,
          requestId,
          phase: 'resolve',
        });
    }
  }

  return { executeShortcutAction };
}
