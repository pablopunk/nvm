import { compareRankedActions } from './action-ranking';
import { createStableSearchResultPreparer } from './search-coordinator';

export const SEARCH_PROVIDER_RESULT_LIMIT = 20;
export const SEARCH_RESULT_LIMIT = 30;

export interface SearchExtension {
  id: string;
  __filePath?: string;
  rootItems?: unknown;
  searchItems?: unknown;
}

export interface SearchProviderDescriptor<Extension extends SearchExtension> {
  extension: Extension;
  key: string;
  kind: 'query' | 'root';
}

export interface SearchAssemblyAction extends Record<string, unknown> {
  __ranked?: boolean;
  commandId?: unknown;
  extensionId?: unknown;
  id?: unknown;
  kind?: unknown;
  lastUsed: number;
  score: number;
  title?: unknown;
}

export function extensionActionContributionIsDiscoverable(item: {
  placement?: unknown;
}) {
  return !(Array.isArray(item.placement) && item.placement.includes('hidden'));
}

export function searchProviderDescriptors<Extension extends SearchExtension>(
  extensions: Extension[],
  query: string,
): SearchProviderDescriptor<Extension>[] {
  const kind = query ? 'query' : 'root';
  return extensions.flatMap((extension, index) => {
    const provider =
      kind === 'query' ? extension.searchItems : extension.rootItems;
    return typeof provider === 'function'
      ? [
          {
            extension,
            key: `${index}:${extension.__filePath || extension.id}`,
            kind,
          },
        ]
      : [];
  });
}

export function searchActionIsVisibleInTestMode(
  action: SearchAssemblyAction,
  options: {
    isSafeExtension(extensionId: unknown): boolean;
    progressiveRootExtensionId: string;
    testMode: boolean;
  },
) {
  if (!options.testMode) {
    return true;
  }
  return (
    action.kind === 'test-action' ||
    (action.kind === 'extension-action' &&
      options.isSafeExtension(action.extensionId)) ||
    (action.kind === 'extension-root-item' &&
      (options.isSafeExtension(action.extensionId) ||
        action.extensionId === options.progressiveRootExtensionId))
  );
}

export function rankSearchProviderContributions<
  Action extends SearchAssemblyAction,
>(
  actions: Action[],
  query: string,
  rankAction: (action: Action, query: string) => Action | null,
) {
  return actions
    .map((action) => {
      const ranked = rankAction(action, query);
      return ranked ? { ...ranked, __ranked: true } : null;
    })
    .filter((action): action is Action & { __ranked: true } => Boolean(action))
    .sort(
      (left, right) =>
        right.score - left.score ||
        right.lastUsed - left.lastUsed ||
        String(left.title || '').localeCompare(String(right.title || '')),
    )
    .slice(0, SEARCH_PROVIDER_RESULT_LIMIT);
}

function extensionContributionIdentity(action: SearchAssemblyAction) {
  if (
    (action.kind !== 'extension-action' &&
      action.kind !== 'extension-root-item') ||
    typeof action.extensionId !== 'string' ||
    typeof action.commandId !== 'string'
  ) {
    return null;
  }
  return JSON.stringify([action.extensionId, action.commandId]);
}

export function createSearchSnapshotAssembler<
  Action extends SearchAssemblyAction,
  Prepared,
>(options: {
  isVisible(action: Action): boolean;
  localItems: Action[];
  prepare(action: Action): Prepared;
  providerKeys: string[];
  query: string;
  rankAction(action: Action, query: string): Action | null;
  withShortcutHint(action: Action): Action;
}) {
  const prepareRows = createStableSearchResultPreparer<Action, Prepared>({
    logicalKey: (action) =>
      `${String(action.extensionId || action.kind || 'action')}:${String(action.id)}`,
    prepare: options.prepare,
  });

  return (resultsByProvider: ReadonlyMap<string, Action[]>) => {
    const providerResults: Action[] = [];
    for (const key of options.providerKeys) {
      for (const item of resultsByProvider.get(key) || []) {
        const withShortcut = options.withShortcutHint(item);
        const ranked = item.__ranked
          ? withShortcut
          : options.rankAction(withShortcut, options.query);
        if (ranked) {
          providerResults.push(ranked);
        }
      }
    }

    const isRootPaletteSearch = !options.query.trim();
    const rootProviderIdentities = new Set(
      isRootPaletteSearch
        ? providerResults
            .filter((action) => action.kind === 'extension-root-item')
            .map(extensionContributionIdentity)
            .filter((identity): identity is string => identity !== null)
        : [],
    );
    const results = options.localItems.filter((action) => {
      if (!isRootPaletteSearch || action.kind !== 'extension-action')
        return true;
      const identity = extensionContributionIdentity(action);
      return !identity || !rootProviderIdentities.has(identity);
    });
    results.push(...providerResults);

    const prepared = prepareRows(
      results
        .filter(options.isVisible)
        .sort(compareRankedActions)
        .slice(0, SEARCH_RESULT_LIMIT),
    );
    structuredClone(prepared);
    return prepared;
  };
}
