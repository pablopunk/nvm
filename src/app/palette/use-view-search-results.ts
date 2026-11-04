import { useEffect, useState } from 'react';
import type { CommandItem, CommandView } from './model';

/**
 * Host half of `ExtensionView.onSearchChange`: local filtering stays instant and
 * owns the first results; this adds the extension's second batch after them.
 */
export const VIEW_SEARCH_RESULTS_DEBOUNCE_MS = 350;

export type ViewSearchResults = {
  viewKey: string;
  query: string;
  itemIds: string[];
};

export function viewSearchKey(view: Pick<CommandView, 'id' | 'title'> | null) {
  return view ? view.id || view.title : '';
}

/** Local matches first, then the named items minus anything already matched locally. */
export function mergeViewSearchResults(
  localMatches: CommandItem[],
  viewItems: CommandItem[],
  results: ViewSearchResults | null,
  view:
    | Pick<CommandView, 'id' | 'title' | 'maxVisibleItems'>
    | null
    | undefined,
  query: string,
) {
  if (
    !(
      view &&
      results &&
      results.itemIds.length > 0 &&
      results.viewKey === viewSearchKey(view) &&
      results.query === query.trim()
    )
  )
    return localMatches;
  const itemsById = new Map(viewItems.map((item) => [item.id, item]));
  const localIds = new Set(localMatches.map((item) => item.id));
  const extra = results.itemIds.flatMap((id) => {
    const item = itemsById.get(id);
    return item && !localIds.has(id) ? [item] : [];
  });
  if (extra.length === 0) return localMatches;
  // A render cap must never hide the second batch behind a long local list.
  const localRoom = view.maxVisibleItems
    ? Math.max(0, view.maxVisibleItems - extra.length)
    : localMatches.length;
  return [...localMatches.slice(0, localRoom), ...extra];
}

/**
 * Runs the view's `onSearchChange` once the user pauses typing and keeps the
 * latest response for the text it answered. Answers for text the user has since
 * changed are dropped; failures stay silent because the batch is enrichment.
 */
export function useViewSearchResults(
  view: CommandView | null,
  query: string,
): ViewSearchResults | null {
  const [results, setResults] = useState<ViewSearchResults | null>(null);
  const action = view?.onSearchChange;
  const viewKey = viewSearchKey(view);
  const searchText = query.trim();

  useEffect(() => {
    if (!(action && searchText)) {
      setResults(null);
      return;
    }
    let superseded = false;
    const timer = setTimeout(async function runSearchChange() {
      try {
        const result = await window.nvm.runViewAction({
          ...action,
          value: searchText,
          text: searchText,
        });
        if (superseded) return;
        setResults({
          viewKey,
          query: searchText,
          itemIds: result?.patch?.searchResultIds ?? [],
        });
      } catch {
        // Enrichment only: the local matches already answered the search.
      }
    }, VIEW_SEARCH_RESULTS_DEBOUNCE_MS);
    return function cancelSearchChange() {
      superseded = true;
      clearTimeout(timer);
    };
  }, [action, viewKey, searchText]);

  return results;
}
