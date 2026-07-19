export class PromiseTimeoutError extends Error {
  readonly timeoutMs: number;

  constructor(timeoutMs: number) {
    super(`Timed out after ${timeoutMs}ms`);
    this.name = 'PromiseTimeoutError';
    this.timeoutMs = timeoutMs;
  }
}

// biome-ignore lint/style/useExportsLast: Error types stay adjacent so callers can distinguish lifecycle outcomes.
export class PromiseAbortError extends Error {
  constructor() {
    super('Operation aborted');
    this.name = 'AbortError';
  }
}

type TimeoutHandle = ReturnType<typeof setTimeout> & { unref?: () => void };

export interface PromiseTimeoutTimers {
  setTimeout: (callback: () => void, timeoutMs: number) => TimeoutHandle;
  clearTimeout: (handle: TimeoutHandle) => void;
}

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
      if (timer) {
        timers.clearTimeout(timer);
      }
      timer = undefined;
      options.signal?.removeEventListener('abort', abort);
    }

    function claimSettlement() {
      if (settled) {
        return false;
      }
      settled = true;
      cleanup();
      return true;
    }

    function abort() {
      if (claimSettlement()) {
        reject(new PromiseAbortError());
      }
    }

    // Attach both handlers before installing cancellation. A provider that
    // ignores cancellation can settle later without becoming unhandled.
    Promise.resolve(promise).then(
      (value) => {
        if (claimSettlement()) {
          resolve(value);
        }
      },
      (error) => {
        if (claimSettlement()) {
          reject(error);
        }
      },
    );

    if (options.signal?.aborted) {
      abort();
      return;
    }

    options.signal?.addEventListener('abort', abort, { once: true });
    timer = timers.setTimeout(() => {
      if (claimSettlement()) {
        reject(new PromiseTimeoutError(timeoutMs));
      }
    }, timeoutMs);
    timer.unref?.();
  });
}
