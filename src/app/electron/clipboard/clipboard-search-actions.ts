// biome-ignore-all lint: Clipboard history entries and ranked actions are dynamic persisted payloads.
export type ClipboardSearchActionsDeps = {
  getHistory: () => any[];
  clipboardRootItem: (item: any) => any;
  rankAction: (item: any, query: string) => unknown;
  prepareRootActionForRenderer: (item: any) => any;
  measureSync: <T>(
    name: string,
    data: Record<string, unknown>,
    fn: () => T,
  ) => T;
  clipboardLimit: number;
};

export function createClipboardSearchActions(deps: ClipboardSearchActionsDeps) {
  const {
    getHistory,
    clipboardRootItem,
    rankAction,
    prepareRootActionForRenderer,
    measureSync,
    clipboardLimit,
  } = deps;

  function clipboardSearchActions(query: string) {
    return measureSync(
      'search.clipboard-only',
      { queryLength: query.length, clipboardCount: getHistory().length },
      () =>
        getHistory()
          .map(clipboardRootItem)
          .filter((item) => (query ? rankAction(item, query) : true))
          .sort((a, b) =>
            query
              ? b.score - a.score || b.lastUsed - a.lastUsed
              : b.lastUsed - a.lastUsed,
          )
          .slice(0, clipboardLimit)
          .map(prepareRootActionForRenderer),
    );
  }

  return { clipboardSearchActions };
}
