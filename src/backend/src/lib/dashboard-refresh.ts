import { Effect, Fiber, Schedule } from 'effect';

/** Refresh only while visible, and never overlap a previous refresh. */
export function watchDashboard(refresh: () => Promise<unknown>, intervalMs = 60_000): () => void {
  let running = false;
  let lastStarted = Date.now();
  let stopped = false;
  async function tick() {
    if (stopped || running || document.visibilityState !== 'visible' || Date.now() - lastStarted < intervalMs) return;
    running = true;
    lastStarted = Date.now();
    try { await refresh(); } catch { /* The next scheduled refresh can recover. */ } finally { running = false; }
  }
  const fiber = Effect.runFork(Effect.sleep(intervalMs).pipe(
    Effect.andThen(Effect.promise(tick)),
    Effect.repeat(Schedule.forever),
  ));
  function onVisibilityChange() { void tick(); }
  document.addEventListener('visibilitychange', onVisibilityChange);
  return () => {
    stopped = true;
    Effect.runFork(Fiber.interrupt(fiber));
    document.removeEventListener('visibilitychange', onVisibilityChange);
  };
}
