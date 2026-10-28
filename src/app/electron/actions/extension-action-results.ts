import {
  measureDebugPerformance,
  performanceTraces,
  summarizeDebugValue,
} from '../diagnostics/debug-performance';
import { feedbackView } from '../../palette/feedback';
import { recordOperationFailure } from '../diagnostics/observability';
import type { createExtensionDraftStore } from '../extension-host/extension-draft-store';
import { error as logError, warn as logWarn } from '../platform/logger';
import { isLoaderHandle } from '../state/data-loader';
import { isView } from './view-types';

function isAction(value) {
  return Boolean(value?.type && !isView(value));
}

export type ExtensionActionResultsDeps = {
  executeViewAction: (action: any, launchContext?: any) => Promise<any>;
  createExtensionContext: (
    extension: any,
    command?: any,
    launchContext?: any,
    options?: any,
  ) => any;
  setDiagnosticExtensionTarget: (extension: any) => void;
  extensionActionHandlers: Map<string, any>;
  extensionErrorView: (entry: any, error: unknown) => any;
  extensionDraftStore: Pick<
    ReturnType<typeof createExtensionDraftStore>,
    'resolveByKey'
  >;
  schedulePersistExtensionDrafts: () => void;
  normalizeViewAction: (action: any, entry: any) => any;
  normalizeExtensionView: (result: any, entry: any) => any;
  normalizeViewPatch: (patch: any, entry: any) => any;
  normalizeViewItems: (items: any, entry: any) => any;
};

export function createExtensionActionResults(deps: ExtensionActionResultsDeps) {
  const {
    executeViewAction,
    createExtensionContext,
    setDiagnosticExtensionTarget,
    extensionActionHandlers,
    extensionErrorView,
    extensionDraftStore,
    schedulePersistExtensionDrafts,
    normalizeViewAction,
    normalizeExtensionView,
    normalizeViewPatch,
    normalizeViewItems,
  } = deps;

  async function executeExtensionRootItem(action) {
    return performanceTraces.run(
      'extension.rootItem.execute',
      {
        extensionId: action?.extensionId,
        actionKind: action?.kind,
        actionToken: action?.executionId,
      },
      () =>
        measureDebugPerformance(
          'extension.root-item.execute',
          { action: summarizeDebugValue(action), alwaysLog: true },
          async () => {
            if (!action.rootAction)
              return {
                view: {
                  type: 'preview',
                  title: action.title || 'Extension item',
                  content: action.subtitle || '',
                },
              };
            if (action.rootAction.type !== 'runExtensionAction')
              return executeViewAction(action.rootAction);
            const record = extensionActionHandlers.get(
              action.rootAction.handlerId,
            );
            if (!record)
              return {
                view: {
                  type: 'preview',
                  title: 'Action unavailable',
                  content: 'This extension item is no longer available.',
                },
              };
            if (!(record.entry && record.entry.extension)) {
              logWarn('extension.rootItem.missingEntry', {
                handlerId: action.rootAction.handlerId,
                actionTitle: action.title,
              });
              return {
                view: {
                  type: 'preview',
                  title: 'Action unavailable',
                  content:
                    'This action is not available in the current context.',
                },
              };
            }
            try {
              setDiagnosticExtensionTarget(record.entry.extension);
              const result = await measureDebugPerformance(
                'extension.root-item.handler',
                {
                  extensionId: record.entry.extension.id,
                  commandId: record.entry.command?.id,
                  alwaysLog: true,
                },
                () =>
                  performanceTraces.run(
                    'extension.rootItem.handler',
                    {
                      extensionId: record.entry.extension.id,
                      commandId: record.entry.command?.id,
                    },
                    () =>
                      record.handler(
                        createExtensionContext(
                          record.entry.extension,
                          record.entry.command || null,
                        ),
                        action,
                      ),
                    action.traceId ? { traceId: action.traceId } : undefined,
                  ),
              );
              return executeViewActionResult(result, record.entry);
            } catch (error) {
              recordOperationFailure(error, 'invoke');
              logError('extension.rootItem.failed', error, {
                source: 'host',
                scope: 'extension',
                extensionId: record.entry.extension?.id,
              });
              if (action.background || action.dismissAfterRun === 'auto')
                return {
                  toast: {
                    message:
                      error instanceof Error
                        ? error.message
                        : 'The action could not be completed',
                    tone: 'error',
                  },
                };
              return { view: extensionErrorView(record.entry, error) };
            }
          },
        ),
      action?.traceId ? { traceId: action.traceId } : undefined,
    ) as Promise<any>;
  }

  async function executeViewActionResult(result, entry, launchContext?: any) {
    if (!result) return result;
    if (result?.type === 'draftResolution') {
      const ownerExtensionId = entry?.extension?.id;
      if (!ownerExtensionId)
        return {
          toast: { message: 'Draft resolution unavailable', tone: 'error' },
        };
      try {
        extensionDraftStore.resolveByKey(
          ownerExtensionId,
          String(result.key || ''),
          result,
        );
        schedulePersistExtensionDrafts();
      } catch (error) {
        logWarn('extension.draft.resolve.failed', {
          extensionId: ownerExtensionId,
          error: error instanceof Error ? error.message : String(error),
        });
        return {
          toast: { message: 'Draft is no longer in conflict', tone: 'error' },
        };
      }
      const followUp = {
        view: result.view,
        navigation: result.navigation,
        toast: result.toast,
        patch: result.patch,
      };
      if (!followUp.view && !followUp.patch)
        return { toast: followUp.toast || { message: 'Draft updated' } };
      result = followUp;
    }
    const returnedAction = isAction(result)
      ? result
      : isAction(result.action)
        ? result.action
        : null;
    if (returnedAction?.requiresConfirmation) {
      const view = feedbackView({
        id: 'extension-action-confirmation',
        title: returnedAction.title || 'Confirm action',
        message: returnedAction.confirmMessage || 'Confirm this action.',
        tone: 'info',
        actions: [
          {
            ...returnedAction,
            title: returnedAction.confirmLabel || returnedAction.title,
            requiresConfirmation: false,
          },
          { type: 'popView', title: returnedAction.cancelLabel || 'Cancel' },
        ],
      });
      return { view: normalizeExtensionView(view, entry), navigation: 'push' };
    }
    if (isAction(result))
      return executeViewAction(
        normalizeViewAction(result, entry),
        launchContext,
      );
    if (isAction(result.action))
      return executeViewAction(
        normalizeViewAction(result.action, entry),
        launchContext,
      );
    const view = normalizeExtensionView(result, entry);
    const draftConflictDepth = Number(launchContext?.draftConflictDepth) || 0;
    if (
      view?.draft?.conflict &&
      view.draft.onConflict &&
      draftConflictDepth < 3
    ) {
      return executeViewAction(
        { ...view.draft.onConflict, draftConflict: view.draft.conflict },
        {
          ...(launchContext || {}),
          draftConflictDepth: draftConflictDepth + 1,
        },
      );
    }
    return view
      ? {
          view,
          navigation: result?.navigation || 'push',
          toast: result?.toast,
          patch: normalizeViewPatch(result?.patch, entry),
        }
      : { ...result, patch: normalizeViewPatch(result?.patch, entry) };
  }

  async function executeHostRefreshAction(
    record,
    launchContext?: any,
    traceId?: string,
  ) {
    if (record.action?.type === 'runExtensionAction') {
      const handlerRecord = extensionActionHandlers.get(
        record.action.handlerId,
      );
      if (!handlerRecord) return { skipped: true };
      const result = await performanceTraces.run(
        'extension.refresh.handler',
        {
          extensionId: handlerRecord.entry.extension.id,
          commandId: handlerRecord.entry.command?.id,
        },
        () =>
          handlerRecord.handler(
            createExtensionContext(
              handlerRecord.entry.extension,
              handlerRecord.entry.command,
              launchContext,
            ),
            record.action,
          ),
        traceId ? { traceId } : undefined,
      );
      return executeViewActionResult(
        result,
        handlerRecord.entry,
        launchContext,
      );
    }
    if (record.action)
      return executeViewAction(
        traceId ? { ...record.action, traceId } : record.action,
        launchContext,
      );
    if (
      !record.entry?.command ||
      typeof record.entry.command.run !== 'function'
    )
      return { skipped: true };
    const result = await performanceTraces.run(
      'extension.refresh.command',
      {
        extensionId: record.entry.extension.id,
        commandId: record.entry.command.id,
      },
      () =>
        record.entry.command.run(
          createExtensionContext(
            record.entry.extension,
            record.entry.command,
            launchContext,
          ),
        ),
      traceId ? { traceId } : undefined,
    );
    const view = result?.type
      ? result
      : result?.view?.type
        ? result.view
        : null;
    if (
      view?.items &&
      !isLoaderHandle(view.items) &&
      Array.isArray(view.items) &&
      view.items.length > 0
    )
      return {
        patch: {
          mode: record.mode || 'replace',
          items: normalizeViewItems(view.items, record.entry),
        },
      };
    return executeViewActionResult(result, record.entry, launchContext);
  }

  return {
    executeExtensionRootItem,
    executeViewActionResult,
    executeHostRefreshAction,
  };
}
