export type JobOwner = 'host' | 'extension';
export type JobStatus =
  | 'idle'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'disabled'
  | 'backing-off';
export type JobTrigger =
  | { type: 'manual' }
  | { type: 'startup'; delayMs?: number; payload?: unknown }
  | { type: 'interval'; everyMs: number; delayMs?: number; payload?: unknown }
  | {
      type: 'event';
      event: string;
      debounceMs?: number;
      maxWaitMs?: number;
      payload?: unknown;
    };

export type JobRunContext = {
  id: string;
  reason: string;
  event?: string;
  payload?: unknown;
  startedAt: number;
};

export type JobHistoryEntry = {
  startedAt: number;
  finishedAt: number;
  durationMs: number;
  status: 'succeeded' | 'failed';
  reason: string;
  error?: string;
};

export type JobDefinition = {
  id: string;
  title: string;
  owner?: JobOwner;
  scope?: string;
  enabled?: boolean;
  triggers?: JobTrigger[];
  timeoutMs?: number;
  maxConcurrency?: 1;
  backoff?: { initialMs?: number; maxMs?: number };
  run: (context: JobRunContext) => unknown | Promise<unknown>;
};

export type JobPerformanceRunner = <T>(
  operation: string,
  attributes: Record<string, unknown>,
  task: () => T | Promise<T>,
) => T | Promise<T>;

export type JobSnapshot = {
  id: string;
  title: string;
  owner: JobOwner;
  scope?: string;
  status: JobStatus;
  enabled: boolean;
  running: boolean;
  runCount: number;
  failureCount: number;
  consecutiveFailures: number;
  backoffUntil?: number;
  lastReason?: string;
  lastStartedAt?: number;
  lastFinishedAt?: number;
  lastDurationMs?: number;
  lastError?: string;
  nextRunAt?: number;
  triggers: JobTrigger[];
  history: JobHistoryEntry[];
};
