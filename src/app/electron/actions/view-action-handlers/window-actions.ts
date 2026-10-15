import { performanceTraces } from '../../diagnostics/debug-performance';

export type WindowActionHandlerDeps = {
  extensionWindowManager: { executeWindowAction: (action: any) => any };
  executeViewAction: (action: any, launchContext?: any) => Promise<any>;
};

export function createWindowActionHandlers(deps: WindowActionHandlerDeps) {
  const { extensionWindowManager, executeViewAction } = deps;

  function executeWindowAction(action: any) {
    return performanceTraces.run(
      'extension.window.action',
      {
        actionType: action?.type,
        extensionId: action?.ownerExtensionId,
        windowType: action?.type,
      },
      () => extensionWindowManager.executeWindowAction(action),
      action?.traceId ? { traceId: action.traceId } : undefined,
    );
  }

  async function handleCreateOrToggleWindow(action, launchContext) {
    const view = action.view;
    const draftConflictDepth = Number(launchContext?.draftConflictDepth) || 0;
    if (
      view?.draft?.conflict &&
      view.draft.onConflict &&
      draftConflictDepth < 3
    ) {
      const conflictResult = await executeViewAction(
        { ...view.draft.onConflict, draftConflict: view.draft.conflict },
        {
          ...(launchContext || {}),
          draftConflictDepth: draftConflictDepth + 1,
        },
      );
      if (conflictResult?.view)
        return executeWindowAction({ ...action, view: conflictResult.view });
    }
    return executeWindowAction(action);
  }

  function handleShowHideCloseWindow(action) {
    return executeWindowAction(action);
  }

  return { handleCreateOrToggleWindow, handleShowHideCloseWindow };
}
