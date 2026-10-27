import type { JobDefinition } from './jobs';

const DEFAULT_EXTENSION_BACKOFF_MS = 30_000;
const DEFAULT_EXTENSION_MAX_BACKOFF_MS = 15 * 60_000;

type BackoffRecord = { definition: JobDefinition; consecutiveFailures: number };

export function timeout<T>(
  promise: Promise<T>,
  timeoutMs: number,
  title: string,
) {
  let timer: NodeJS.Timeout | undefined;
  const guard = new Promise<T>((_, reject) => {
    timer = setTimeout(() => {
      const error = new Error(`${title} timed out after ${timeoutMs}ms`);
      error.name = 'TimeoutError';
      reject(error);
    }, timeoutMs);
    timer.unref?.();
  });
  return Promise.race([promise, guard]).finally(() => {
    if (timer) clearTimeout(timer);
  });
}

export function backoffDelay(record: BackoffRecord) {
  if (record.definition.owner !== 'extension' && !record.definition.backoff)
    return 0;
  const initial =
    record.definition.backoff?.initialMs ?? DEFAULT_EXTENSION_BACKOFF_MS;
  const max =
    record.definition.backoff?.maxMs ?? DEFAULT_EXTENSION_MAX_BACKOFF_MS;
  return Math.min(
    max,
    initial * Math.max(1, 2 ** Math.max(0, record.consecutiveFailures - 1)),
  );
}

export function mergeEventPayloads(
  previous?: { event?: string; payload?: unknown },
  next?: { event?: string; payload?: unknown },
) {
  if (!previous) return next || {};
  if (!next) return previous;
  const previousPayload: any = previous.payload;
  const nextPayload: any = next.payload;
  if (
    Array.isArray(previousPayload?.changedPaths) ||
    Array.isArray(nextPayload?.changedPaths)
  ) {
    const changedPaths = [
      ...new Set([
        ...(previousPayload?.changedPaths || []),
        ...(nextPayload?.changedPaths || []),
      ]),
    ].slice(-100);
    return {
      event: next.event || previous.event,
      payload: {
        ...(previousPayload || {}),
        ...(nextPayload || {}),
        changedPaths,
      },
    };
  }
  return next;
}
