import {
  measureDebugPerformance,
  performanceTraces,
  summarizeDebugValue,
} from '../diagnostics/debug-performance';
import { actionFeatureLabel } from '../diagnostics/feature-usage';
import {
  operationLabel,
  operationStage,
  recordOperationFailure,
} from '../diagnostics/observability';
import { isInternalExtension } from '../extension-host/internal-extension';
import { runInWindowInvocationScope } from '../extension-host/window-invocation-scope';
import { error as logError, warn as logWarn } from '../platform/logger';
import type { ViewRefreshRecord } from './action-execution-records';

function refreshBackoffDelay(failureCount: number) {
  return Math.min(
    30_000,
    1000 * Math.max(1, 2 ** Math.max(0, failureCount - 1)),
  );
}

export type ActionIpcDeps = {
  resolveRootActionForIpc: (action: any) => any;
  resolveViewActionForIpc: (action: any) => any;
  executeAction: (action: any, options?: any) => Promise<any>;
  executeViewAction: (action: any, launchContext?: any) => Promise<any>;
  executeHostRefreshAction: (
    record: any,
    launchContext?: any,
    traceId?: string,
  ) => Promise<any>;
  presentActionResultFeedback: (result: any, action: any) => any;
  presentActionErrorFeedback: (error: unknown, action: any) => any;
  extensionEntryForAction: (action: any) => any;
  extensionActionEntryForAction: (action: any) => any;
  extensionErrorView: (entry: any, error: unknown) => any;
  actionFailedFeedbackView: () => any;
  normalizeHostViewResult: (result: any) => any;
  spawnPendingViewLoaders: (result: any, traceId?: string) => void;
  pruneViewRefreshRecords: () => void;
  viewRefreshRecords: Map<string, ViewRefreshRecord>;
  extensionActionHandlers: Map<string, any>;
  paneInvocations: {
    owns: (context: unknown, senderId?: number) => boolean;
  };
};

export function createActionIpc(deps: ActionIpcDeps) {
  const {
    resolveRootActionForIpc,
    resolveViewActionForIpc,
    executeAction,
    executeViewAction,
    executeHostRefreshAction,
    presentActionResultFeedback,
    presentActionErrorFeedback,
    extensionEntryForAction,
    extensionActionEntryForAction,
    extensionErrorView,
    actionFailedFeedbackView,
    normalizeHostViewResult,
    spawnPendingViewLoaders,
    pruneViewRefreshRecords,
    viewRefreshRecords,
    extensionActionHandlers,
    paneInvocations,
  } = deps;

  async function executeActionForIpc(
    action,
    options: { onFailure?: (error: unknown) => void } = {},
  ) {
    return performanceTraces.run(
      'action.root',
      {
        actionKind: action?.kind,
        actionToken: action?.executionId,
        extensionId: action?.extensionId,
      },
      () =>
        measureDebugPerformance(
          'ipc.actions.execute',
          { action: summarizeDebugValue(action), alwaysLog: true },
          async () => {
            let trustedAction: any = null;
            try {
              trustedAction = resolveRootActionForIpc(action);
              operationLabel(
                actionFeatureLabel(
                  trustedAction,
                  isInternalExtension(
                    extensionEntryForAction(trustedAction)?.extension,
                  ),
                ),
              );
              operationStage('invoke');
              const result = presentActionResultFeedback(
                normalizeHostViewResult(
                  await measureDebugPerformance(
                    'action.execute',
                    {
                      action: summarizeDebugValue(trustedAction),
                      alwaysLog: true,
                    },
                    () =>
                      runInWindowInvocationScope({ source: 'palette' }, () =>
                        executeAction(trustedAction),
                      ),
                  ),
                ),
                trustedAction,
              );
              structuredClone(result);
              spawnPendingViewLoaders(result, trustedAction?.traceId);
              return result;
            } catch (error) {
              recordOperationFailure(error);
              options.onFailure?.(error);
              if (!(error instanceof Error && error.name === 'AbortError'))
                logError(
                  'action.execute.failed',
                  {
                    error,
                    actionKind: trustedAction?.kind || action?.kind,
                    actionId: trustedAction?.id || action?.id,
                  },
                  { source: 'host', scope: 'action' },
                );
              if (
                trustedAction?.background ||
                trustedAction?.dismissAfterRun === 'auto'
              )
                return presentActionErrorFeedback(error, trustedAction);
              if (trustedAction?.kind === 'extension-command') {
                const entry = extensionEntryForAction(trustedAction);
                if (entry) return { view: extensionErrorView(entry, error) };
              }
              if (action?.kind === 'extension-action') {
                const entry = extensionActionEntryForAction(action);
                if (entry) return { view: extensionErrorView(entry, error) };
              }
              return { view: actionFailedFeedbackView() };
            }
          },
        ),
      action?.traceId ? { traceId: action.traceId } : undefined,
    ) as Promise<any>;
  }

  async function refreshViewForIpc(input: any = {}) {
    return performanceTraces.run(
      'view.refresh',
      { viewId: input?.viewId },
      () =>
        measureDebugPerformance(
          'ipc.view.refresh',
          { input: summarizeDebugValue(input), alwaysLog: true },
          async () => {
            pruneViewRefreshRecords();
            const refreshId =
              typeof input === 'string' ? input : String(input?.id || '');
            const record = refreshId ? viewRefreshRecords.get(refreshId) : null;
            if (!record) return { skipped: true };
            if (
              input?.viewId &&
              record.viewId &&
              input.viewId !== record.viewId
            )
              return { skipped: true };
            const now = Date.now();
            if (record.backoffUntil && record.backoffUntil > now)
              return { skipped: true };
            if (record.running) return { skipped: true };
            record.running = measureDebugPerformance(
              'view.refresh.host-action',
              {
                refreshId,
                viewId: record.viewId,
                extensionId: record.entry?.extension?.id,
                commandId: record.entry?.command?.id,
                alwaysLog: true,
              },
              async () => {
                try {
                  const result = normalizeHostViewResult(
                    await executeHostRefreshAction(
                      record,
                      {
                        refresh: true,
                        reason: 'refresh',
                        startedAt: Date.now(),
                      },
                      input?.traceId,
                    ),
                  );
                  structuredClone(result);
                  spawnPendingViewLoaders(result, input?.traceId);
                  record.failureCount = 0;
                  record.backoffUntil = undefined;
                  return result;
                } catch (error) {
                  record.failureCount += 1;
                  record.backoffUntil =
                    Date.now() + refreshBackoffDelay(record.failureCount);
                  logWarn(
                    'extension.viewRefresh.failed',
                    {
                      viewId: record.viewId,
                      error:
                        error instanceof Error ? error.message : String(error),
                    },
                    {
                      source: 'host',
                      scope: 'extension',
                      extensionId: record.entry?.extension?.id,
                      commandId: record.entry?.command?.id,
                    },
                  );
                  return { skipped: true };
                } finally {
                  record.running = null;
                }
              },
            );
            return record.running;
          },
        ),
      input?.traceId ? { traceId: input.traceId } : undefined,
    ) as Promise<any>;
  }

  async function executeViewActionForIpc(
    action,
    context?: unknown,
    senderId?: number,
  ) {
    return performanceTraces.run(
      'action.view',
      {
        actionType: action?.type,
        actionToken: action?.executionId,
        extensionId: action?.extensionId,
      },
      () =>
        measureDebugPerformance(
          'ipc.view-action.execute',
          { action: summarizeDebugValue(action), alwaysLog: true },
          async () => {
            let trustedAction: any = null;
            try {
              trustedAction = resolveViewActionForIpc(action);
              if (paneInvocations.owns(context, senderId))
                trustedAction = { ...trustedAction, keepPaletteOpen: true };
              operationStage('invoke');
              const result = presentActionResultFeedback(
                normalizeHostViewResult(
                  await measureDebugPerformance(
                    'view-action.execute',
                    {
                      action: summarizeDebugValue(trustedAction),
                      alwaysLog: true,
                    },
                    () =>
                      runInWindowInvocationScope({ source: 'palette' }, () =>
                        executeViewAction(trustedAction),
                      ),
                  ),
                ),
                trustedAction,
              );
              structuredClone(result);
              spawnPendingViewLoaders(
                result,
                trustedAction?.traceId || action?.traceId,
              );
              return result;
            } catch (error) {
              recordOperationFailure(error);
              const record =
                trustedAction?.type === 'runExtensionAction'
                  ? extensionActionHandlers.get(trustedAction.handlerId)
                  : null;
              if (!(error instanceof Error && error.name === 'AbortError'))
                logError(
                  'extension.viewAction.failed',
                  {
                    error,
                    actionType: trustedAction?.type || action?.type,
                    actionTitle: trustedAction?.title || action?.title,
                  },
                  {
                    source: 'host',
                    scope: 'extension',
                    extensionId: record?.entry?.extension?.id,
                  },
                );
              if (
                trustedAction?.dismissAfterRun === 'auto' &&
                !trustedAction?.keepPaletteOpen
              )
                return presentActionErrorFeedback(error, trustedAction);
              if (record)
                return {
                  view: extensionErrorView(record.entry, error),
                  navigation: 'push',
                };
              return {
                view: actionFailedFeedbackView(),
                navigation: 'push',
              };
            }
          },
        ),
      action?.traceId ? { traceId: action.traceId } : undefined,
    ) as Promise<any>;
  }

  return { executeActionForIpc, refreshViewForIpc, executeViewActionForIpc };
}
