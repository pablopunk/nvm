import { defineMiddleware } from 'astro:middleware';
import { waitUntil } from '@vercel/functions';
import { flushLogs } from './lib/log';
import * as Sentry from '@sentry/astro';
import { requestIdFromHeaders } from './lib/compatibility';
import { correlateBackendResponse, safeDiagnosticId } from './lib/diagnostics';

// Axiom batches events in memory. Keep the function alive after the response so
// each request's logs are delivered without adding latency to the response.
export const onRequest = defineMiddleware(async (context, next) => {
  if (!context.url.pathname.startsWith('/api/')) {
    try { return await next(); } finally { waitUntil(flushLogs()); }
  }
  return Sentry.withIsolationScope(async function observeApiRequest(scope) {
  const requestId = requestIdFromHeaders(context.request.headers);
  const actionId = safeDiagnosticId(context.request.headers.get('x-nevermind-action-id'));
  const journeyId = safeDiagnosticId(context.request.headers.get('x-nevermind-journey-id'));
  scope.setTag('request_id', requestId);
  if (actionId) scope.setTag('desktop_action_id', actionId);
  if (journeyId) scope.setTag('desktop_journey_id', journeyId);
  try {
    const response = await next();
    return correlateBackendResponse(response, requestId, actionId, journeyId);
  } finally {
    waitUntil(flushLogs());
  }
  });
});
