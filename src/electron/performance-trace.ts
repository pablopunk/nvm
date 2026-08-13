import { AsyncLocalStorage } from 'node:async_hooks';
import crypto from 'node:crypto';
import { performance } from 'node:perf_hooks';

export type PerformanceTraceStatus = 'ok' | 'error' | 'cancelled' | 'timeout';

export type PerformanceTraceAttributes = Record<string, unknown>;

export type PerformanceTraceContext = {
  traceId: string;
  parentSpanId?: string;
};

export type PerformanceTraceLog = {
  traceId: string;
  spanId: string;
  parentSpanId?: string;
  operation: string;
  durationMs: number;
  status: PerformanceTraceStatus;
  attributes: Record<string, unknown>;
};

export type PerformanceTraceEvent = {
  traceId: unknown;
  operation: unknown;
  durationMs: unknown;
  status?: unknown;
  [key: string]: unknown;
};

type PerformanceTraceSpan = PerformanceTraceContext & {
  spanId: string;
  operation: string;
  attributes: Record<string, unknown>;
  startedAt: number;
};

type ActivePerformanceTraceContext = {
  traceId: string;
  parentSpanId: string;
};

type PerformanceTraceServiceOptions = {
  log: (entry: PerformanceTraceLog) => void;
  enabled?: () => boolean;
};

const SAFE_ATTRIBUTE_KEYS = new Set([
  'actionKind',
  'actionType',
  'background',
  'cache',
  'complete',
  'errorType',
  'eventType',
  'extensionId',
  'generation',
  'itemCount',
  'messageCount',
  'messageLength',
  'mode',
  'navigation',
  'phase',
  'providerCount',
  'queryLength',
  'reason',
  'resultCount',
  'revision',
  'stale',
  'status',
  'viewId',
  'viewType',
  'windowType',
  'jobId',
  'jobOwner',
  'commandId',
  'actionToken',
  'payloadBytes',
  'queueMs',
  'deltaLength',
  'dismissedImmediately',
  'showsLoading',
]);
const IDENTIFIER_ATTRIBUTE_KEYS = new Set([
  'actionToken',
  'commandId',
  'extensionId',
  'jobId',
  'viewId',
]);
const activeTraceContext =
  new AsyncLocalStorage<ActivePerformanceTraceContext>();
const MAX_RECORDED_EVENTS_PER_SECOND = 1000;

function newId() {
  return crypto.randomUUID();
}

function safeTraceId(traceId: string) {
  return traceId.replace(/[^a-zA-Z0-9._:-]/g, '_').slice(0, 120);
}

function normalizedParentSpanId(value: unknown) {
  return typeof value === 'string' && value ? safeTraceId(value) : undefined;
}

function numberAttribute(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value)
    ? value
    : undefined;
}

function sanitizeAttributeValue(key: string, value: unknown): unknown {
  if (value == null || typeof value === 'boolean') return value;
  if (typeof value === 'number') return numberAttribute(value);
  if (typeof value === 'string' && IDENTIFIER_ATTRIBUTE_KEYS.has(key))
    return crypto.createHash('sha256').update(value).digest('hex').slice(0, 16);
  if (typeof value === 'string')
    return value.length > 80 ? value.slice(0, 80) : value;
  if (Array.isArray(value)) return { type: 'array', length: value.length };
  return { type: 'object' };
}

function safeOperationName(operation: string) {
  return operation.replace(/[^a-zA-Z0-9._:-]/g, '_').slice(0, 120);
}

function safeRecordedOperationName(operation: string) {
  const safe = safeOperationName(operation);
  const allowedPrefixes = [
    'ai.',
    'extension.',
    'interaction.',
    'ipc.',
    'os.',
    'search.',
    'shortcut.',
    'view.',
  ];
  return allowedPrefixes.some((prefix) => safe.startsWith(prefix))
    ? safe
    : 'renderer.custom';
}

function sanitizeAttributes(attributes: PerformanceTraceAttributes = {}) {
  const safe: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(attributes)) {
    if (!SAFE_ATTRIBUTE_KEYS.has(key)) continue;
    const sanitized = sanitizeAttributeValue(key, value);
    if (sanitized !== undefined) safe[key] = sanitized;
  }
  return safe;
}

function statusForError(error: unknown): PerformanceTraceStatus {
  const name = error instanceof Error ? error.name : '';
  if (name === 'AbortError' || name === 'AbortErrorException')
    return 'cancelled';
  if (name === 'TimeoutError' || name === 'PromiseTimeoutError')
    return 'timeout';
  return 'error';
}

function statusFromAttributes(attributes: PerformanceTraceAttributes) {
  if (attributes.status === 'error') return 'error' as const;
  if (attributes.status === 'cancelled') return 'cancelled' as const;
  if (attributes.status === 'timeout') return 'timeout' as const;
  return 'ok' as const;
}

function errorAttributes(error: unknown) {
  return {
    errorType: error instanceof Error ? error.name : typeof error,
  };
}

export function createPerformanceTraceService({
  log,
  enabled = () => true,
}: PerformanceTraceServiceOptions) {
  let recordedEventWindowStartedAt = 0;
  let recordedEventCount = 0;

  function canRecordEvent() {
    const now = Date.now();
    if (now - recordedEventWindowStartedAt >= 1000) {
      recordedEventWindowStartedAt = now;
      recordedEventCount = 0;
    }
    if (recordedEventCount >= MAX_RECORDED_EVENTS_PER_SECOND) return false;
    recordedEventCount += 1;
    return true;
  }

  function start(
    operation: string,
    attributes: PerformanceTraceAttributes = {},
    context?: PerformanceTraceContext,
  ): PerformanceTraceSpan {
    const ambient = activeTraceContext.getStore();
    const explicitTraceId = context?.traceId
      ? safeTraceId(context.traceId)
      : undefined;
    const ambientParent =
      !explicitTraceId || explicitTraceId === ambient?.traceId ? ambient : undefined;
    return {
      traceId: explicitTraceId || ambient?.traceId || newId(),
      spanId: newId(),
      parentSpanId: normalizedParentSpanId(
        context?.parentSpanId || ambientParent?.parentSpanId,
      ),
      operation: safeOperationName(operation),
      attributes: sanitizeAttributes(attributes),
      startedAt: performance.now(),
    };
  }

  function finish(
    span: PerformanceTraceSpan,
    status: PerformanceTraceStatus = 'ok',
    attributes: PerformanceTraceAttributes = {},
  ) {
    if (!enabled()) return;
    writeTrace({
      traceId: span.traceId,
      spanId: span.spanId,
      ...(span.parentSpanId ? { parentSpanId: span.parentSpanId } : {}),
      operation: span.operation,
      durationMs: Math.round((performance.now() - span.startedAt) * 100) / 100,
      status,
      attributes: { ...span.attributes, ...sanitizeAttributes(attributes) },
    });
  }

  function record(event: unknown) {
    if (Array.isArray(event)) {
      for (const item of event.slice(0, 100)) record(item);
      return;
    }
    if (!enabled() || !event || typeof event !== 'object') return;
    if (!canRecordEvent()) return;
    const traceEvent = event as PerformanceTraceEvent;
    if (
      typeof traceEvent.traceId !== 'string' ||
      !traceEvent.traceId ||
      typeof traceEvent.operation !== 'string' ||
      typeof traceEvent.durationMs !== 'number' ||
      !Number.isFinite(traceEvent.durationMs)
    )
      return;
    const { traceId, operation, durationMs, status, ...attributes } = traceEvent;
    const ambient = activeTraceContext.getStore();
    const normalizedTraceId = safeTraceId(traceId);
    const normalizedStatus: PerformanceTraceStatus =
      status === 'error' || status === 'cancelled' || status === 'timeout'
        ? status
        : 'ok';
    writeTrace({
      traceId: normalizedTraceId,
      spanId: newId(),
      ...(ambient?.traceId === normalizedTraceId
        ? { parentSpanId: ambient.parentSpanId }
        : {}),
      operation: safeRecordedOperationName(operation),
      durationMs: Math.max(0, Math.round(durationMs * 100) / 100),
      status: normalizedStatus,
      attributes: sanitizeAttributes(attributes),
    });
  }

  function writeTrace(entry: PerformanceTraceLog) {
    try {
      log(entry);
    } catch {}
  }

  function run<T>(
    operation: string,
    attributes: PerformanceTraceAttributes,
    task: () => T | Promise<T>,
    context?: PerformanceTraceContext,
  ): T | Promise<T> {
    if (!enabled()) return task();
    const span = start(operation, attributes, context);
    try {
      const result = activeTraceContext.run(
        { traceId: span.traceId, parentSpanId: span.spanId },
        task,
      );
      if (result && typeof (result as Promise<T>).then === 'function') {
        return Promise.resolve(result).then(
          (value) => {
            finish(span);
            return value;
          },
          (error) => {
            finish(span, statusForError(error), errorAttributes(error));
            throw error;
          },
        ) as Promise<T>;
      }
      finish(span);
      return result;
    } catch (error) {
      finish(span, statusForError(error), errorAttributes(error));
      throw error;
    }
  }

  function event(
    operation: string,
    attributes: PerformanceTraceAttributes = {},
    context?: PerformanceTraceContext,
  ) {
    if (!enabled()) return;
    const span = start(operation, attributes, context);
    finish(span, statusFromAttributes(attributes));
  }

  return { event, finish, record, run, start };
}

export type PerformanceTraceService = ReturnType<
  typeof createPerformanceTraceService
>;
