import assert from 'node:assert/strict'
import test from 'node:test'
import { registerAppIpcHandlers, type AppIpcHandlersDeps } from './app-ipc-handlers'

function createDeps(overrides: Partial<AppIpcHandlersDeps> = {}) {
  const handles = new Map<string, (event: any, ...args: any[]) => unknown>()
  const listeners = new Map<string, (...args: any[]) => unknown>()
  const calls: string[] = []
  const deps: AppIpcHandlersDeps = {
    ipcMain: {
      handle: (channel, handler) => handles.set(channel, handler),
      on: (channel, handler) => listeners.set(channel, handler),
    },
    measureDebugPerformance: (_name, _data, fn) => fn(),
    summarizeDebugValue: (value) => value,
    searchActions: (query, options) => ({ query, options }),
    executeActionForIpc: (action) => action,
    executeViewActionForIpc: (action) => action,
    refreshViewForIpc: (input) => ({ refreshed: input }),
    pickFormFieldPaths: (_event, input) => ({ picked: input }),
    startFileDrag: () => calls.push('drag'),
    sendAiChatMessage: (message, chatId) => ({ message, chatId }),
    noteAiChatExited: (chatId) => ({ chatId }),
    abortAiChat: (chatId) => ({ chatId }),
    resetAiChat: (chatId) => ({ chatId }),
    setAlias: (action, alias) => ({ action, alias }),
    removeAlias: (action, alias) => ({ action, alias }),
    setShortcut: (action, shortcut) => ({ action, shortcut }),
    setPaletteHotkey: (accelerator) => ({ accelerator }),
    getSetting: (id) => ({ id }),
    openSystemKeyboardSettings: () => 'keyboard-settings',
    getShortcuts: () => [],
    removeShortcut: (actionId) => ({ actionId }),
    unregisterActionShortcuts: () => 'suspended',
    registerActionShortcuts: () => 'resumed',
    setOverride: (action, instruction) => ({ action, instruction }),
    clearOverride: (action) => action,
    duplicateCreatedAction: (action) => action,
    removeCreatedAction: (action) => action,
    getOrCreateExtensionChat: (file, title) => ({ file, title, messages: [] }),
    aiChatView: (item, options) => ({ type: 'chat', item, options }),
    normalizeHostViewResult: (result) => ({ normalized: result }),
    createDraftAiChat: (prompt) => ({ prompt, messages: [] }),
    getNevermindAuth: async () => null,
    setActiveNevermindBaseUrl: (baseUrl) => calls.push(`base:${baseUrl || ''}`),
    warmNevermindCompatibilityCache: (baseUrl) => calls.push(`warm:${baseUrl}`),
    logInfo: (message) => calls.push(`info:${message}`),
    userDataPath: () => '/user-data',
    signInToNevermind: async () => ({ ok: false, error: 'nope' }),
    invalidateExtensionRootItems: () => calls.push('invalidate'),
    broadcastAuthChanged: () => calls.push('broadcast-auth'),
    appIconCache: { get: (appPath) => ({ appPath }) },
    runningAppStatus: { getForRenderer: (appPaths) => ({ appPaths }) },
    paletteWindow: {
      setPaletteSizeForMode: (mode) => calls.push(`mode:${mode}`),
      centerWindow: () => calls.push('center'),
      hidePalette: () => calls.push('hide'),
      revealPalette: () => calls.push('reveal'),
    },
    requestQuitApp: (reason) => calls.push(`quit:${reason}`),
    hasCapability: () => true,
    processPlatform: 'darwin',
    getCameraMediaAccessStatus: () => 'granted',
    extensionWindowManager: { getState: (id) => ({ id }) },
    BrowserWindow: { fromWebContents: () => ({ close: () => calls.push('close-window') }) },
    logError: (message) => calls.push(`error:${message}`),
    logWarn: (message) => calls.push(`warn:${message}`),
    loggerDebug: (message) => calls.push(`debug:${message}`),
    ...overrides,
  }
  registerAppIpcHandlers(deps)
  return { handles, listeners, calls }
}

test('registerAppIpcHandlers registers core invoke handlers and drag listener', () => {
  const { handles, listeners } = createDeps()

  assert.equal(handles.has('actions:search'), true)
  assert.equal(handles.has('view:refresh'), true)
  assert.equal(handles.has('nevermind:auth-status'), true)
  assert.equal(handles.has('camera:request-access'), true)
  assert.equal(handles.has('logs:write'), true)
  assert.equal(listeners.has('drag:file'), true)
})

test('registerAppIpcHandlers preserves palette, camera, and window behavior', async () => {
  const { handles, calls } = createDeps()

  assert.deepEqual(await handles.get('palette:set-mode')?.({}, 'preview'), undefined)
  assert.deepEqual(await handles.get('camera:request-access')?.({}), { ok: true, status: 'granted' })
  assert.deepEqual(await handles.get('extension-window:get-state')?.({}, 'window-a'), { id: 'window-a' })
  await handles.get('extension-window:close')?.({ sender: 'sender' })

  assert.deepEqual(calls, ['mode:preview', 'center', 'close-window'])
})

test('registerAppIpcHandlers redacts auth status logs and keeps sign-in failures stable', async () => {
  const logEntries: unknown[] = []
  const { handles } = createDeps({
    getNevermindAuth: async () => ({ baseUrl: 'https://nvm.example', email: 'pablo@example.com' }),
    logInfo: (_message, data) => logEntries.push(data),
    signInToNevermind: async () => ({ ok: false, error: 'provider-secret' }),
  })

  assert.deepEqual(await handles.get('nevermind:auth-status')?.({}), { authed: true, email: 'pablo@example.com' })
  assert.deepEqual(logEntries, [{ authed: true }])
  assert.deepEqual(await handles.get('nevermind:sign-in')?.({}), { ok: false, error: 'Unable to sign in' })
})

test('registerAppIpcHandlers handles auth status and sign in side effects', async () => {
  const { handles, calls } = createDeps({
    getNevermindAuth: async () => ({ baseUrl: 'https://nvm.example', email: 'pablo@example.com' }),
    signInToNevermind: async () => ({ ok: true, auth: { baseUrl: 'https://nvm.example', email: 'pablo@example.com' } }),
  })

  assert.deepEqual(await handles.get('nevermind:auth-status')?.({}), { authed: true, email: 'pablo@example.com' })
  assert.deepEqual(await handles.get('nevermind:sign-in')?.({}), { ok: true, email: 'pablo@example.com' })
  assert.deepEqual(calls, [
    'base:https://nvm.example',
    'warm:https://nvm.example',
    'info:nevermind.auth-status.check',
    'base:https://nvm.example',
    'warm:https://nvm.example',
    'invalidate',
    'broadcast-auth',
  ])
})
