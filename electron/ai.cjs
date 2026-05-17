const fs = require('node:fs/promises')
const path = require('node:path')
const vm = require('node:vm')

const DEFAULT_MODEL = 'deepseek-v4-flash'

function createNevermindAi(options) {
  const sessions = new Map()

  async function getSession(chatId = 'default') {
    const current = sessions.get(chatId)
    if (current) return current.promise

    const entry = {
      unsubscribe: null,
      promise: createSession({ ...options, chatId }, (event) => options.onEvent?.({ ...event, chatId })),
    }
    sessions.set(chatId, entry)
    return entry.promise
  }

  async function send(message, chatId = 'default') {
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

  async function createSession({ agentDir, workspaceDir, extensionsDir, extensionApiPath, skillPath, chatId = 'default', reloadExtensions, getActiveChat, markGeneratedExtension, onEvent }, emit) {
    await fs.mkdir(agentDir, { recursive: true })
    await fs.mkdir(workspaceDir, { recursive: true })
    await fs.mkdir(extensionsDir, { recursive: true })

    const [pi, ai] = await Promise.all([
      import('@earendil-works/pi-coding-agent'),
      import('@earendil-works/pi-ai'),
    ])

    const apiKey = process.env.OPENCODE_API_KEY || process.env.NEVERMIND_OPENCODE_API_KEY
    const authStorage = pi.AuthStorage.create(path.join(agentDir, 'auth.json'))
    if (apiKey) authStorage.setRuntimeApiKey('opencode-go', apiKey)

    const modelRegistry = pi.ModelRegistry.inMemory(authStorage)
    const model = ai.getModel('opencode-go', process.env.NEVERMIND_AI_MODEL || DEFAULT_MODEL)
    if (!model) throw new Error(`Missing opencode-go model: ${process.env.NEVERMIND_AI_MODEL || DEFAULT_MODEL}`)
    const modelDebug = {
      provider: model.provider,
      id: model.id,
      api: model.api,
      reasoning: model.reasoning,
      thinkingLevelMap: model.thinkingLevelMap,
      compat: model.compat,
    }
    // console.info('[Nevermind AI] model', modelDebug)
    onEvent?.({ type: 'debug', label: 'model', data: modelDebug })

    const settingsManager = pi.SettingsManager.inMemory({
      compaction: { enabled: true },
      retry: { enabled: true, maxRetries: 2 },
    })

    const resourceLoader = createResourceLoader(pi, { extensionApiPath, skillPath })
    const customTools = createTools(pi, ai.Type, { extensionsDir, extensionApiPath, reloadExtensions, getActiveChat, markGeneratedExtension })

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
    })

    const toolNames = result.session.agent.state.tools.map((tool) => tool.name)
    // console.info('[Nevermind AI] tools', toolNames)
    onEvent?.({ type: 'debug', label: 'tools', data: toolNames })

    const entry = sessions.get(chatId)
    if (entry) entry.unsubscribe = result.session.subscribe((event) => {
      if (event.type === 'message_update' && event.assistantMessageEvent.type === 'text_delta') {
        const delta = event.assistantMessageEvent.delta
        if (delta.includes('<tool_calls>') || delta.includes('<tool name=')) {
          console.warn('[Nevermind AI] model emitted raw tool-call text instead of structured tool calls', {
            provider: model.provider,
            id: model.id,
            api: model.api,
            delta,
          })
        }
        emit({ type: 'delta', text: delta })
      }
      if (event.type === 'tool_execution_start') {
        // console.info('[Nevermind AI] tool start', event.toolName)
        emit({ type: 'tool_start', name: event.toolName })
      }
      if (event.type === 'tool_execution_end') {
        // console.info('[Nevermind AI] tool end', event.toolName, { isError: event.isError })
        emit({ type: 'tool_end', name: event.toolName, isError: event.isError })
      }
    })

    onEvent?.({ type: 'ready' })
    return result.session
  }

  return { send, abort, reset }
}

function createResourceLoader(pi, { extensionApiPath, skillPath }) {
  const extensionBuilderSkill = {
    name: 'nevermind-extension-builder',
    description: 'Builds local Nevermind extensions that return declarative UI views. Use whenever the user asks to automate or create a command/action in Nevermind.',
    filePath: skillPath,
    baseDir: path.dirname(skillPath),
    sourceInfo: pi.createSyntheticSourceInfo(skillPath, { source: 'nevermind' }),
    disableModelInvocation: false,
  }

  return {
    getExtensions: () => ({ extensions: [], errors: [], runtime: pi.createExtensionRuntime() }),
    getSkills: () => ({ skills: [extensionBuilderSkill], diagnostics: [] }),
    getPrompts: () => ({ prompts: [], diagnostics: [] }),
    getThemes: () => ({ themes: [], diagnostics: [] }),
    getAgentsFiles: () => ({ agentsFiles: [{ path: 'NEVERMIND.md', content: systemContext(extensionApiPath) }] }),
    getSystemPrompt: () => systemPrompt(),
    getAppendSystemPrompt: () => [],
    extendResources: () => {},
    reload: async () => {},
  }
}

function createTools(pi, Type, { extensionsDir, extensionApiPath, reloadExtensions, getActiveChat, markGeneratedExtension, addAliasForChat }) {
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
      name: 'read_current_extension',
      label: 'Read Current Extension',
      description: 'Read the active generated extension source for this chat/action before tweaking it.',
      parameters: Type.Object({}),
      execute: async () => {
        const chat = getActiveChat?.()
        if (!chat?.generatedExtensionFile) return { content: [{ type: 'text', text: 'No current generated extension for this chat.' }], details: {} }
        const filePath = safeExtensionPath(extensionsDir, chat.generatedExtensionFile)
        const code = await fs.readFile(filePath, 'utf8')
        return { content: [{ type: 'text', text: code }], details: { filePath } }
      },
    }),
    pi.defineTool({
      name: 'write_extension',
      label: 'Write Extension',
      description: 'Write a generated Nevermind extension CommonJS module into the generated extensions directory.',
      parameters: Type.Object({
        filename: Type.String({ description: 'Safe filename ending in .cjs, for example image-grid.cjs' }),
        code: Type.String({ description: 'Complete CommonJS extension source code using module.exports = { ... }' }),
      }),
      execute: async (_toolCallId, params) => {
        const filePath = extensionPathForActiveChat(extensionsDir, params.filename, getActiveChat)
        validateCommonJs(params.code)
        await fs.writeFile(filePath, params.code)
        markGeneratedExtension?.(filePath)
        await reloadExtensions()
        const chat = getActiveChat?.()
        if (chat?.id) addAliasForChat?.(chat.id)
        return { content: [{ type: 'text', text: `Installed ${path.basename(filePath)}` }], details: { filePath } }
      },
    }),
    pi.defineTool({
      name: 'validate_extension',
      label: 'Validate Extension',
      description: 'Validate that the active generated extension file is syntactically valid CommonJS.',
      parameters: Type.Object({ filename: Type.String() }),
      execute: async (_toolCallId, params) => {
        const filePath = extensionPathForActiveChat(extensionsDir, params.filename, getActiveChat)
        validateCommonJs(await fs.readFile(filePath, 'utf8'))
        return { content: [{ type: 'text', text: `Validated ${path.basename(filePath)}` }], details: { filePath } }
      },
    }),
    pi.defineTool({
      name: 'install_extension',
      label: 'Install Extension',
      description: 'No-op compatibility tool. write_extension already installs/replaces the active generated extension idempotently.',
      parameters: Type.Object({ filename: Type.String() }),
      execute: async (_toolCallId, params) => {
        const filePath = extensionPathForActiveChat(extensionsDir, params.filename, getActiveChat)
        markGeneratedExtension?.(filePath)
        await reloadExtensions()
        return { content: [{ type: 'text', text: `${path.basename(filePath)} is already active.` }], details: { filePath } }
      },
    }),
  ]
}

function extensionPathForActiveChat(root, filename, getActiveChat) {
  const chat = getActiveChat?.()
  if (!chat?.id) return safeExtensionPath(root, filename)
  return safeExtensionPath(root, `${slugValue(chat.query || chat.title || 'action')}-${chat.id.slice(0, 8)}.cjs`)
}

function slugValue(value) {
  return String(value || 'action')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48) || 'action'
}

function safeExtensionPath(root, filename) {
  const safeName = path.basename(filename).replace(/[^a-zA-Z0-9._-]/g, '-')
  if (!safeName.endsWith('.cjs')) throw new Error('Extension filename must end in .cjs')
  const fullPath = path.resolve(root, safeName)
  if (!fullPath.startsWith(path.resolve(root) + path.sep)) throw new Error('Invalid extension path')
  return fullPath
}

function validateCommonJs(code) {
  new vm.Script(`(function (module, exports, require) {\n${code}\n})`)
}

function capabilities() {
  return {
    views: ['list', 'grid', 'preview', 'chat', 'form', 'progress'],
    viewOptions: ['sections', 'selectedItemId', 'onSelectionChange', 'isLoading', 'emptyView', 'searchBarPlaceholder', 'searchAccessory', 'pagination'],
    itemOptions: ['accessories', 'keywords', 'actionPanel'],
    actionPanel: ['sections', 'submenus'],
    shortcuts: ['local action shortcut', 'command globalShortcut', 'shortcutScope'],
    gridOptions: { layout: ['square', 'wide', 'compact'], aspectRatio: ['1', '16 / 9', '4 / 3'], columns: 'number' },
    actions: ['openPath', 'revealPath', 'quickLook', 'openWith', 'openUrl', 'copyText', 'pasteText', 'copyImage', 'trash', 'push', 'replace', 'pop', 'run', 'shellExec', 'shellScript'],
    namespaces: ['clipboard', 'files', 'apps', 'shell', 'storage', 'cache', 'state', 'ai'],
    shell: ['openExternal', 'exec', 'script', 'appleScript', 'which'],
    fileHelpers: ['find', 'findImages', 'findVideos', 'findMedia', 'selectedInFinder', 'openWithApps', 'open', 'readText', 'toFileUrl'],
    findOptions: ['limit', 'depth', 'extensions', 'kind', 'pattern', 'sortBy', 'order'],
    fileKinds: ['image', 'video', 'media'],
    sortBy: ['recent', 'modified', 'added', 'created', 'name', 'size'],
    fileFields: ['path', 'name', 'displayPath', 'url', 'fileUrl', 'videoUrl', 'thumbnailUrl', 'kind', 'extension', 'mtime', 'mtimeMs', 'birthtime', 'birthtimeMs', 'size'],
    storage: ['get', 'set', 'delete', 'clear', 'memo'],
  }
}

function systemPrompt() {
  return `You are Nevermind's extension-building agent.
Build local Nevermind extensions, not shell scripts.
When the first user message is vague, ask clarifying questions before using tools.
Do not call read_extension_api, list_capabilities, write_extension, validate_extension, or install_extension until the user has confirmed the desired command behavior.
Once the user provides enough details, start building immediately by calling read_extension_api in the same turn. Do not say you are going to call a tool; call it.
The only available tool names are: read_extension_api, list_capabilities, read_current_extension, write_extension, validate_extension, install_extension. Never invent tool names like read_file, write_file, list_directory, or bash.
Never write XML or pseudo tool calls in the chat. Use real structured tool calls only.
Never provide instructions to manually save extension files; if tool access fails, report the failure briefly and ask the user to retry.
Use read_extension_api before writing an extension.
When tweaking an existing generated action, call read_current_extension before writing and preserve existing behavior unless the user asks to remove it.
Use list_capabilities when unsure which UI or OS capabilities exist.
Only write .cjs extension files with write_extension.
write_extension is idempotent: it replaces and activates the current action's generated extension.
validate_extension can be used to syntax-check the active generated extension after writing.
install_extension is only a backwards-compatible no-op; do not rely on it for writing or replacing.
Keep generated commands small, local, and native-feeling.
For image grids, use file.url from ctx.files.findImages() or ctx.files.toFileUrl(path), never raw filesystem paths, so thumbnails render in Electron.
Use primaryAction for the Enter behavior. Put secondary item actions in actions; Nevermind exposes them under Cmd+K automatically.
Use ctx.actions.push/replace/pop for nested native views. Use ctx.actions.run for script work triggered from UI; handlers may return a native view.
When done, tell the user what command was installed and how to find it.`
}

function systemContext(extensionApiPath) {
  return `Nevermind is an Electron command palette. Your job is to create first-class local extensions using the documented API.
Extension API docs path: ${extensionApiPath}
Generated extensions are loaded by Nevermind after install_extension.`
}

module.exports = { createNevermindAi }
