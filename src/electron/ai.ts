import fs from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import * as logger from './logger'
import { readRecentLogs, type LogLevel, type LogSource } from './logger'
import { checkNevermindCompatibility } from './nevermind-compatibility'
import type { CommandAction } from '../model'
import { nevermindDesktopHeaders } from './nevermind-api'
import { getNevermindAuth, NevermindAuthRequiredError } from './nevermind-auth'

type AiEvent = {
  type: string
  chatId?: string
  text?: string
  message?: string
  name?: string
  label?: string
  data?: unknown
  isError?: boolean
}

type AiLimitNotice = {
  kind: 'insufficient_credits' | 'rate_limited' | 'prompt_too_large' | 'unsupported_client'
  title: string
  message: string
  actionTitle?: string
  dashboardUrl?: string
  action?: CommandAction
  retryAfterSec?: number
}

const NEVERMIND_DASHBOARD_URL = 'https://nvm.fyi/dashboard'
const NEVERMIND_UPDATE_URL = 'https://github.com/pablopunk/nvm/releases/latest'

type ActiveChat = {
  id?: string
  title?: string
  query?: string
  contextExtensionFile?: string
  touchedExtensionFiles?: string[]
  generatedExtensionFile?: string
}

type NevermindAiOptions = {
  agentDir: string
  workspaceDir: string
  extensionsDir: string
  extensionApiPath: string
  extensionTypesPath: string
  skillPath: string
  reloadExtensions: () => Promise<unknown> | unknown
  getShortcuts?: () => Array<{ actionId: string; title: string; subtitle?: string; accelerator: string; scope: 'global'; source: 'user' | 'extension' }>
  getPaletteShortcut?: () => { title: string; accelerator: string; scope: 'palette' }
  getExtensionRuntimeState?: (filename: string) => {
    loaded: boolean
    extensionId?: string
    commandIds: string[]
  }
  getActiveChat?: () => ActiveChat | null
  getChat?: (chatId: string) => ActiveChat | null
  markGeneratedExtension?: (filePath: string, chatId?: string) => void
  canWriteExtension?: (filename: string, chatId?: string) => boolean
  removeExtension?: (filename: string, chatId?: string) => Promise<{ removed: boolean; filePath?: string }> | { removed: boolean; filePath?: string }
  addAliasForChat?: (chatId: string) => void
  onEvent?: (event: AiEvent) => void
}

type AiImageContent = { type: 'image'; data: string; mimeType: string }

type AiPromptOptions = {
  sessionId?: string
  system?: string
  context?: string
  images?: AiImageContent[]
  signal?: { aborted?: boolean; addEventListener?: (type: 'abort', listener: () => void, options?: { once?: boolean }) => void; removeEventListener?: (type: 'abort', listener: () => void) => void }
  onEvent?: (event: AiEvent) => void
}

type AgentSession = {
  prompt: (message: string, options?: { images?: AiImageContent[] }) => Promise<unknown>
  abort?: () => Promise<unknown>
  dispose?: () => void
  subscribe: (callback: (event: AgentSessionEvent) => void) => () => void
  agent: { state: { tools: Array<{ name: string }> } }
}

type AgentSessionEvent = { type: string; [key: string]: unknown }

type MessageUpdateEvent = AgentSessionEvent & {
  type: 'message_update'
  assistantMessageEvent: { type: 'text_delta'; delta: string }
}

type ToolExecutionStartEvent = AgentSessionEvent & {
  type: 'tool_execution_start'
  toolName: string
}

type ToolExecutionEndEvent = AgentSessionEvent & {
  type: 'tool_execution_end'
  toolName: string
  isError: boolean
}

type AssistantMessageEndEvent = AgentSessionEvent & {
  type: 'message_end'
  message: { role: 'assistant'; stopReason?: string; errorMessage?: string }
}

type SessionEntry = {
  unsubscribe: (() => void) | null
  promise: Promise<AgentSession>
}

type PiApi = any
type TypeApi = any


function isMessageUpdateEvent(event: AgentSessionEvent): event is MessageUpdateEvent {
  const assistantMessageEvent = event.assistantMessageEvent as { type?: unknown; delta?: unknown } | undefined
  return event.type === 'message_update' && assistantMessageEvent?.type === 'text_delta' && typeof assistantMessageEvent.delta === 'string'
}

function isToolExecutionStartEvent(event: AgentSessionEvent): event is ToolExecutionStartEvent {
  return event.type === 'tool_execution_start' && typeof event.toolName === 'string'
}

function isToolExecutionEndEvent(event: AgentSessionEvent): event is ToolExecutionEndEvent {
  return event.type === 'tool_execution_end' && typeof event.toolName === 'string' && typeof event.isError === 'boolean'
}

function isAssistantErrorEndEvent(event: AgentSessionEvent): event is AssistantMessageEndEvent {
  const message = event.message as { role?: unknown; stopReason?: unknown } | undefined
  return event.type === 'message_end' && message?.role === 'assistant' && message.stopReason === 'error'
}

function createNevermindAi(options: NevermindAiOptions) {
  const sessions = new Map<string, SessionEntry>()
  const generalSessions = new Map<string, Promise<AgentSession>>()

  async function getSession(chatId = 'default') {
    const current = sessions.get(chatId)
    if (current) return current.promise

    const entry: SessionEntry = {
      unsubscribe: null,
      promise: createSession({ ...options, chatId }, (event) => options.onEvent?.({ ...event, chatId })),
    }
    sessions.set(chatId, entry)
    entry.promise.catch(() => { if (sessions.get(chatId) === entry) sessions.delete(chatId) })
    return entry.promise
  }

  async function send(message: string, chatId = 'default') {
    options.onEvent?.({ type: 'start', chatId })
    try {
      const session = await getSession(chatId)
      await session.prompt(message)
      options.onEvent?.({ type: 'done', chatId })
    } catch (error) {
      logger.error('ai.chat.send.failed', error, { source: 'host', scope: 'ai' })
      const limit = aiLimitNoticeFromError(error)
      options.onEvent?.({ type: 'error', chatId, message: limit?.message || (error instanceof Error ? error.message : String(error)), data: limit })
    }
  }

  async function abort(chatId = 'default') {
    const session = await sessions.get(chatId)?.promise.catch(() => null)
    await session?.abort?.()
    options.onEvent?.({ type: 'aborted', chatId })
  }

  async function reset(chatId = 'default') {
    const entry = sessions.get(chatId)
    if (!entry) return
    if (entry.unsubscribe) entry.unsubscribe()
    const session = await entry.promise.catch(() => null)
    session?.dispose?.()
    sessions.delete(chatId)
  }

  async function ask(message: string, askOptions: AiPromptOptions = {}) {
    const text: string[] = []
    let session: Awaited<ReturnType<typeof generalSession>> | null = null
    let unsubscribe = () => {}
    let removeAbortListener = () => {}
    askOptions.onEvent?.({ type: 'start' })
    try {
      if (askOptions.signal?.aborted) throw aiAbortError()
      session = await generalSession(askOptions)
      unsubscribe = session.subscribe((event) => {
        if (isMessageUpdateEvent(event)) {
          const delta = event.assistantMessageEvent.delta
          text.push(delta)
          askOptions.onEvent?.({ type: 'delta', text: delta })
        }
        if (isToolExecutionStartEvent(event)) askOptions.onEvent?.({ type: 'tool_start', name: event.toolName })
        if (isToolExecutionEndEvent(event)) askOptions.onEvent?.({ type: 'tool_end', name: event.toolName, isError: event.isError })
        if (isAssistantErrorEndEvent(event)) askOptions.onEvent?.({ type: 'error', message: event.message.errorMessage || 'AI request failed', data: aiLimitNoticeFromError(event.message.errorMessage) })
      })
      removeAbortListener = bindAbortSignal(askOptions.signal, () => { void session?.abort?.() })
      if (askOptions.signal?.aborted) throw aiAbortError()
      await session.prompt(aiPromptWithContext(message, askOptions.context), { images: askOptions.images })
      if (askOptions.signal?.aborted) throw aiAbortError()
      askOptions.onEvent?.({ type: 'done' })
      return text.join('')
    } catch (error) {
      if (askOptions.signal?.aborted || isAbortError(error)) {
        askOptions.onEvent?.({ type: 'aborted' })
        throw aiAbortError()
      }
      askOptions.onEvent?.({ type: 'error', message: error instanceof Error ? error.message : String(error), data: aiLimitNoticeFromError(error) })
      throw error
    } finally {
      removeAbortListener()
      unsubscribe()
      if (!askOptions.sessionId) session?.dispose?.()
    }
  }

  function stream(message: string, streamOptions: AiPromptOptions = {}) {
    const controller = new AbortController()
    const removeExternalAbortListener = bindAbortSignal(streamOptions.signal, () => controller.abort())
    const result = ask(message, { ...streamOptions, signal: controller.signal }).finally(removeExternalAbortListener)
    return {
      result,
      abort: () => controller.abort(),
    }
  }

  async function generalSession(sessionOptions: { sessionId?: string; system?: string }) {
    if (!sessionOptions.sessionId) return createGeneralSession(options, sessionOptions)
    const key = sessionOptions.sessionId
    let promise = generalSessions.get(key)
    if (!promise) {
      promise = createGeneralSession(options, sessionOptions)
      generalSessions.set(key, promise)
      promise.catch(() => { if (generalSessions.get(key) === promise) generalSessions.delete(key) })
    }
    return promise
  }

  function session(sessionId: string, sessionOptions: { system?: string } = {}) {
    return {
      ask: (message: string, options: AiPromptOptions = {}) => ask(message, { ...sessionOptions, ...options, sessionId }),
      stream: (message: string, options: AiPromptOptions = {}) => stream(message, { ...sessionOptions, ...options, sessionId }),
      reset: async () => {
        const current = await generalSessions.get(sessionId)?.catch(() => null)
        current?.dispose?.()
        generalSessions.delete(sessionId)
      },
    }
  }

  async function disposeAllSessions() {
    const chatIds = Array.from(sessions.keys())
    for (const chatId of chatIds) await reset(chatId)
    const generalIds = Array.from(generalSessions.keys())
    for (const id of generalIds) {
      const s = await generalSessions.get(id)?.catch(() => null)
      s?.dispose?.()
      generalSessions.delete(id)
    }
  }

  return { send, abort, reset, ask, stream, session, disposeAllSessions }

  async function createSession({ agentDir, workspaceDir, extensionsDir, extensionApiPath, extensionTypesPath, skillPath, chatId = 'default', reloadExtensions, getExtensionRuntimeState, getActiveChat, getChat, markGeneratedExtension, canWriteExtension, removeExtension, addAliasForChat, onEvent }: NevermindAiOptions & { chatId?: string }, emit: (event: AiEvent) => void) {
    await fs.mkdir(agentDir, { recursive: true })
    await fs.mkdir(workspaceDir, { recursive: true })
    await fs.mkdir(extensionsDir, { recursive: true })

    const [pi, ai] = await Promise.all([
      import('@earendil-works/pi-coding-agent') as Promise<PiApi>,
      import('@earendil-works/pi-ai') as Promise<any>,
    ])

    const authStorage = pi.AuthStorage.create(path.join(agentDir, 'auth.json'))
    const { model, source: modelSource } = await resolveAiModelAndAuth(pi, ai, authStorage)
    const modelRegistry = pi.ModelRegistry.inMemory(authStorage)
    onEvent?.({ type: 'debug', label: 'model-source', data: { source: modelSource, baseUrl: model.baseUrl } })
    const modelDebug = {
      provider: model.provider,
      id: model.id,
      api: model.api,
      reasoning: model.reasoning,
    }
    onEvent?.({ type: 'debug', label: 'model', data: modelDebug })

    const settingsManager = pi.SettingsManager.inMemory({
      compaction: { enabled: true },
      retry: { enabled: true, maxRetries: 2 },
    })

    const resourceLoader = await createResourceLoader(pi, { agentDir, workspaceDir, extensionApiPath, skillPath })
    const customTools = createTools(pi, ai.Type, {
      extensionsDir,
      extensionApiPath,
      extensionTypesPath,
      reloadExtensions,
      getExtensionRuntimeState,
      getActiveChat: () => getChat?.(chatId) || getActiveChat?.() || null,
      markGeneratedExtension: (filePath) => markGeneratedExtension?.(filePath, chatId),
      canWriteExtension: (filename) => canWriteExtension?.(filename, chatId) ?? true,
      removeExtension: (filename) => removeExtension?.(filename, chatId) || { removed: false },
      addAliasForChat,
      emitEvent: onEvent,
    })

    const result = await pi.createAgentSession({
      cwd: workspaceDir,
      agentDir,
      model,
      thinkingLevel: 'low',
      authStorage,
      modelRegistry,
      resourceLoader,
      noTools: 'builtin',
      customTools,
      sessionManager: pi.SessionManager.inMemory(workspaceDir),
      settingsManager,
    }) as { session: AgentSession }

    const toolNames = result.session.agent.state.tools.map((tool) => tool.name)
    onEvent?.({ type: 'debug', label: 'tools', data: toolNames })

    const entry = sessions.get(chatId)
    if (entry) entry.unsubscribe = result.session.subscribe((event) => {
      if (isMessageUpdateEvent(event)) {
        const delta = event.assistantMessageEvent.delta
        if (delta.includes('<tool_calls>') || delta.includes('<tool name=')) {
          logger.warn('ai.rawToolCallText', {
            provider: model.provider,
            id: model.id,
            api: model.api,
            delta,
          }, { source: 'host', scope: 'ai' })
        }
        emit({ type: 'delta', text: delta })
      }
      if (isToolExecutionStartEvent(event)) emit({ type: 'tool_start', name: event.toolName })
      if (isToolExecutionEndEvent(event)) emit({ type: 'tool_end', name: event.toolName, isError: event.isError })
      if (isAssistantErrorEndEvent(event)) {
        const message = event.message.errorMessage || 'AI request failed'
        emit({ type: 'error', message, data: aiLimitNoticeFromError(message) })
      }
    })

    onEvent?.({ type: 'ready' })
    return result.session
  }
}

function aiPromptWithContext(message: string, context?: string) {
  const prompt = String(message || '')
  const trimmedContext = String(context || '').trim()
  if (!trimmedContext) return prompt
  return `Use these attachments as context for the request.\n\n${trimmedContext}\n\nRequest:\n${prompt}`
}

function bindAbortSignal(signal: AiPromptOptions['signal'], abort: () => void) {
  if (!signal?.addEventListener) return () => {}
  if (signal.aborted) {
    abort()
    return () => {}
  }
  const listener = () => abort()
  signal.addEventListener('abort', listener, { once: true })
  return () => signal.removeEventListener?.('abort', listener)
}

function aiAbortError() {
  return Object.assign(new Error('AI request aborted'), { name: 'AbortError', code: 'ai-request-aborted' })
}

function isAbortError(error: unknown) {
  return error instanceof Error && (error.name === 'AbortError' || /aborted/i.test(error.message))
}

function aiLimitNoticeFromError(error: unknown): AiLimitNotice | null {
  const text = searchableErrorText(error)
  const retryAfterSec = Number(text.match(/retry[_ -]?after[^0-9]*(\d+)/i)?.[1] || 0) || undefined
  if (/insufficient[_ -]?credits|no credits remaining|402\b/i.test(text)) {
    return {
      kind: 'insufficient_credits',
      title: 'Credits needed',
      message: 'You’ve reached your Nevermind AI credit limit. Open your dashboard to review your account.',
      actionTitle: 'Open Dashboard',
      dashboardUrl: NEVERMIND_DASHBOARD_URL,
    }
  }
  if (/rate[_ -]?limited|rate limit exceeded|429\b/i.test(text)) {
    return {
      kind: 'rate_limited',
      title: 'AI limit reached',
      message: retryAfterSec ? `You’re sending messages too quickly. Try again in about ${retryAfterSec} seconds, or open your dashboard for account details.` : 'You’ve reached a Nevermind AI usage limit. Open your dashboard for account details.',
      actionTitle: 'Open Dashboard',
      dashboardUrl: NEVERMIND_DASHBOARD_URL,
      retryAfterSec,
    }
  }
  if (/prompt[_ -]?too[_ -]?large|prompt exceeds|413\b/i.test(text)) {
    return {
      kind: 'prompt_too_large',
      title: 'Prompt too large',
      message: 'This request is too large for the current AI limit. Try a shorter message or open your dashboard for account details.',
      actionTitle: 'Open Dashboard',
      dashboardUrl: NEVERMIND_DASHBOARD_URL,
    }
  }
  if (/unsupported[_ -]?client|unsupported[_ -]?desktop|unsupported[_ -]?api|no longer supported|426\b/i.test(text)) {
    return {
      kind: 'unsupported_client',
      title: 'Update Nevermind',
      message: 'This version of Nevermind is no longer supported by the backend. Install the latest version to keep using AI features.',
      actionTitle: 'Check for Update',
      dashboardUrl: updateUrlFromErrorText(text) || NEVERMIND_UPDATE_URL,
      action: { type: 'checkForUpdates', title: 'Check for Update' },
    }
  }
  return null
}

function updateUrlFromErrorText(text: string) {
  return text.match(/https:\/\/[^\s"'}]+/i)?.[0]
}

function searchableErrorText(error: unknown) {
  if (!error) return ''
  if (!(error instanceof Error)) return stringifyError(error)
  const own = Object.fromEntries(Object.getOwnPropertyNames(error).map((key) => [key, (error as any)[key]]))
  return [error.name, error.message, error.stack, stringifyError(own)].filter(Boolean).join('\n')
}

function stringifyError(value: unknown) {
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

async function createGeneralSession(options: NevermindAiOptions, sessionOptions: { sessionId?: string; system?: string }) {
  const { agentDir, workspaceDir } = options
  await fs.mkdir(agentDir, { recursive: true })
  await fs.mkdir(workspaceDir, { recursive: true })
  const [pi, ai] = await Promise.all([
    import('@earendil-works/pi-coding-agent') as Promise<PiApi>,
    import('@earendil-works/pi-ai') as Promise<any>,
  ])
  const authStorage = pi.AuthStorage.create(path.join(agentDir, 'auth.json'))
  const { model } = await resolveAiModelAndAuth(pi, ai, authStorage)
  const resourceLoader = {
    getExtensions: () => ({ extensions: [], errors: [], runtime: pi.createExtensionRuntime() }),
    getSkills: () => ({ skills: [], diagnostics: [] }),
    getPrompts: () => ({ prompts: [], diagnostics: [] }),
    getThemes: () => ({ themes: [], diagnostics: [] }),
    getAgentsFiles: () => ({ agentsFiles: [] }),
    getSystemPrompt: () => sessionOptions.system || 'You are a helpful AI assistant inside a Nevermind extension. Answer directly and concisely.',
    getAppendSystemPrompt: () => [],
    extendResources: () => {},
    reload: async () => {},
  }
  const result = await pi.createAgentSession({
    cwd: workspaceDir,
    agentDir,
    model,
    thinkingLevel: 'low',
    authStorage,
    modelRegistry: pi.ModelRegistry.inMemory(authStorage),
    resourceLoader,
    noTools: 'builtin',
    sessionManager: pi.SessionManager.inMemory(path.join(workspaceDir, 'extension-ai', sessionOptions.sessionId || `${Date.now()}-${Math.random()}`)),
    settingsManager: pi.SettingsManager.inMemory({ compaction: { enabled: true }, retry: { enabled: true, maxRetries: 2 } }),
  }) as { session: AgentSession }
  return result.session
}

const NEVERMIND_PROVIDER_ID = 'nevermind'

type BackendDescriptor = {
  id: string
  name: string
  contextWindow: number
  maxTokens: number
  reasoning: boolean
  input: string[]
  api: string
  provider: string
  baseUrl: string
}

async function fetchActiveModelDescriptor(baseUrl: string, token: string): Promise<BackendDescriptor> {
  const trimmed = baseUrl.replace(/\/$/, '')
  await checkNevermindCompatibility(trimmed)
  const res = await fetch(`${trimmed}/api/v1/active-model`, {
    headers: nevermindDesktopHeaders({ Authorization: `Bearer ${token}` }),
  })
  if (res.status === 401) throw new NevermindAuthRequiredError()
  if (!res.ok) {
    const body = await res.text().catch(() => '')
    throw new Error(`active-model fetch failed: ${res.status} ${body}`)
  }
  return (await res.json()) as BackendDescriptor
}

async function resolveAiModelAndAuth(_pi: any, _ai: any, authStorage: any) {
  const nevermind = await getNevermindAuth()
  if (!nevermind) throw new NevermindAuthRequiredError()
  authStorage.setRuntimeApiKey(NEVERMIND_PROVIDER_ID, nevermind.token)
  const descriptor = await fetchActiveModelDescriptor(nevermind.baseUrl, nevermind.token)
  const model = {
    id: descriptor.id,
    name: descriptor.name,
    api: descriptor.api as any,
    provider: descriptor.provider,
    baseUrl: descriptor.baseUrl,
    reasoning: descriptor.reasoning,
    input: descriptor.input as ReadonlyArray<string>,
    cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
    contextWindow: descriptor.contextWindow,
    maxTokens: descriptor.maxTokens,
    headers: nevermindDesktopHeaders(),
  }
  return { model, source: 'nevermind' as const }
}

async function createResourceLoader(pi: PiApi, { agentDir, workspaceDir, extensionApiPath, skillPath }: Pick<NevermindAiOptions, 'agentDir' | 'workspaceDir' | 'extensionApiPath' | 'skillPath'>) {
  const webAccessPath = await findPiWebAccessPath()
  const webAccessLoader = webAccessPath && pi.DefaultResourceLoader
    ? new pi.DefaultResourceLoader({ agentDir, cwd: workspaceDir, additionalExtensionPaths: [webAccessPath] })
    : null
  if (webAccessLoader) await webAccessLoader.reload()

  const extensionBuilderSkill = {
    name: 'nevermind-extension-builder',
    description: 'Builds local Nevermind extensions that return declarative UI views. Use whenever the user asks to automate or create a command/action in Nevermind.',
    filePath: skillPath,
    baseDir: path.dirname(skillPath),
    sourceInfo: pi.createSyntheticSourceInfo(skillPath, { source: 'nevermind' }),
    disableModelInvocation: false,
  }

  return {
    getExtensions: () => webAccessLoader?.getExtensions() || ({ extensions: [], errors: [], runtime: pi.createExtensionRuntime() }),
    getSkills: () => {
      const webSkills = (webAccessLoader?.getSkills().skills || []).filter((skill: { filePath?: string }) => skill.filePath?.includes('pi-web-access'))
      return { skills: [extensionBuilderSkill, ...webSkills], diagnostics: [] }
    },
    getPrompts: () => ({ prompts: [], diagnostics: [] }),
    getThemes: () => ({ themes: [], diagnostics: [] }),
    getAgentsFiles: () => ({ agentsFiles: [{ path: 'NEVERMIND.md', content: systemContext(extensionApiPath) }] }),
    getSystemPrompt: () => systemPrompt(),
    getAppendSystemPrompt: () => [],
    extendResources: () => {},
    reload: async () => { await webAccessLoader?.reload() },
  }
}

async function findPiWebAccessPath() {
  const candidates = [
    process.env.NEVERMIND_PI_WEB_ACCESS_PATH,
    '/opt/homebrew/lib/node_modules/pi-web-access',
    '/usr/local/lib/node_modules/pi-web-access',
    path.join(process.env.HOME || '', '.pi', 'agent', 'node_modules', 'pi-web-access'),
  ].filter((candidate): candidate is string => Boolean(candidate))
  for (const candidate of candidates) {
    try {
      await fs.access(path.join(candidate, 'package.json'))
      return candidate
    } catch {}
  }
  return null
}

function createTools(pi: PiApi, Type: TypeApi, { extensionsDir, extensionApiPath, extensionTypesPath, reloadExtensions, getShortcuts, getPaletteShortcut, getExtensionRuntimeState, getActiveChat, markGeneratedExtension, canWriteExtension, removeExtension, addAliasForChat, emitEvent }: Pick<NevermindAiOptions, 'extensionsDir' | 'extensionApiPath' | 'extensionTypesPath' | 'reloadExtensions' | 'getShortcuts' | 'getPaletteShortcut' | 'getExtensionRuntimeState' | 'getActiveChat' | 'markGeneratedExtension' | 'canWriteExtension' | 'removeExtension' | 'addAliasForChat'> & { emitEvent?: (event: AiEvent) => void }) {
  const readFiles = new Set<string>()

  function markRead(filename: string) {
    readFiles.add(path.basename(filename))
  }

  function currentExtensionFile(chat?: ActiveChat | null) {
    return chat?.contextExtensionFile || chat?.generatedExtensionFile || chat?.touchedExtensionFiles?.[0]
  }

  function runtimeDetails(filename: string) {
    const runtime = getExtensionRuntimeState?.(filename)
    return {
      extensionId: runtime?.extensionId,
      commandIds: runtime?.commandIds || [],
      loaded: runtime?.loaded ?? false,
    }
  }

  function observedTool<TParams>(name: string, execute: (toolCallId: string, params: TParams) => Promise<any>) {
    return async (toolCallId: string, params: TParams) => {
      emitEvent?.({ type: 'tool_trace_start', name, data: { toolCallId, input: summarizedToolInput(params) } })
      try {
        const result = await execute(toolCallId, params)
        emitEvent?.({ type: 'tool_trace_end', name, data: { toolCallId, output: summarizedToolResult(result) }, isError: false })
        return result
      } catch (error) {
        emitEvent?.({ type: 'tool_trace_end', name, data: { toolCallId, error: error instanceof Error ? error.message : String(error) }, isError: true })
        throw error
      }
    }
  }

  return [
    pi.defineTool({
      name: 'read_extension_api',
      label: 'Read Extension API',
      description: 'Read the typed Nevermind extension API reference.',
      parameters: Type.Object({}),
      execute: observedTool('read_extension_api', async () => ({
        content: [{ type: 'text', text: await fs.readFile(extensionApiPath, 'utf8') }],
        details: {},
      })),
    }),
    pi.defineTool({
      name: 'list_capabilities',
      label: 'List Capabilities',
      description: 'List UI views and OS capabilities available to generated Nevermind extensions.',
      parameters: Type.Object({}),
      execute: observedTool('list_capabilities', async () => ({
        content: [{ type: 'text', text: JSON.stringify(capabilities(), null, 2) }],
        details: {},
      })),
    }),
    pi.defineTool({
      name: 'list_shortcuts',
      label: 'List Shortcuts',
      description: 'Read the current Nevermind keyboard shortcuts, including the app shortcut used to open Nevermind and active global action shortcuts.',
      parameters: Type.Object({}),
      execute: observedTool('list_shortcuts', async () => {
        const palette = getPaletteShortcut?.()
        const shortcuts = getShortcuts?.() || []
        return {
          content: [{ type: 'text', text: JSON.stringify({ palette, shortcuts }, null, 2) }],
          details: { palette, count: shortcuts.length, shortcuts },
        }
      }),
    }),
    pi.defineTool({
      name: 'read_app_logs',
      label: 'Read App Logs',
      description: 'Read recent structured Nevermind app logs to debug host, renderer, extension, or API failures. Use this when an extension fails, an API helper behaves unexpectedly, or the user asks about app diagnostics.',
      parameters: Type.Object({
        limit: Type.Optional(Type.Number({ description: 'Maximum number of entries to return. Defaults to 200 and is capped by the host.' })),
        level: Type.Optional(Type.String({ description: 'Optional level filter: debug, info, warn, or error.' })),
        source: Type.Optional(Type.String({ description: 'Optional source filter: main, renderer, extension, or host.' })),
        sinceMs: Type.Optional(Type.Number({ description: 'Only return entries from the last N milliseconds.' })),
        query: Type.Optional(Type.String({ description: 'Case-insensitive text filter across the structured log entry.' })),
        extensionId: Type.Optional(Type.String({ description: 'Optional extension id filter.' })),
      }),
      execute: observedTool('read_app_logs', async (_toolCallId: string, params: { limit?: number; level?: string; source?: string; sinceMs?: number; query?: string; extensionId?: string }) => {
        const logs = await readRecentLogs({
          limit: params.limit,
          level: normalizeLogLevel(params.level),
          source: normalizeLogSource(params.source),
          sinceMs: params.sinceMs,
          query: params.query,
          extensionId: params.extensionId,
        })
        return { content: [{ type: 'text', text: logs.length ? JSON.stringify(logs, null, 2) : 'No matching app logs.' }], details: { count: logs.length } }
      }),
    }),
    pi.defineTool({
      name: 'list_extensions',
      label: 'List Extensions',
      description: 'List generated Nevermind extension files available to inspect or reference.',
      parameters: Type.Object({}),
      execute: observedTool('list_extensions', async () => {
        const entries = await fs.readdir(extensionsDir, { withFileTypes: true }).catch(() => [])
        const files = entries.filter((entry) => isExtensionSourceFile(entry.name)).map((entry) => entry.name).sort()
        const listed = files.map((filename) => ({ filename, ...runtimeDetails(filename) }))
        const lines = listed.map((item) => {
          const commands = item.commandIds.length ? ` commands=${item.commandIds.join(', ')}` : ''
          const extensionId = item.extensionId ? ` extensionId=${item.extensionId}` : ''
          return `${item.filename} loaded=${item.loaded}${extensionId}${commands}`
        })
        return {
          content: [{ type: 'text', text: lines.length ? lines.join('\n') : 'No generated extensions installed.' }],
          details: { extensionsDir, files: listed },
        }
      }),
    }),
    pi.defineTool({
      name: 'read_extension',
      label: 'Read Extension',
      description: 'Read any generated Nevermind TypeScript extension source by filename.',
      parameters: Type.Object({ filename: Type.String({ description: 'Safe generated extension filename ending in .ts' }) }),
      execute: observedTool('read_extension', async (_toolCallId: string, params: { filename: string }) => {
        const filePath = safeExtensionPath(extensionsDir, params.filename)
        const code = await fs.readFile(filePath, 'utf8')
        markRead(filePath)
        return { content: [{ type: 'text', text: code }], details: { filePath } }
      }),
    }),
    pi.defineTool({
      name: 'read_current_extension',
      label: 'Read Current Extension',
      description: 'Read the active generated extension source for this chat/action before tweaking it.',
      parameters: Type.Object({}),
      execute: observedTool('read_current_extension', async () => {
        const filename = currentExtensionFile(getActiveChat?.())
        if (!filename) return { content: [{ type: 'text', text: 'No focused generated extension for this chat.' }], details: {} }
        const filePath = safeExtensionPath(extensionsDir, filename)
        const code = await fs.readFile(filePath, 'utf8')
        markRead(filePath)
        return { content: [{ type: 'text', text: code }], details: { filePath } }
      }),
    }),
    pi.defineTool({
      name: 'write_extension',
      label: 'Write Extension',
      description: 'Write a generated Nevermind TypeScript extension into the generated extensions directory.',
      parameters: Type.Object({
        filename: Type.String({ description: 'Safe filename ending in .ts, for example image-grid.ts' }),
        code: Type.String({ description: 'Complete TypeScript extension source code using export default { ... } satisfies NevermindExtension' }),
      }),
      execute: observedTool('write_extension', async (_toolCallId: string, params: { filename: string; code: string }) => {
        const filePath = safeExtensionPath(extensionsDir, params.filename)
        const filename = path.basename(filePath)
        const chat = getActiveChat?.()
        const exists = await fileExists(filePath)
        const focused = currentExtensionFile(chat) === filename
        if (exists && !canWriteExtension?.(filename)) throw new Error(`Refusing to overwrite ${filename}: this AI chat does not own that extension.`)
        if (exists && !focused && !readFiles.has(filename)) throw new Error(`Refusing to overwrite ${filename} before reading it in this chat. Call read_extension first.`)
        const previous = exists ? await fs.readFile(filePath, 'utf8') : null
        await fs.writeFile(filePath, params.code)
        try {
          await validateTypeScriptExtension(filePath, extensionTypesPath, extensionsDir)
        } catch (error) {
          if (previous !== null) await fs.writeFile(filePath, previous)
          else await fs.unlink(filePath).catch(() => {})
          throw error
        }
        markGeneratedExtension?.(filePath)
        await reloadExtensions()
        const runtime = runtimeDetails(filename)
        if (!runtime.loaded) {
          if (previous !== null) await fs.writeFile(filePath, previous)
          else await fs.unlink(filePath).catch(() => {})
          await reloadExtensions()
          throw new Error(`Extension ${filename} was written to ${filePath} but did not load. Check read_app_logs for extension.load.failed.`)
        }
        if (chat?.id) addAliasForChat?.(chat.id)
        return {
          content: [{ type: 'text', text: `Installed ${filename} from ${filePath}${runtime.commandIds.length ? ` with commands: ${runtime.commandIds.join(', ')}` : ''}` }],
          details: { filePath, extensionsDir, ...runtime },
        }
      }),
    }),
    pi.defineTool({
      name: 'remove_extension',
      label: 'Remove Extension',
      description: 'Remove a generated Nevermind TypeScript extension owned by this AI chat.',
      parameters: Type.Object({ filename: Type.String({ description: 'Safe generated extension filename ending in .ts' }) }),
      execute: observedTool('remove_extension', async (_toolCallId: string, params: { filename: string }) => {
        const filePath = safeExtensionPath(extensionsDir, params.filename)
        const filename = path.basename(filePath)
        if (!canWriteExtension?.(filename)) throw new Error(`Refusing to remove ${filename}: this AI chat does not own that extension.`)
        const result = await removeExtension?.(filename)
        if (!result?.removed) return { content: [{ type: 'text', text: `${filename} is already removed.` }], details: { filePath } }
        return {
          content: [{ type: 'text', text: `Removed ${filename} from ${result.filePath || filePath}` }],
          details: { filePath: result.filePath || filePath, extensionsDir },
        }
      }),
    }),
    pi.defineTool({
      name: 'validate_extension',
      label: 'Validate Extension',
      description: 'Typecheck and runtime-validate a generated TypeScript extension file.',
      parameters: Type.Object({ filename: Type.String() }),
      execute: observedTool('validate_extension', async (_toolCallId: string, params: { filename: string }) => {
        const filePath = safeExtensionPath(extensionsDir, params.filename)
        await validateTypeScriptExtension(filePath, extensionTypesPath, extensionsDir)
        return { content: [{ type: 'text', text: `Validated ${path.basename(filePath)}` }], details: { filePath } }
      }),
    }),
    pi.defineTool({
      name: 'install_extension',
      label: 'Install Extension',
      description: 'No-op compatibility tool. write_extension already installs/replaces the active generated extension idempotently.',
      parameters: Type.Object({ filename: Type.String() }),
      execute: observedTool('install_extension', async (_toolCallId: string, params: { filename: string }) => {
        const filePath = safeExtensionPath(extensionsDir, params.filename)
        markGeneratedExtension?.(filePath)
        await reloadExtensions()
        const runtime = runtimeDetails(path.basename(filePath))
        if (!runtime.loaded) throw new Error(`Extension ${path.basename(filePath)} exists at ${filePath} but is not loaded. Check read_app_logs for extension.load.failed.`)
        return {
          content: [{ type: 'text', text: `${path.basename(filePath)} is active from ${filePath}` }],
          details: { filePath, extensionsDir, ...runtime },
        }
      }),
    }),
  ]
}

function summarizedToolInput(params: unknown) {
  if (params == null || typeof params !== 'object') return params
  const value = { ...(params as Record<string, unknown>) }
  if (typeof value.code === 'string') value.code = summarizeText(value.code, 500)
  if (typeof value.script === 'string') value.script = summarizeText(value.script, 500)
  return value
}

function summarizedToolResult(result: any) {
  if (!result || typeof result !== 'object') return result
  if (result.details && typeof result.details === 'object') return result.details
  const text = Array.isArray(result.content)
    ? result.content.map((item: { text?: string }) => item?.text).filter(Boolean).join('\n\n')
    : ''
  return text ? { text: summarizeText(text, 1_000) } : undefined
}

function summarizeText(text: string, limit: number) {
  return String(text || '').slice(0, limit)
}

function normalizeLogLevel(level?: string): LogLevel | undefined {
  return level === 'debug' || level === 'info' || level === 'warn' || level === 'error' ? level : undefined
}

function normalizeLogSource(source?: string): LogSource | undefined {
  return source === 'main' || source === 'renderer' || source === 'extension' || source === 'host' ? source : undefined
}

async function fileExists(filePath: string) {
  try {
    await fs.access(filePath)
    return true
  } catch {
    return false
  }
}

function isExtensionSourceFile(filename: string) {
  return filename.endsWith('.ts') && !filename.endsWith('.d.ts')
}

function safeExtensionPath(root: string, filename: string) {
  const safeName = path.basename(filename).replace(/[^a-zA-Z0-9._-]/g, '-')
  if (!isExtensionSourceFile(safeName)) throw new Error('Extension filename must end in .ts')
  const fullPath = path.resolve(root, safeName)
  if (!fullPath.startsWith(path.resolve(root) + path.sep)) throw new Error('Invalid extension path')
  return fullPath
}

async function ensureExtensionTypeDefinitions(extensionsDir: string, extensionTypesPath: string) {
  await fs.mkdir(extensionsDir, { recursive: true })
  await fs.copyFile(extensionTypesPath, path.join(extensionsDir, 'nevermind-extension-api.d.ts'))
  await fs.writeFile(path.join(extensionsDir, 'package.json'), `${JSON.stringify({ type: 'module' }, null, 2)}\n`)
}

function typecheckExtension(filePath: string, typeDefinitionsPath: string) {
  const options: ts.CompilerOptions = {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
    moduleResolution: ts.ModuleResolutionKind.Bundler,
    strict: false,
    noEmit: true,
    skipLibCheck: true,
    esModuleInterop: true,
    allowSyntheticDefaultImports: true,
    allowImportingTsExtensions: true,
  }
  const program = ts.createProgram([filePath, typeDefinitionsPath], options)
  const diagnostics = ts.getPreEmitDiagnostics(program).filter((diagnostic) => diagnostic.category === ts.DiagnosticCategory.Error)
  if (!diagnostics.length) return
  const host: ts.FormatDiagnosticsHost = {
    getCanonicalFileName: (fileName) => fileName,
    getCurrentDirectory: () => path.dirname(filePath),
    getNewLine: () => '\n',
  }
  throw new Error(`TypeScript validation failed:\n${ts.formatDiagnosticsWithColorAndContext(diagnostics, host)}`)
}

async function importExtensionForValidation(filePath: string) {
  const url = pathToFileURL(filePath)
  url.searchParams.set('validate', String(Date.now()))
  const imported = await import(url.href)
  const extension = imported.default || imported
  if (!extension?.id || !extension?.title) throw new Error('Extension must export an object with id and title')
  if (extension.commands !== undefined && !Array.isArray(extension.commands)) throw new Error('Extension commands must be an array')
  if (extension.actions !== undefined && typeof extension.actions !== 'function') throw new Error('Extension actions must be a function')
  return extension
}

function validateExtensionPermissions(extension: any, source: string) {
  const permissions = new Set(Array.isArray(extension?.permissions) ? extension.permissions.map(String) : [])
  const usesSystemShell = /ctx\.desktop\.shell\b/.test(source) || /ctx\.actions\.(?:shellExec|shellScript|system)\b/.test(source)
  if (usesSystemShell && !permissions.has('system')) {
    throw new Error("Extension uses shell or system helpers but does not declare the 'system' permission. Add `permissions: ['system']` to the extension manifest.")
  }
}

async function validateTypeScriptExtension(filePath: string, extensionTypesPath: string, extensionsDir: string) {
  await ensureExtensionTypeDefinitions(extensionsDir, extensionTypesPath)
  typecheckExtension(filePath, path.join(extensionsDir, 'nevermind-extension-api.d.ts'))
  const [source, extension] = await Promise.all([fs.readFile(filePath, 'utf8'), importExtensionForValidation(filePath)])
  validateExtensionPermissions(extension, source)
}

function capabilities() {
  return {
    extensionExports: ['export default { id, title, permissions, actions, commands, rootItems, searchItems } satisfies NevermindExtension'],
    rootContributions: ['actions(ctx) returns persistent shortcutable actions; commands are shorthand durable actions; use ctx.actions.ref(id) inside views to reference a durable action; rootItems(ctx) returns high-signal dynamic/status items; searchItems(ctx, query) returns query-aware dynamic items'], 
    icons: ['Any Lucide icon name in camel/Pascal case or kebab case, for example mic, volume-2, audio-lines, camera, calendar, image, folder. Legacy aliases include restart, grid, sparkles.'],
    views: ['list', 'grid', 'preview', 'chat', 'form', 'editor', 'progress', 'camera', 'webview'],
    formFields: ['text', 'textarea', 'password', 'email', 'url', 'number', 'date', 'checkbox', 'dropdown/select', 'multiselect', 'file', 'files', 'folder', 'description', 'separator'],
    viewOptions: ['sections', 'selectedItemId', 'onSelectionChange', 'isLoading', 'emptyView', 'detail side pane', 'searchBarPlaceholder', 'searchAccessory', 'pagination', 'refresh'],
    itemOptions: ['accessories with tone/tooltip', 'keywords', 'detail markdown/metadata/actions', 'image descriptor', 'actionPanel', 'appearance.foreground: muted named color yellow, blue, purple, green, red, orange, or pink'],
    actionPanel: ['sections', 'submenus'],
    shortcuts: ['local action shortcut', 'durable action globalShortcut', 'command globalShortcut as shorthand', 'shortcutScope'], 
    gridOptions: { layout: ['square', 'wide', 'compact'], aspectRatio: ['1', '16 / 9', '4 / 3'], columns: 'number' },
    actions: ['openPath', 'revealPath', 'quickLook', 'openWith', 'openUrl', 'copyText', 'pasteText(options: keepPaletteOpen, restoreClipboard, plainText, concealed)', 'paste(text/html/image/files content with concealed/restore options)', 'typeText', 'copyImage', 'trash', 'push', 'replace', 'pop', 'run', 'shellExec (requires system permission)', 'shellScript (requires system permission)', 'durable actions/commands may declare mode and triggers for host-managed background jobs'],
    namespaces: ['desktop', 'text', 'input', 'windows', 'storage', 'extension', 'navigation', 'cache', 'state', 'ai', 'ocr'],
    backgroundJobs: ['commands and actions(ctx) contributions can declare mode: view|noView|background and triggers startup, interval, clipboard.changed, files.changed, app.frontmost.changed, wake, and login; Nevermind owns scheduling, no-overlap, timeouts, backoff, enable/disable persistence, and diagnostics in Background Tasks; triggered runs receive ctx.launch with reason, trigger, startedAt, and changed file context when applicable'], 
    files: ['ctx.desktop.files.metadata(path) returns canonical metadata and host-safe URLs; thumbnail(path) returns a thumbnail URL; indexedRoots(), indexSnapshot(options), searchIndex(query, options), and reindex(options) provide bounded host file-index controls'],
    ocr: ['requires ocr permission; ctx.ocr.image(pathOrFileOrDataUrl), ctx.ocr.screen(options), and ctx.ocr.region(rect, options) return recognized text plus blocks/confidence; check ctx.system.capabilities.has(\'ocr\') and render graceful unavailable states'], 
    text: ['template(input, variablesOrOptions) expands {name}/{{name}} placeholders plus date, time, datetime, uuid, selectedText, clipboard, cursor, {argument:name}, and {calculator:1 + 2}; pass { variables, returnCursor/returnResult, includeClipboard, includeSelectedText, promptMissing } for cursor/missing-variable results'], 
    input: ['prompt({ title, message, fields, action, submitTitle }) opens a host prompt form, then runs the wrapped action with submitted values in action.formValues'],
    ai: ['ask(prompt, { system, attachments, signal })', 'stream(prompt, { onDelta, onEvent, attachments, signal }).result/abort()', 'session(id, options).ask/stream/reset()', 'attachments.text/image/file/selectedText/selectedFiles/clipboard/ocrImage'],
    webTools: ['web_search', 'code_search', 'fetch_content', 'get_search_content'],
    desktop: {
      keyboard: ['typeText(text, options) types into the frontmost app without touching the clipboard when keyboard.type-text is available'],
      clipboard: ['readText', 'writeText(options.concealed)', 'readHtml', 'writeHtml', 'readImage', 'writeImage', 'readFiles', 'writeFiles', 'read({formats})', 'write(text/html/image/files content)'],
      selection: ['text', 'files', 'read'],
      apps: ['frontmost', 'launch'],
      files: ['find', 'findImages', 'findVideos', 'findMedia', 'openWithApps', 'open', 'reveal', 'preview', 'readText', 'toFileUrl', 'thumbnail', 'metadata', 'indexedRoots', 'indexSnapshot', 'reindex', 'recent', 'searchIndex'],
      shell: ['requires system permission', 'openExternal', 'exec', 'script', 'appleScript', 'which'],
    },
    fileHelpers: ['find', 'findImages', 'findVideos', 'findMedia', 'openWithApps', 'open', 'reveal', 'preview', 'readText', 'toFileUrl', 'thumbnail', 'metadata', 'indexedRoots', 'indexSnapshot', 'reindex', 'recent', 'searchIndex'],
    findOptions: ['limit', 'depth', 'extensions', 'kind', 'pattern', 'sortBy', 'order', 'roots', 'includeHidden', 'ignore'],
    fileKinds: ['image', 'video', 'media'],
    sortBy: ['recent', 'modified', 'added', 'created', 'name', 'size'],
    sortByDescriptions: {
      recent: 'filesystem modification time (mtimeMs), not necessarily download/add time',
      modified: 'filesystem modification time (mtimeMs), same as recent',
      added: 'Finder/Spotlight Date Added (dateAddedMs) with creation-time fallback; prefer for newest Downloads, screenshots, and mixed media galleries',
      created: 'filesystem birth/creation time (birthtimeMs)',
    },
    fileFields: ['path', 'name', 'displayPath', 'url', 'fileUrl', 'videoUrl', 'thumbnailUrl', 'kind', 'extension', 'mimeType', 'width', 'height', 'mtime', 'mtimeMs', 'birthtime', 'birthtimeMs', 'dateAdded', 'dateAddedMs', 'size'],
    storage: ['get', 'set', 'delete', 'clear', 'memo', 'memoStale'],
    triggerTypes: ['startup', 'interval', 'clipboard.changed requires clipboard.history', 'files.changed requires desktop.files and supports roots, debounceMs, includeHidden, extensions, kind, ignore, plus ctx.launch.changedPaths/files', 'app.frontmost.changed requires desktop.apps', 'wake', 'login'],
  }
}

function systemPrompt() {
  return `You are Nevermind's extension-building agent.
Build local Nevermind extensions, not shell scripts.
When the first user message is vague, ask clarifying questions before using tools.
Do not call read_extension_api, list_capabilities, write_extension, remove_extension, validate_extension, or install_extension until the user has confirmed the desired command behavior.
Once the user provides enough details, start building immediately by calling read_extension_api in the same turn. Do not say you are going to call a tool; call it.
The available Nevermind extension-building tool names are: read_extension_api, list_capabilities, list_shortcuts, read_app_logs, list_extensions, read_extension, read_current_extension, write_extension, remove_extension, validate_extension, install_extension. Web access tools may also be available as web_search, code_search, fetch_content, and get_search_content. Never invent tool names like read_file, write_file, list_directory, or bash.
Never write XML or pseudo tool calls in the chat. Use real structured tool calls only.
Never provide instructions to manually save extension files; if tool access fails, report the failure briefly and ask the user to retry.
The nevermind-extension-builder skill is the workflow and safety checklist; read_extension_api returns the typed API reference and is the source of truth for extension authoring details.
Use read_extension_api before writing an extension.
Use list_shortcuts when the extension should mention or depend on currently configured keyboard shortcuts. Never guess shortcut bindings.
Use web_search, code_search, fetch_content, or get_search_content when current external information, URL contents, or library examples are needed.
When tweaking an existing generated action, call read_current_extension before writing and preserve existing behavior unless the user asks to remove it. You may read any generated extension, but you may only write or remove extensions owned by this chat.
Use list_capabilities when unsure which UI or OS capabilities exist.
Use read_app_logs when debugging host/API/renderer/extension failures or when an extension error view does not explain the root cause.
Use list_extensions and read_extension when you need awareness of other installed extensions.
Only write .ts extension files with write_extension.
write_extension creates standalone TypeScript extension files. It may overwrite the focused extension or a file you already read in this chat; otherwise read the file first before updating it.
remove_extension deletes a generated .ts extension file only when this chat owns it.
validate_extension typechecks changed extension files and verifies they load with Electron's native TypeScript runtime.
install_extension is only a backwards-compatible no-op; do not rely on it for writing or replacing.
Keep generated commands small, local, and native-feeling.
Nevermind catches thrown extension errors and shows a native error view, so throw meaningful Error objects instead of swallowing failures unless the extension can recover or add context and rethrow.
Declare permissions explicitly: use 'system' for shell helpers and system actions, 'desktop.files' for file helpers, 'desktop.apps' for app helpers, 'clipboard.history' for clipboard history, and 'ai' for AI calls.
For image grids, use file.url from ctx.desktop.files.findImages() or ctx.desktop.files.toFileUrl(path), never raw filesystem paths, so thumbnails render in Electron.
Use primaryAction for the Enter behavior. Put secondary item actions in actions; Nevermind exposes them under Cmd+K automatically.
Use actions(ctx) for persistent shortcut-worthy variants such as “Compress 720p”, “Compress 1080p”, “Toggle Floating Note”, or fixed snippet actions; these are searchable, aliasable, and global-shortcutable without opening a view. When a view row should run one of those durable actions, set primaryAction: ctx.actions.ref('action-id') instead of duplicating inline logic. Use rootItems(ctx) for high-signal empty-query dynamic/status contributions such as upcoming events or active status; keep root items few, stable, cached, and bounded because Nevermind owns ranking and limits.
Use ctx.navigation.push/replace/pop/run as the preferred explicit return helpers from action handlers. Use ctx.actions.push/replace/pop for static declarative navigation actions. Use ctx.input.prompt({ fields, action }) when an action needs lightweight arguments before it runs; the wrapped action receives submitted values in action.formValues. Use ctx.actions.pasteText(text, title, { restoreClipboard: true, concealed: true }) for snippets/transforms that should paste without polluting clipboard history, and use ctx.actions.typeText or ctx.desktop.keyboard.typeText when an extension must avoid touching the clipboard. Use ctx.ui.editor({ title, content, format: 'markdown', submitAction }) for host-owned editable text/markdown surfaces; submit actions receive action.editorContent. Prefer host-owned native views such as ctx.ui.camera({ title, actions }) for media/interactive surfaces; camera views include host-owned desktop camera switching, so extensions should bind camera controls with ctx.actions.camera.switchDevice/nextDevice/previousDevice/toggleMuted/toggleControls and normal action shortcuts instead of owning the stream. Use ctx.ui.webview only as an advanced escape hatch for custom live browser UI. Set size: 'large' when a view needs a larger palette. Use ctx.actions.run for script work triggered from UI.
When done, tell the user what command was installed and how to find it.`
}

function systemContext(extensionApiPath: string) {
  return `Nevermind is an Electron command palette. Your job is to create first-class local extensions using the typed API reference.
Extension API reference path: ${extensionApiPath}
The builder skill is workflow; the extension API declaration file is the canonical authoring reference.
Generated extensions are standalone app contributions. AI chats are builder/history sessions and can inspect or edit multiple extensions.`
}

export { createNevermindAi }
