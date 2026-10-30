import { defineMiddleware } from 'astro:middleware';
import { waitUntil } from '@vercel/functions';
import { flushLogs } from './lib/log';
import { requestIdFromHeaders } from './lib/compatibility';
import { correlateBackendResponse, safeDiagnosticId } from './lib/diagnostics';
import {
  captureBackendException,
  captureBackendRequest,
  flushErrorTracking,
  runWithRequestContext,
} from './lib/error-tracking';

// Axiom and PostHog batch events in memory. Keep the function alive after the
// response so each request's logs and analytics are delivered without adding
// latency to the response.
function flushTelemetryAfterResponse() {
  waitUntil(Promise.all([flushLogs(), flushErrorTracking()]));
}

export const onRequest = defineMiddleware(async (context, next) => {
  if (!context.url.pathname.startsWith('/api/')) {
    try { return await next(); } finally { flushTelemetryAfterResponse(); }
  }
  const requestId = requestIdFromHeaders(context.request.headers);
  const actionId = safeDiagnosticId(context.request.headers.get('x-nevermind-action-id'));
  const journeyId = safeDiagnosticId(context.request.headers.get('x-nevermind-journey-id'));
  return runWithRequestContext({
    request_id: requestId,
    desktop_action_id: actionId,
    desktop_journey_id: journeyId,
  }, async function observeApiRequest() {
    const startedAt = Date.now();
    let status = 500;
    try {
      const response = await next();
      status = response.status;
      return correlateBackendResponse(response, requestId, actionId, journeyId);
    } catch (error) {
      captureBackendException(error, { operation: 'backend.request' });
      throw error;
    } finally {
      captureBackendRequest({
        route: context.routePattern,
        method: context.request.method,
        status,
        durationMs: Date.now() - startedAt,
      });
      flushTelemetryAfterResponse();
    }
  });
});
