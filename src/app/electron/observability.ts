import { AsyncLocalStorage } from 'node:async_hooks';
import { randomBytes } from 'node:crypto';
import type {
  DiagnosticOperation,
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
  failure?: DiagnosticRecord;
  closed: boolean;
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

export function operationStage(stage: DiagnosticStage) {
  const context = currentOperation();
  if (!context || context.closed) return;
  context.stage = stage;
  context.breadcrumbs.push({
    timestamp: Date.now() / 1000,
    category: `${context.operation}.${stage}`,
  });
  context.breadcrumbs.splice(0, Math.max(0, context.breadcrumbs.length - 50));
}

export function operationOutcome(outcome: DiagnosticOutcome) {
  const context = currentOperation();
  if (context && !context.closed && context.outcome === 'unknown')
    context.outcome = outcome;
}

export function failureRecord(
  operation?: DiagnosticOperation,
  stage?: DiagnosticStage,
): DiagnosticRecord {
  const context = currentOperation();
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
        }
      : {}),
  };
}

export function recordOperationFailure(
  error: unknown,
  stage?: DiagnosticStage,
) {
  const context = currentOperation();
  if (context?.closed) return context.failure;
  if (context?.failure) return context.failure;
  const name = error instanceof Error ? error.name : '';
  if (name === 'AbortError' || name === 'AbortErrorException') {
    operationOutcome('cancelled');
    return;
  }
  if (name === 'TimeoutError' || name === 'PromiseTimeoutError')
    operationOutcome('timed_out');
  else operationOutcome('failed');
  const record = failureRecord(undefined, stage);
  if (context) {
    record.outcome = context.outcome;
    context.failure = record;
  }
  try {
    hooks.record?.(record);
    hooks.capture?.(error, record);
  } catch {}
  return record;
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
  options: { windowId?: number; freshJourney?: boolean } = {},
): T {
  const parent = currentOperation();
  const context: OperationContext = {
    operation,
    stage: 'dispatch',
    outcome: 'unknown',
    closed: false,
    journeyId:
      !options.freshJourney && parent ? parent.journeyId : diagnosticId(),
    actionId: diagnosticId(),
    windowId: options.windowId ?? parent?.windowId,
    breadcrumbs: parent ? parent.breadcrumbs.slice(-49) : [],
  };
  function finish() {
    if (context.closed) return;
    if (context.outcome === 'unknown') context.outcome = 'success';
    context.closed = true;
    if (parent && context.failure && !parent.failure) {
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
