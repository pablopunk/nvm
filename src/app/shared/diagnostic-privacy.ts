import type { Event, StackFrame } from '@sentry/electron/main';
import {
  type DiagnosticRecord,
  type DiagnosticComponent,
  diagnosticOperation,
  DIAGNOSTIC_COMPONENTS,
  isDiagnosticId,
  isRequestId,
} from './diagnostics';

const ERROR_NAMES = new Set([
  'Error',
  'TypeError',
  'RangeError',
  'SyntaxError',
  'ReferenceError',
  'AbortError',
  'TimeoutError',
]);
export const MAIN_DIAGNOSTIC_INTEGRATIONS = new Set([
  'EventFilters',
  'FunctionToString',
  'OnUncaughtException',
  'OnUnhandledRejection',
  'PreloadInjection',
  'NormalizePaths',
]);
export const RENDERER_DIAGNOSTIC_INTEGRATIONS = new Set([
  'EventFilters',
  'FunctionToString',
  'GlobalHandlers',
  'BrowserApiErrors',
  'DebugIds',
]);

function safeFrame(frame: StackFrame): StackFrame | undefined {
  const filename = frame.filename?.replace(/\\/g, '/');
  const match = filename?.match(
    /(?:app:\/\/\/|\/)(dist\/(?:main|preload|renderer)\/[a-zA-Z0-9/_.-]+\.(?:js|cjs|mjs))$/,
  );
  if (!match || match[1]?.includes('..')) return;
  return {
    filename: `app:///${match[1]}`,
    in_app: true,
    ...(Number.isSafeInteger(frame.lineno) ? { lineno: frame.lineno } : {}),
    ...(Number.isSafeInteger(frame.colno) ? { colno: frame.colno } : {}),
  };
}

export function sanitizeDiagnosticEvent<T extends Event>(
  event: T,
  record?: DiagnosticRecord,
): T {
  const trace = event.contexts?.trace;
  const safe: Event = {
    platform: record?.process === 'renderer' ? 'javascript' : 'node',
    ...(isDiagnosticId(event.event_id) ? { event_id: event.event_id } : {}),
    ...(typeof event.timestamp === 'number' && Number.isFinite(event.timestamp)
      ? { timestamp: event.timestamp }
      : {}),
    level:
      record?.operation === 'support.report' || event.type === 'transaction'
        ? 'info'
        : event.level === 'fatal'
          ? 'fatal'
          : 'error',
    contexts: {},
    tags: {},
  };
  if (record) {
    safe.contexts = { diagnostic: { ...record } };
    safe.tags = {
      support_ref: record.reference,
      operation: record.operation,
      stage: record.stage,
      outcome: record.outcome,
      process: record.process,
      ...(isDiagnosticId(record.journeyId)
        ? { journey_id: record.journeyId }
        : {}),
      ...(isDiagnosticId(record.actionId)
        ? { action_id: record.actionId }
        : {}),
      ...(record.component ? { component: record.component } : {}),
    };
  }
  if (
    trace &&
    isDiagnosticId(trace.trace_id) &&
    typeof trace.span_id === 'string' &&
    /^[a-f0-9]{16}$/.test(trace.span_id)
  ) {
    safe.contexts!.trace = {
      trace_id: trace.trace_id,
      span_id: trace.span_id,
      op: diagnosticOperation(trace.op),
      ...(typeof trace.status === 'string' &&
      [
        'ok',
        'internal_error',
        'cancelled',
        'deadline_exceeded',
        'permission_denied',
      ].includes(trace.status)
        ? { status: trace.status }
        : {}),
      data: safeSpanData(trace.data),
    };
  }
  if (event.type === 'transaction') {
    const data = safeSpanData(trace?.data);
    safe.tags = {
      operation: diagnosticOperation(event.transaction),
      ...(data['diagnostic.boot_id']
        ? { boot_id: data['diagnostic.boot_id'] }
        : {}),
      ...(data['diagnostic.journey_id']
        ? { journey_id: data['diagnostic.journey_id'] }
        : {}),
      ...(data['diagnostic.action_id']
        ? { action_id: data['diagnostic.action_id'] }
        : {}),
      ...(data['diagnostic.outcome']
        ? { outcome: data['diagnostic.outcome'] }
        : {}),
      ...(data['diagnostic.stage'] ? { stage: data['diagnostic.stage'] } : {}),
      ...(data['diagnostic.component']
        ? { component: data['diagnostic.component'] }
        : {}),
    };
    safe.type = 'transaction';
    safe.transaction = diagnosticOperation(event.transaction);
    safe.start_timestamp =
      typeof event.start_timestamp === 'number'
        ? event.start_timestamp
        : undefined;
    safe.spans = (event.spans ?? [])
      .slice(0, 100)
      .filter(
        (span) =>
          isDiagnosticId(span.trace_id) && /^[a-f0-9]{16}$/.test(span.span_id),
      )
      .map((span) => ({
        trace_id: span.trace_id,
        span_id: span.span_id,
        ...(span.parent_span_id && /^[a-f0-9]{16}$/.test(span.parent_span_id)
          ? { parent_span_id: span.parent_span_id }
          : {}),
        op: diagnosticOperation(span.op),
        description: diagnosticOperation(span.description),
        start_timestamp: span.start_timestamp,
        timestamp: span.timestamp,
        data: safeSpanData(span.data),
      }));
  } else if (event.exception?.values?.length) {
    safe.exception = {
      values: event.exception.values.slice(-3).map((exception) => ({
        type: ERROR_NAMES.has(exception.type ?? '') ? exception.type : 'Error',
        value: `Unexpected failure in ${record?.operation ?? 'process.unhandled'}`,
        ...(exception.stacktrace
          ? {
              stacktrace: {
                frames: (exception.stacktrace.frames ?? [])
                  .slice(-40)
                  .map(safeFrame)
                  .filter(Boolean) as StackFrame[],
              },
            }
          : {}),
        ...(exception.mechanism
          ? {
              mechanism: {
                type: 'generic',
                handled: exception.mechanism.handled !== false,
              },
            }
          : {}),
      })),
    };
  } else {
    safe.message =
      record?.operation === 'support.report'
        ? 'User requested diagnostics'
        : 'Unexpected operation failure';
  }
  const images = (event.debug_meta?.images ?? [])
    .slice(0, 100)
    .filter(
      (image) =>
        image.type === 'sourcemap' &&
        isDiagnosticId(image.debug_id?.replaceAll('-', '')) &&
        safeFrame({ filename: image.code_file }),
    )
    .map((image) => ({
      type: 'sourcemap' as const,
      debug_id: image.debug_id,
      code_file: safeFrame({ filename: image.code_file })!.filename!,
    }));
  if (images.length) safe.debug_meta = { images };
  return safe as T;
}

function safeSpanData(data: unknown): Record<string, string> {
  if (!data || typeof data !== 'object') return {};
  const source = data as Record<string, unknown>;
  const outcomes = [
    'success',
    'failed',
    'cancelled',
    'blocked',
    'timed_out',
    'interrupted',
    'unknown',
  ];
  const stages = [
    'dispatch',
    'invoke',
    'load',
    'render',
    'permission',
    'request',
    'stream',
    'validate',
    'save',
    'install',
    'complete',
    'unknown',
  ];
  return {
    ...Object.fromEntries(
      [
        'diagnostic.boot_id',
        'diagnostic.journey_id',
        'diagnostic.action_id',
        'diagnostic.target_id',
      ]
        .filter((key) => isDiagnosticId(source[key]))
        .map((key) => [key, String(source[key])]),
    ),
    ...(isRequestId(source['diagnostic.request_id'])
      ? { 'diagnostic.request_id': String(source['diagnostic.request_id']) }
      : {}),
    ...(DIAGNOSTIC_COMPONENTS.includes(
      source['diagnostic.component'] as DiagnosticComponent,
    )
      ? { 'diagnostic.component': String(source['diagnostic.component']) }
      : {}),
    ...(outcomes.includes(String(source['diagnostic.outcome']))
      ? { 'diagnostic.outcome': String(source['diagnostic.outcome']) }
      : {}),
    ...(stages.includes(String(source['diagnostic.stage']))
      ? { 'diagnostic.stage': String(source['diagnostic.stage']) }
      : {}),
  };
}
