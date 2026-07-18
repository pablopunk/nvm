export type SearchSnapshot<T> = {
  generation: number;
  revision: number;
  results: T[];
  complete: boolean;
};

export type SearchRequest = {
  query: string;
  generation: number;
  clipboardOnly?: boolean;
};

export type SearchProvider<T> = {
  key: string;
  run(signal: AbortSignal): Promise<T[]>;
};

export type SearchWork<T> = {
  initialResults: T[];
  providers: SearchProvider<T>[];
  buildResults(resultsByProvider: ReadonlyMap<string, T[]>): T[];
  completeImmediately?: boolean;
  onProvidersLaunched?: (durationMs: number) => void;
  onComplete?: () => void;
};

export type SearchSender = {
  isDestroyed(): boolean;
  send(channel: string, payload: unknown): void;
  on(event: 'destroyed' | 'render-process-gone', listener: () => void): void;
  removeListener(
    event: 'destroyed' | 'render-process-gone',
    listener: () => void,
  ): void;
};

type ScheduledFlush = unknown;

export type SearchCoordinatorOptions<T> = {
  createWork(request: SearchRequest): SearchWork<T>;
  fingerprint(results: T[]): string;
  scheduleFlush(callback: () => void): ScheduledFlush;
  cancelFlush(handle: ScheduledFlush): void;
  updateChannel?: string;
};

type ActiveSearch<T> = {
  generation: number;
  revision: number;
  controller: AbortController;
  resultsByProvider: Map<string, T[]>;
  pendingProviders: number;
  lastFingerprint: string;
  scheduledFlush?: ScheduledFlush;
  work: SearchWork<T>;
};

type SenderState<T> = {
  sender: SearchSender;
  highestSeenGeneration: number;
  active?: ActiveSearch<T>;
  destroy: () => void;
};

function defaultScheduleFlush(callback: () => void) {
  const handle = setTimeout(callback, 16);
  handle.unref?.();
  return handle;
}

export function createSearchCoordinator<T>(
  options: Partial<
    Pick<SearchCoordinatorOptions<T>, 'scheduleFlush' | 'cancelFlush'>
  > &
    Pick<SearchCoordinatorOptions<T>, 'createWork' | 'fingerprint'> & {
      updateChannel?: string;
    },
) {
  const scheduleFlush = options.scheduleFlush || defaultScheduleFlush;
  const cancelFlush =
    options.cancelFlush ||
    ((handle) => clearTimeout(handle as ReturnType<typeof setTimeout>));
  const updateChannel = options.updateChannel || 'actions:search:update';
  const states = new Map<SearchSender, SenderState<T>>();

  function removeState(state: SenderState<T>) {
    state.active?.controller.abort();
    if (state.active?.scheduledFlush !== undefined)
      cancelFlush(state.active.scheduledFlush);
    state.sender.removeListener('destroyed', state.destroy);
    state.sender.removeListener('render-process-gone', state.destroy);
    states.delete(state.sender);
  }

  function stateFor(sender: SearchSender) {
    const current = states.get(sender);
    if (current) return current;
    const state = {
      sender,
      highestSeenGeneration: 0,
      destroy: () => removeState(state),
    } as SenderState<T>;
    sender.on('destroyed', state.destroy);
    sender.on('render-process-gone', state.destroy);
    states.set(sender, state);
    return state;
  }

  function isCurrent(state: SenderState<T>, active: ActiveSearch<T>) {
    return (
      state.active === active &&
      !active.controller.signal.aborted &&
      !state.sender.isDestroyed()
    );
  }

  function flush(state: SenderState<T>, active: ActiveSearch<T>) {
    active.scheduledFlush = undefined;
    if (!isCurrent(state, active)) return;
    const results = active.work.buildResults(active.resultsByProvider);
    const fingerprint = options.fingerprint(results);
    const complete = active.pendingProviders === 0;
    if (!complete && fingerprint === active.lastFingerprint) return;
    active.lastFingerprint = fingerprint;
    active.revision += 1;
    if (complete) active.work.onComplete?.();
    if (!isCurrent(state, active)) return;
    const snapshot: SearchSnapshot<T> = {
      generation: active.generation,
      revision: active.revision,
      results,
      complete,
    };
    state.sender.send(updateChannel, snapshot);
    if (complete) state.active = undefined;
  }

  function requestFlush(state: SenderState<T>, active: ActiveSearch<T>) {
    if (!isCurrent(state, active) || active.scheduledFlush !== undefined)
      return;
    active.scheduledFlush = scheduleFlush(() => flush(state, active));
  }

  function settleProvider(
    state: SenderState<T>,
    active: ActiveSearch<T>,
    provider: SearchProvider<T>,
    results: T[],
  ) {
    if (!isCurrent(state, active)) return;
    active.resultsByProvider.set(provider.key, results);
    active.pendingProviders -= 1;
    requestFlush(state, active);
  }

  function launchProviders(state: SenderState<T>, active: ActiveSearch<T>) {
    if (!isCurrent(state, active)) return;
    if (!active.work.providers.length) {
      requestFlush(state, active);
      return;
    }
    const startedAt = performance.now();
    for (const provider of active.work.providers) {
      let promise: Promise<T[]>;
      try {
        promise = Promise.resolve(provider.run(active.controller.signal));
      } catch (error) {
        promise = Promise.reject(error);
      }
      promise.then(
        (results) => settleProvider(state, active, provider, results),
        () => settleProvider(state, active, provider, []),
      );
    }
    active.work.onProvidersLaunched?.(performance.now() - startedAt);
  }

  function search(sender: SearchSender, request: SearchRequest) {
    if (!(Number.isSafeInteger(request.generation) && request.generation > 0))
      throw new RangeError('Search generation must be a positive safe integer');
    const state = stateFor(sender);
    if (request.generation <= state.highestSeenGeneration)
      throw new RangeError('Search generation must increase monotonically');

    state.highestSeenGeneration = request.generation;
    state.active?.controller.abort();
    if (state.active?.scheduledFlush !== undefined)
      cancelFlush(state.active.scheduledFlush);

    const work = options.createWork(request);
    const snapshot: SearchSnapshot<T> = {
      generation: request.generation,
      revision: 0,
      results: work.initialResults,
      complete: Boolean(work.completeImmediately),
    };
    if (snapshot.complete) {
      state.active = undefined;
      return snapshot;
    }

    const active: ActiveSearch<T> = {
      generation: request.generation,
      revision: 0,
      controller: new AbortController(),
      resultsByProvider: new Map(),
      pendingProviders: work.providers.length,
      lastFingerprint: options.fingerprint(work.initialResults),
      work,
    };
    state.active = active;
    queueMicrotask(() => launchProviders(state, active));
    return snapshot;
  }

  function cancel(sender: SearchSender, generation: number) {
    const state = states.get(sender);
    const active = state?.active;
    if (!(state && active && active.generation === generation)) return;
    active.controller.abort();
    if (active.scheduledFlush !== undefined) cancelFlush(active.scheduledFlush);
    state.active = undefined;
  }

  function dispose() {
    for (const state of [...states.values()]) removeState(state);
  }

  return {
    search,
    cancel,
    dispose,
    senderCount: () => states.size,
  };
}

function stableClone(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(stableClone);
  if (!(value && typeof value === 'object')) return value;
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>)
      .filter(([key, entry]) => key !== 'executionId' && entry !== undefined)
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([key, entry]) => [key, stableClone(entry)]),
  );
}

export function searchResultsFingerprint(results: unknown[]) {
  return JSON.stringify(stableClone(results));
}

export function createStableSearchResultPreparer<TSource, TPrepared>(options: {
  logicalKey(source: TSource): string;
  prepare(source: TSource): TPrepared;
}) {
  const cache = new Map<
    string,
    { sourceFingerprint: string; prepared: TPrepared }
  >();
  return (sources: TSource[]) => {
    const occurrences = new Map<string, number>();
    return sources.map((source) => {
      const logicalKey = options.logicalKey(source);
      const occurrence = occurrences.get(logicalKey) || 0;
      occurrences.set(logicalKey, occurrence + 1);
      const cacheKey = `${logicalKey}:${occurrence}`;
      const sourceFingerprint = searchResultsFingerprint([source]);
      const cached = cache.get(cacheKey);
      if (cached?.sourceFingerprint === sourceFingerprint)
        return cached.prepared;
      const prepared = options.prepare(source);
      structuredClone(prepared);
      cache.set(cacheKey, { sourceFingerprint, prepared });
      return prepared;
    });
  };
}
