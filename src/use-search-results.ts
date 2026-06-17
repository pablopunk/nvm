import { useEffect, useRef, useState } from 'react';
import {
  markDebugPerformance,
  measureDebugPerformance,
} from './debug-performance';

export function useSearchResults<T>(
  search: (query: string) => Promise<T[]>,
  query: string,
  refreshNonce: number,
) {
  const [results, setResults] = useState<T[]>([]);
  const requestIdRef = useRef(0);

  useEffect(() => {
    let cancelled = false;
    const requestId = ++requestIdRef.current;
    markDebugPerformance('search.schedule', {
      requestId,
      queryLength: query.length,
      refreshNonce,
    });
    const timer = window.setTimeout(async () => {
      const next = await measureDebugPerformance(
        'search.renderer-to-results',
        { requestId, queryLength: query.length, refreshNonce, alwaysLog: true },
        () => search(query),
      );
      if (cancelled) {
        markDebugPerformance('search.drop-stale', {
          requestId,
          queryLength: query.length,
          resultCount: next.length,
        });
        return;
      }
      markDebugPerformance('search.set-results', {
        requestId,
        queryLength: query.length,
        resultCount: next.length,
      });
      setResults(next);
    }, 20);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [search, query, refreshNonce]);

  return [results, setResults] as const;
}
