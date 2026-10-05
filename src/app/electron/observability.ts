import { AsyncLocalStorage } from 'node:async_hooks';
import { randomBytes } from 'node:crypto';
import type {
  DiagnosticOperation,
  DiagnosticComponent,
  DiagnosticOutcome,
  DiagnosticRecord,
  DiagnosticStage,
} from '../shared/diagnostics';

export type OperationContext = {
  journeyId: string;
  actionId: string;
  operation: DiagnosticOperation;
  stage: DiagnosticStage;
  outcome: DiagnosticOutcome;
  windowId?: number;
  requestId?: string;
  traceHeaders?: Record<string, string>;
  serverFailure?: boolean;
  responseStatus?: number;
  component?: DiagnosticComponent;
  targetId?: string;
  failure?: DiagnosticRecord;
  closed: boolean;
  completionUnverified?: boolean;
  breadcrumbs: { timestamp: number; category: string }[];
};

type ObservabilityHooks = {
  record?: (record: DiagnosticRecord) => void;
  capture?: (error: unknown, record: DiagnosticRecord) => void;
  span?: <T>(context: OperationContext, task: () => T) => T;
};

export function diagnosticId() {
  return randomBytes(16).toString('hex');
}
const contextStorage = new AsyncLocalStorage<OperationContext>();
const bootId = diagnosticId();
const windowHistory = new Map<number, OperationContext['breadcrumbs']>();
const extensionTargets = new WeakMap<object, string>();
let hooks: ObservabilityHooks = {};
let build = 'development';

export function configureObservability(
  options: ObservabilityHooks & { build: string },
) {
  hooks = options;
  build = options.build;
}

export function currentOperation() {
  return contextStorage.getStore();
}

export function diagnosticBreadcrumbs(windowId?: number) {
  return (
    (windowId
      ? windowHistory.get(windowId)
      : currentOperation()?.breadcrumbs
    )?.slice(-50) ?? []
  );
}

export function operationTarget(
  extension: object,
  component: DiagnosticComponent,
) {
  const context = currentOperation();
  if (!context || context.closed || !extension || typeof extension !== 'object')
    return;
  let targetId = extensionTargets.get(extension);
  if (!targetId) {
    targetId = diagnosticId();
    extensionTargets.set(extension, targetId);
  }
  context.component = component;
  context.targetId = targetId;
  return targetId;
}

export function operationStage(stage: DiagnosticStage) {
  const context = currentOperation();
  if (!context || context.closed) return;
  context.stage = stage;
  context.breadcrumbs.push({
    timestamp: Date.now() / 1000,
    category: `${context.operation}.${stage}`,
  });
  context.breadcrumbs.splice(0, Math.max(0, context.breadcrumbs.length - 50));
  if (context.windowId) {
    const history = windowHistory.get(context.windowId) ?? [];
    history.push({
      timestamp: Date.now() / 1000,
      category: `${context.operation}.${stage}`,
    });
    windowHistory.set(context.windowId, history.slice(-50));
    while (windowHistory.size > 64)
      windowHistory.delete(windowHistory.keys().next().value!);
  }
}

export function operationOutcome(outcome: DiagnosticOutcome) {
  const context = currentOperation();
  if (context && !context.closed && context.outcome === 'unknown')
    context.outcome = outcome;
}

export function operationPending() {
  const context = currentOperation();
  if (context && !context.closed) context.completionUnverified = true;
}

export function failureRecord(
  operation?: DiagnosticOperation,
  stage?: DiagnosticStage,
  includeOperationContext = true,
): DiagnosticRecord {
  const context = includeOperationContext ? currentOperation() : undefined;
  return {
    reference: diagnosticId(),
    timestamp: new Date().toISOString(),
    bootId,
    build,
    operation: operation ?? context?.operation ?? 'process.unhandled',
    stage: stage ?? context?.stage ?? 'unknown',
    outcome: 'failed',
    process: 'main',
    reporting: 'local_only',
    ...(context
      ? {
          journeyId: context.journeyId,
          actionId: context.actionId,
          windowId: context.windowId,
          requestId: context.requestId,
          component: context.component,
          targetId: context.targetId,
        }
      : {}),
  };
}

export function recordOperationFailure(
  error: unknown,
  stage?: DiagnosticStage,
  captureOwner: 'client' | 'backend' | 'local' = 'client',
) {
  const context = currentOperation();
  if (context?.closed) return context.failure;
  if (context?.failure) return context.failure;
  const name = error instanceof Error ? error.name : '';
  if (name === 'AbortError' || name === 'AbortErrorException') {
    operationOutcome('cancelled');
    return;
  }
  if (
    captureOwner === 'backend' &&
    context?.serverFailure &&
    [401, 402, 403, 410, 429].includes(context.responseStatus ?? 0)
  )
    operationOutcome('blocked');
  else if (
    name === 'NevermindAuthRequiredError' ||
    name === 'NevermindCompatibilityError'
  )
    operationOutcome('blocked');
  else if (name === 'TimeoutError' || name === 'PromiseTimeoutError')
    operationOutcome('timed_out');
  else operationOutcome('failed');
  const record = failureRecord(undefined, stage);
  if (context) {
    record.outcome = context.outcome;
    context.failure = record;
  }
  try {
    hooks.record?.(record);
    if (captureOwner === 'client' && record.outcome !== 'blocked')
      hooks.capture?.(error, record);
  } catch {}
  return record;
}

export function recordServerOutcome(status: number, requestId?: string) {
  const context = currentOperation();
  if (!context || context.closed) return;
  context.requestId = requestId;
  context.responseStatus = status;
  context.serverFailure = status >= 400;
  operationStage(status >= 400 ? 'request' : 'stream');
}

export function bindOperation<T extends (...args: any[]) => any>(
  callback: T,
): T {
  const context = currentOperation();
  return function boundOperation(...args: Parameters<T>) {
    return context
      ? contextStorage.run(context, () => callback(...args))
      : callback(...args);
  } as T;
}

export function runOperation<T>(
  operation: DiagnosticOperation,
  task: () => T,
  options: {
    windowId?: number;
    freshJourney?: boolean;
    defaultOutcome?: DiagnosticOutcome;
  } = {},
): T {
  const parent = options.freshJourney ? undefined : currentOperation();
  const context: OperationContext = {
    operation,
    stage: 'dispatch',
    outcome: 'unknown',
    closed: false,
    journeyId:
      !options.freshJourney && parent ? parent.journeyId : diagnosticId(),
    actionId: diagnosticId(),
    windowId: options.windowId ?? parent?.windowId,
    component: parent?.component,
    targetId: parent?.targetId,
    breadcrumbs: parent
      ? parent.breadcrumbs.slice(-49)
      : diagnosticBreadcrumbs(options.windowId).slice(-20),
  };
  function finish() {
    if (context.closed) return;
    if (context.outcome === 'unknown')
      context.outcome = context.completionUnverified
        ? 'unknown'
        : (options.defaultOutcome ?? 'success');
    context.closed = true;
    if (parent && !parent.closed) {
      if (context.requestId) parent.requestId = context.requestId;
      if (context.responseStatus !== undefined) {
        parent.serverFailure = context.serverFailure;
        parent.responseStatus = context.responseStatus;
      }
      if (
        context.outcome !== 'success' &&
        context.outcome !== 'unknown' &&
        parent.outcome === 'unknown'
      )
        parent.outcome = context.outcome;
      parent.breadcrumbs.push(...context.breadcrumbs.slice(-10));
      parent.breadcrumbs.splice(0, Math.max(0, parent.breadcrumbs.length - 50));
    }
    if (parent && !parent.closed && context.failure && !parent.failure) {
      parent.failure = context.failure;
      if (!parent.closed && parent.outcome === 'unknown')
        parent.outcome = context.outcome;
    }
  }
  function invoke() {
    operationStage('dispatch');
    try {
      const result = task();
      if (result && typeof (result as Promise<unknown>).then === 'function') {
        return Promise.resolve(result).then(
          function operationSucceeded(value) {
            finish();
            return value;
          },
          function operationFailed(error) {
            recordOperationFailure(error);
            finish();
            throw error;
          },
        ) as T;
      }
      finish();
      return result;
    } catch (error) {
      recordOperationFailure(error);
      finish();
      throw error;
    }
  }
  return contextStorage.run(context, () =>
    hooks.span ? hooks.span(context, invoke) : invoke(),
  );
}
