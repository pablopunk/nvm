import type { Event } from '@sentry/electron/main';
import { sanitizeDiagnosticEvent } from '../shared/diagnostic-privacy';
import {
  DIAGNOSTIC_OPERATIONS,
  type DiagnosticRecord,
  isDiagnosticId,
} from '../shared/diagnostics';

type Envelope = Parameters<
  ReturnType<
    typeof import('@sentry/electron/main').makeElectronTransport
  >['send']
>[0];
type DiagnosticEventEnvelope = Extract<
  Envelope,
  [{ event_id: string; sent_at: string }, unknown]
>;
type DiagnosticEnvelopeInput = [
  { event_id?: unknown },
  Array<[{ type: string; filename?: string; length?: number }, unknown]>,
];
const STAGES = [
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
const CATEGORIES = new Set(
  DIAGNOSTIC_OPERATIONS.flatMap((operation) =>
    STAGES.map((stage) => `${operation}.${stage}`),
  ),
);

export function diagnosticEnvelope(
  envelope: DiagnosticEnvelopeInput,
  recordForReference: (reference: string) => DiagnosticRecord | undefined,
  release: string,
  environment: string,
): DiagnosticEventEnvelope | undefined {
  const items: DiagnosticEventEnvelope[1] = [];
  for (const [header, payload] of envelope[1]) {
    if (
      (header.type !== 'event' && header.type !== 'transaction') ||
      !payload ||
      typeof payload !== 'object' ||
      payload instanceof Uint8Array
    )
      continue;
    const event = payload as Event;
    const reference = event.tags?.support_ref;
    const record =
      typeof reference === 'string' ? recordForReference(reference) : undefined;
    if (header.type === 'event' && !record) continue;
    if (header.type === 'transaction' && event.type !== 'transaction') continue;
    const safe = sanitizeDiagnosticEvent(event, record);
    safe.release = release;
    safe.environment = environment;
    safe.breadcrumbs = (event.breadcrumbs ?? [])
      .slice(-50)
      .filter(
        (crumb) =>
          typeof crumb.timestamp === 'number' &&
          Number.isFinite(crumb.timestamp) &&
          CATEGORIES.has(crumb.category ?? ''),
      )
      .map((crumb) => ({
        timestamp: crumb.timestamp,
        category: crumb.category,
      }));
    items.push([{ type: header.type }, safe]);
  }
  if (!items.length || Buffer.byteLength(JSON.stringify(items)) > 64 * 1024)
    return;
  const eventId = envelope[0].event_id ?? (items[0]?.[1] as Event).event_id;
  if (!isDiagnosticId(eventId)) return;
  return [
    {
      event_id: eventId,
      sent_at: new Date().toISOString(),
    },
    items,
  ];
}
