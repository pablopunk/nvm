import type { APIRoute } from 'astro';
import { log } from '../../../lib/log';
import { clientIp, rateLimitIp, tooManyRequests } from '../../../lib/ratelimit';
import {
  countByEvent,
  forwardToPosthog,
  readIngestBody,
  sanitizeBatch,
} from '../../../lib/telemetry-ingest';

const DEFAULT_POSTHOG_HOST = 'https://eu.i.posthog.com';
const JSON_HEADERS = { 'Content-Type': 'application/json' };

// PostHog-compatible `/batch/` endpoint for the desktop app's host process.
// Accepted events are forwarded under the server-held project key; everything
// else is dropped. `TELEMETRY_INGEST_ENABLED=false` is the emergency kill switch:
// it answers success so installed apps stop retrying.
export const POST: APIRoute = async ({ request }) => {
  if (process.env.TELEMETRY_INGEST_ENABLED === 'false') {
    return new Response(null, { status: 204 });
  }
  const decision = await rateLimitIp('ingest', clientIp(request), 120, '1 m');
  if (!decision.ok) return tooManyRequests(decision);
  const body = await readIngestBody(request);
  if (!body.ok) return new Response(null, { status: body.status });
  const batch = sanitizeBatch(body.value, Date.now());
  if (!batch) return new Response(null, { status: 400 });
  const key = process.env.POSTHOG_KEY;
  if (batch.events.length > 0 && key) {
    let delivered = false;
    try {
      delivered = await forwardToPosthog(batch.events, {
        key,
        host: process.env.POSTHOG_HOST || DEFAULT_POSTHOG_HOST,
      });
    } catch (error) {
      log.warn('telemetry_ingest_forward_failed', { error });
    }
    // A 503 makes the app keep the batch and retry with backoff.
    if (!delivered) return new Response(null, { status: 503 });
  }
  log.info('telemetry_ingest', {
    accepted: batch.events.length,
    dropped: batch.dropped,
    events: countByEvent(batch.events),
    forwarded: Boolean(key),
  });
  return new Response(JSON.stringify({ status: 1 }), { headers: JSON_HEADERS });
};
