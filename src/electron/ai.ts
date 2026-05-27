import fs from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'
import ts from 'typescript'
import * as logger from './logger'
import { readRecentLogs, type LogLevel, type LogSource } from './logger'

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
  getActiveChat?: () => ActiveChat | null
  getChat?: (chatId: string) => ActiveChat | null
  markGeneratedExtension?: (filePath: string, chatId?: string) => void
  canWriteExtension?: (filename: string, chatId?: string) => boolean
  addAliasForChat?: (chatId: string) => void
  onEvent?: (event: AiEvent) => void
}

type AgentSession = {
  prompt: (message: string) => Promise<unknown>
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

type SessionEntry = {
  unsubscribe: (() => void) | null
  promise: Promise<AgentSession>
}

type PiApi = any
type TypeApi = any

const DEFAULT_MODEL = 'gemini-3-flash'

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
    return entry.promise
  }

  async function send(message: string, chatId = 'default') {
    const session = await getSession(chatId)
    options.onEvent?.({ type: 'start', chatId })
    try {
      await session.prompt(message)
      options.onEvent?.({ type: 'done', chatId })
    } catch (error) {
      options.onEvent?.({ type: 'error', chatId, message: error instanceof Error ? error.message : String(error) })
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

  async function ask(message: string, askOptions: { sessionId?: string; system?: string } = {}) {
    const text: string[] = []
    const session = await generalSession(askOptions)
    const unsubscribe = session.subscribe((event) => {
      if (isMessageUpdateEvent(event)) text.push(event.assistantMessageEvent.delta)
    })
    try {
      await session.prompt(message)
      return text.join('')
    } finally {
      unsubscribe()
      if (!askOptions.sessionId) session.dispose?.()
    }
  }

  async function generalSession(sessionOptions: { sessionId?: string; system?: string }) {
    if (!sessionOptions.sessionId) return createGeneralSession(options, sessionOptions)
    const key = sessionOptions.sessionId
    let promise = generalSessions.get(key)
    if (!promise) {
      promise = createGeneralSession(options, sessionOptions)
      generalSessions.set(key, promise)
    }
    return promise
  }

  function session(sessionId: string, sessionOptions: { system?: string } = {}) {
    return {
      ask: (message: string) => ask(message, { ...sessionOptions, sessionId }),
      reset: async () => {
        const current = await generalSessions.get(sessionId)?.catch(() => null)
        current?.dispose?.()
        generalSessions.delete(sessionId)
      },
    }
  }

  return { send, abort, reset, ask, session }

  async function createSession({ agentDir, workspaceDir, extensionsDir, extensionApiPath, extensionTypesPath, skillPath, chatId = 'default', reloadExtensions, getActiveChat, getChat, markGeneratedExtension, canWriteExtension, addAliasForChat, onEvent }: NevermindAiOptions & { chatId?: string }, emit: (event: AiEvent) => void) {
    await fs.mkdir(agentDir, { recursive: true })
    await fs.mkdir(workspaceDir, { recursive: true })
    await fs.mkdir(extensionsDir, { recursive: true })

    const [pi, ai] = await Promise.all([
      import('@earendil-works/pi-coding-agent') as Promise<PiApi>,
      import('@earendil-works/pi-ai') as Promise<any>,
    ])

    const apiKey = await resolveOpenCodeApiKey()
    const authStorage = pi.AuthStorage.create(path.join(agentDir, 'auth.json'))
    if (apiKey) authStorage.setRuntimeApiKey('opencode', apiKey)

    const modelRegistry = pi.ModelRegistry.inMemory(authStorage)
    const model = ai.getModel('opencode', process.env.NEVERMIND_AI_MODEL || DEFAULT_MODEL)
    if (!model) throw new Error(`Missing opencode model: ${process.env.NEVERMIND_AI_MODEL || DEFAULT_MODEL}`)
    const modelDebug = {
      provider: model.provider,
      id: model.id,
      api: model.api,
      reasoning: model.reasoning,
      thinkingLevelMap: model.thinkingLevelMap,
      compat: model.compat,
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
      getActiveChat: () => getChat?.(chatId) || getActiveChat?.() || null,
      markGeneratedExtension: (filePath) => markGeneratedExtension?.(filePath, chatId),
      canWriteExtension: (filename) => canWriteExtension?.(filename, chatId) ?? true,
      addAliasForChat,
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
    })

    onEvent?.({ type: 'ready' })
    return result.session
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
  const apiKey = await resolveOpenCodeApiKey()
  const authStorage = pi.AuthStorage.create(path.join(agentDir, 'auth.json'))
  if (apiKey) authStorage.setRuntimeApiKey('opencode', apiKey)
  const model = ai.getModel('opencode', process.env.NEVERMIND_AI_MODEL || DEFAULT_MODEL)
  if (!model) throw new Error(`Missing opencode model: ${process.env.NEVERMIND_AI_MODEL || DEFAULT_MODEL}`)
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

async function resolveOpenCodeApiKey() {
  const envKey = process.env.OPENCODE_API_KEY || process.env.NEVERMIND_OPENCODE_API_KEY
  if (envKey) return envKey
  const secretsPath = path.join(process.env.HOME || '', '.zshrc.d', '01-secrets.sh')
  const secrets = await fs.readFile(secretsPath, 'utf8').catch(() => '')
  const match = secrets.match(/^\s*export\s+(?:OPENCODE_API_KEY|NEVERMIND_OPENCODE_API_KEY)=(['"]?)([^'"\n]+)\1\s*$/m)
  return match?.[2]
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

function createTools(pi: PiApi, Type: TypeApi, { extensionsDir, extensionApiPath, extensionTypesPath, reloadExtensions, getActiveChat, markGeneratedExtension, canWriteExtension, addAliasForChat }: Pick<NevermindAiOptions, 'extensionsDir' | 'extensionApiPath' | 'extensionTypesPath' | 'reloadExtensions' | 'getActiveChat' | 'markGeneratedExtension' | 'canWriteExtension' | 'addAliasForChat'>) {
  const readFiles = new Set<string>()

  function markRead(filename: string) {
    readFiles.add(path.basename(filename))
  }

  function currentExtensionFile(chat?: ActiveChat | null) {
    return chat?.contextExtensionFile || chat?.generatedExtensionFile || chat?.touchedExtensionFiles?.[0]
  }

  return [
    pi.defineTool({
      name: 'read_extension_api',
      label: 'Read Extension API',
      description: 'Read Nevermind extension API documentation.',
      parameters: Type.Object({}),
      execute: async () => ({
        content: [{ type: 'text', text: await fs.readFile(extensionApiPath, 'utf8') }],
        details: {},
      }),
    }),
    pi.defineTool({
      name: 'list_capabilities',
      label: 'List Capabilities',
      description: 'List UI views and OS capabilities available to generated Nevermind extensions.',
      parameters: Type.Object({}),
      execute: async () => ({
        content: [{ type: 'text', text: JSON.stringify(capabilities(), null, 2) }],
        details: {},
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
      execute: async (_toolCallId: string, params: { limit?: number; level?: string; source?: string; sinceMs?: number; query?: string; extensionId?: string }) => {
        const logs = await readRecentLogs({
          limit: params.limit,
          level: normalizeLogLevel(params.level),
          source: normalizeLogSource(params.source),
          sinceMs: params.sinceMs,
          query: params.query,
          extensionId: params.extensionId,
        })
        return { content: [{ type: 'text', text: logs.length ? JSON.stringify(logs, null, 2) : 'No matching app logs.' }], details: { count: logs.length } }
      },
    }),
    pi.defineTool({
      name: 'list_extensions',
      label: 'List Extensions',
      description: 'List generated Nevermind extension files available to inspect or reference.',
      parameters: Type.Object({}),
      execute: async () => {
        const entries = await fs.readdir(extensionsDir, { withFileTypes: true }).catch(() => [])
        const files = entries.filter((entry) => isExtensionSourceFile(entry.name)).map((entry) => entry.name).sort()
        return { content: [{ type: 'text', text: files.length ? files.join('\n') : 'No generated extensions installed.' }], details: { files } }
      },
    }),
    pi.defineTool({
      name: 'read_extension',
      label: 'Read Extension',
      description: 'Read any generated Nevermind TypeScript extension source by filename.',
      parameters: Type.Object({ filename: Type.String({ description: 'Safe generated extension filename ending in .ts' }) }),
      execute: async (_toolCallId: string, params: { filename: string }) => {
        const filePath = safeExtensionPath(extensionsDir, params.filename)
        const code = await fs.readFile(filePath, 'utf8')
        markRead(filePath)
        return { content: [{ type: 'text', text: code }], details: { filePath } }
      },
    }),
    pi.defineTool({
      name: 'read_current_extension',
      label: 'Read Current Extension',
      description: 'Read the active generated extension source for this chat/action before tweaking it.',
      parameters: Type.Object({}),
      execute: async () => {
        const filename = currentExtensionFile(getActiveChat?.())
        if (!filename) return { content: [{ type: 'text', text: 'No focused generated extension for this chat.' }], details: {} }
        const filePath = safeExtensionPath(extensionsDir, filename)
        const code = await fs.readFile(filePath, 'utf8')
        markRead(filePath)
        return { content: [{ type: 'text', text: code }], details: { filePath } }
      },
    }),
    pi.defineTool({
      name: 'write_extension',
      label: 'Write Extension',
      description: 'Write a generated Nevermind TypeScript extension into the generated extensions directory.',
      parameters: Type.Object({
        filename: Type.String({ description: 'Safe filename ending in .ts, for example image-grid.ts' }),
        code: Type.String({ description: 'Complete TypeScript extension source code using export default { ... } satisfies NevermindExtension' }),
      }),
      execute: async (_toolCallId: string, params: { filename: string; code: string }) => {
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
        if (chat?.id) addAliasForChat?.(chat.id)
        return { content: [{ type: 'text', text: `Installed ${filename}` }], details: { filePath } }
      },
    }),
    pi.defineTool({
      name: 'validate_extension',
      label: 'Validate Extension',
      description: 'Typecheck and runtime-validate a generated TypeScript extension file.',
      parameters: Type.Object({ filename: Type.String() }),
      execute: async (_toolCallId: string, params: { filename: string }) => {
        const filePath = safeExtensionPath(extensionsDir, params.filename)
        await validateTypeScriptExtension(filePath, extensionTypesPath, extensionsDir)
        return { content: [{ type: 'text', text: `Validated ${path.basename(filePath)}` }], details: { filePath } }
      },
    }),
    pi.defineTool({
      name: 'install_extension',
      label: 'Install Extension',
      description: 'No-op compatibility tool. write_extension already installs/replaces the active generated extension idempotently.',
      parameters: Type.Object({ filename: Type.String() }),
      execute: async (_toolCallId: string, params: { filename: string }) => {
        const filePath = safeExtensionPath(extensionsDir, params.filename)
        markGeneratedExtension?.(filePath)
        await reloadExtensions()
        return { content: [{ type: 'text', text: `${path.basename(filePath)} is already active.` }], details: { filePath } }
      },
    }),
  ]
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
}

async function validateTypeScriptExtension(filePath: string, extensionTypesPath: string, extensionsDir: string) {
  await ensureExtensionTypeDefinitions(extensionsDir, extensionTypesPath)
  typecheckExtension(filePath, path.join(extensionsDir, 'nevermind-extension-api.d.ts'))
  await importExtensionForValidation(filePath)
}

function capabilities() {
  return {
    extensionExports: ['export default { id, title, permissions, commands, rootItems, searchItems } satisfies NevermindExtension'],
    rootContributions: ['rootItems(ctx) returns high-signal empty-query root palette items with stable ids, titles, optional subtitles/icons/scores, primaryAction, actions, and actionPanel'],
    icons: ['Any Lucide icon name in camel/Pascal case or kebab case, for example mic, volume-2, audio-lines, camera, calendar, image, folder. Legacy aliases include restart, grid, sparkles.'],
    views: ['list', 'grid', 'preview', 'chat', 'form', 'progress', 'camera', 'webview'],
    viewOptions: ['sections', 'selectedItemId', 'onSelectionChange', 'isLoading', 'emptyView', 'searchBarPlaceholder', 'searchAccessory', 'pagination', 'refresh'],
    itemOptions: ['accessories', 'keywords', 'actionPanel', 'appearance.foreground: muted named color yellow, blue, purple, green, red, orange, or pink'],
    actionPanel: ['sections', 'submenus'],
    shortcuts: ['local action shortcut', 'command globalShortcut', 'shortcutScope'],
    gridOptions: { layout: ['square', 'wide', 'compact'], aspectRatio: ['1', '16 / 9', '4 / 3'], columns: 'number' },
    actions: ['openPath', 'revealPath', 'quickLook', 'openWith', 'openUrl', 'copyText', 'pasteText', 'copyImage', 'trash', 'push', 'replace', 'pop', 'run', 'shellExec', 'shellScript'],
    namespaces: ['desktop', 'storage', 'extension', 'navigation', 'cache', 'state', 'ai'],
    ai: ['ask(prompt, options)', 'session(id, options).ask(prompt)', 'session(id).reset()'],
    webTools: ['web_search', 'code_search', 'fetch_content', 'get_search_content'],
    desktop: {
      clipboard: ['readText', 'writeText', 'readImage', 'writeImage', 'readFiles', 'read', 'write'],
      selection: ['text', 'files', 'read'],
      apps: ['frontmost', 'launch'],
      files: ['find', 'findImages', 'findVideos', 'findMedia', 'openWithApps', 'open', 'reveal', 'preview', 'readText', 'toFileUrl'],
      shell: ['openExternal', 'exec', 'script', 'appleScript', 'which'],
    },
    fileHelpers: ['find', 'findImages', 'findVideos', 'findMedia', 'openWithApps', 'open', 'reveal', 'preview', 'readText', 'toFileUrl'],
    findOptions: ['limit', 'depth', 'extensions', 'kind', 'pattern', 'sortBy', 'order'],
    fileKinds: ['image', 'video', 'media'],
    sortBy: ['recent', 'modified', 'added', 'created', 'name', 'size'],
    sortByDescriptions: {
      recent: 'filesystem modification time (mtimeMs), not necessarily download/add time',
      modified: 'filesystem modification time (mtimeMs), same as recent',
      added: 'Finder/Spotlight Date Added (dateAddedMs) with creation-time fallback; prefer for newest Downloads, screenshots, and mixed media galleries',
      created: 'filesystem birth/creation time (birthtimeMs)',
    },
    fileFields: ['path', 'name', 'displayPath', 'url', 'fileUrl', 'videoUrl', 'thumbnailUrl', 'kind', 'extension', 'mtime', 'mtimeMs', 'birthtime', 'birthtimeMs', 'dateAdded', 'dateAddedMs', 'size'],
    storage: ['get', 'set', 'delete', 'clear', 'memo', 'memoStale'],
  }
}

function systemPrompt() {
  return `You are Nevermind's extension-building agent.
Build local Nevermind extensions, not shell scripts.
When the first user message is vague, ask clarifying questions before using tools.
Do not call read_extension_api, list_capabilities, write_extension, validate_extension, or install_extension until the user has confirmed the desired command behavior.
Once the user provides enough details, start building immediately by calling read_extension_api in the same turn. Do not say you are going to call a tool; call it.
The available Nevermind extension-building tool names are: read_extension_api, list_capabilities, read_app_logs, list_extensions, read_extension, read_current_extension, write_extension, validate_extension, install_extension. Web access tools may also be available as web_search, code_search, fetch_content, and get_search_content. Never invent tool names like read_file, write_file, list_directory, or bash.
Never write XML or pseudo tool calls in the chat. Use real structured tool calls only.
Never provide instructions to manually save extension files; if tool access fails, report the failure briefly and ask the user to retry.
The nevermind-extension-builder skill is the workflow and safety checklist; read_extension_api is the source of truth for extension API details and guideline overflow.
Use read_extension_api before writing an extension.
Use web_search, code_search, fetch_content, or get_search_content when current external information, URL contents, or library examples are needed.
When tweaking an existing generated action, call read_current_extension before writing and preserve existing behavior unless the user asks to remove it. You may read any generated extension, but you may only write extensions owned by this chat.
Use list_capabilities when unsure which UI or OS capabilities exist.
Use read_app_logs when debugging host/API/renderer/extension failures or when an extension error view does not explain the root cause.
Use list_extensions and read_extension when you need awareness of other installed extensions.
Only write .ts extension files with write_extension.
write_extension creates standalone TypeScript extension files. It may overwrite the focused extension or a file you already read in this chat; otherwise read the file first before updating it.
validate_extension typechecks changed extension files and verifies they load with Electron's native TypeScript runtime.
install_extension is only a backwards-compatible no-op; do not rely on it for writing or replacing.
Keep generated commands small, local, and native-feeling.
Nevermind catches thrown extension errors and shows a native error view, so throw meaningful Error objects instead of swallowing failures unless the extension can recover or add context and rethrow.
For image grids, use file.url from ctx.desktop.files.findImages() or ctx.desktop.files.toFileUrl(path), never raw filesystem paths, so thumbnails render in Electron.
Use primaryAction for the Enter behavior. Put secondary item actions in actions; Nevermind exposes them under Cmd+K automatically.
Use rootItems(ctx) for high-signal empty-query root palette contributions such as upcoming events or active status; keep root items few, stable, cached, and bounded because Nevermind owns ranking and limits.
Use ctx.navigation.push/replace/pop/run as the preferred explicit return helpers from action handlers. Use ctx.actions.push/replace/pop for static declarative navigation actions. Prefer host-owned native views such as ctx.ui.camera({ title, actions }) for media/interactive surfaces; camera views include host-owned desktop camera switching, so extensions should bind camera controls with ctx.actions.camera.switchDevice/nextDevice/previousDevice/toggleMuted/toggleControls and normal action shortcuts instead of owning the stream. Use ctx.ui.webview only as an advanced escape hatch for custom live browser UI. Set size: 'large' when a view needs a larger palette. Use ctx.actions.run for script work triggered from UI.
When done, tell the user what command was installed and how to find it.`
}

function systemContext(extensionApiPath: string) {
  return `Nevermind is an Electron command palette. Your job is to create first-class local extensions using the documented API.
Extension API docs path: ${extensionApiPath}
The builder skill is workflow; the extension API docs are the canonical guideline/API reference.
Generated extensions are standalone app contributions. AI chats are builder/history sessions and can inspect or edit multiple extensions.`
}

export { createNevermindAi }
