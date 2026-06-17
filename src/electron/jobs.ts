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
  | { type: 'event'; event: string; debounceMs?: number; payload?: unknown };

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

type JobRecord = {
  definition: JobDefinition;
  enabled: boolean;
  running: boolean;
  pendingReason: string | null;
  runCount: number;
  failureCount: number;
  consecutiveFailures: number;
  status: JobStatus;
  backoffUntil?: number;
  lastReason?: string;
  lastStartedAt?: number;
  lastFinishedAt?: number;
  lastDurationMs?: number;
  lastError?: string;
  nextRunAt?: number;
  history: JobHistoryEntry[];
  timers: NodeJS.Timeout[];
  scheduledTimers: Map<string, NodeJS.Timeout>;
  scheduledPayloads: Map<string, { event?: string; payload?: unknown }>;
};

const HISTORY_LIMIT = 10;
const DEFAULT_EXTENSION_BACKOFF_MS = 30_000;
const DEFAULT_EXTENSION_MAX_BACKOFF_MS = 15 * 60_000;

type EnabledOverrides = Record<string, boolean>;

function timeout<T>(promise: Promise<T>, timeoutMs: number, title: string) {
  let timer: NodeJS.Timeout | undefined;
  const guard = new Promise<T>((_, reject) => {
    timer = setTimeout(
      () => reject(new Error(`${title} timed out after ${timeoutMs}ms`)),
      timeoutMs,
    );
    timer.unref?.();
  });
  return Promise.race([promise, guard]).finally(() => {
    if (timer) clearTimeout(timer);
  });
}

function backoffDelay(record: JobRecord) {
  if (record.definition.owner !== 'extension' && !record.definition.backoff)
    return 0;
  const initial =
    record.definition.backoff?.initialMs ?? DEFAULT_EXTENSION_BACKOFF_MS;
  const max =
    record.definition.backoff?.maxMs ?? DEFAULT_EXTENSION_MAX_BACKOFF_MS;
  return Math.min(
    max,
    initial * Math.max(1, 2 ** Math.max(0, record.consecutiveFailures - 1)),
  );
}

function mergeEventPayloads(
  previous?: { event?: string; payload?: unknown },
  next?: { event?: string; payload?: unknown },
) {
  if (!previous) return next || {};
  if (!next) return previous;
  const previousPayload: any = previous.payload;
  const nextPayload: any = next.payload;
  if (
    Array.isArray(previousPayload?.changedPaths) ||
    Array.isArray(nextPayload?.changedPaths)
  ) {
    const changedPaths = [
      ...new Set([
        ...(previousPayload?.changedPaths || []),
        ...(nextPayload?.changedPaths || []),
      ]),
    ].slice(-100);
    return {
      event: next.event || previous.event,
      payload: {
        ...(previousPayload || {}),
        ...(nextPayload || {}),
        changedPaths,
      },
    };
  }
  return next;
}

export class JobRegistry {
  private records = new Map<string, JobRecord>();
  private listeners = new Set<() => void>();
  private enabledOverrides: EnabledOverrides = {};

  hydrateEnabled(overrides: EnabledOverrides = {}) {
    this.enabledOverrides = { ...overrides };
  }

  enabledOverridesSnapshot() {
    return { ...this.enabledOverrides };
  }

  register(definition: JobDefinition) {
    const existing = this.records.get(definition.id);
    if (existing) this.clearTimers(existing);
    const override = this.enabledOverrides[definition.id];
    const enabled =
      typeof override === 'boolean' ? override : definition.enabled !== false;
    const record: JobRecord = existing || {
      definition,
      enabled,
      running: false,
      pendingReason: null,
      runCount: 0,
      failureCount: 0,
      consecutiveFailures: 0,
      status: enabled ? 'idle' : 'disabled',
      history: [],
      timers: [],
      scheduledTimers: new Map(),
      scheduledPayloads: new Map(),
    };
    record.definition = definition;
    record.enabled = enabled;
    record.status = record.enabled
      ? record.running
        ? 'running'
        : record.status === 'disabled'
          ? 'idle'
          : record.status
      : 'disabled';
    record.timers = [];
    record.scheduledTimers ||= new Map();
    record.scheduledPayloads ||= new Map();
    record.history ||= [];
    this.records.set(definition.id, record);
    this.installTriggers(record);
    this.notify();
    return definition.id;
  }

  unregister(id: string) {
    const record = this.records.get(id);
    if (!record) return;
    this.clearTimers(record);
    this.records.delete(id);
    this.notify();
  }

  unregisterWhere(predicate: (snapshot: JobSnapshot) => boolean) {
    for (const snapshot of this.snapshot()) {
      if (predicate(snapshot)) this.unregister(snapshot.id);
    }
  }

  has(id: string) {
    return this.records.has(id);
  }

  async run(
    id: string,
    reason = 'manual',
    eventPayload?: { event?: string; payload?: unknown },
  ) {
    const record = this.records.get(id);
    if (!record) throw new Error(`Unknown background job: ${id}`);
    if (!record.enabled) return null;
    if (record.running) {
      record.pendingReason = reason;
      this.notify();
      return null;
    }
    const now = Date.now();
    if (
      reason !== 'manual' &&
      record.backoffUntil &&
      record.backoffUntil > now
    ) {
      record.status = 'backing-off';
      this.schedule(id, 'backoff', record.backoffUntil - now);
      this.notify();
      return null;
    }
    record.running = true;
    record.status = 'running';
    record.lastReason = reason;
    record.lastStartedAt = Date.now();
    record.lastError = undefined;
    this.notify();
    try {
      const promise = Promise.resolve(
        record.definition.run({
          id,
          reason,
          event: eventPayload?.event,
          payload: eventPayload?.payload,
          startedAt: record.lastStartedAt,
        }),
      );
      const result = record.definition.timeoutMs
        ? await timeout(
            promise,
            record.definition.timeoutMs,
            record.definition.title,
          )
        : await promise;
      record.status = 'succeeded';
      record.runCount += 1;
      record.consecutiveFailures = 0;
      record.backoffUntil = undefined;
      return result;
    } catch (error) {
      record.status = 'failed';
      record.failureCount += 1;
      record.consecutiveFailures += 1;
      record.lastError = error instanceof Error ? error.message : String(error);
      const delay = backoffDelay(record);
      if (delay) record.backoffUntil = Date.now() + delay;
      throw error;
    } finally {
      record.running = false;
      record.lastFinishedAt = Date.now();
      record.lastDurationMs = record.lastStartedAt
        ? record.lastFinishedAt - record.lastStartedAt
        : undefined;
      this.rememberHistory(record);
      const pending = record.pendingReason;
      record.pendingReason = null;
      this.notify();
      if (pending && record.enabled) void this.run(id, pending).catch(() => {});
    }
  }

  schedule(
    id: string,
    reason = 'scheduled',
    delayMs = 0,
    eventPayload?: { event?: string; payload?: unknown },
  ) {
    const record = this.records.get(id);
    if (!record || !record.enabled) return;
    const previous = record.scheduledTimers.get(reason);
    if (previous) clearTimeout(previous);
    record.nextRunAt = Date.now() + Math.max(0, delayMs);
    record.scheduledPayloads.set(
      reason,
      mergeEventPayloads(record.scheduledPayloads.get(reason), eventPayload),
    );
    const timer = setTimeout(
      () => {
        const payload = record.scheduledPayloads.get(reason);
        record.scheduledTimers.delete(reason);
        record.scheduledPayloads.delete(reason);
        record.nextRunAt = undefined;
        void this.run(id, reason, payload).catch(() => {});
      },
      Math.max(0, delayMs),
    );
    timer.unref?.();
    record.scheduledTimers.set(reason, timer);
    this.notify();
  }

  emit(event: string, payload?: unknown) {
    for (const record of this.records.values()) {
      for (const trigger of record.definition.triggers || []) {
        if (trigger.type !== 'event' || trigger.event !== event) continue;
        this.schedule(
          record.definition.id,
          `event:${event}`,
          trigger.debounceMs || 0,
          { event, payload: payload ?? trigger.payload },
        );
      }
    }
  }

  setEnabled(id: string, enabled: boolean) {
    const record = this.records.get(id);
    if (!record) return false;
    this.enabledOverrides[id] = enabled;
    record.enabled = enabled;
    record.status = enabled ? 'idle' : 'disabled';
    if (!enabled) this.clearTimers(record);
    else this.installTriggers(record);
    this.notify();
    return true;
  }

  clearError(id: string) {
    const record = this.records.get(id);
    if (!record) return false;
    record.lastError = undefined;
    record.consecutiveFailures = 0;
    record.backoffUntil = undefined;
    if (
      !record.running &&
      (record.status === 'failed' || record.status === 'backing-off')
    )
      record.status = 'idle';
    this.notify();
    return true;
  }

  snapshot(): JobSnapshot[] {
    const snapshots = Array.from(this.records.values())
      .map((record) => ({
        id: record.definition.id,
        title: record.definition.title,
        owner: record.definition.owner || 'host',
        scope: record.definition.scope,
        status: record.status,
        enabled: record.enabled,
        running: record.running,
        runCount: record.runCount,
        failureCount: record.failureCount,
        consecutiveFailures: record.consecutiveFailures,
        backoffUntil: record.backoffUntil,
        lastReason: record.lastReason,
        lastStartedAt: record.lastStartedAt,
        lastFinishedAt: record.lastFinishedAt,
        lastDurationMs: record.lastDurationMs,
        lastError: record.lastError,
        nextRunAt: record.nextRunAt,
        triggers: record.definition.triggers || [],
        history: record.history || [],
      }))
      .sort((a, b) => a.title.localeCompare(b.title));
    return structuredClone(snapshots);
  }

  onChange(listener: () => void) {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  clear() {
    for (const record of this.records.values()) this.clearTimers(record);
    this.records.clear();
    this.notify();
  }

  private rememberHistory(record: JobRecord) {
    if (
      !record.lastStartedAt ||
      !record.lastFinishedAt ||
      !record.lastDurationMs
    )
      return;
    record.history = [
      {
        startedAt: record.lastStartedAt,
        finishedAt: record.lastFinishedAt,
        durationMs: record.lastDurationMs,
        status:
          record.status === 'failed'
            ? ('failed' as const)
            : ('succeeded' as const),
        reason: record.lastReason || 'manual',
        error: record.lastError,
      },
      ...(record.history || []),
    ].slice(0, HISTORY_LIMIT);
  }

  private installTriggers(record: JobRecord) {
    if (!record.enabled) return;
    for (const trigger of record.definition.triggers || []) {
      if (trigger.type === 'startup')
        this.schedule(record.definition.id, 'startup', trigger.delayMs || 0, {
          payload: trigger.payload,
        });
      if (trigger.type === 'interval') {
        this.schedule(
          record.definition.id,
          'interval',
          trigger.delayMs ?? trigger.everyMs,
          { payload: trigger.payload },
        );
        const timer = setInterval(
          () =>
            void this.run(record.definition.id, 'interval', {
              payload: trigger.payload,
            }).catch(() => {}),
          Math.max(1000, trigger.everyMs),
        );
        timer.unref?.();
        record.timers.push(timer);
      }
    }
  }

  private clearTimers(record: JobRecord) {
    for (const timer of record.timers) clearTimeout(timer);
    for (const timer of record.scheduledTimers.values()) clearTimeout(timer);
    record.timers = [];
    record.scheduledTimers.clear();
    record.scheduledPayloads.clear();
    record.nextRunAt = undefined;
  }

  private notify() {
    for (const listener of this.listeners) listener();
  }
}
