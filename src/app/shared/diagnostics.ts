export const DIAGNOSTIC_OPERATIONS = [
  'command.execute',
  'view.action',
  'view.render',
  'view.load',
  'extension.load',
  'extension.install',
  'extension.generate',
  'ai.stream',
  'auth.signin',
  'auth.request',
  'backend.request',
  'job.run',
  'os.dispatch',
  'update.check',
  'update.download',
  'update.install',
  'support.report',
  'process.unhandled',
] as const;

export type DiagnosticOperation = (typeof DIAGNOSTIC_OPERATIONS)[number];
export type DiagnosticOutcome =
  | 'success'
  | 'failed'
  | 'cancelled'
  | 'blocked'
  | 'timed_out'
  | 'interrupted'
  | 'unknown';
export type DiagnosticStage =
  | 'dispatch'
  | 'invoke'
  | 'load'
  | 'render'
  | 'permission'
  | 'request'
  | 'stream'
  | 'validate'
  | 'save'
  | 'install'
  | 'complete'
  | 'unknown';
export type DiagnosticProcess = 'main' | 'renderer';
export type DiagnosticRecord = {
  reference: string;
  timestamp: string;
  bootId: string;
  build: string;
  operation: DiagnosticOperation;
  stage: DiagnosticStage;
  outcome: DiagnosticOutcome;
  process: DiagnosticProcess;
  windowId?: number;
  journeyId?: string;
  actionId?: string;
  requestId?: string;
  eventId?: string;
  projectId?: string;
  reporting: 'local_only' | 'capture_requested' | 'dropped';
};

export function isDiagnosticId(value: unknown): value is string {
  return typeof value === 'string' && /^[a-f0-9]{32}$/.test(value);
}

export function isRequestId(value: unknown): value is string {
  return typeof value === 'string' && /^[a-zA-Z0-9_-]{1,80}$/.test(value);
}

export function diagnosticOperation(value: unknown): DiagnosticOperation {
  return DIAGNOSTIC_OPERATIONS.includes(value as DiagnosticOperation)
    ? (value as DiagnosticOperation)
    : 'process.unhandled';
}
