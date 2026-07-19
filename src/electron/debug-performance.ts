// biome-ignore-all lint: Performance instrumentation intentionally uses best-effort platform APIs and compact hot-path guards.
import { performance } from 'node:perf_hooks';
import { debug as logDebug } from './logger';

export type DebugPerformanceDetail = Record<string, unknown> | undefined;

const DEFAULT_SLOW_LOG_THRESHOLD_MS = 8;
let sequence = 0;

export function debugPerformanceEnabled() {
  if (
    process.env.NVM_DEBUG_PERFORMANCE === '0' ||
    process.env.NVM_DEBUG_PERFORMANCE === 'false'
  )
    return false;
  return Boolean(
    process.env.NVM_DEBUG_PERFORMANCE ||
      process.env.NVM_PALETTE_DEBUG ||
      process.env.ELECTRON_RENDERER_URL,
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
  task: () => Promise<T> | T,
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

export function summarizeDebugValue(value: unknown): unknown {
  if (value == null) return value;
  if (typeof value === 'string')
    return value.length > 80
      ? { type: 'string', length: value.length, preview: value.slice(0, 80) }
      : value;
  if (typeof value === 'number' || typeof value === 'boolean') return value;
  if (Array.isArray(value)) return { type: 'array', length: value.length };
  if (typeof value === 'object') {
    const record = value as Record<string, unknown>;
    const summary: Record<string, unknown> = { type: 'object' };
    for (const key of [
      'id',
      'kind',
      'type',
      'title',
      'extensionId',
      'commandId',
      'handlerId',
      'viewId',
    ]) {
      if (record[key] != null) summary[key] = record[key];
    }
    return summary;
  }
  return typeof value;
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
    process.env.NVM_DEBUG_PERFORMANCE_SLOW_MS || DEFAULT_SLOW_LOG_THRESHOLD_MS,
  );
  if (durationMs < thresholdMs && !detail?.alwaysLog) return;
  logDebug(
    'performance.measure',
    {
      name,
      durationMs: Math.round(durationMs * 100) / 100,
      ...detail,
    },
    { source: 'host', scope: 'performance' },
  );
}
