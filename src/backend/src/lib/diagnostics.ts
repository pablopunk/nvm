import { waitUntil } from '@vercel/functions';
import { captureBackendException, flushErrorTracking } from './error-tracking';
import { log } from './log';

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

export function captureStreamFailure(error: unknown, requestId: string, stage: 'stream' | 'complete' = 'stream') {
  captureBackendException(error, {
    operation: 'ai.stream',
    stage,
    ...(safeRequestId(requestId) ? { request_id: requestId } : {}),
  });
  // Stream failures surface after the response already returned, so the
  // middleware's flush has run; deliver this capture on its own.
  waitUntil(flushErrorTracking());
}
