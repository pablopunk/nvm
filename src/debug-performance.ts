type DebugPerformanceDetail = Record<string, unknown> | undefined;

const DEBUG_PERFORMANCE_STORAGE_KEY = 'nvm.debugPerformance';
const DEFAULT_SLOW_LOG_THRESHOLD_MS = 8;
let sequence = 0;

export function debugPerformanceEnabled() {
  if (typeof window === 'undefined') return false;
  const override = window.localStorage?.getItem(DEBUG_PERFORMANCE_STORAGE_KEY);
  if (override === '0' || override === 'false') return false;
  if (override === '1' || override === 'true') return true;
  return Boolean(
    (import.meta as ImportMeta & { env?: { DEV?: boolean } }).env?.DEV,
  );
}

export function markDebugPerformance(
  name: string,
  detail?: DebugPerformanceDetail,
) {
  if (!debugPerformanceEnabled()) return '';
  const markName = `nvm:${name}`;
  markPerformance(markName, detail);
  return markName;
}

export async function measureDebugPerformance<T>(
  name: string,
  detail: DebugPerformanceDetail,
  task: () => Promise<T>,
) {
  if (!debugPerformanceEnabled()) return task();
  const measurement = startDebugPerformanceMeasure(name, detail);
  try {
    return await task();
  } finally {
    finishDebugPerformanceMeasure(measurement);
  }
}

export function measureDebugPerformanceSync<T>(
  name: string,
  detail: DebugPerformanceDetail,
  task: () => T,
) {
  if (!debugPerformanceEnabled()) return task();
  const measurement = startDebugPerformanceMeasure(name, detail);
  try {
    return task();
  } finally {
    finishDebugPerformanceMeasure(measurement);
  }
}

export function recordDebugPerformance(
  name: string,
  durationMs: number,
  detail?: DebugPerformanceDetail,
) {
  if (!debugPerformanceEnabled()) return;
  try {
    performance.measure(`nvm:${name}`, {
      start: Math.max(0, performance.now() - durationMs),
      duration: Math.max(0, durationMs),
      detail,
    });
  } catch {}
  logSlowDebugPerformance(name, durationMs, detail);
}

function startDebugPerformanceMeasure(
  name: string,
  detail?: DebugPerformanceDetail,
) {
  const id = `${name}:${++sequence}`;
  const startMark = `nvm:${id}:start`;
  markPerformance(startMark, detail);
  return { name, detail, startMark, startedAt: performance.now() };
}

function finishDebugPerformanceMeasure(measurement: {
  name: string;
  detail?: DebugPerformanceDetail;
  startMark: string;
  startedAt: number;
}) {
  const endMark = `${measurement.startMark}:end`;
  const durationMs = performance.now() - measurement.startedAt;
  markPerformance(endMark, measurement.detail);
  try {
    performance.measure(
      `nvm:${measurement.name}`,
      measurement.startMark,
      endMark,
    );
  } catch {}
  performance.clearMarks(measurement.startMark);
  performance.clearMarks(endMark);
  logSlowDebugPerformance(measurement.name, durationMs, measurement.detail);
}

function markPerformance(name: string, detail?: DebugPerformanceDetail) {
  try {
    performance.mark(name, detail === undefined ? undefined : { detail });
  } catch {
    try {
      performance.mark(name);
    } catch {}
  }
}

function logSlowDebugPerformance(
  name: string,
  durationMs: number,
  detail?: DebugPerformanceDetail,
) {
  const thresholdMs = Number(
    window.localStorage?.getItem('nvm.debugPerformance.slowMs') ||
      DEFAULT_SLOW_LOG_THRESHOLD_MS,
  );
  if (durationMs < thresholdMs && !detail?.alwaysLog) return;
  queueMicrotask(() => {
    window.nvm
      ?.log?.('debug', 'performance.measure', {
        name,
        durationMs: Math.round(durationMs * 100) / 100,
        ...detail,
      })
      .catch(() => {});
  });
}
