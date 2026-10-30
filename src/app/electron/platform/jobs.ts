import {
  backoffDelay,
  DebounceDeadlines,
  JobWorkBarrier,
  mergeEventPayloads,
  timeout,
} from './job-helpers';
import type {
  JobDefinition,
  JobHistoryEntry,
  JobPerformanceRunner,
  JobSnapshot,
  JobStatus,
} from './job-types';

export type {
  JobDefinition,
  JobHistoryEntry,
  JobOwner,
  JobPerformanceRunner,
  JobRunContext,
  JobSnapshot,
  JobStatus,
  JobTrigger,
} from './job-types';

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

type EnabledOverrides = Record<string, boolean>;

export class JobRegistry {
  private records = new Map<string, JobRecord>();
  private listeners = new Set<() => void>();
  private enabledOverrides: EnabledOverrides = {};
  private performanceTrace?: JobPerformanceRunner;
  private work = new JobWorkBarrier();
  private debounceDeadlines = new DebounceDeadlines();

  constructor(options: { performanceTrace?: JobPerformanceRunner } = {}) {
    this.performanceTrace = options.performanceTrace;
  }

  hydrateEnabled(overrides: EnabledOverrides = {}) {
    this.enabledOverrides = { ...overrides };
  }

  enabledOverridesSnapshot() {
    return { ...this.enabledOverrides };
  }

  register(definition: JobDefinition, options: { skipStartup?: boolean } = {}) {
    if (this.work.paused)
      throw new Error('Background jobs are paused for maintenance');
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
    this.installTriggers(record, options);
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
    if (this.work.paused) return null;
    const record = this.records.get(id);
    if (!record) throw new Error(`Unknown background job: ${id}`);
    if (!record.enabled) return null;
    if (record.running) {
      if (reason !== 'interval') record.pendingReason = reason;
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
      const run = () =>
        record.definition.run({
          id,
          reason,
          event: eventPayload?.event,
          payload: eventPayload?.payload,
          startedAt: record.lastStartedAt,
        });
      const runWithTimeout = () => {
        const promise = this.work.track(Promise.resolve(run()));
        return record.definition.timeoutMs
          ? timeout(
              promise,
              record.definition.timeoutMs,
              record.definition.title,
            )
          : promise;
      };
      const result = await this.work.track(
        Promise.resolve(
          this.performanceTrace
            ? this.performanceTrace(
                'job.run',
                {
                  jobId: id,
                  jobOwner: record.definition.owner || 'host',
                  reason,
                },
                runWithTimeout,
              )
            : runWithTimeout(),
        ),
      );
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
    if (this.work.paused) return;
    const record = this.records.get(id);
    if (!(record && record.enabled)) return;
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
        this.debounceDeadlines.settle(`${id}\0${reason}`);
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
        const { id } = record.definition;
        const reason = `event:${event}`;
        this.schedule(
          id,
          reason,
          this.debounceDeadlines.delayFor(
            `${id}\0${reason}`,
            trigger.debounceMs || 0,
            trigger.maxWaitMs,
          ),
          { event, payload: payload ?? trigger.payload },
        );
      }
    }
  }

  setEnabled(id: string, enabled: boolean) {
    const record = this.records.get(id);
    if (!record) return false;
    this.enabledOverrides[id] = enabled;
    this.applyRecordEnabled(record, enabled);
    return true;
  }

  applyEnabledOverrides(overrides: EnabledOverrides) {
    this.enabledOverrides = { ...overrides };
    for (const record of this.records.values()) {
      const override = this.enabledOverrides[record.definition.id];
      const enabled =
        typeof override === 'boolean'
          ? override
          : record.definition.enabled !== false;
      if (enabled !== record.enabled) this.applyRecordEnabled(record, enabled);
    }
  }

  private applyRecordEnabled(record: JobRecord, enabled: boolean) {
    record.enabled = enabled;
    record.status = enabled ? 'idle' : 'disabled';
    if (enabled) this.installTriggers(record);
    else this.clearTimers(record);
    this.notify();
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

  definitionsWhere(
    predicate: (definition: JobDefinition) => boolean,
  ): JobDefinition[] {
    return Array.from(this.records.values())
      .map((record) => record.definition)
      .filter(predicate);
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

  async quiesce() {
    const draining = this.work.drain();
    for (const record of this.records.values()) {
      this.clearTimers(record);
      record.pendingReason = null;
    }
    await draining;
  }

  resume() {
    if (!this.work.paused) return;
    if (this.work.draining)
      throw new Error('Background jobs are still draining');
    this.work.paused = false;
    for (const record of this.records.values())
      this.installTriggers(record, { skipStartup: true });
  }

  private rememberHistory(record: JobRecord) {
    if (
      !(record.lastStartedAt && record.lastFinishedAt && record.lastDurationMs)
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

  private installTriggers(
    record: JobRecord,
    options: { skipStartup?: boolean } = {},
  ) {
    if (!record.enabled || this.work.paused) return;
    for (const trigger of record.definition.triggers || []) {
      if (trigger.type === 'startup' && !options.skipStartup)
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
    this.debounceDeadlines.settleWithPrefix(`${record.definition.id}\0`);
  }

  private notify() {
    for (const listener of this.listeners) listener();
  }
}
