import { gunzipSync } from 'node:zlib';

// The desktop app reports through this first-party route instead of straight to
// PostHog: the project key stays server-side, noise and abuse are filtered in
// one place, an emergency kill switch exists, and the vendor can change
// without shipping a new app.

export const MAX_RAW_BYTES = 256 * 1024;
export const MAX_DECODED_BYTES = 1024 * 1024;
export const MAX_BATCH_EVENTS = 100;
const MAX_EVENT_BYTES = 64 * 1024;
const MAX_PROPERTY_KEYS = 80;
const MAX_EVENT_AGE_MS = 7 * 24 * 60 * 60 * 1000;
const MAX_EVENT_SKEW_MS = 5 * 60 * 1000;

export const INGEST_EVENTS: ReadonlySet<string> = new Set([
  'app.launched',
  'feature.used',
  'operation_completed',
  'install.owner_observed',
  'renderer_failure',
  'support_report',
  '$exception',
]);

const INSTALL_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const OWNER_ID = /^[0-9a-f]{64}$/;
const PROPERTY_KEY = /^[$a-z][a-z0-9_.]{0,63}$/i;
// Clients may not set person properties, location, or the project key.
const FORBIDDEN_PROPERTIES = new Set([
  '$set',
  '$set_once',
  '$unset',
  '$ip',
  '$geoip_disable',
  '$process_person_profile',
  '$groups',
  'api_key',
  'token',
]);

export type IngestEvent = {
  event: string;
  distinct_id: string;
  uuid?: string;
  timestamp?: string;
  properties: Record<string, unknown>;
};

export type IngestBody =
  | { ok: true; value: unknown }
  | { ok: false; status: 400 | 413 };

export async function readIngestBody(request: Request): Promise<IngestBody> {
  const declared = Number(request.headers.get('content-length'));
  if (Number.isFinite(declared) && declared > MAX_RAW_BYTES) {
    return { ok: false, status: 413 };
  }
  let raw: Buffer;
  try {
    raw = Buffer.from(await request.arrayBuffer());
  } catch {
    return { ok: false, status: 400 };
  }
  if (raw.length > MAX_RAW_BYTES) return { ok: false, status: 413 };
  try {
    const decoded =
      request.headers.get('content-encoding')?.toLowerCase() === 'gzip'
        ? gunzipSync(raw, { maxOutputLength: MAX_DECODED_BYTES })
        : raw;
    return { ok: true, value: JSON.parse(decoded.toString('utf8')) };
  } catch (error) {
    const tooLarge =
      error instanceof RangeError ||
      (error as { code?: string } | null)?.code === 'ERR_BUFFER_TOO_LARGE';
    return { ok: false, status: tooLarge ? 413 : 400 };
  }
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function sanitizeTimestamp(value: unknown, now: number) {
  if (typeof value !== 'string') return undefined;
  const time = Date.parse(value);
  if (Number.isNaN(time)) return undefined;
  if (time < now - MAX_EVENT_AGE_MS || time > now + MAX_EVENT_SKEW_MS) {
    return undefined;
  }
  return new Date(time).toISOString();
}

function sanitizeProperties(source: unknown) {
  if (!isPlainObject(source)) return null;
  const properties: Record<string, unknown> = {};
  let kept = 0;
  for (const [key, value] of Object.entries(source)) {
    if (FORBIDDEN_PROPERTIES.has(key) || !PROPERTY_KEY.test(key)) continue;
    if (++kept > MAX_PROPERTY_KEYS) break;
    properties[key] = value;
  }
  // Every event is anonymous and location-free, whatever the client claims.
  properties.$process_person_profile = false;
  properties.$geoip_disable = true;
  return properties;
}

function sanitizeEvent(input: unknown, now: number): IngestEvent | null {
  if (!isPlainObject(input)) return null;
  const { event, distinct_id: distinctId, uuid } = input;
  if (typeof event !== 'string' || !INGEST_EVENTS.has(event)) return null;
  if (typeof distinctId !== 'string' || !INSTALL_ID.test(distinctId)) {
    return null;
  }
  const properties = sanitizeProperties(input.properties);
  if (!properties) return null;
  if (
    event === 'install.owner_observed' &&
    !(
      typeof properties.owner_id === 'string' &&
      OWNER_ID.test(properties.owner_id)
    )
  ) {
    return null;
  }
  const timestamp = sanitizeTimestamp(input.timestamp, now);
  const sanitized: IngestEvent = {
    event,
    distinct_id: distinctId,
    ...(typeof uuid === 'string' && INSTALL_ID.test(uuid) ? { uuid } : {}),
    ...(timestamp ? { timestamp } : {}),
    properties,
  };
  return JSON.stringify(sanitized).length > MAX_EVENT_BYTES ? null : sanitized;
}

export function sanitizeBatch(body: unknown, now: number) {
  const items =
    isPlainObject(body) && Array.isArray(body.batch) ? body.batch : null;
  if (!items) return null;
  const events: IngestEvent[] = [];
  for (const item of items.slice(0, MAX_BATCH_EVENTS)) {
    const event = sanitizeEvent(item, now);
    if (event) events.push(event);
  }
  return { events, dropped: items.length - events.length };
}

export function countByEvent(events: ReadonlyArray<IngestEvent>) {
  const counts: Record<string, number> = {};
  for (const { event } of events) counts[event] = (counts[event] ?? 0) + 1;
  return counts;
}

export async function forwardToPosthog(
  events: ReadonlyArray<IngestEvent>,
  options: { key: string; host: string; fetchImpl?: typeof fetch },
) {
  const send = options.fetchImpl ?? fetch;
  const response = await send(`${options.host.replace(/\/+$/, '')}/batch/`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ api_key: options.key, batch: events }),
    signal: AbortSignal.timeout(8_000),
  });
  return response.ok;
}
