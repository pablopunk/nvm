import { performanceTraces } from '../../diagnostics/debug-performance';
import type { createViewLoaderRegistry } from '../../state/data-loader';
import type { MaintenanceActivity } from '../../platform/maintenance-activity';

export type NativeActionHandlerDeps = {
  viewLoaderRegistry: Pick<
    ReturnType<typeof createViewLoaderRegistry>,
    'has' | 'retry'
  >;
  executeAction: (action: any, options?: any) => Promise<any>;
  activity: Pick<MaintenanceActivity, 'run'>;
  verifyStateView: (nativeAction: { confirmation?: unknown }) => unknown;
};

export function createNativeActionHandlers(deps: NativeActionHandlerDeps) {
  const { viewLoaderRegistry, executeAction } = deps;

  function handleNativeAction(action) {
    const native = action.nativeAction as
      | { kind?: string; viewId?: string; confirmation?: unknown }
      | undefined;
    if (native?.kind === 'state-verification')
      return deps.verifyStateView(native);
    if (native?.kind === 'view-hydrate-retry' && native.viewId) {
      if (viewLoaderRegistry.has(native.viewId))
        void deps.activity
          .run(() =>
            Promise.resolve(
              performanceTraces.run(
                'view.loader.retry',
                { viewId: native.viewId },
                () => viewLoaderRegistry.retry(native.viewId),
                action?.traceId ? { traceId: action.traceId } : undefined,
              ),
            ),
          )
          .catch(() => {});
      // Return a loading skeleton so the renderer replaces the error view
      return {
        view: {
          type: 'list' as const,
          id: native.viewId,
          title: 'Loading...',
          isLoading: true,
          items: [],
        },
        navigation: 'replace' as const,
      };
    }
    return executeAction(
      { ...action.nativeAction, traceId: action.traceId },
      { keepPaletteOpen: true },
    );
  }

  return { handleNativeAction };
}
