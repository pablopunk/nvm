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
  diagnosticBreadcrumbs,
  failureRecord,
  recordOperationFailure,
} from './observability';
import { diagnosticEnvelope } from './diagnostic-envelope';
import { auditLegacyDiagnosticCache } from './legacy-diagnostic-cache';

type SentryMain = typeof import('@sentry/electron/main');

const requireSentryModule = createRequire(import.meta.url);
let initialized = false;
let sentry: SentryMain | undefined;
let didTryLoadSentry = false;
let store: ReturnType<typeof createDiagnosticStore> | undefined;
let updateStore: ReturnType<typeof createDiagnosticStore> | undefined;
let reportingEnabled = false;
const pendingFailures = new Map<string, DiagnosticRecord>();
let captureWindowStarted = Date.now();
let captureWindowCount = 0;
const pendingBreadcrumbs = new Map<
  string,
  ReturnType<typeof diagnosticBreadcrumbs>
>();
declare const __NEVERMIND_BUILD__: string;

export function recentDiagnosticRecords() {
  return [...(store?.recent() ?? []), ...(updateStore?.recent() ?? [])]
    .sort((left, right) => right.timestamp.localeCompare(left.timestamp))
    .slice(0, 100);
}

export function setDiagnosticReporting(enabled: boolean) {
  reportingEnabled = enabled;
}

export async function flushDiagnosticRecords() {
  await Promise.all([store?.flush(), updateStore?.flush()]);
}

export function recordUpdateInstallAttempt(targetVersion?: string) {
  if (
    !targetVersion ||
    !/^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$/.test(targetVersion)
  )
    return;
  updateStore?.put({
    ...failureRecord('update.install', 'install'),
    outcome: 'unknown',
    targetVersion,
  });
}

function rememberFailure(record: DiagnosticRecord) {
  if (!record.eventId) pendingFailures.set(record.reference, record);
  if (!pendingBreadcrumbs.has(record.reference))
    pendingBreadcrumbs.set(
      record.reference,
      diagnosticBreadcrumbs(record.windowId),
    );
  while (pendingFailures.size > 100)
    pendingFailures.delete(pendingFailures.keys().next().value!);
  while (pendingBreadcrumbs.size > 100)
    pendingBreadcrumbs.delete(pendingBreadcrumbs.keys().next().value!);
  store?.put(record);
}

export function rendererFailureHandle(windowId: number) {
  const record = {
    ...failureRecord('view.render', 'render', false),
    process: 'renderer' as const,
    windowId,
  };
  rememberFailure(record);
  return record.reference;
}

function prepareEvent(event: import('@sentry/electron/main').ErrorEvent) {
  const processTag = String(event.tags?.['event.process'] ?? '');
  const renderer = /^renderer\.(\d+)$/.exec(processTag);
  const isRenderer = Boolean(renderer) || processTag === 'renderer';
  const handle = event.contexts?.diagnostic?.failure_handle;
  const pending =
    typeof handle === 'string' ? pendingFailures.get(handle) : undefined;
  const record =
    pending &&
    (isRenderer
      ? renderer &&
        pending.process === 'renderer' &&
        pending.windowId === Number(renderer[1])
      : pending.process === 'main')
      ? pending
      : {
          ...failureRecord(
            isRenderer ? 'view.render' : undefined,
            isRenderer ? 'unknown' : undefined,
            !isRenderer,
          ),
          ...(isRenderer
            ? {
                process: 'renderer' as const,
                ...(renderer ? { windowId: Number(renderer[1]) } : {}),
                operation: 'view.render' as const,
              }
            : {}),
        };
  if (typeof handle === 'string') pendingFailures.delete(handle);
  if (isDiagnosticId(event.event_id)) record.eventId = event.event_id;
  record.reporting = reportingEnabled ? 'capture_requested' : 'local_only';
  if (Date.now() - captureWindowStarted > 60_000) {
    captureWindowStarted = Date.now();
    captureWindowCount = 0;
  }
  if (reportingEnabled && ++captureWindowCount > 60)
    record.reporting = 'dropped';
  record.projectId = sentry?.getClient()?.getDsn()?.projectId;
  rememberFailure(record);
  if (!reportingEnabled || record.reporting === 'dropped') return null;
  const safe = sanitizeDiagnosticEvent(event, record);
  safe.release = buildIdentity();
  safe.environment = app.isPackaged ? 'production' : 'development';
  safe.breadcrumbs = (
    pendingBreadcrumbs.get(record.reference) ??
    diagnosticBreadcrumbs(record.windowId)
  ).map((crumb) => ({
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
  updateStore ??= createDiagnosticStore(
    path.join(app.getPath('logs'), 'nevermind-update-diagnostics.json'),
    10,
  );
  void updateStore.load().then(function observeInstalledVersion() {
    const current = failureRecord();
    for (const attempt of updateStore?.recent() ?? []) {
      if (
        attempt.bootId !== current.bootId &&
        attempt.targetVersion === app.getVersion() &&
        !attempt.targetRunning
      )
        updateStore?.put({ ...attempt, targetRunning: true });
    }
  });
  configureObservability({
    build: buildIdentity(),
    record: rememberFailure,
    capture: captureFailure,
    span: traceOperation,
  });
  if (!app.isPackaged) return;
  void auditLegacyDiagnosticCache(app.getPath('userData')).then(
    function reportInertLegacyCache(present) {
      if (present)
        console.warn(
          'Legacy Sentry data exists; automatic replay and native-dump collection are disabled.',
        );
    },
  );
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
    maxBreadcrumbs: 50,
    integrations: (defaults) =>
      defaults.filter((integration) =>
        MAIN_DIAGNOSTIC_INTEGRATIONS.has(integration.name),
      ),
    getRendererName: (contents) => `renderer.${contents.id}`,
    beforeBreadcrumb: () => null,
    beforeSend: prepareEvent,
    beforeSendTransaction: (event) => {
      if (!reportingEnabled) return null;
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
          if (!reportingEnabled) return Promise.resolve({});
          const safe = diagnosticEnvelope(
            envelope,
            (reference) =>
              store?.recent().find((record) => record.reference === reference),
            buildIdentity(),
            'production',
          );
          if (!safe) return Promise.resolve({});
          return transport.send(safe);
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
  if (!initialized || !sentry || !reportingEnabled) return;
  sentry.withScope(function captureDiagnosticFailure(scope) {
    scope.setContext('diagnostic', { failure_handle: record.reference });
    sentry?.captureException(error);
  });
}

function traceOperation<T>(
  context: import('./observability').OperationContext,
  task: () => T,
): T {
  if (!initialized || !sentry || !reportingEnabled) return task();
  return sentry.startSpan(
    { name: context.operation, op: context.operation },
    function runDiagnosticSpan(span) {
      context.traceHeaders = sentry?.getTraceData();
      function finish() {
        span.setAttribute('diagnostic.outcome', context.outcome);
        span.setAttribute('diagnostic.stage', context.stage);
        if (context.component)
          span.setAttribute('diagnostic.component', context.component);
        span.setStatus({
          code:
            context.outcome === 'unknown'
              ? 0
              : context.outcome === 'success'
                ? 1
                : 2,
          ...(context.outcome === 'failed'
            ? { message: 'internal_error' }
            : context.outcome === 'timed_out'
              ? { message: 'deadline_exceeded' }
              : context.outcome === 'cancelled'
                ? { message: 'cancelled' }
                : context.outcome === 'blocked'
                  ? { message: 'permission_denied' }
                  : {}),
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
  if (initialized && sentry && reportingEnabled)
    sentry.withScope(function captureProblemReport(scope) {
      scope.setContext('diagnostic', { failure_handle: record.reference });
      sentry?.captureMessage('User requested diagnostics', 'info');
    });
  return record.reference;
}
