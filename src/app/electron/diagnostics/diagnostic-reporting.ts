// biome-ignore-all lint/style/useNamingConvention: PostHog event and exception properties are snake_case on the wire.
// biome-ignore-all lint/complexity/noVoid: Fire-and-forget loads must not block the diagnostic path.
// biome-ignore-all lint/style/useExportsLast: Exports are grouped by concern, matching the module it replaces.
import path from 'node:path';
import { performance } from 'node:perf_hooks';
import { app, BrowserWindow } from 'electron';
import type { DiagnosticRecord } from '../../shared/diagnostics';
import {
  createDiagnosticStore,
  DIAGNOSTIC_FILE_NAME,
} from './diagnostic-store';
import {
  createFeatureUsageDeduper,
  featureUsage,
  isRoutineBackgroundSuccess,
} from './feature-usage';
import {
  configureObservability,
  failureRecord,
  type OperationContext,
  recordOperationFailure,
} from './observability';
import {
  captureDesktopEvent,
  commonEventProperties,
  configurePosthogIdentity,
  flushPosthog,
  posthogClient,
  posthogDistinctId,
  setPosthogConsent,
  whenInstallIdReady,
} from './posthog-client';

const REPORTING_CHANGED_CHANNEL = 'diagnostics:reporting-changed';
const CAPTURES_PER_MINUTE = 60;
const CAPTURE_WINDOW_MS = 60_000;
const RECENT_RECORD_LIMIT = 100;
const UPDATE_VERSION_PATTERN = /^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$/;
let initialized = false;
let store: ReturnType<typeof createDiagnosticStore> | undefined;
let updateStore: ReturnType<typeof createDiagnosticStore> | undefined;
let reportingEnabled = false;
let launchReported = false;
const firstUseInJourney = createFeatureUsageDeduper();
let captureWindowStarted = Date.now();
let captureWindowCount = 0;
declare const __NEVERMIND_BUILD__: string;

export function recentDiagnosticRecords() {
  return [...(store?.recent() ?? []), ...(updateStore?.recent() ?? [])]
    .sort((left, right) => right.timestamp.localeCompare(left.timestamp))
    .slice(0, RECENT_RECORD_LIMIT);
}

export function setDiagnosticReporting(enabled: boolean) {
  reportingEnabled = enabled;
  setPosthogConsent(enabled);
  if (enabled && !launchReported) {
    launchReported = true;
    // The install id loads asynchronously; a launch must carry the stored one.
    void whenInstallIdReady().then(function reportLaunch() {
      if (reportingEnabled) {
        captureDesktopEvent('app.launched', { release: buildIdentity() });
      }
    });
  }
  for (const window of BrowserWindow.getAllWindows()) {
    window.webContents.send(REPORTING_CHANGED_CHANNEL, enabled);
  }
}

export async function flushDiagnosticRecords() {
  await Promise.all([store?.flush(), updateStore?.flush(), flushPosthog()]);
}

export function recordUpdateInstallAttempt(targetVersion?: string) {
  if (!(targetVersion && UPDATE_VERSION_PATTERN.test(targetVersion))) {
    return;
  }
  updateStore?.put({
    ...failureRecord('update.install', 'install'),
    outcome: 'unknown',
    targetVersion,
  });
}

function rememberFailure(record: DiagnosticRecord) {
  store?.put(record);
}

// The renderer reports its own exceptions through posthog-js; the host only
// mints the support reference that ties the renderer event to the local record.
export function rendererFailureHandle(windowId: number) {
  const record = {
    ...failureRecord('view.render', 'render', false),
    process: 'renderer' as const,
    windowId,
    reporting: reportingEnabled
      ? ('capture_requested' as const)
      : ('local_only' as const),
  };
  rememberFailure(record);
  // Renderer exceptions travel anonymously from the renderer SDK; this host
  // event carries the install and shares the support reference as the join key.
  if (reportingEnabled) {
    captureDesktopEvent('renderer_failure', recordProperties(record));
  }
  return record.reference;
}

function buildIdentity() {
  return typeof __NEVERMIND_BUILD__ === 'string'
    ? __NEVERMIND_BUILD__
    : `nevermind@${app.getVersion()}`;
}

function reportingState(): DiagnosticRecord['reporting'] {
  if (!reportingEnabled) {
    return 'local_only';
  }
  return withinCaptureBudget() ? 'capture_requested' : 'dropped';
}

function withinCaptureBudget() {
  if (Date.now() - captureWindowStarted > CAPTURE_WINDOW_MS) {
    captureWindowStarted = Date.now();
    captureWindowCount = 0;
  }
  return ++captureWindowCount <= CAPTURES_PER_MINUTE;
}

function recordProperties(record: DiagnosticRecord) {
  return {
    support_ref: record.reference,
    operation: record.operation,
    stage: record.stage,
    outcome: record.outcome,
    process: record.process,
    release: record.build,
    boot_id: record.bootId,
    ...(record.journeyId ? { journey_id: record.journeyId } : {}),
    ...(record.actionId ? { action_id: record.actionId } : {}),
    ...(record.requestId ? { request_id: record.requestId } : {}),
    ...(record.component ? { component: record.component } : {}),
    ...(record.targetId ? { target_id: record.targetId } : {}),
  };
}

export function initDiagnostics(options: {
  currentUserId: () => string | undefined;
}) {
  if (initialized) {
    return;
  }
  initialized = true;
  configurePosthogIdentity(options.currentUserId);
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
      ) {
        updateStore?.put({ ...attempt, targetRunning: true });
      }
    }
  });
  configureObservability({
    build: buildIdentity(),
    record: rememberFailure,
    capture: captureFailure,
    span: traceOperation,
  });
}

function captureFailure(error: unknown, record: DiagnosticRecord) {
  const client = posthogClient();
  if (!client) {
    return;
  }
  record.reporting = reportingState();
  rememberFailure(record);
  if (record.reporting !== 'capture_requested') {
    return;
  }
  client.captureException(error, posthogDistinctId(), {
    ...commonEventProperties(),
    ...recordProperties(record),
  });
}

function captureOperation(context: OperationContext, durationMs: number) {
  if (!(reportingEnabled && posthogClient())) {
    return;
  }
  if (isRoutineBackgroundSuccess(context)) {
    return;
  }
  captureDesktopEvent('operation_completed', {
    operation: context.operation,
    outcome: context.outcome,
    stage: context.stage,
    duration_ms: Math.round(durationMs),
    boot_id: context.bootId,
    journey_id: context.journeyId,
    action_id: context.actionId,
    release: buildIdentity(),
    ...(context.label ? { label: context.label } : {}),
    ...(context.component ? { component: context.component } : {}),
    ...(context.targetId ? { target_id: context.targetId } : {}),
    ...(context.requestId ? { request_id: context.requestId } : {}),
    ...(context.responseStatus === undefined
      ? {}
      : { response_status: context.responseStatus }),
  });
  const usage = featureUsage(context);
  if (usage && firstUseInJourney(context.journeyId, usage.feature)) {
    captureDesktopEvent('feature.used', {
      feature: usage.feature,
      ...(usage.variant ? { variant: usage.variant } : {}),
      outcome: context.outcome,
      release: buildIdentity(),
    });
  }
}

function traceOperation<T>(context: OperationContext, task: () => T): T {
  if (!(reportingEnabled && posthogClient())) {
    return task();
  }
  const startedAt = performance.now();
  function finish() {
    captureOperation(context, performance.now() - startedAt);
  }
  try {
    const result = task();
    if (result && typeof (result as Promise<unknown>).then === 'function') {
      return Promise.resolve(result).finally(finish) as T;
    }
    finish();
    return result;
  } catch (error) {
    finish();
    throw error;
  }
}

export function reportDiagnosticProblem() {
  const record = failureRecord('support.report');
  record.outcome = 'unknown';
  rememberFailure(record);
  if (reportingEnabled) {
    captureDesktopEvent('support_report', recordProperties(record));
  }
  return record.reference;
}

// Global safety net: unhandled rejections and uncaught exceptions must never
// crash the app. An extension action or shell call that throws unexpectedly
// should be logged, reported and contained, not silently kill the process.
export function installProcessErrorSafetyNet() {
  process.on('unhandledRejection', (reason) => {
    console.error('FATAL: unhandled rejection (would crash Electron):', reason);
    recordOperationFailure(reason);
  });
  process.on('uncaughtException', (error) => {
    console.error('FATAL: uncaught exception (would crash Electron):', error);
    recordOperationFailure(error);
  });
}
