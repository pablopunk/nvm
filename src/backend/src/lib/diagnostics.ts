import * as Sentry from '@sentry/astro';
import { log } from './log';

const OPERATIONS = new Set(['backend.request', 'ai.stream', 'auth.request', 'health.check', 'abuse.check']);
const STAGES = new Set(['request', 'stream', 'invoke', 'complete', 'unknown']);
const SAFE_MESSAGES = new Set(['health_check_failed', 'abuse_credit_spike', 'ai_chain_exhausted']);

export function safeDiagnosticId(value: unknown): string | undefined {
  return typeof value === 'string' && /^[a-f0-9]{32}$/.test(value) ? value : undefined;
}

export function safeRequestId(value: unknown): string | undefined {
  return typeof value === 'string' && /^[a-zA-Z0-9_-]{1,80}$/.test(value) ? value : undefined;
}

export function correlateBackendResponse(response: Response, requestId: string, actionId?: string, journeyId?: string) {
  const finalRequestId = safeRequestId(response.headers.get('x-request-id')) || requestId;
  log.info('diagnostic_response', { request_id: finalRequestId, status: response.status, ...(safeDiagnosticId(actionId) ? { desktop_action_id: actionId } : {}), ...(safeDiagnosticId(journeyId) ? { desktop_journey_id: journeyId } : {}) });
  if (response.headers.has('x-request-id')) return response;
  const headers = new Headers(response.headers);
  headers.set('x-request-id', finalRequestId);
  return new Response(response.body, { status: response.status, statusText: response.statusText, headers });
}

export function sanitizeBackendEvent<T extends Sentry.Event>(event: T): T {
  const operation = OPERATIONS.has(String(event.tags?.operation)) ? String(event.tags?.operation) : 'backend.request';
  const stage = STAGES.has(String(event.tags?.stage)) ? String(event.tags?.stage) : 'request';
  const requestId = safeRequestId(event.tags?.request_id);
  const actionId = safeDiagnosticId(event.tags?.desktop_action_id);
  const journeyId = safeDiagnosticId(event.tags?.desktop_journey_id);
  const trace = event.contexts?.trace;
  const safe: Sentry.Event = {
    platform: 'node',
    event_id: safeDiagnosticId(event.event_id), timestamp: event.timestamp,
    release: event.release, environment: event.environment, level: event.level,
    tags: { operation, stage, ...(requestId ? { request_id: requestId } : {}) },
    contexts: { diagnostic: { ...(requestId ? { request_id: requestId } : {}), ...(actionId ? { desktop_action_id: actionId } : {}), ...(journeyId ? { desktop_journey_id: journeyId } : {}) } },
  };
  if (trace && safeDiagnosticId(trace.trace_id) && typeof trace.span_id === 'string' && /^[a-f0-9]{16}$/.test(trace.span_id)) safe.contexts!.trace = { trace_id: trace.trace_id, span_id: trace.span_id, op: operation };
  if (event.type === 'transaction') {
    safe.type = 'transaction'; safe.transaction = operation; safe.start_timestamp = event.start_timestamp;
    safe.spans = (event.spans ?? []).slice(0, 100).map(span => ({ trace_id: span.trace_id, span_id: span.span_id, parent_span_id: span.parent_span_id, start_timestamp: span.start_timestamp, timestamp: span.timestamp, op: 'backend.request', description: 'backend.request' }));
  } else if (event.exception?.values) {
    safe.exception = { values: event.exception.values.slice(-3).map(exception => ({
      type: ['Error', 'TypeError', 'RangeError', 'SyntaxError', 'ReferenceError'].includes(exception.type ?? '') ? exception.type : 'Error',
      value: `Unexpected failure in ${operation}`,
      stacktrace: { frames: (exception.stacktrace?.frames ?? []).slice(-40).filter(frame => /^file:\/\/\/var\/task\/|^\/var\/task\//.test(frame.filename ?? '') && !/[?#]/.test(frame.filename ?? '')).map(frame => ({ filename: frame.filename, lineno: frame.lineno, colno: frame.colno, in_app: frame.in_app })) },
    })) };
    safe.debug_meta = event.debug_meta ? { images: event.debug_meta.images?.filter(image => image.type === 'sourcemap' && /^file:\/\/\/var\/task\/|^\/var\/task\//.test(image.code_file ?? '')).map(image => ({ type: 'sourcemap' as const, code_file: image.code_file, debug_id: image.debug_id })) } : undefined;
  } else {
    safe.message = SAFE_MESSAGES.has(event.message ?? '') ? event.message : 'Unexpected backend failure';
  }
  return safe as T;
}

export function captureStreamFailure(error: unknown, requestId: string, stage: 'stream' | 'complete' = 'stream') {
  Sentry.withScope(function captureStreamDiagnostic(scope) {
    scope.setTag('operation', 'ai.stream'); scope.setTag('stage', stage);
    if (safeRequestId(requestId)) scope.setTag('request_id', requestId);
    Sentry.captureException(error);
  });
}
