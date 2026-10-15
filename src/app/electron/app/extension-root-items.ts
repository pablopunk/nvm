// biome-ignore-all lint: Extension root items and ranked actions are dynamic extension-owned payloads moved verbatim from main.ts.
import type { BrowserWindow } from 'electron';
import {
  measureDebugPerformance,
  measureDebugPerformanceSync,
  performanceTraces,
} from '../diagnostics/debug-performance';
import { error as logError } from '../platform/logger';
import {
  isPromiseAbortError,
  isPromiseTimeoutError,
  withAbortableTimeout,
} from '../platform/promise-timeout';
import { rankSearchProviderContributions } from '../search/search-snapshot';

export type ExtensionRootItemsDeps = {
  paletteWindow: { readonly win: BrowserWindow | null };
  createExtensionContext: (
    extension: any,
    command: any,
    launchContext?: any,
    options?: any,
  ) => any;
  extensionRootActionFromItem: (entry: any, item: any) => any;
  rankAction: (action: any, query: any) => any;
  EXTENSION_ROOT_ITEMS_TTL_MS: number;
  EXTENSION_ROOT_ITEMS_TIMEOUT_MS: number;
};

export function createExtensionRootItems(deps: ExtensionRootItemsDeps) {
  const {
    paletteWindow,
    createExtensionContext,
    extensionRootActionFromItem,
    rankAction,
    EXTENSION_ROOT_ITEMS_TTL_MS,
    EXTENSION_ROOT_ITEMS_TIMEOUT_MS,
  } = deps;

  const extensionRootItemsCache = new Map<
    string,
    { updatedAt: number; items: any[] }
  >();
  const extensionRootItemsRefreshes = new Map<string, Promise<any[]>>();

  function validCachedRootActions(extension) {
    const cacheKey = extension.__filePath || extension.id;
    const cached = extensionRootItemsCache.get(cacheKey);
    return cached && Date.now() - cached.updatedAt < EXTENSION_ROOT_ITEMS_TTL_MS
      ? cached.items
      : null;
  }

  function invalidateExtensionRootItems() {
    extensionRootItemsCache.clear();
    paletteWindow.win?.webContents.send('root-items:changed');
  }

  function invalidateExtensionRootItemsForExtension(extension) {
    const cacheKey = extension.__filePath || extension.id;
    extensionRootItemsCache.delete(cacheKey);
    paletteWindow.win?.webContents.send('root-items:changed');
  }

  function rankContributionActions(actions, query) {
    return rankSearchProviderContributions(actions, query, rankAction);
  }

  async function extensionSearchActionsForExtension(
    extension,
    query,
    signal?,
    traceId?: string,
  ) {
    if (typeof extension.searchItems !== 'function') return [];
    return performanceTraces.run(
      'extension.searchItems',
      { extensionId: extension.id, queryLength: String(query || '').length },
      () =>
        measureDebugPerformance(
          'extension.search.provider',
          {
            extensionId: extension.id,
            queryLength: String(query || '').length,
            phase: 'final',
          },
          async () => {
            try {
              const entry = {
                extension,
                command: {
                  id: 'search',
                  title: extension.title || extension.id,
                },
              };
              const providerPromise = measureDebugPerformanceSync(
                'extension.search.provider',
                {
                  extensionId: extension.id,
                  queryLength: String(query || '').length,
                  phase: 'launch',
                },
                () =>
                  extension.searchItems(
                    createExtensionContext(extension, null, undefined, {
                      signal,
                    }),
                    query,
                  ),
              );
              const items = await withAbortableTimeout(
                providerPromise,
                EXTENSION_ROOT_ITEMS_TIMEOUT_MS,
                { signal },
              );
              const list = Array.isArray(items)
                ? items
                : Array.isArray(items?.items)
                  ? items.items
                  : [];
              return measureDebugPerformanceSync(
                'extension.search.provider.rank',
                {
                  extensionId: extension.id,
                  itemCount: list.length,
                  queryLength: String(query || '').length,
                },
                () =>
                  rankContributionActions(
                    list
                      .map((item) => extensionRootActionFromItem(entry, item))
                      .filter(Boolean),
                    query,
                  ),
              );
            } catch (error) {
              if (!(isPromiseAbortError(error) || isPromiseTimeoutError(error)))
                logError('extension.searchItems.failed', error, {
                  source: 'host',
                  scope: 'extension',
                  extensionId: extension.id,
                });
              throw error;
            }
          },
        ),
      traceId ? { traceId } : undefined,
    ) as Promise<any[]>;
  }

  function refreshExtensionRootActions(extension, cacheKey, traceId?: string) {
    const current = extensionRootItemsRefreshes.get(cacheKey);
    if (current) return current;
    const promise = Promise.resolve(
      performanceTraces.run(
        'extension.rootItems',
        { extensionId: extension.id },
        () =>
          measureDebugPerformance(
            'extension.root.provider',
            { extensionId: extension.id },
            async () => {
              const entry = {
                extension,
                command: { id: 'root', title: extension.title || extension.id },
              };
              const items = await withAbortableTimeout(
                extension.rootItems(createExtensionContext(extension, null)),
                EXTENSION_ROOT_ITEMS_TIMEOUT_MS,
              );
              const list = Array.isArray(items)
                ? items
                : Array.isArray(items?.items)
                  ? items.items
                  : [];
              const actions = measureDebugPerformanceSync(
                'extension.root.provider.rank',
                { extensionId: extension.id, itemCount: list.length },
                () =>
                  rankContributionActions(
                    list
                      .map((item) => extensionRootActionFromItem(entry, item))
                      .filter(Boolean),
                    '',
                  ),
              );
              extensionRootItemsCache.set(cacheKey, {
                updatedAt: Date.now(),
                items: actions,
              });
              return actions;
            },
          ),
        traceId ? { traceId } : undefined,
      ),
    )
      .catch((error) => {
        if (!isPromiseTimeoutError(error))
          logError('extension.rootItems.failed', error, {
            source: 'host',
            scope: 'extension',
            extensionId: extension.id,
          });
        return extensionRootItemsCache.get(cacheKey)?.items || [];
      })
      .finally(() => {
        extensionRootItemsRefreshes.delete(cacheKey);
      });
    extensionRootItemsRefreshes.set(cacheKey, promise);
    return promise;
  }

  return {
    extensionRootItemsCache,
    extensionRootItemsRefreshes,
    validCachedRootActions,
    invalidateExtensionRootItems,
    invalidateExtensionRootItemsForExtension,
    rankContributionActions,
    extensionSearchActionsForExtension,
    refreshExtensionRootActions,
  };
}
