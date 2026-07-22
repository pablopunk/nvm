import { useEffect, useRef, useState } from 'react';
import {
  markDebugPerformance,
  measureDebugPerformance,
  recordDebugPerformance,
} from './debug-performance';
import {
  createSearchSession,
  type SearchSessionTransport,
} from './search-session';

const SEARCH_DEBOUNCE_MS = 20;

export function visibleResultsForSearchSnapshot<T>(
  currentResults: T[],
  snapshot: { complete: boolean; results: T[]; revision: number },
) {
  return currentResults.length > 0 &&
    snapshot.revision === 0 &&
    !snapshot.complete
    ? currentResults
    : snapshot.results;
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: Hook keeps one search session lifecycle and one debounce lifecycle together.
export function useSearchResults<T>(
  transport: SearchSessionTransport<T>,
  query: string,
  refreshNonce: number,
) {
  const [results, setResults] = useState<T[]>([]);
  const sessionRef = useRef<ReturnType<typeof createSearchSession<T>> | null>(
    null,
  );

  useEffect(() => {
    sessionRef.current = createSearchSession({
      transport: {
        ...transport,
        search: (nextQuery, options) =>
          measureDebugPerformance(
            'search.renderer-to-results',
            {
              generation: options.generation,
              queryLength: nextQuery.length,
              phase: 'initial',
              alwaysLog: true,
            },
            () => transport.search(nextQuery, options),
          ),
      },
      onSnapshot: (snapshot, timing) => {
        if (snapshot.complete) {
          recordDebugPerformance(
            'search.renderer-to-results',
            timing.elapsedMs,
            {
              generation: snapshot.generation,
              revision: snapshot.revision,
              resultCount: snapshot.results.length,
              phase: 'final',
              alwaysLog: true,
            },
          );
        }
        markDebugPerformance('search.set-results', {
          generation: snapshot.generation,
          revision: snapshot.revision,
          resultCount: snapshot.results.length,
          complete: snapshot.complete,
        });
        setResults((current) =>
          visibleResultsForSearchSnapshot(current, snapshot),
        );
      },
      onError: (_error, generation) =>
        markDebugPerformance('search.failed', { generation }),
    });
    return () => {
      sessionRef.current?.dispose();
      sessionRef.current = null;
    };
  }, [transport]);

  useEffect(() => {
    let generation: number | undefined;
    markDebugPerformance('search.schedule', {
      queryLength: query.length,
      refreshNonce,
    });
    const timer = window.setTimeout(() => {
      generation = sessionRef.current?.start(query);
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      window.clearTimeout(timer);
      if (generation !== undefined) {
        sessionRef.current?.cancel(generation);
      }
    };
  }, [query, refreshNonce]);

  return [results, setResults] as const;
}
