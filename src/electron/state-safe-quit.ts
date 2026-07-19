interface QuitEvent {
  preventDefault: () => void;
}

interface StateSafeQuitDeps {
  flushPendingSave: () => Promise<void>;
  quit: () => void;
  cleanup: () => void;
  exit: () => void;
  onFlushError?: (error: unknown, reason: string) => void;
  onFlushTimeout?: (reason: string) => void;
  onFallbackExit?: (reason: string) => void;
  flushTimeoutMs?: number;
  exitFallbackMs?: number;
  setTimer?: typeof setTimeout;
  clearTimer?: typeof clearTimeout;
}

const DEFAULT_FLUSH_TIMEOUT_MS = 2000;
const DEFAULT_EXIT_FALLBACK_MS = 2000;

function flushWithinTimeout(deps: StateSafeQuitDeps, reason: string) {
  const timeoutMs = deps.flushTimeoutMs ?? DEFAULT_FLUSH_TIMEOUT_MS;
  const setTimer = deps.setTimer ?? setTimeout;
  const clearTimer = deps.clearTimer ?? clearTimeout;
  return new Promise<void>((resolve) => {
    let settled = false;
    const finish = () => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimer(timeoutTimer);
      resolve();
    };
    const timeoutTimer = setTimer(() => {
      deps.onFlushTimeout?.(reason);
      finish();
    }, timeoutMs);
    timeoutTimer.unref?.();
    Promise.resolve()
      .then(deps.flushPendingSave)
      .catch((error) => deps.onFlushError?.(error, reason))
      .then(finish);
  });
}

function createStateSafeQuit(deps: StateSafeQuitDeps) {
  const exitFallbackMs = deps.exitFallbackMs ?? DEFAULT_EXIT_FALLBACK_MS;
  const setTimer = deps.setTimer ?? setTimeout;
  const clearTimer = deps.clearTimer ?? clearTimeout;
  let quitAfterStateFlush = false;
  let quitRequest: Promise<void> | null = null;
  let fallbackTimer: ReturnType<typeof setTimeout> | undefined;

  function scheduleFallbackExit(reason: string) {
    fallbackTimer = setTimer(() => {
      fallbackTimer = undefined;
      deps.onFallbackExit?.(reason);
      deps.cleanup();
      deps.exit();
    }, exitFallbackMs);
    fallbackTimer.unref?.();
  }

  function requestQuit(reason: string, quit: () => void = deps.quit) {
    if (quitAfterStateFlush) {
      quit();
      return Promise.resolve();
    }
    if (quitRequest) {
      return quitRequest;
    }
    quitRequest = flushWithinTimeout(deps, reason).then(() => {
      quitAfterStateFlush = true;
      scheduleFallbackExit(reason);
      quit();
    });
    return quitRequest;
  }

  function handleBeforeQuit(event: QuitEvent) {
    if (quitAfterStateFlush) {
      return;
    }
    event.preventDefault();
    requestQuit('before-quit').catch(() => undefined);
  }

  function handleWillQuit() {
    if (fallbackTimer !== undefined) {
      clearTimer(fallbackTimer);
      fallbackTimer = undefined;
    }
    deps.cleanup();
  }

  return {
    requestQuit,
    handleBeforeQuit,
    handleWillQuit,
    isQuitAfterStateFlush: () => quitAfterStateFlush,
  };
}

export type { QuitEvent, StateSafeQuitDeps };
export { createStateSafeQuit };
