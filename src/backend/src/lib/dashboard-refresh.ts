import { Effect, Fiber, Schedule } from 'effect';

export function createDashboardRefreshGate(
  refresh: () => Promise<unknown>,
  intervalMs: number,
  now: () => number,
  isVisible: () => boolean,
) {
  let running = false;
  let lastStarted = now();
  let stopped = false;
  return {
    async tick() {
      if (stopped || running || !isVisible() || now() - lastStarted < intervalMs) return;
      running = true;
      lastStarted = now();
      try { await refresh(); } catch { /* The next scheduled refresh can recover. */ } finally { running = false; }
    },
    stop() { stopped = true; },
  };
}

/** Refresh only while visible, and never overlap a previous refresh. */
export function watchDashboard(refresh: () => Promise<unknown>, intervalMs = 60_000): () => void {
  const gate = createDashboardRefreshGate(refresh, intervalMs, Date.now, () => document.visibilityState === 'visible');
  const fiber = Effect.runFork(Effect.sleep(intervalMs).pipe(
    Effect.andThen(Effect.promise(() => gate.tick())),
    Effect.repeat(Schedule.forever),
  ));
  function onVisibilityChange() { void gate.tick(); }
  document.addEventListener('visibilitychange', onVisibilityChange);
  return () => {
    gate.stop();
    Effect.runFork(Fiber.interrupt(fiber));
    document.removeEventListener('visibilitychange', onVisibilityChange);
  };
}
