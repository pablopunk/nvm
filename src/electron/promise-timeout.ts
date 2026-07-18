export class PromiseTimeoutError extends Error {
  readonly timeoutMs: number;

  constructor(timeoutMs: number) {
    super(`Timed out after ${timeoutMs}ms`);
    this.name = 'PromiseTimeoutError';
    this.timeoutMs = timeoutMs;
  }
}

export class PromiseAbortError extends Error {
  constructor() {
    super('Operation aborted');
    this.name = 'AbortError';
  }
}

type TimeoutHandle = ReturnType<typeof setTimeout> & { unref?: () => void };

export type PromiseTimeoutTimers = {
  setTimeout: (callback: () => void, timeoutMs: number) => TimeoutHandle;
  clearTimeout: (handle: TimeoutHandle) => void;
};

const defaultTimers: PromiseTimeoutTimers = {
  setTimeout: (callback, timeoutMs) => setTimeout(callback, timeoutMs),
  clearTimeout: (handle) => clearTimeout(handle),
};

export function isPromiseAbortError(error: unknown) {
  return error instanceof PromiseAbortError;
}

export function isPromiseTimeoutError(error: unknown) {
  return error instanceof PromiseTimeoutError;
}

export function withAbortableTimeout<T>(
  promise: PromiseLike<T> | T,
  timeoutMs: number,
  options: {
    signal?: AbortSignal;
    timers?: PromiseTimeoutTimers;
  } = {},
): Promise<T> {
  const timers = options.timers || defaultTimers;
  return new Promise<T>((resolve, reject) => {
    let settled = false;
    let timer: TimeoutHandle | undefined;

    function cleanup() {
      if (timer) timers.clearTimeout(timer);
      timer = undefined;
      options.signal?.removeEventListener('abort', abort);
    }

    function settle(callback: (value: any) => void, value: unknown) {
      if (settled) return;
      settled = true;
      cleanup();
      callback(value);
    }

    function abort() {
      settle(reject, new PromiseAbortError());
    }

    // Attach both handlers before installing cancellation. A provider that
    // ignores cancellation can settle later without becoming unhandled.
    Promise.resolve(promise).then(
      (value) => settle(resolve, value),
      (error) => settle(reject, error),
    );

    if (options.signal?.aborted) {
      abort();
      return;
    }

    options.signal?.addEventListener('abort', abort, { once: true });
    timer = timers.setTimeout(
      () => settle(reject, new PromiseTimeoutError(timeoutMs)),
      timeoutMs,
    );
    timer.unref?.();
  });
}
