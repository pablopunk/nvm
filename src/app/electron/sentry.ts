import { createRequire } from 'node:module';
import path from 'node:path';
import { app } from 'electron';
import type { DiagnosticRecord } from '../shared/diagnostics';
import { isDiagnosticId } from '../shared/diagnostics';
import {
  MAIN_DIAGNOSTIC_INTEGRATIONS,
  sanitizeDiagnosticEvent,
} from '../shared/diagnostic-privacy';
import {
  createDiagnosticStore,
  DIAGNOSTIC_FILE_NAME,
} from './diagnostic-store';
import {
  configureObservability,
  currentOperation,
  failureRecord,
  recordOperationFailure,
} from './observability';

type SentryMain = typeof import('@sentry/electron/main');

const requireSentryModule = createRequire(import.meta.url);
let initialized = false;
let sentry: SentryMain | undefined;
let didTryLoadSentry = false;
let store: ReturnType<typeof createDiagnosticStore> | undefined;
const pendingFailures = new Map<string, DiagnosticRecord>();
declare const __NEVERMIND_BUILD__: string;

export function recentDiagnosticRecords() {
  return store?.recent() ?? [];
}

function rememberFailure(record: DiagnosticRecord) {
  pendingFailures.set(record.reference, record);
  while (pendingFailures.size > 100)
    pendingFailures.delete(pendingFailures.keys().next().value!);
  store?.put(record);
}

export function rendererFailureHandle(windowId: number) {
  const record = {
    ...failureRecord('view.render', 'render'),
    process: 'renderer' as const,
    windowId,
  };
  rememberFailure(record);
  return record.reference;
}

function prepareEvent(event: import('@sentry/electron/main').Event) {
  const renderer = /^renderer\.(\d+)$/.exec(
    String(event.tags?.['event.process'] ?? ''),
  );
  const handle = event.contexts?.diagnostic?.failure_handle;
  const pending =
    typeof handle === 'string' ? pendingFailures.get(handle) : undefined;
  const record =
    pending &&
    (renderer
      ? pending.process === 'renderer' &&
        pending.windowId === Number(renderer[1])
      : pending.process === 'main')
      ? pending
      : {
          ...failureRecord(),
          ...(renderer
            ? {
                process: 'renderer' as const,
                windowId: Number(renderer[1]),
                operation: 'view.render' as const,
              }
            : {}),
        };
  if (isDiagnosticId(event.event_id)) record.eventId = event.event_id;
  record.reporting = 'capture_requested';
  record.projectId = sentry?.getClient()?.getDsn()?.projectId;
  rememberFailure(record);
  const safe = sanitizeDiagnosticEvent(event, record);
  safe.release = buildIdentity();
  safe.environment = app.isPackaged ? 'production' : 'development';
  safe.breadcrumbs = (currentOperation()?.breadcrumbs ?? []).map((crumb) => ({
    timestamp: crumb.timestamp,
    category: crumb.category,
  }));
  return safe;
}

function buildIdentity() {
  return typeof __NEVERMIND_BUILD__ === 'string'
    ? __NEVERMIND_BUILD__
    : `nevermind@${app.getVersion()}`;
}

function loadSentry() {
  if (sentry || didTryLoadSentry) return sentry;
  didTryLoadSentry = true;
  try {
    sentry = requireSentryModule('@sentry/electron/main') as SentryMain;
  } catch (error) {
    console.warn(
      'Sentry disabled because @sentry/electron/main could not be loaded.',
      error,
    );
  }
  return sentry;
}

export function initSentry() {
  if (initialized) return;
  store ??= createDiagnosticStore(
    path.join(app.getPath('logs'), DIAGNOSTIC_FILE_NAME),
  );
  void store.load();
  configureObservability({
    build: buildIdentity(),
    record: rememberFailure,
    capture: captureFailure,
    span: traceOperation,
  });
  if (!app.isPackaged) return;
  const Sentry = loadSentry();
  if (!Sentry) return;
  const DEFAULT_DSN =
    'https://42084c8627e67df78e0de6816e67df66@o1175778.ingest.us.sentry.io/4511502032502784';
  const dsn =
    process.env.SENTRY_DSN_DESKTOP ||
    process.env.NEVERMIND_SENTRY_DSN ||
    DEFAULT_DSN;
  if (!dsn) return;
  Sentry.init({
    dsn,
    release: buildIdentity(),
    environment: app.isPackaged ? 'production' : 'development',
    tracesSampleRate: 0.1,
    sendDefaultPii: false,
    sendClientReports: false,
    enableLogs: false,
    autoSessionTracking: false,
    maxBreadcrumbs: 50,
    integrations: (defaults) =>
      defaults.filter((integration) =>
        MAIN_DIAGNOSTIC_INTEGRATIONS.has(integration.name),
      ),
    getRendererName: (contents) => `renderer.${contents.id}`,
    beforeBreadcrumb: () => null,
    beforeSend: prepareEvent,
    beforeSendTransaction: (event) => {
      const safe = sanitizeDiagnosticEvent(event);
      safe.release = buildIdentity();
      safe.environment = 'production';
      return safe;
    },
    transport: (options) => {
      const transport = Sentry.makeElectronTransport({
        ...options,
        bufferSize: 20,
      });
      return {
        flush: (timeout) => transport.flush(timeout),
        send: (envelope) => {
          const items = envelope[1].filter(
            (item) =>
              item[0].type === 'event' || item[0].type === 'transaction',
          );
          if (
            !items.length ||
            Buffer.byteLength(JSON.stringify(items)) > 64 * 1024
          )
            return Promise.resolve({});
          const header = envelope[0];
          return transport.send([
            {
              ...(isDiagnosticId(header.event_id)
                ? { event_id: header.event_id }
                : {}),
              sent_at: new Date().toISOString(),
            },
            items,
          ]);
        },
      };
    },
  });
  initialized = true;
}

export function captureException(
  err: unknown,
  context?: Record<string, unknown>,
) {
  void context;
  return recordOperationFailure(err);
}

function captureFailure(error: unknown, record: DiagnosticRecord) {
  if (!initialized || !sentry) return;
  sentry.withScope(function captureDiagnosticFailure(scope) {
    scope.setContext('diagnostic', { failure_handle: record.reference });
    sentry?.captureException(error);
  });
}

function traceOperation<T>(
  context: import('./observability').OperationContext,
  task: () => T,
): T {
  if (!initialized || !sentry) return task();
  return sentry.startSpan(
    { name: context.operation, op: context.operation },
    function runDiagnosticSpan(span) {
      function finish() {
        span.setStatus({
          code:
            context.outcome === 'failed' || context.outcome === 'timed_out'
              ? 2
              : 1,
        });
      }
      try {
        const result = task();
        if (result && typeof (result as Promise<unknown>).then === 'function')
          return Promise.resolve(result).finally(finish) as T;
        finish();
        return result;
      } catch (error) {
        finish();
        throw error;
      }
    },
  );
}

export function reportDiagnosticProblem() {
  const record = failureRecord('support.report');
  record.outcome = 'unknown';
  rememberFailure(record);
  if (initialized && sentry)
    sentry.withScope(function captureProblemReport(scope) {
      scope.setContext('diagnostic', { failure_handle: record.reference });
      sentry?.captureMessage('User requested diagnostics', 'info');
    });
  return record.reference;
}
