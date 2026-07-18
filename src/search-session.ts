import type { SearchSnapshot } from './preload-api';

export type SearchSessionTransport<T> = {
  search(
    query: string,
    options: { generation: number; clipboardOnly?: boolean },
  ): Promise<SearchSnapshot<T>>;
  cancelSearch(generation: number): void;
  onSearchUpdate(callback: (snapshot: SearchSnapshot<T>) => void): () => void;
};

export function createSearchSession<T>(options: {
  transport: SearchSessionTransport<T>;
  onSnapshot(snapshot: SearchSnapshot<T>, timing: { elapsedMs: number }): void;
  onError?(error: unknown, generation: number): void;
  now?: () => number;
}) {
  const now = options.now || (() => performance.now());
  let nextGeneration = 0;
  let active:
    | { generation: number; highestRevision: number; startedAt: number }
    | undefined;

  function accept(snapshot: SearchSnapshot<T>) {
    if (
      !active ||
      snapshot.generation !== active.generation ||
      snapshot.revision <= active.highestRevision
    )
      return;
    active.highestRevision = snapshot.revision;
    options.onSnapshot(snapshot, { elapsedMs: now() - active.startedAt });
  }

  const unsubscribe = options.transport.onSearchUpdate(accept);

  function start(query: string, searchOptions?: { clipboardOnly?: boolean }) {
    const generation = ++nextGeneration;
    active = { generation, highestRevision: -1, startedAt: now() };
    options.transport
      .search(query, { generation, ...searchOptions })
      .then(accept)
      .catch((error) => {
        if (active?.generation === generation)
          options.onError?.(error, generation);
      });
    return generation;
  }

  function cancel(generation: number) {
    if (active?.generation !== generation) return;
    options.transport.cancelSearch(generation);
    active = undefined;
  }

  function dispose() {
    if (active) options.transport.cancelSearch(active.generation);
    active = undefined;
    unsubscribe();
  }

  return { start, cancel, dispose };
}
