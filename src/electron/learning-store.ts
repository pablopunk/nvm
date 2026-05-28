import fs from 'node:fs/promises'
import path from 'node:path'

export type LearningMessageRole = 'user' | 'assistant' | 'system'
export type LearningKind = 'environment' | 'workflow' | 'preference'
export type LearningConfidence = 'low' | 'medium' | 'high'
export type LearningStatus = 'observed' | 'active' | 'rejected'

export type LearningTraceMessage = {
  role: LearningMessageRole
  content: string
  createdAt: number
}

export type LearningTraceToolCall = {
  id: string
  name: string
  startedAt: number
  endedAt?: number
  inputSummary?: unknown
  outputSummary?: unknown
  ok?: boolean
  error?: string
}

export type LearningTraceExtensionEvent = {
  kind: 'write_extension' | 'validate_extension' | 'remove_extension' | 'install_extension'
  filename?: string
  extensionId?: string
  commandIds?: string[]
  ok: boolean
  error?: string
  createdAt: number
  details?: unknown
}

export type LearningTraceStatusEvent = {
  status: 'start' | 'done' | 'error' | 'aborted'
  createdAt: number
  error?: string
}

export type StoredLearningTrace = {
  id: string
  chatId: string
  query?: string
  title?: string
  createdAt: number
  updatedAt: number
  reviewedAt?: number
  contextExtensionFile?: string
  extensionFiles: string[]
  messages: LearningTraceMessage[]
  toolCalls: LearningTraceToolCall[]
  extensionEvents: LearningTraceExtensionEvent[]
  statusEvents: LearningTraceStatusEvent[]
}

export type StoredLearning = {
  id: string
  fingerprint: string
  kind: LearningKind
  summary: string
  appliesWhen?: string
  keywords: string[]
  confidence: LearningConfidence
  status: LearningStatus
  evidence: string
  evidenceChatIds: string[]
  extensionFiles: string[]
  toolNames: string[]
  reinforcementCount: number
  createdAt: number
  updatedAt: number
}

type LearningStoreState = {
  version: 1
  traces: StoredLearningTrace[]
  learnings: StoredLearning[]
}

type LocalLearningStorePaths = {
  tracesPath: string
  learningsPath: string
  legacyLearningsPath?: string
}

type TraceMetadata = {
  query?: string
  title?: string
  contextExtensionFile?: string
  extensionFiles?: string[]
}

type ReviewLearningInput = {
  kind: LearningKind
  summary: string
  appliesWhen?: string
  keywords?: string[]
  confidence?: LearningConfidence
  evidence?: string
}

const MAX_TRACE_COUNT = 200
const MAX_TRACE_MESSAGES = 160
const MAX_TRACE_TOOL_CALLS = 80
const MAX_TRACE_EXTENSION_EVENTS = 80
const MAX_TRACE_STATUSES = 40
const MAX_LEARNING_COUNT = 200
const MAX_MESSAGE_LENGTH = 4_000

function limitedText(value: unknown, maxLength = MAX_MESSAGE_LENGTH) {
  return String(value || '').trim().slice(0, maxLength)
}

function normalizedKeywords(keywords: unknown) {
  return Array.from(new Set((Array.isArray(keywords) ? keywords : [])
    .map((value) => String(value || '').trim().toLowerCase())
    .filter(Boolean)))
}

function normalizedExtensionFiles(extensionFiles: unknown) {
  return Array.from(new Set((Array.isArray(extensionFiles) ? extensionFiles : [])
    .map((value) => path.basename(String(value || '').trim()))
    .filter(Boolean)))
}

function learningFingerprint(kind: LearningKind, summary: string) {
  return `${kind}:${summary.trim().toLowerCase().replace(/\s+/g, ' ')}`
}

function deepClone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value))
}

export class LocalLearningStore {
  private state: LearningStoreState = { version: 1, traces: [], learnings: [] }
  private loaded = false
  private saveTimer: NodeJS.Timeout | null = null
  private openToolCallIds = new Map<string, string[]>()

  constructor(private readonly paths: LocalLearningStorePaths) {}

  async load() {
    if (this.loaded) return
    this.loaded = true
    const [tracesText, learningsText] = await Promise.all([
      fs.readFile(this.paths.tracesPath, 'utf8').catch(() => ''),
      readLearningRulesText(this.paths).catch(() => ''),
    ])
    try {
      const legacy = tracesText ? null : parseLegacyCombinedState(learningsText)
      const parsedTraces = tracesText ? JSON.parse(tracesText) as Partial<Pick<LearningStoreState, 'traces'>> : legacy
      const parsedLearnings = parseLearningsContent(learningsText) || legacy
      this.state = {
        version: 1,
        traces: Array.isArray(parsedTraces?.traces) ? parsedTraces.traces.map((trace) => this.normalizedTrace(trace)) : [],
        learnings: Array.isArray(parsedLearnings?.learnings) ? parsedLearnings.learnings.map((learning) => this.normalizedLearning(learning)) : [],
      }
      this.compactLearnings()
    } catch {
      this.state = { version: 1, traces: [], learnings: [] }
    }
  }

  snapshot() {
    return deepClone(this.state)
  }

  upsertTraceMetadata(chatId: string, metadata: TraceMetadata = {}) {
    this.ensureTrace(chatId, metadata)
    this.scheduleSave()
  }

  appendMessage(chatId: string, role: LearningMessageRole, content: string, metadata: TraceMetadata = {}) {
    const trace = this.ensureTrace(chatId, metadata)
    const text = limitedText(content)
    if (!text) return
    trace.messages.push({ role, content: text, createdAt: Date.now() })
    trace.messages = trace.messages.slice(-MAX_TRACE_MESSAGES)
    trace.updatedAt = Date.now()
    this.scheduleSave()
  }

  appendAssistantDelta(chatId: string, text: string, metadata: TraceMetadata = {}) {
    const trace = this.ensureTrace(chatId, metadata)
    const delta = limitedText(text)
    if (!delta) return
    const last = trace.messages[trace.messages.length - 1]
    if (last?.role === 'assistant') last.content = `${last.content}${delta}`.slice(0, MAX_MESSAGE_LENGTH)
    else trace.messages.push({ role: 'assistant', content: delta, createdAt: Date.now() })
    trace.messages = trace.messages.slice(-MAX_TRACE_MESSAGES)
    trace.updatedAt = Date.now()
    this.scheduleSave()
  }

  recordToolStart(chatId: string, name: string, inputSummary?: unknown, metadata: TraceMetadata = {}, preferredId?: string) {
    const trace = this.ensureTrace(chatId, metadata)
    const toolCallId = preferredId || `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
    trace.toolCalls.push({ id: toolCallId, name: String(name || 'tool'), startedAt: Date.now(), inputSummary: sanitizedData(inputSummary) })
    trace.toolCalls = trace.toolCalls.slice(-MAX_TRACE_TOOL_CALLS)
    trace.updatedAt = Date.now()
    this.trackOpenToolCall(trace.id, name, toolCallId)
    this.scheduleSave()
    return toolCallId
  }

  recordToolEnd(chatId: string, name: string, detail: { ok?: boolean; outputSummary?: unknown; error?: string; toolCallId?: string } = {}, metadata: TraceMetadata = {}) {
    const trace = this.ensureTrace(chatId, metadata)
    const toolCallId = detail.toolCallId || this.openToolCallId(trace.id, name)
    const toolCall = trace.toolCalls.find((item) => item.id === toolCallId) || lastMatchingToolCall(trace.toolCalls, name)
    if (!toolCall) return
    toolCall.endedAt = Date.now()
    toolCall.ok = detail.ok ?? !detail.error
    toolCall.outputSummary = sanitizedData(detail.outputSummary)
    toolCall.error = detail.error ? limitedText(detail.error, 2_000) : undefined
    trace.updatedAt = Date.now()
    this.finishOpenToolCall(trace.id, name, toolCall.id)
    this.scheduleSave()
  }

  recordExtensionEvent(chatId: string, event: Omit<LearningTraceExtensionEvent, 'createdAt'>, metadata: TraceMetadata = {}) {
    const trace = this.ensureTrace(chatId, metadata)
    trace.extensionEvents.push({
      ...event,
      filename: event.filename ? path.basename(event.filename) : undefined,
      commandIds: Array.isArray(event.commandIds) ? event.commandIds.filter(Boolean).map(String) : undefined,
      createdAt: Date.now(),
      details: sanitizedData(event.details),
    })
    trace.extensionEvents = trace.extensionEvents.slice(-MAX_TRACE_EXTENSION_EVENTS)
    trace.updatedAt = Date.now()
    this.scheduleSave()
  }

  recordStatus(chatId: string, status: LearningTraceStatusEvent['status'], metadata: TraceMetadata = {}, error?: string) {
    const trace = this.ensureTrace(chatId, metadata)
    trace.statusEvents.push({ status, createdAt: Date.now(), error: error ? limitedText(error, 2_000) : undefined })
    trace.statusEvents = trace.statusEvents.slice(-MAX_TRACE_STATUSES)
    trace.updatedAt = Date.now()
    this.scheduleSave()
  }

  shouldReview(chatId: string) {
    const trace = this.state.traces.find((item) => item.chatId === chatId)
    if (!trace) return false
    if (trace.updatedAt <= (trace.reviewedAt || 0)) return false
    return trace.toolCalls.length > 0 || trace.messages.length > 2 || trace.extensionEvents.length > 0
  }

  markReviewed(chatId: string) {
    const trace = this.state.traces.find((item) => item.chatId === chatId)
    if (!trace) return
    trace.reviewedAt = Date.now()
    trace.updatedAt = Date.now()
    this.scheduleSave()
  }

  replaceLearningsFromReview(chatId: string, reviewLearnings: ReviewLearningInput[]) {
    const trace = this.state.traces.find((item) => item.chatId === chatId)
    if (!trace) return []
    const now = Date.now()
    const next = [] as StoredLearning[]
    for (const item of reviewLearnings) {
      const summary = limitedText(item.summary, 300)
      if (!summary) continue
      const kind = item.kind === 'workflow' || item.kind === 'preference' ? item.kind : 'environment'
      if (!qualifiesUserLearning({ ...item, kind, summary })) continue
      const fingerprint = learningFingerprint(kind, summary)
      const existing = this.state.learnings.find((learning) => learning.fingerprint === fingerprint)
      next.push({
        id: existing?.id || `${now}-${Math.random().toString(36).slice(2, 10)}`,
        fingerprint,
        kind,
        summary,
        appliesWhen: item.appliesWhen ? limitedText(item.appliesWhen, 300) : undefined,
        keywords: normalizedKeywords(item.keywords),
        confidence: item.confidence === 'low' || item.confidence === 'high' ? item.confidence : 'medium',
        status: 'active',
        evidence: limitedText(item.evidence || '', 500),
        evidenceChatIds: Array.from(new Set([...(existing?.evidenceChatIds || []), chatId])),
        extensionFiles: Array.from(new Set([...(existing?.extensionFiles || []), ...trace.extensionFiles])),
        toolNames: Array.from(new Set([...(existing?.toolNames || []), ...trace.toolCalls.map((toolCall) => toolCall.name)])),
        reinforcementCount: Math.max(1, Number(existing?.reinforcementCount || 1)),
        createdAt: existing?.createdAt || now,
        updatedAt: now,
      })
    }
    this.state.learnings = next
      .sort((left, right) => right.updatedAt - left.updatedAt)
      .slice(0, MAX_LEARNING_COUNT)
    this.markReviewed(chatId)
    this.scheduleSave()
    return this.state.learnings.map((learning) => deepClone(learning))
  }

  private compactLearnings() {
    this.state.learnings = this.state.learnings
      .filter((learning) => preservesLearning(learning, this.state.traces))
      .sort((left, right) => right.updatedAt - left.updatedAt)
      .slice(0, MAX_LEARNING_COUNT)
  }

  relevantLearnings(input: { message?: string; query?: string; contextExtensionFile?: string; limit?: number }) {
    const haystack = `${String(input.query || '')}\n${String(input.message || '')}`.toLowerCase()
    const contextFile = path.basename(String(input.contextExtensionFile || ''))
    const limit = Math.max(1, Number(input.limit || 4))
    return this.state.learnings
      .filter((learning) => learning.status === 'active')
      .map((learning) => ({ learning, score: relevanceScore(learning, haystack, contextFile) }))
      .filter((entry) => entry.score > 0)
      .sort((left, right) => right.score - left.score || right.learning.updatedAt - left.learning.updatedAt)
      .slice(0, limit)
      .map((entry) => deepClone(entry.learning))
  }

  reviewSnapshot(chatId: string) {
    const trace = this.state.traces.find((item) => item.chatId === chatId)
    if (!trace) return null
    return deepClone({
      chatId: trace.chatId,
      query: trace.query,
      title: trace.title,
      contextExtensionFile: trace.contextExtensionFile,
      extensionFiles: trace.extensionFiles,
      messages: trace.messages.slice(-20),
      toolCalls: trace.toolCalls.slice(-20),
      extensionEvents: trace.extensionEvents.slice(-20),
      statusEvents: trace.statusEvents.slice(-10),
      activeLearnings: this.state.learnings.filter((learning) => learning.status === 'active').slice(0, 20),
      traceSummary: {
        userMessages: trace.messages.filter((message) => message.role === 'user').length,
        assistantMessages: trace.messages.filter((message) => message.role === 'assistant').length,
        toolCalls: trace.toolCalls.length,
        extensionEvents: trace.extensionEvents.length,
        statusEvents: trace.statusEvents.length,
      },
    })
  }

  private ensureTrace(chatId: string, metadata: TraceMetadata = {}) {
    let trace = this.state.traces.find((item) => item.chatId === chatId)
    if (!trace) {
      trace = {
        id: chatId,
        chatId,
        query: metadata.query ? String(metadata.query) : undefined,
        title: metadata.title ? String(metadata.title) : undefined,
        createdAt: Date.now(),
        updatedAt: Date.now(),
        contextExtensionFile: metadata.contextExtensionFile ? path.basename(metadata.contextExtensionFile) : undefined,
        extensionFiles: normalizedExtensionFiles(metadata.extensionFiles),
        messages: [],
        toolCalls: [],
        extensionEvents: [],
        statusEvents: [],
      }
      this.state.traces.unshift(trace)
      this.state.traces = this.state.traces.slice(0, MAX_TRACE_COUNT)
    }
    if (metadata.query) trace.query = String(metadata.query)
    if (metadata.title) trace.title = String(metadata.title)
    if (metadata.contextExtensionFile) trace.contextExtensionFile = path.basename(metadata.contextExtensionFile)
    if (metadata.extensionFiles?.length) trace.extensionFiles = Array.from(new Set([...trace.extensionFiles, ...normalizedExtensionFiles(metadata.extensionFiles)]))
    trace.updatedAt = Date.now()
    return trace
  }

  private normalizedTrace(trace: Partial<StoredLearningTrace>) {
    return {
      id: String(trace.id || trace.chatId || `${Date.now()}`),
      chatId: String(trace.chatId || trace.id || `${Date.now()}`),
      query: trace.query ? String(trace.query) : undefined,
      title: trace.title ? String(trace.title) : undefined,
      createdAt: Number(trace.createdAt || Date.now()),
      updatedAt: Number(trace.updatedAt || trace.createdAt || Date.now()),
      reviewedAt: trace.reviewedAt ? Number(trace.reviewedAt) : undefined,
      contextExtensionFile: trace.contextExtensionFile ? path.basename(String(trace.contextExtensionFile)) : undefined,
      extensionFiles: normalizedExtensionFiles(trace.extensionFiles),
      messages: Array.isArray(trace.messages) ? trace.messages.map((message) => ({ role: message.role === 'user' || message.role === 'system' ? message.role : 'assistant', content: limitedText(message.content), createdAt: Number(message.createdAt || Date.now()) })) : [],
      toolCalls: Array.isArray(trace.toolCalls) ? trace.toolCalls.map((toolCall) => ({ id: String(toolCall.id || `${Date.now()}`), name: limitedText(toolCall.name, 100), startedAt: Number(toolCall.startedAt || Date.now()), endedAt: toolCall.endedAt ? Number(toolCall.endedAt) : undefined, inputSummary: sanitizedData(toolCall.inputSummary), outputSummary: sanitizedData(toolCall.outputSummary), ok: toolCall.ok, error: toolCall.error ? limitedText(toolCall.error, 2_000) : undefined })) : [],
      extensionEvents: Array.isArray(trace.extensionEvents) ? trace.extensionEvents.map((event) => ({ kind: event.kind === 'validate_extension' || event.kind === 'remove_extension' || event.kind === 'install_extension' ? event.kind : 'write_extension', filename: event.filename ? path.basename(String(event.filename)) : undefined, extensionId: event.extensionId ? String(event.extensionId) : undefined, commandIds: Array.isArray(event.commandIds) ? event.commandIds.map(String) : undefined, ok: Boolean(event.ok), error: event.error ? limitedText(event.error, 2_000) : undefined, createdAt: Number(event.createdAt || Date.now()), details: sanitizedData(event.details) })) : [],
      statusEvents: Array.isArray(trace.statusEvents) ? trace.statusEvents.map((event) => ({ status: event.status === 'done' || event.status === 'error' || event.status === 'aborted' ? event.status : 'start', createdAt: Number(event.createdAt || Date.now()), error: event.error ? limitedText(event.error, 2_000) : undefined })) : [],
    } satisfies StoredLearningTrace
  }

  private normalizedLearning(learning: Partial<StoredLearning>) {
    const kind = learning.kind === 'workflow' || learning.kind === 'preference' ? learning.kind : 'environment'
    const summary = limitedText(learning.summary, 300)
    return {
      id: String(learning.id || `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`),
      fingerprint: learningFingerprint(kind, summary),
      kind,
      summary,
      appliesWhen: learning.appliesWhen ? limitedText(learning.appliesWhen, 300) : undefined,
      keywords: normalizedKeywords(learning.keywords),
      confidence: learning.confidence === 'low' || learning.confidence === 'high' ? learning.confidence : 'medium',
      status: learning.status === 'active' || learning.status === 'rejected' ? learning.status : 'observed',
      evidence: limitedText(learning.evidence, 500),
      evidenceChatIds: Array.isArray(learning.evidenceChatIds) ? learning.evidenceChatIds.map(String) : [],
      extensionFiles: normalizedExtensionFiles(learning.extensionFiles),
      toolNames: Array.isArray(learning.toolNames) ? learning.toolNames.map((toolName) => limitedText(toolName, 100)).filter(Boolean) : [],
      reinforcementCount: Math.max(1, Number(learning.reinforcementCount || 1)),
      createdAt: Number(learning.createdAt || Date.now()),
      updatedAt: Number(learning.updatedAt || learning.createdAt || Date.now()),
    } satisfies StoredLearning
  }

  private trackOpenToolCall(traceId: string, name: string, toolCallId: string) {
    const key = `${traceId}:${name}`
    this.openToolCallIds.set(key, [...(this.openToolCallIds.get(key) || []), toolCallId])
  }

  private openToolCallId(traceId: string, name: string) {
    const key = `${traceId}:${name}`
    const ids = this.openToolCallIds.get(key) || []
    return ids[ids.length - 1]
  }

  private finishOpenToolCall(traceId: string, name: string, toolCallId: string) {
    const key = `${traceId}:${name}`
    const remaining = (this.openToolCallIds.get(key) || []).filter((id) => id !== toolCallId)
    if (remaining.length) this.openToolCallIds.set(key, remaining)
    else this.openToolCallIds.delete(key)
  }

  private scheduleSave() {
    if (this.saveTimer) clearTimeout(this.saveTimer)
    this.saveTimer = setTimeout(() => {
      this.saveTimer = null
      void this.save().catch(() => {})
    }, 200)
    this.saveTimer.unref?.()
  }

  private async save() {
    await Promise.all([
      fs.mkdir(path.dirname(this.paths.tracesPath), { recursive: true }),
      fs.mkdir(path.dirname(this.paths.learningsPath), { recursive: true }),
    ])
    await Promise.all([
      fs.writeFile(this.paths.tracesPath, JSON.stringify({ version: 1, traces: this.state.traces }, null, 2)),
      fs.writeFile(this.paths.learningsPath, renderLearningsMarkdown(this.state.learnings)),
    ])
  }
}

function strongestConfidence(left: LearningConfidence, right?: LearningConfidence) {
  if (left === 'high' || right === 'high') return 'high'
  if (left === 'medium' || right === 'medium') return 'medium'
  return 'low'
}

function relevanceScore(learning: StoredLearning, haystack: string, contextExtensionFile: string) {
  let score = learning.confidence === 'high' ? 4 : learning.confidence === 'medium' ? 3 : 1
  if (learning.kind === 'environment') score += 2
  if (contextExtensionFile && learning.extensionFiles.includes(contextExtensionFile)) score += 4
  if (learning.keywords.length === 0) score += 1
  for (const keyword of learning.keywords) {
    if (haystack.includes(keyword)) score += 3
  }
  if (learning.appliesWhen && haystack.includes(learning.appliesWhen.toLowerCase())) score += 2
  return score
}

function sanitizedData(value: unknown) {
  if (value == null) return undefined
  try {
    return JSON.parse(JSON.stringify(value))
  } catch {
    return limitedText(value, 2_000)
  }
}

function lastMatchingToolCall(toolCalls: LearningTraceToolCall[], name: string) {
  for (let index = toolCalls.length - 1; index >= 0; index -= 1) {
    const toolCall = toolCalls[index]
    if (toolCall.name === name && toolCall.endedAt == null) return toolCall
  }
  return undefined
}

function qualifiesUserLearning(input: ReviewLearningInput & { summary: string; kind: LearningKind }) {
  const haystack = `${input.summary}\n${input.appliesWhen || ''}\n${input.evidence || ''}\n${(input.keywords || []).join(' ')}`.toLowerCase()
  if (containsBlockedLearningPattern(haystack)) return false
  return true
}

function preservesLearning(learning: StoredLearning, traces: StoredLearningTrace[]) {
  const haystack = `${learning.summary}\n${learning.appliesWhen || ''}\n${learning.evidence || ''}\n${learning.keywords.join(' ')}`.toLowerCase()
  if (containsBlockedLearningPattern(haystack)) return false
  const relatedTrace = traces.find((trace) => learning.evidenceChatIds.includes(trace.chatId))
  return relatedTrace ? relatedTrace.toolCalls.length > 0 || relatedTrace.extensionEvents.length > 0 : learning.reinforcementCount >= 1
}

function containsBlockedLearningPattern(haystack: string) {
  return /extensionsdir|application support\/nvm|reference existing|pattern matching|get inspired|local extension implementations|stored in \/|extension files live|media-compressor|\/users\//.test(haystack)
}

function parseLegacyCombinedState(text: string): Partial<LearningStoreState> | null {
  if (!text) return null
  try {
    const parsed = JSON.parse(text) as Partial<LearningStoreState>
    return Array.isArray(parsed.traces) || Array.isArray(parsed.learnings) ? parsed : null
  } catch {
    return null
  }
}

async function readLearningRulesText(paths: LocalLearningStorePaths) {
  const current = await fs.readFile(paths.learningsPath, 'utf8').catch(() => '')
  if (current) return current
  if (!paths.legacyLearningsPath) return ''
  return fs.readFile(paths.legacyLearningsPath, 'utf8').catch(() => '')
}

function parseLearningsContent(text: string): { learnings?: Partial<StoredLearning>[] } | null {
  if (!text) return null
  const json = parseLegacyCombinedState(text)
  if (json?.learnings) return { learnings: json.learnings }
  return { learnings: parseLearningsMarkdown(text) }
}

function parseLearningsMarkdown(markdown: string) {
  const learnings = [] as Partial<StoredLearning>[]
  const blocks = markdown.split(/^## /m).slice(1)
  for (const block of blocks) {
    const lines = block.split('\n')
    const heading = lines.shift() || ''
    const kind = normalizedKindFromHeading(heading)
    let current: Partial<StoredLearning> | null = null
    for (const rawLine of lines) {
      const line = rawLine.trim()
      if (!line) continue
      if (line.startsWith('- ')) {
        if (current?.summary) learnings.push({ ...current, kind })
        current = { kind, summary: line.slice(2).trim(), status: 'active' }
        continue
      }
      if (!current) continue
      if (line.startsWith('When: ')) current.appliesWhen = line.slice('When: '.length).trim()
      else if (line.startsWith('Keywords: ')) current.keywords = line.slice('Keywords: '.length).split(',').map((item) => item.trim()).filter(Boolean)
      else if (line.startsWith('Confidence: ')) current.confidence = normalizeConfidence(line.slice('Confidence: '.length).trim())
      else if (line.startsWith('Evidence: ')) current.evidence = line.slice('Evidence: '.length).trim()
    }
    if (current?.summary) learnings.push({ ...current, kind })
  }
  return learnings
}

function renderLearningsMarkdown(learnings: StoredLearning[]) {
  const sections: Array<{ heading: string; kind: LearningKind }> = [
    { heading: 'Environment', kind: 'environment' },
    { heading: 'Workflow', kind: 'workflow' },
    { heading: 'Preferences', kind: 'preference' },
  ]
  const lines = [
    '# AI Learnings',
    '',
    '<!-- Canonical user learnings for future Nevermind extension-building chats. Keep this file small, generic, and merged. -->',
    '',
  ]
  for (const section of sections) {
    lines.push(`## ${section.heading}`, '')
    const items = learnings.filter((learning) => learning.status === 'active' && learning.kind === section.kind)
    if (!items.length) {
      lines.push('_None_', '')
      continue
    }
    for (const learning of items) {
      lines.push(`- ${learning.summary}`)
      if (learning.appliesWhen) lines.push(`  When: ${learning.appliesWhen}`)
      if (learning.keywords.length) lines.push(`  Keywords: ${learning.keywords.join(', ')}`)
      lines.push(`  Confidence: ${learning.confidence}`)
      if (learning.evidence) lines.push(`  Evidence: ${learning.evidence}`)
      lines.push('')
    }
  }
  return `${lines.join('\n').replace(/\n{3,}/g, '\n\n').trim()}\n`
}

function normalizedKindFromHeading(heading: string): LearningKind {
  const normalized = heading.toLowerCase()
  if (normalized.startsWith('workflow')) return 'workflow'
  if (normalized.startsWith('preference')) return 'preference'
  return 'environment'
}

function normalizeConfidence(value: string): LearningConfidence {
  return value === 'low' || value === 'high' ? value : 'medium'
}
