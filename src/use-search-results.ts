import { useEffect, useRef, useState } from 'react';
import {
  recordPerformanceTrace,
  markDebugPerformance,
  measureDebugPerformance,
  recordDebugPerformance,
} from './debug-performance';
import { createRendererPerformanceTrace } from './performance-trace';
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
  const traceByGenerationRef = useRef(
    new Map<number, ReturnType<typeof createRendererPerformanceTrace>>(),
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
              traceId: options.traceId,
              queryLength: nextQuery.length,
              phase: 'initial',
              alwaysLog: true,
            },
            () => transport.search(nextQuery, options),
          ),
      },
      onSnapshot: (snapshot, timing) => {
        const trace = traceByGenerationRef.current.get(snapshot.generation);
        if (snapshot.traceId && snapshot.complete && trace) {
          recordPerformanceTrace(
            snapshot.traceId,
            'search.results',
            trace.startedAt,
            'ok',
            {
              generation: snapshot.generation,
              revision: snapshot.revision,
              resultCount: snapshot.results.length,
              complete: snapshot.complete,
            },
          );
        }
        if (trace)
          requestAnimationFrame(() => {
            if (traceByGenerationRef.current.get(snapshot.generation) !== trace)
              return;
            recordPerformanceTrace(
              trace.traceId,
              'search.results.paint',
              trace.startedAt,
              'ok',
              {
                generation: snapshot.generation,
                revision: snapshot.revision,
                resultCount: snapshot.results.length,
                complete: snapshot.complete,
              },
            );
            if (snapshot.complete)
              traceByGenerationRef.current.delete(snapshot.generation);
          });
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
      onError: (_error, generation) => {
        const trace = traceByGenerationRef.current.get(generation);
        if (trace) {
          recordPerformanceTrace(
            trace.traceId,
            'search.results',
            trace.startedAt,
            'error',
            { generation },
          );
          traceByGenerationRef.current.delete(generation);
        }
        markDebugPerformance('search.failed', { generation });
      },
    });
    return () => {
      sessionRef.current?.dispose();
      sessionRef.current = null;
    };
  }, [transport]);

  useEffect(() => {
    let generation: number | undefined;
    const trace = createRendererPerformanceTrace();
    markDebugPerformance('search.schedule', {
      queryLength: query.length,
      refreshNonce,
    });
    const timer = window.setTimeout(() => {
      generation = sessionRef.current?.start(query, { traceId: trace.traceId });
      if (generation !== undefined)
        traceByGenerationRef.current.set(generation, trace);
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      window.clearTimeout(timer);
      if (generation !== undefined) {
        sessionRef.current?.cancel(generation);
        const trace = traceByGenerationRef.current.get(generation);
        if (trace)
          recordPerformanceTrace(
            trace.traceId,
            'search.cancelled',
            trace.startedAt,
            'cancelled',
            { generation },
          );
        traceByGenerationRef.current.delete(generation);
      }
    };
  }, [query, refreshNonce]);

  return [results, setResults] as const;
}
