export type JobOwner = 'host' | 'extension'
export type JobStatus = 'idle' | 'running' | 'succeeded' | 'failed' | 'disabled'
export type JobTrigger =
  | { type: 'manual' }
  | { type: 'startup'; delayMs?: number }
  | { type: 'interval'; everyMs: number; delayMs?: number }
  | { type: 'event'; event: string; debounceMs?: number }

export type JobDefinition = {
  id: string
  title: string
  owner?: JobOwner
  scope?: string
  enabled?: boolean
  triggers?: JobTrigger[]
  timeoutMs?: number
  maxConcurrency?: 1
  run: (context: { id: string; reason: string }) => unknown | Promise<unknown>
}

export type JobSnapshot = {
  id: string
  title: string
  owner: JobOwner
  scope?: string
  status: JobStatus
  enabled: boolean
  running: boolean
  runCount: number
  failureCount: number
  lastReason?: string
  lastStartedAt?: number
  lastFinishedAt?: number
  lastDurationMs?: number
  lastError?: string
  nextRunAt?: number
  triggers: JobTrigger[]
}

type JobRecord = {
  definition: JobDefinition
  enabled: boolean
  running: boolean
  pendingReason: string | null
  runCount: number
  failureCount: number
  status: JobStatus
  lastReason?: string
  lastStartedAt?: number
  lastFinishedAt?: number
  lastDurationMs?: number
  lastError?: string
  nextRunAt?: number
  timers: NodeJS.Timeout[]
  scheduledTimers: Map<string, NodeJS.Timeout>
}

function timeout<T>(promise: Promise<T>, timeoutMs: number, title: string) {
  let timer: NodeJS.Timeout | undefined
  const guard = new Promise<T>((_, reject) => {
    timer = setTimeout(() => reject(new Error(`${title} timed out after ${timeoutMs}ms`)), timeoutMs)
    timer.unref?.()
  })
  return Promise.race([promise, guard]).finally(() => {
    if (timer) clearTimeout(timer)
  })
}

export class JobRegistry {
  private records = new Map<string, JobRecord>()
  private listeners = new Set<() => void>()

  register(definition: JobDefinition) {
    const existing = this.records.get(definition.id)
    if (existing) this.clearTimers(existing)
    const record: JobRecord = existing || {
      definition,
      enabled: definition.enabled !== false,
      running: false,
      pendingReason: null,
      runCount: 0,
      failureCount: 0,
      status: definition.enabled === false ? 'disabled' : 'idle',
      timers: [],
      scheduledTimers: new Map(),
    }
    record.definition = definition
    record.enabled = definition.enabled !== false && record.enabled !== false
    record.status = record.enabled ? (record.running ? 'running' : record.status === 'disabled' ? 'idle' : record.status) : 'disabled'
    record.timers = []
    record.scheduledTimers ||= new Map()
    this.records.set(definition.id, record)
    this.installTriggers(record)
    this.notify()
    return definition.id
  }

  unregister(id: string) {
    const record = this.records.get(id)
    if (!record) return
    this.clearTimers(record)
    this.records.delete(id)
    this.notify()
  }

  unregisterWhere(predicate: (snapshot: JobSnapshot) => boolean) {
    for (const snapshot of this.snapshot()) {
      if (predicate(snapshot)) this.unregister(snapshot.id)
    }
  }

  has(id: string) {
    return this.records.has(id)
  }

  async run(id: string, reason = 'manual') {
    const record = this.records.get(id)
    if (!record) throw new Error(`Unknown background job: ${id}`)
    if (!record.enabled) return null
    if (record.running) {
      record.pendingReason = reason
      this.notify()
      return null
    }
    record.running = true
    record.status = 'running'
    record.lastReason = reason
    record.lastStartedAt = Date.now()
    record.lastError = undefined
    this.notify()
    try {
      const promise = Promise.resolve(record.definition.run({ id, reason }))
      const result = record.definition.timeoutMs ? await timeout(promise, record.definition.timeoutMs, record.definition.title) : await promise
      record.status = 'succeeded'
      record.runCount += 1
      return result
    } catch (error) {
      record.status = 'failed'
      record.failureCount += 1
      record.lastError = error instanceof Error ? error.message : String(error)
      throw error
    } finally {
      record.running = false
      record.lastFinishedAt = Date.now()
      record.lastDurationMs = record.lastStartedAt ? record.lastFinishedAt - record.lastStartedAt : undefined
      const pending = record.pendingReason
      record.pendingReason = null
      this.notify()
      if (pending && record.enabled) void this.run(id, pending).catch(() => {})
    }
  }

  schedule(id: string, reason = 'scheduled', delayMs = 0) {
    const record = this.records.get(id)
    if (!record || !record.enabled) return
    const previous = record.scheduledTimers.get(reason)
    if (previous) clearTimeout(previous)
    record.nextRunAt = Date.now() + Math.max(0, delayMs)
    const timer = setTimeout(() => {
      record.scheduledTimers.delete(reason)
      record.nextRunAt = undefined
      void this.run(id, reason).catch(() => {})
    }, Math.max(0, delayMs))
    timer.unref?.()
    record.scheduledTimers.set(reason, timer)
    this.notify()
  }

  emit(event: string) {
    for (const record of this.records.values()) {
      for (const trigger of record.definition.triggers || []) {
        if (trigger.type !== 'event' || trigger.event !== event) continue
        this.schedule(record.definition.id, `event:${event}`, trigger.debounceMs || 0)
      }
    }
  }

  setEnabled(id: string, enabled: boolean) {
    const record = this.records.get(id)
    if (!record) return false
    record.enabled = enabled
    record.status = enabled ? 'idle' : 'disabled'
    if (!enabled) this.clearTimers(record)
    else this.installTriggers(record)
    this.notify()
    return true
  }

  clearError(id: string) {
    const record = this.records.get(id)
    if (!record) return false
    record.lastError = undefined
    if (!record.running && record.status === 'failed') record.status = 'idle'
    this.notify()
    return true
  }

  snapshot(): JobSnapshot[] {
    return Array.from(this.records.values()).map((record) => ({
      id: record.definition.id,
      title: record.definition.title,
      owner: record.definition.owner || 'host',
      scope: record.definition.scope,
      status: record.status,
      enabled: record.enabled,
      running: record.running,
      runCount: record.runCount,
      failureCount: record.failureCount,
      lastReason: record.lastReason,
      lastStartedAt: record.lastStartedAt,
      lastFinishedAt: record.lastFinishedAt,
      lastDurationMs: record.lastDurationMs,
      lastError: record.lastError,
      nextRunAt: record.nextRunAt,
      triggers: record.definition.triggers || [],
    })).sort((a, b) => a.title.localeCompare(b.title))
  }

  onChange(listener: () => void) {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  clear() {
    for (const record of this.records.values()) this.clearTimers(record)
    this.records.clear()
    this.notify()
  }

  private installTriggers(record: JobRecord) {
    if (!record.enabled) return
    for (const trigger of record.definition.triggers || []) {
      if (trigger.type === 'startup') this.schedule(record.definition.id, 'startup', trigger.delayMs || 0)
      if (trigger.type === 'interval') {
        this.schedule(record.definition.id, 'interval', trigger.delayMs ?? trigger.everyMs)
        const timer = setInterval(() => void this.run(record.definition.id, 'interval').catch(() => {}), Math.max(1000, trigger.everyMs))
        timer.unref?.()
        record.timers.push(timer)
      }
    }
  }

  private clearTimers(record: JobRecord) {
    for (const timer of record.timers) clearTimeout(timer)
    for (const timer of record.scheduledTimers.values()) clearTimeout(timer)
    record.timers = []
    record.scheduledTimers.clear()
    record.nextRunAt = undefined
  }

  private notify() {
    for (const listener of this.listeners) listener()
  }
}
