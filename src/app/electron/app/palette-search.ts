// biome-ignore-all lint: Search actions are dynamic extension payloads moved verbatim from main.ts.
import {
  markDebugPerformance,
  measureDebugPerformance,
  measureDebugPerformanceSync,
  performanceTraces,
  recordDebugPerformance,
} from '../diagnostics/debug-performance';
import { withAbortableTimeout } from '../platform/promise-timeout';
import { isNvmTestMode } from '../platform/test-mode-flags';
import {
  createSearchCoordinator,
  searchResultsFingerprint,
} from '../search/search-coordinator';
import {
  createSearchSnapshotAssembler,
  extensionActionContributionIsDiscoverable,
  searchProviderDescriptors,
} from '../search/search-snapshot';
import {
  testModeExtensionIsSafe,
  testModePaletteActionIsSafe,
  testModeSafeAction,
} from './test-mode-safety';

export type PaletteSearchDeps = {
  extensionModules: Map<string, any>;
  extensionActionRegistry: Map<string, any>;
  extensionActionFromContribution: (entry: any) => any;
  withShortcutHint: (action: any) => any;
  rankAction: (action: any, query: any) => any;
  clipboardSearchActions: (query: string) => any[];
  normalizeViewAction: (action: any, entry?: any) => any;
  extensionEntryForAction: (action: any) => any;
  registerRootActionForRenderer: (action: any) => any;
  validCachedRootActions: (extension: any) => any[] | null;
  refreshExtensionRootActions: (
    extension: any,
    cacheKey: any,
    traceId?: string,
  ) => Promise<any[]>;
  extensionSearchActionsForExtension: (
    extension: any,
    query: any,
    signal?: any,
    traceId?: string,
  ) => Promise<any[]>;
  EXTENSION_ROOT_ITEMS_INITIAL_TIMEOUT_MS: number;
};

function isFixtureExtension(extension) {
  return Boolean(extension?.__fixture);
}

export function createPaletteSearch(deps: PaletteSearchDeps) {
  const {
    extensionModules,
    extensionActionRegistry,
    extensionActionFromContribution,
    withShortcutHint,
    rankAction,
    clipboardSearchActions,
    normalizeViewAction,
    extensionEntryForAction,
    registerRootActionForRenderer,
    validCachedRootActions,
    refreshExtensionRootActions,
    extensionSearchActionsForExtension,
    EXTENSION_ROOT_ITEMS_INITIAL_TIMEOUT_MS,
  } = deps;

  function visibleExtensions() {
    return Array.from(extensionModules.values()).filter(
      (extension) => !isFixtureExtension(extension),
    );
  }

  function visibleExtensionActionEntries() {
    return Array.from(extensionActionRegistry.values()).filter(
      (entry) => !isFixtureExtension(entry.extension),
    );
  }

  function searchableExtensions() {
    const extensions = visibleExtensions();
    return isNvmTestMode
      ? extensions.filter((extension) => testModeExtensionIsSafe(extension.id))
      : extensions;
  }

  function searchableExtensionActionEntries() {
    const entries = visibleExtensionActionEntries();
    const searchable = entries.filter((entry) =>
      extensionActionContributionIsDiscoverable(entry.item),
    );
    return isNvmTestMode
      ? searchable.filter((entry) =>
          testModeExtensionIsSafe(entry.extension.id),
        )
      : searchable;
  }

  function localSearchContributions(query) {
    const testAction = isNvmTestMode
      ? rankAction(testModeSafeAction(), query)
      : null;
    const results = testAction ? [testAction] : [];
    const entries = searchableExtensionActionEntries();
    measureDebugPerformanceSync(
      'search.rank-registered-actions',
      { queryLength: query.length, actionCount: entries.length },
      () => {
        for (const entry of entries) {
          const action = extensionActionFromContribution(entry);
          const ranked = action
            ? rankAction(withShortcutHint(action), query)
            : null;
          if (ranked) results.push(ranked);
        }
      },
    );
    return results;
  }

  function createSearchSnapshotBuilder(query, localItems, providerKeys) {
    const assemble = createSearchSnapshotAssembler<any, any>({
      isVisible: testModePaletteActionIsSafe,
      localItems,
      prepare: prepareRootActionForRenderer,
      providerKeys,
      query,
      rankAction,
      withShortcutHint,
    });
    return (resultsByProvider: ReadonlyMap<string, any[]>) =>
      measureDebugPerformanceSync(
        'search.sort-prepare',
        {
          queryLength: query.length,
          resultCount:
            localItems.length +
            providerKeys.reduce(
              (count, key) => count + (resultsByProvider.get(key)?.length || 0),
              0,
            ),
        },
        () => assemble(resultsByProvider),
      );
  }

  function createProgressiveSearchWork(input) {
    const query = String(input.query || '').trim();
    const startedAt = performance.now();
    if (input.clipboardOnly) {
      const initialResults = clipboardSearchActions(query);
      return {
        initialResults,
        providers: [],
        buildResults: () => initialResults,
        completeImmediately: true,
      };
    }

    return measureDebugPerformanceSync(
      'search.actions',
      {
        queryLength: query.length,
        clipboardOnly: false,
        generation: input.generation,
        phase: 'initial',
        alwaysLog: true,
      },
      () => {
        const extensions = searchableExtensions();
        const localItems = localSearchContributions(query);
        const providerDescriptors = searchProviderDescriptors(
          extensions,
          query,
        );
        const providerKeys = providerDescriptors.map(({ key }) => key);
        const initialProviderResults = new Map<string, any[]>();
        const providers = providerDescriptors.flatMap(
          ({ extension, key, kind }) => {
            if (kind === 'query') {
              return [
                {
                  key,
                  run: (signal) =>
                    extensionSearchActionsForExtension(
                      extension,
                      query,
                      signal,
                      input.traceId,
                    ),
                },
              ];
            }
            const cached = validCachedRootActions(extension);
            if (cached) {
              initialProviderResults.set(key, cached);
              return [];
            }
            return [
              {
                key,
                run: () =>
                  withAbortableTimeout(
                    refreshExtensionRootActions(
                      extension,
                      extension.__filePath || extension.id,
                      input.traceId,
                    ),
                    EXTENSION_ROOT_ITEMS_INITIAL_TIMEOUT_MS,
                  ).catch(() => []),
              },
            ];
          },
        );
        const buildSnapshot = createSearchSnapshotBuilder(
          query,
          localItems,
          providerKeys,
        );
        function buildResults(resultsByProvider) {
          const combined = new Map(initialProviderResults);
          for (const [key, value] of resultsByProvider)
            combined.set(key, value);
          return buildSnapshot(combined);
        }
        const initialResults = buildResults(new Map());
        markDebugPerformance('search.actions.result', {
          queryLength: query.length,
          generation: input.generation,
          phase: 'initial',
          providerCount: providers.length,
          resultCount: initialResults.length,
        });
        return {
          initialResults,
          providers,
          buildResults,
          onProvidersLaunched: (durationMs) =>
            recordDebugPerformance('extension.search.all', durationMs, {
              generation: input.generation,
              extensionCount: providers.length,
              queryLength: query.length,
              phase: 'launch',
            }),
          onComplete: () => {
            const durationMs = performance.now() - startedAt;
            recordDebugPerformance('extension.search.all', durationMs, {
              generation: input.generation,
              extensionCount: providers.length,
              queryLength: query.length,
              phase: 'final',
              alwaysLog: true,
            });
            recordDebugPerformance('search.actions', durationMs, {
              generation: input.generation,
              queryLength: query.length,
              phase: 'final',
              alwaysLog: true,
            });
          },
        };
      },
    );
  }

  const progressiveSearchCoordinator = createSearchCoordinator<any>({
    createWork: createProgressiveSearchWork,
    fingerprint: searchResultsFingerprint,
  });

  function normalizedProgressiveSearchInput(input: any = {}) {
    return {
      query: String(input.query || ''),
      generation: Number(input.generation),
      clipboardOnly: Boolean(input.clipboardOnly),
      traceId: typeof input.traceId === 'string' ? input.traceId : undefined,
    };
  }

  function startProgressiveSearch(sender, input) {
    const normalized = normalizedProgressiveSearchInput(input);
    return performanceTraces.run(
      'search.request',
      {
        generation: normalized.generation,
        queryLength: normalized.query.length,
        ...(normalized.clipboardOnly ? { cache: 'clipboard' } : {}),
      },
      () => progressiveSearchCoordinator.search(sender, normalized),
      normalized.traceId ? { traceId: normalized.traceId } : undefined,
    );
  }

  function cancelProgressiveSearch(sender, input) {
    progressiveSearchCoordinator.cancel(sender, Number(input?.generation));
  }

  async function searchActions(query, options: any = {}) {
    const q = String(query || '').trim();
    if (options.clipboardOnly) return clipboardSearchActions(q);
    return measureDebugPerformance(
      'search.actions',
      {
        queryLength: q.length,
        clipboardOnly: false,
        phase: 'final',
        alwaysLog: true,
      },
      async () => {
        const extensions = searchableExtensions();
        const providerDescriptors = searchProviderDescriptors(extensions, q);
        const providerKeys = providerDescriptors.map(({ key }) => key);
        const groups = await measureDebugPerformance(
          'extension.search.all',
          {
            extensionCount: providerKeys.length,
            queryLength: q.length,
            phase: 'final',
          },
          () =>
            Promise.all(
              providerDescriptors.map(({ extension, kind }) =>
                kind === 'query'
                  ? extensionSearchActionsForExtension(extension, q)
                  : Promise.resolve(
                      validCachedRootActions(extension) ||
                        refreshExtensionRootActions(
                          extension,
                          extension.__filePath || extension.id,
                        ),
                    ),
              ),
            ),
        );
        const providerResults = new Map<string, any[]>();
        providerKeys.forEach((key, index) =>
          providerResults.set(key, groups[index] || []),
        );
        return createSearchSnapshotBuilder(
          q,
          localSearchContributions(q),
          providerKeys,
        )(providerResults);
      },
    );
  }

  function prepareActionPanelForRenderer(panel, entry?: any) {
    if (!panel?.sections) return panel;
    return {
      ...panel,
      sections: panel.sections.map((section) => {
        const { lazyActions, ...rest } = section;
        return {
          ...rest,
          actions: [...(section.actions || []), ...(section.lazyActions || [])]
            .map((action) => normalizeViewAction(action, entry))
            .filter(Boolean),
        };
      }),
    };
  }

  function prepareRootActionForRenderer(action) {
    if (!action || typeof action !== 'object') return action;
    const entry = action.kind?.startsWith('extension-')
      ? extensionEntryForAction(action)
      : null;
    return registerRootActionForRenderer({
      ...action,
      primaryAction: normalizeViewAction(action.primaryAction, entry),
      rootAction: normalizeViewAction(action.rootAction, entry),
      actionPanel: prepareActionPanelForRenderer(action.actionPanel, entry),
    });
  }

  return {
    visibleExtensionActionEntries,
    progressiveSearchCoordinator,
    startProgressiveSearch,
    cancelProgressiveSearch,
    searchActions,
    prepareRootActionForRenderer,
  };
}
