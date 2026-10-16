// biome-ignore-all lint: Host-side extension payload boundary keeps dynamic view shapes by design.
import { isLoaderHandle } from '../state/data-loader';

export type ContextViewsDeps = {
  checkRefreshBurst: (extension: any) => boolean;
  invalidateExtensionRootItemsForExtension: (extension: any) => void;
  extensionCacheFor: (extensionId: string) => Map<string, unknown>;
};

export function createExtensionViewsApi(
  extension,
  command,
  deps: ContextViewsDeps,
) {
  const {
    checkRefreshBurst,
    invalidateExtensionRootItemsForExtension,
    extensionCacheFor,
  } = deps;
  return {
    refresh: () => ({
      type: 'runExtensionAction',
      title: 'Refresh',
      __handler: async (innerCtx) => {
        if (!command || typeof command.run !== 'function')
          return { toast: { message: 'View cannot refresh', tone: 'error' } };
        if (!checkRefreshBurst(extension)) return { skipped: true };
        const result = await command.run(innerCtx);
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
          return { patch: { mode: 'replace', items: view.items } };
        if (view) return { view, navigation: 'replace' };
        return result;
      },
    }),
    invalidate: () => {
      invalidateExtensionRootItemsForExtension(extension);
      extensionCacheFor(extension.id).clear();
    },
  };
}
