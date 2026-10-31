export type PreviewRunOutcome =
  | { status: 'ran' }
  | { status: 'failed'; message: string }
  | { status: 'not-run' };

export type PreviewRunReader = (filename: string) => Promise<PreviewRunOutcome>;

export type PreviewRunObservation = {
  succeeded: () => void;
  failed: (error: unknown) => void;
};

type ObservedAction = { id?: unknown; extensionFile?: unknown };

type Expectation = {
  commandId: string;
  outcome?: PreviewRunOutcome;
  listeners: Set<(outcome: PreviewRunOutcome) => void>;
};

export const PREVIEW_RUN_WAIT_MS = 10_000;
const MAX_TRACKED_FILES = 50;

function failureMessageFrom(error: unknown) {
  if (error instanceof Error) return error.stack || error.message;
  return String(error);
}

function isAbort(error: unknown) {
  return error instanceof Error && error.name === 'AbortError';
}

export function createPreviewRunObserver(deps: { waitMs?: number } = {}) {
  const defaultWaitMs = deps.waitMs ?? PREVIEW_RUN_WAIT_MS;
  const expectations = new Map<string, Expectation>();

  function settle(
    filename: string,
    expectation: Expectation,
    outcome: PreviewRunOutcome,
  ) {
    if (expectations.get(filename) !== expectation || expectation.outcome)
      return;
    expectation.outcome = outcome;
    for (const listener of expectation.listeners) listener(outcome);
    expectation.listeners.clear();
  }

  function forget(filename: string, expectation: Expectation) {
    for (const listener of expectation.listeners)
      listener({ status: 'not-run' });
    expectation.listeners.clear();
    expectations.delete(filename);
  }

  function evictOldest() {
    const oldest = expectations.entries().next().value;
    if (oldest) forget(oldest[0], oldest[1]);
  }

  // The first command is run by the renderer, only for the active preview.
  // The observer never runs anything: it records how that run ends.
  function expectRun(filename: string, commandId: string) {
    const previous = expectations.get(filename);
    if (previous) forget(filename, previous);
    if (expectations.size >= MAX_TRACKED_FILES) evictOldest();
    expectations.set(filename, { commandId, listeners: new Set() });
  }

  function observeRun(action: ObservedAction): PreviewRunObservation | null {
    if (typeof action?.extensionFile !== 'string') return null;
    const filename = action.extensionFile;
    const expectation = expectations.get(filename);
    if (!expectation || expectation.outcome) return null;
    if (action.id !== expectation.commandId) return null;
    return {
      succeeded: () => settle(filename, expectation, { status: 'ran' }),
      failed: (error) => {
        if (isAbort(error)) return;
        settle(filename, expectation, {
          status: 'failed',
          message: failureMessageFrom(error),
        });
      },
    };
  }

  function outcomeFor(
    filename: string,
    waitMs = defaultWaitMs,
  ): Promise<PreviewRunOutcome> {
    const expectation = expectations.get(filename);
    if (!expectation) return Promise.resolve({ status: 'not-run' });
    if (expectation.outcome) return Promise.resolve(expectation.outcome);
    return new Promise((resolve) => {
      const listener = (outcome: PreviewRunOutcome) => {
        clearTimeout(timer);
        resolve(outcome);
      };
      const timer = setTimeout(() => {
        expectation.listeners.delete(listener);
        resolve({ status: 'not-run' });
      }, waitMs);
      expectation.listeners.add(listener);
    });
  }

  return { expectRun, observeRun, outcomeFor };
}
