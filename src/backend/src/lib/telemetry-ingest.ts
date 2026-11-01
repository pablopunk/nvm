import { gunzipSync } from 'node:zlib';

// The desktop app reports through this first-party route instead of straight to
// PostHog: the project key stays server-side, noise and abuse are filtered in
// one place, an emergency kill switch exists, and the vendor can change
// without shipping a new app.

export const MAX_RAW_BYTES = 256 * 1024;
export const MAX_DECODED_BYTES = 1024 * 1024;
export const MAX_BATCH_EVENTS = 100;
const MAX_EVENT_BYTES = 64 * 1024;
const MAX_EXCEPTION_TEXT = 2_000;
const MAX_EXCEPTION_FRAMES = 50;
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
  '$pageview',
]);

const INSTALL_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const SESSION_ID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
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

// Desktop pageviews describe a host-defined category, never what was on screen.
const APP_VIEWS: ReadonlySet<unknown> = new Set(['palette', 'extension', 'ai-chat']);
const APP_VIEW_HOST = 'app.nevermind.invalid';
// Everything else a pageview may carry comes from the host's common properties.
const PAGEVIEW_CONTEXT_KEYS = [
  'app_version',
  'os_type',
  'os_release',
  'arch',
  'channel',
] as const;
const MAX_CONTEXT_VALUE_LENGTH = 64;

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

// Reads the body incrementally and stops as soon as the cap is crossed, so a
// request with a missing or false Content-Length cannot be buffered whole.
async function readCapped(
  request: Request,
  limit: number,
): Promise<{ ok: true; value: Buffer } | { ok: false; status: 400 | 413 }> {
  if (!request.body) return { ok: true, value: Buffer.alloc(0) };
  const reader = request.body.getReader();
  const chunks: Buffer[] = [];
  let total = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      total += value.byteLength;
      if (total > limit) {
        await reader.cancel();
        return { ok: false, status: 413 };
      }
      chunks.push(Buffer.from(value));
    }
  } catch {
    return { ok: false, status: 400 };
  }
  return { ok: true, value: Buffer.concat(chunks) };
}

export async function readIngestBody(request: Request): Promise<IngestBody> {
  const declared = Number(request.headers.get('content-length'));
  if (Number.isFinite(declared) && declared > MAX_RAW_BYTES) {
    return { ok: false, status: 413 };
  }
  const read = await readCapped(request, MAX_RAW_BYTES);
  if (!read.ok) return read;
  const raw = read.value;
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
  // Every event on this route comes from the desktop app, so website and
  // desktop data can be told apart even for clients that predate this field.
  properties.source = 'desktop';
  if (!isSessionId(properties.$session_id)) delete properties.$session_id;
  return properties;
}

function isSessionId(value: unknown): value is string {
  return typeof value === 'string' && SESSION_ID.test(value);
}

// A pageview is rebuilt from validated parts instead of filtered, so nothing
// the client adds besides the host's own context can ride along.
function pageviewProperties(properties: Record<string, unknown>) {
  const { view, $session_id: sessionId } = properties;
  if (!(APP_VIEWS.has(view) && isSessionId(sessionId))) return null;
  const rebuilt: Record<string, unknown> = {
    $process_person_profile: false,
    $geoip_disable: true,
    source: 'desktop',
    view,
    $session_id: sessionId,
    $current_url: `https://${APP_VIEW_HOST}/${view}`,
    $host: APP_VIEW_HOST,
    $pathname: `/${view}`,
  };
  for (const key of PAGEVIEW_CONTEXT_KEYS) {
    const value = properties[key];
    if (typeof value === 'string' && value.length <= MAX_CONTEXT_VALUE_LENGTH) {
      rebuilt[key] = value;
    }
  }
  return rebuilt;
}

// A long error message or stack must not cost the whole event: keep the head of
// each message and the frames nearest the throw, which is what debugging needs.
function truncateException(properties: Record<string, unknown>) {
  const list = properties.$exception_list;
  if (!Array.isArray(list)) return;
  properties.$exception_list = list.slice(0, 5).map((entry) => {
    if (!isPlainObject(entry)) return entry;
    const trimmed: Record<string, unknown> = { ...entry };
    if (typeof trimmed.value === 'string') {
      trimmed.value = trimmed.value.slice(0, MAX_EXCEPTION_TEXT);
    }
    const stack = trimmed.stacktrace;
    if (isPlainObject(stack) && Array.isArray(stack.frames)) {
      trimmed.stacktrace = {
        ...stack,
        frames: stack.frames.slice(-MAX_EXCEPTION_FRAMES),
      };
    }
    return trimmed;
  });
}

function sanitizeEvent(input: unknown, now: number): IngestEvent | null {
  if (!isPlainObject(input)) return null;
  const { event, distinct_id: distinctId, uuid } = input;
  if (typeof event !== 'string' || !INGEST_EVENTS.has(event)) return null;
  if (typeof distinctId !== 'string' || !INSTALL_ID.test(distinctId)) {
    return null;
  }
  const sanitizedProperties = sanitizeProperties(input.properties);
  if (!sanitizedProperties) return null;
  const properties =
    event === '$pageview'
      ? pageviewProperties(sanitizedProperties)
      : sanitizedProperties;
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
  if (JSON.stringify(sanitized).length <= MAX_EVENT_BYTES) return sanitized;
  if (event !== '$exception') return null;
  truncateException(sanitized.properties);
  return JSON.stringify(sanitized).length <= MAX_EVENT_BYTES ? sanitized : null;
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
