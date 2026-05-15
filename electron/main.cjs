const { app, BrowserWindow, globalShortcut, ipcMain, shell, screen, clipboard, nativeImage, nativeTheme, protocol, net } = require('electron')
const fs = require('node:fs/promises')
const path = require('node:path')
const os = require('node:os')
const crypto = require('node:crypto')
const { spawn, execFile } = require('node:child_process')
const { pathToFileURL } = require('node:url')
const { createNevermindAi } = require('./ai.cjs')

const isDev = Boolean(process.env.ELECTRON_RENDERER_URL)
const HOTKEY = 'Alt+Space'
const CLIPBOARD_LIMIT = 300
const FILE_RESULT_LIMIT = 6
const CLIPBOARD_POLL_INTERVAL_MS = 1000
const DEFAULT_WINDOW_SIZE = { width: 760, height: 520 }
const AI_CHAT_WINDOW_SIZE = { width: 900, height: 720 }
const IMAGE_EXTENSIONS = new Set(['.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp', '.tiff', '.heic'])
const LOCAL_FILE_PROTOCOL = 'nvm-file'
const LOCAL_THUMB_PROTOCOL = 'nvm-thumb'
const THUMBNAIL_SIZE = 512

protocol.registerSchemesAsPrivileged([
  { scheme: LOCAL_FILE_PROTOCOL, privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true, bypassCSP: true } },
  { scheme: LOCAL_THUMB_PROTOCOL, privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true, bypassCSP: true } },
])

let win
let appIndex = []
let fileIndex = []
let clipboardHistory = []
let statePath
let iconCacheDir
let extensionsDir
let saveTimer
let nevermindAi
let activeAiChatId
let userState = {
  recents: {},
  aliases: {},
  shortcuts: {},
  shortcutActions: {},
  overrides: {},
  clipboardHistory: [],
  aiChats: {},
}
const appIconCache = new Map()
const extensionRegistry = new Map()
const extensionActionHandlers = new Map()

const INTERNAL_EXTENSIONS = []

const BUILT_IN_ACTIONS = [
  {
    id: 'builtin:lock-screen',
    kind: 'builtin',
    builtin: 'lock-screen',
    title: 'Lock Screen',
    subtitle: 'Secure this computer',
    icon: 'lock',
    score: 22,
  },
  {
    id: 'builtin:sleep',
    kind: 'builtin',
    builtin: 'sleep',
    title: 'Sleep',
    subtitle: 'Put this computer to sleep',
    icon: 'moon',
    score: 21,
  },
  {
    id: 'builtin:restart',
    kind: 'builtin',
    builtin: 'restart',
    title: 'Restart Computer',
    subtitle: 'Restart this computer',
    icon: 'restart',
    score: 20,
  },
  {
    id: 'builtin:settings',
    kind: 'builtin',
    builtin: 'settings',
    title: process.platform === 'darwin' ? 'Open System Settings' : 'Open Settings',
    subtitle: 'Open system preferences',
    icon: 'settings',
    score: 19,
  },
  {
    id: 'builtin:downloads',
    kind: 'builtin',
    builtin: 'open-path',
    targetPath: path.join(os.homedir(), 'Downloads'),
    title: 'Open Downloads',
    subtitle: '~/Downloads',
    icon: 'folder',
    score: 18,
  },
  {
    id: 'builtin:documents',
    kind: 'builtin',
    builtin: 'open-path',
    targetPath: path.join(os.homedir(), 'Documents'),
    title: 'Open Documents',
    subtitle: '~/Documents',
    icon: 'folder',
    score: 17,
  },
  {
    id: 'builtin:desktop',
    kind: 'builtin',
    builtin: 'open-path',
    targetPath: path.join(os.homedir(), 'Desktop'),
    title: 'Open Desktop',
    subtitle: '~/Desktop',
    icon: 'folder',
    score: 16,
  },
  {
    id: 'keyboard-shortcuts',
    kind: 'keyboard-shortcuts',
    title: 'Keyboard Shortcuts',
    subtitle: 'View, change, or remove global shortcuts',
    icon: 'keyboard',
    score: 16,
  },
  {
    id: 'builtin:quit',
    kind: 'builtin',
    builtin: 'quit',
    title: 'Quit Nevermind',
    subtitle: 'Close the app',
    icon: 'power',
    score: 15,
  },
]

function createWindow() {
  win = new BrowserWindow({
    width: DEFAULT_WINDOW_SIZE.width,
    height: DEFAULT_WINDOW_SIZE.height,
    show: false,
    frame: false,
    transparent: true,
    resizable: false,
    movable: true,
    alwaysOnTop: true,
    skipTaskbar: true,
    hasShadow: false,
    title: 'Nevermind',
    backgroundColor: '#00000000',
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  })

  win.on('blur', () => hidePalette())
  win.on('close', (event) => {
    if (!app.isQuiting) {
      event.preventDefault()
      hidePalette()
    }
  })

  win.webContents.once('did-finish-load', () => {
    if (isDev) showPalette()
  })

  if (isDev) {
    win.loadURL(process.env.ELECTRON_RENDERER_URL)
  } else {
    win.loadFile(path.join(__dirname, '..', 'dist', 'index.html'))
  }
}

function setPaletteSizeForMode(mode = 'default') {
  if (!win) return
  const size = mode === 'ai-chat' ? AI_CHAT_WINDOW_SIZE : DEFAULT_WINDOW_SIZE
  win.setSize(size.width, size.height, false)
}

function centerWindow() {
  if (!win) return
  const cursor = screen.getCursorScreenPoint()
  const display = screen.getDisplayNearestPoint(cursor)
  const { width, height } = win.getBounds()
  const { x, y, width: sw, height: sh } = display.workArea
  win.setBounds({
    x: Math.round(x + (sw - width) / 2),
    y: Math.round(y + Math.min(sh * 0.18, 180)),
    width,
    height,
  })
}

function showPalette(options = {}) {
  if (!win) return
  centerWindow()
  if (options.deferReveal) {
    win.setOpacity(0)
    setTimeout(() => win?.setOpacity(1), 250)
  } else {
    win.setOpacity(1)
  }
  if (options.skipShownEvent) win.webContents.send('palette:shortcut-show')
  else win.webContents.send('palette:shown')
  win.show()
  win.focus()
  win.webContents.focus()
}

function revealPalette() {
  win?.setOpacity(1)
}

function hidePalette() {
  if (!win) return
  win.webContents.send('palette:hidden')
  win.hide()
}

function togglePalette() {
  if (win?.isVisible()) hidePalette()
  else showPalette()
}

function registerHotkey() {
  const ok = globalShortcut.register(HOTKEY, togglePalette)
  if (ok) console.log(`Registered global shortcut: ${HOTKEY}`)
  else console.warn(`Could not register global shortcut: ${HOTKEY}`)

  globalShortcut.register('CommandOrControl+Alt+I', () => {
    if (!win) return
    if (win.webContents.isDevToolsOpened()) win.webContents.closeDevTools()
    else win.webContents.openDevTools({ mode: 'detach' })
  })
}

function normalize(value) {
  return String(value || '').toLowerCase().trim()
}

function hashValue(value) {
  return crypto.createHash('sha1').update(String(value)).digest('hex')
}

function score(value, query) {
  const v = normalize(value)
  const q = normalize(query)
  if (!q) return 0
  if (v === q) return 100
  if (v.startsWith(q)) return 80
  if (v.includes(q)) return 50
  let pos = 0
  for (const ch of q) {
    pos = v.indexOf(ch, pos)
    if (pos === -1) return 0
    pos += 1
  }
  return 20
}

function actionAliases(actionId) {
  const value = userState.aliases[actionId]
  if (!value) return []
  return Array.isArray(value) ? value : [value]
}

function actionSearchScore(action, query) {
  if (!query.trim()) return action.score || 0
  return Math.max(
    score(action.title, query),
    score(action.subtitle, query),
    ...(action.aliases || []).map((alias) => score(alias, query)),
    ...actionAliases(action.id).map((alias) => score(alias, query)),
  )
}

function recentBoost(actionId) {
  const recent = userState.recents[actionId]
  if (!recent) return 0
  const ageHours = Math.max(0, (Date.now() - recent.lastUsed) / 36e5)
  const recency = Math.max(0, 120 - ageHours * 3)
  const frequency = Math.min(30, (recent.count || 1) * 3)
  return recency + frequency
}

function defaultActionIdFor(action) {
  if (action.defaultActionId) return action.defaultActionId
  if (action.kind === 'builtin') return action.id
  if (action.kind === 'calculate') return 'default:calculator'
  return null
}

function withDefaultOverride(action) {
  const defaultActionId = defaultActionIdFor(action)
  if (!defaultActionId) return action
  const override = userState.overrides[defaultActionId]
  return {
    ...action,
    defaultActionId,
    isOverridden: Boolean(override),
    overrideSummary: override?.instruction,
  }
}

function rankAction(action, query) {
  const base = actionSearchScore(action, query)
  if (query.trim() && base <= 0) return null
  return {
    ...action,
    aliases: [...(action.aliases || []), ...actionAliases(action.id)],
    score: base + recentBoost(action.id),
    lastUsed: userState.recents[action.id]?.lastUsed || 0,
  }
}

function recordRecent(action) {
  if (!action?.id) return
  const current = userState.recents[action.id] || { count: 0, lastUsed: 0, title: action.title }
  userState.recents[action.id] = {
    count: current.count + 1,
    lastUsed: Date.now(),
    title: action.title,
    kind: action.kind,
  }
  scheduleSaveState()
}

function isLikelyUrl(input) {
  const value = input.trim()
  if (!value || value.includes(' ')) return false
  if (/^https?:\/\//i.test(value)) return true
  return /^[\w-]+(\.[\w-]+)+([/:?#].*)?$/i.test(value)
}

function getUrlFromQuery(query) {
  const trimmed = query.trim()
  const opened = trimmed.match(/^open\s+(.+)$/i)?.[1]?.trim() || trimmed
  if (!isLikelyUrl(opened)) return null
  return /^https?:\/\//i.test(opened) ? opened : `https://${opened}`
}

function calculate(query) {
  const expression = query.trim().replace(/^=?\s*/, '').replace(/^calc(?:ulate)?\s+/i, '')
  if (!expression || !/[+\-*/%^()]/.test(expression)) return null
  if (!/^[\d\s.+\-*/%^(),]+$/.test(expression)) return null

  try {
    const jsExpression = expression.replace(/%/g, '/100')
    const result = Function(`"use strict"; return (${jsExpression})`)()
    if (typeof result !== 'number' || !Number.isFinite(result)) return null
    return Number.isInteger(result) ? String(result) : String(Number(result.toPrecision(12)))
  } catch {
    return null
  }
}

async function getAppIconDataUrl(appPath) {
  if (process.platform !== 'darwin' || !appPath || !appPath.endsWith('.app')) return null
  if (appIconCache.has(appPath)) return appIconCache.get(appPath)

  const promise = (async () => {
    try {
      const cachePath = path.join(iconCacheDir, `${hashValue(appPath)}.png`)
      const cached = await fs.readFile(cachePath).catch(() => null)
      if (cached) return `data:image/png;base64,${cached.toString('base64')}`

      const { fileIconToBuffer } = await import('file-icon')
      const png = Buffer.from(await fileIconToBuffer(appPath, { size: 64 }))
      await fs.mkdir(iconCacheDir, { recursive: true })
      await fs.writeFile(cachePath, png).catch(() => {})
      return `data:image/png;base64,${png.toString('base64')}`
    } catch (error) {
      console.warn(`Failed to load app icon for ${appPath}`, error)
      return null
    }
  })()

  appIconCache.set(appPath, promise)
  const result = await promise
  appIconCache.set(appPath, result)
  return result
}

function extensionActionFromCommand(extension, command) {
  return {
    id: `extension:${extension.id}:${command.id}`,
    kind: 'extension-command',
    extensionId: extension.id,
    commandId: command.id,
    aiChatId: extension.__chatId,
    removable: Boolean(extension.__generated),
    title: command.title,
    subtitle: command.subtitle || extension.title || 'Extension command',
    aliases: command.aliases || [],
    icon: command.icon || 'sparkles',
    score: command.score || 12,
  }
}

function hasReadyGeneratedExtension(item) {
  return item.status === 'ready' && item.generatedExtensionFile
}

function aiChatActionFromItem(item) {
  return {
    id: `ai-chat:${item.id}`,
    kind: 'ai-chat',
    title: item.title || item.query,
    subtitle: item.status === 'ready' ? 'Tweak AI-created action' : 'Continue AI automation chat',
    query: item.query,
    aiChatId: item.id,
    aliases: [item.query],
    icon: 'sparkles',
    score: 13,
    lastUsed: item.updatedAt || item.createdAt || 0,
  }
}

function getOrCreateAiChat(query, options = {}) {
  const id = hashValue(query.trim())
  const current = userState.aiChats[id]
  if (current && !options.fresh) return current
  const item = {
    id,
    query: query.trim(),
    title: query.trim(),
    status: 'running',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    messages: [
      { role: 'assistant', content: `What should “${query.trim()}” do? Tell me the exact behavior, inputs, and what UI you want, then I’ll build it.` },
    ],
  }
  userState.aiChats[id] = item
  scheduleSaveState()
  return item
}

function appendAiChatMessage(chatId, role, content) {
  const chat = userState.aiChats[chatId]
  if (!chat || !content) return
  chat.messages = [...(chat.messages || []), { role, content }].slice(-100)
  chat.updatedAt = Date.now()
  scheduleSaveState()
}

function appendAiChatDelta(chatId, text) {
  const chat = userState.aiChats[chatId]
  if (!chat || !text) return
  const messages = chat.messages || []
  const last = messages[messages.length - 1]
  if (last?.role === 'assistant') last.content = `${last.content}${text}`
  else messages.push({ role: 'assistant', content: text })
  chat.messages = messages.slice(-100)
  chat.updatedAt = Date.now()
  scheduleSaveState()
}

function aiChatView(item, options = {}) {
  return {
    type: 'chat',
    title: `Automate “${item.query}”`,
    aiChat: true,
    chatId: item.id,
    initialPrompt: undefined,
    messages: item.messages || [],
  }
}

function clipboardActionFromItem(item) {
  return {
    id: `clipboard:${item.id}`,
    kind: 'clipboard',
    title: clipboardItemTitle(item),
    subtitle: clipboardItemSubtitle(item),
    clipboardType: item.type,
    text: item.text,
    imageDataUrl: item.imageDataUrl,
    thumbnailUrl: item.thumbnailUrl,
    icon: 'clipboard',
    score: 3,
    lastUsed: item.createdAt || 0,
  }
}

function searchActions(query, options = {}) {
  const q = query.trim()

  if (options.clipboardOnly) {
    const results = []
    for (const item of clipboardHistory) {
      const action = clipboardActionFromItem(item)
      const ranked = q ? rankAction(action, q) : action
      if (ranked) results.push(ranked)
    }
    return results
      .sort((a, b) => q ? b.score - a.score || b.lastUsed - a.lastUsed : b.lastUsed - a.lastUsed)
      .slice(0, CLIPBOARD_LIMIT)
  }

  const results = []
  const url = getUrlFromQuery(q)
  const mathResult = q ? calculate(q) : null

  if (url) {
    results.push({
      id: `open-url:${url}`,
      kind: 'open-url',
      title: `Open ${url.replace(/^https?:\/\//, '')}`,
      subtitle: 'Open website',
      url,
      icon: 'globe',
      score: 100,
    })
  }

  if (mathResult !== null) {
    results.push({
      id: `calculate:${q}`,
      kind: 'calculate',
      title: `${q} = ${mathResult}`,
      subtitle: 'Copy result to clipboard',
      query: q,
      result: mathResult,
      icon: 'calculator',
      score: 105,
    })
  }

  for (const item of BUILT_IN_ACTIONS) {
    const ranked = rankAction(withDefaultOverride(item), q)
    if (ranked) results.push(ranked)
  }

  for (const command of extensionRegistry.values()) {
    const ranked = rankAction(extensionActionFromCommand(command.extension, command.command), q)
    if (ranked) results.push(ranked)
  }

  for (const item of Object.values(userState.aiChats || {})) {
    if (hasReadyGeneratedExtension(item)) continue
    const ranked = rankAction(aiChatActionFromItem(item), q)
    if (ranked) {
      ranked.lastUsed = Math.max(ranked.lastUsed || 0, item.updatedAt || item.createdAt || 0)
      results.push(ranked)
    }
  }

  const latestClipboardTime = clipboardHistory[0]?.createdAt || 0
  const clipboardHistoryAction = rankAction({
    id: 'clipboard-history',
    kind: 'clipboard-history',
    title: 'Clipboard History',
    subtitle: clipboardHistory.length ? `Show all ${clipboardHistory.length} copied items` : 'Show copied items',
    icon: 'clipboard',
    score: 14,
    lastUsed: latestClipboardTime ? latestClipboardTime - 1 : 0,
  }, q)
  if (clipboardHistoryAction) {
    clipboardHistoryAction.lastUsed = Math.max(clipboardHistoryAction.lastUsed || 0, latestClipboardTime ? latestClipboardTime - 1 : 0)
    results.push(clipboardHistoryAction)
  }

  for (const item of appIndex) {
    const action = {
      id: `app:${item.id}`,
      kind: 'app',
      title: item.name,
      subtitle: 'Launch application',
      app: item,
      icon: 'app',
      score: 5,
    }
    const ranked = rankAction(action, q)
    if (ranked) results.push(ranked)
  }

  let fileMatches = 0
  for (const item of fileIndex) {
    if (fileMatches >= FILE_RESULT_LIMIT) break
    const action = {
      id: `file:${item.path}`,
      kind: 'file',
      title: item.name,
      subtitle: item.displayPath,
      filePath: item.path,
      icon: 'folder',
      score: 4,
    }
    const ranked = rankAction(action, q)
    if (ranked) {
      results.push(ranked)
      fileMatches += 1
    }
  }

  for (const item of clipboardHistory) {
    const ranked = rankAction(clipboardActionFromItem(item), q)
    if (ranked) {
      ranked.lastUsed = Math.max(ranked.lastUsed || 0, item.createdAt || 0)
      results.push(ranked)
    }
  }

  if (q && !url && mathResult === null) {
    results.push({
      id: `ai:${q}`,
      kind: 'ai-placeholder',
      title: `Press Tab to automate "${q}"`,
      subtitle: 'Automate with AI',
      query: q,
      icon: 'bolt',
      score: 90,
    })
  }

  if (q) {
    results.push({
      id: `web-search:${q}`,
      kind: 'web-search',
      title: `Search the web for "${q}"`,
      subtitle: 'Search instead',
      query: q,
      icon: 'search',
      score: 10 + recentBoost(`web-search:${q}`),
    })
  }

  return results
    .sort((a, b) => {
      if (!q && a.lastUsed !== b.lastUsed) return b.lastUsed - a.lastUsed
      return b.score - a.score || b.lastUsed - a.lastUsed || a.title.localeCompare(b.title)
    })
    .slice(0, 10)
}

async function executeAction(action, options = {}) {
  if (!action) return
  recordRecent(action)

  switch (action.kind) {
    case 'open-url':
      await shell.openExternal(action.url)
      break
    case 'web-search':
      await shell.openExternal(`https://www.google.com/search?q=${encodeURIComponent(action.query)}`)
      break
    case 'app':
      await launchApp(action.app)
      break
    case 'clipboard':
      if (action.clipboardType === 'image' && action.imageDataUrl) {
        clipboard.writeImage(nativeImage.createFromDataURL(action.imageDataUrl))
      } else {
        clipboard.writeText(action.text || '')
      }
      break
    case 'clipboard-history': {
      const wasVisible = Boolean(win?.isVisible())
      if (wasVisible) {
        centerWindow()
        win.focus()
        win.webContents.focus()
      } else {
        showPalette({ skipShownEvent: true, deferReveal: true })
      }
      win?.webContents.send('clipboard-history:open', { openedFromHidden: !wasVisible, revealWhenReady: !wasVisible })
      return
    }
    case 'file':
      await shell.openPath(action.filePath)
      break
    case 'calculate':
      clipboard.writeText(action.result)
      break
    case 'builtin':
      await executeBuiltin(action)
      break
    case 'extension-command': {
      const view = await executeExtensionCommand(action)
      if (view) return { view }
      break
    }
    case 'ai-chat': {
      const item = userState.aiChats[action.aiChatId]
      if (item) return { view: aiChatView(item) }
      break
    }
    case 'ai-placeholder': {
      const item = getOrCreateAiChat(action.query, { fresh: true })
      return { view: aiChatView(item, { start: item.messages.length <= 1 }) }
    }
  }

  if (!options.keepPaletteOpen) hidePalette()
}

function extensionEntryForAction(action) {
  const direct = extensionRegistry.get(`${action.extensionId}:${action.commandId}`)
  if (direct) return direct
  const matches = Array.from(extensionRegistry.values()).filter((entry) => entry.extension.id === action.extensionId)
  return matches.length === 1 ? matches[0] : null
}

async function executeExtensionCommand(action) {
  const entry = extensionEntryForAction(action)
  if (!entry) {
    console.warn(`Extension command not found: ${action.extensionId}:${action.commandId}`)
    return null
  }
  const ctx = createExtensionContext(entry.extension, entry.command)
  const result = await entry.command.run(ctx)
  return normalizeExtensionView(result, entry)
}

function normalizeExtensionView(result, entry) {
  if (!result) return null
  const view = result.type ? result : result.view?.type ? result.view : null
  return view ? normalizeView(view, entry) : null
}

function normalizeView(view, entry) {
  return {
    ...view,
    actions: normalizeViewActions(view.actions, entry),
    items: Array.isArray(view.items) ? view.items.map((item) => ({
      ...item,
      actions: normalizeViewActions(item.actions, entry),
      primaryAction: normalizeViewAction(item.primaryAction, entry),
    })) : view.items,
  }
}

function normalizeViewActions(actions, entry) {
  return Array.isArray(actions) ? actions.map((action) => normalizeViewAction(action, entry)).filter(Boolean) : actions
}

function normalizeViewAction(action, entry) {
  if (!action) return null
  if (action.__handler && typeof action.__handler === 'function') {
    const handlerId = crypto.randomUUID()
    extensionActionHandlers.set(handlerId, { entry, handler: action.__handler })
    const { __handler, ...rest } = action
    return { ...rest, type: 'runExtensionAction', handlerId }
  }
  if ((action.type === 'pushView' || action.type === 'replaceView') && action.view) {
    return { ...action, view: normalizeView(action.view, entry) }
  }
  return action
}

async function executeViewAction(action) {
  switch (action?.type) {
    case 'openPath':
      await shell.openPath(action.path)
      break
    case 'revealPath':
      shell.showItemInFolder(action.path)
      break
    case 'openUrl':
      await shell.openExternal(action.url)
      break
    case 'copyText':
      clipboard.writeText(action.text || '')
      break
    case 'copyImage':
      if (action.path) clipboard.writeImage(nativeImage.createFromPath(expandUserPath(action.path)))
      else clipboard.writeImage(nativeImage.createFromDataURL(action.imageDataUrl))
      break
    case 'pushView':
      return { view: action.view, navigation: 'push' }
    case 'replaceView':
      return { view: action.view, navigation: 'replace' }
    case 'popView':
      return { navigation: 'pop' }
    case 'runExtensionAction': {
      const record = extensionActionHandlers.get(action.handlerId)
      if (!record) return { toast: { message: 'Action is no longer available', tone: 'error' } }
      const result = await record.handler(createExtensionContext(record.entry.extension, record.entry.command))
      const view = normalizeExtensionView(result, record.entry)
      return view ? { view, navigation: result?.navigation || 'push' } : result
    }
  }
}

async function executeBuiltin(action) {
  switch (action.builtin) {
    case 'lock-screen':
      if (process.platform === 'darwin') {
        spawn('/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession', ['-suspend'], { detached: true, stdio: 'ignore' }).unref()
      } else if (process.platform === 'win32') {
        spawn('rundll32.exe', ['user32.dll,LockWorkStation'], { detached: true, stdio: 'ignore' }).unref()
      } else {
        spawn('sh', ['-lc', 'loginctl lock-session || xdg-screensaver lock || gnome-screensaver-command -l'], { detached: true, stdio: 'ignore' }).unref()
      }
      break
    case 'sleep':
      if (process.platform === 'darwin') spawn('pmset', ['sleepnow'], { detached: true, stdio: 'ignore' }).unref()
      else if (process.platform === 'win32') spawn('rundll32.exe', ['powrprof.dll,SetSuspendState', '0,1,0'], { detached: true, stdio: 'ignore' }).unref()
      else spawn('systemctl', ['suspend'], { detached: true, stdio: 'ignore' }).unref()
      break
    case 'restart':
      if (process.platform === 'darwin') spawn('osascript', ['-e', 'tell application "System Events" to restart'], { detached: true, stdio: 'ignore' }).unref()
      else spawn('shutdown', [process.platform === 'win32' ? '/r' : '-r', process.platform === 'win32' ? '/t' : 'now', process.platform === 'win32' ? '0' : ''], { detached: true, stdio: 'ignore' }).unref()
      break
    case 'settings':
      if (process.platform === 'darwin') await shell.openExternal('x-apple.systempreferences:')
      else if (process.platform === 'win32') await shell.openExternal('ms-settings:')
      else spawn('sh', ['-lc', 'gnome-control-center || systemsettings || xfce4-settings-manager'], { detached: true, stdio: 'ignore' }).unref()
      break
    case 'open-path':
      await shell.openPath(action.targetPath)
      break
    case 'quit':
      app.isQuiting = true
      app.quit()
      break
  }
}

function expandUserPath(value) {
  if (!value) return value
  if (value === '~') return os.homedir()
  if (value.startsWith('~/')) return path.join(os.homedir(), value.slice(2))
  return value
}

function fileUrlForPath(filePath) {
  return `${LOCAL_FILE_PROTOCOL}:${pathToFileURL(filePath).href.slice('file:'.length)}`
}

function thumbnailUrlForPath(filePath) {
  return `${LOCAL_THUMB_PROTOCOL}://thumb?path=${encodeURIComponent(filePath)}`
}

function isImagePath(filePath) {
  return IMAGE_EXTENSIONS.has(path.extname(filePath).toLowerCase())
}

async function thumbnailResponseForPath(filePath) {
  const stat = await fs.stat(filePath)
  const key = crypto.createHash('sha1').update(`${filePath}:${stat.mtimeMs}:${stat.size}:${THUMBNAIL_SIZE}`).digest('hex')
  const cachedPath = path.join(iconCacheDir, 'thumbs', `${key}.png`)

  try {
    const cached = await fs.readFile(cachedPath)
    return new Response(cached, { headers: { 'content-type': 'image/png', 'cache-control': 'public, max-age=31536000, immutable' } })
  } catch {}

  let image = null
  if (typeof nativeImage.createThumbnailFromPath === 'function') {
    image = await nativeImage.createThumbnailFromPath(filePath, { width: THUMBNAIL_SIZE, height: THUMBNAIL_SIZE })
  }
  if (!image || image.isEmpty()) image = nativeImage.createFromPath(filePath).resize({ width: THUMBNAIL_SIZE, quality: 'good' })
  if (!image || image.isEmpty()) return net.fetch(pathToFileURL(filePath).href)

  const png = image.toPNG()
  await fs.mkdir(path.dirname(cachedPath), { recursive: true })
  await fs.writeFile(cachedPath, png).catch(() => {})
  return new Response(png, { headers: { 'content-type': 'image/png', 'cache-control': 'public, max-age=31536000, immutable' } })
}

function registerLocalFileProtocol() {
  protocol.handle(LOCAL_FILE_PROTOCOL, (request) => {
    const url = new URL(request.url)
    const encodedPath = url.host ? `/${url.host}${url.pathname}` : url.pathname
    const requestPath = decodeURIComponent(encodedPath)
    if (!path.isAbsolute(requestPath)) return new Response('Invalid file path', { status: 400 })
    return net.fetch(pathToFileURL(requestPath).href)
  })

  protocol.handle(LOCAL_THUMB_PROTOCOL, async (request) => {
    const requestPath = decodeURIComponent(new URL(request.url).searchParams.get('path') || '')
    if (!path.isAbsolute(requestPath)) return new Response('Invalid file path', { status: 400 })
    try {
      return await thumbnailResponseForPath(requestPath)
    } catch (error) {
      console.error('Failed to create thumbnail', requestPath, error)
      return new Response('Thumbnail not found', { status: 404 })
    }
  })
}

function fileToExtensionFile(filePath) {
  return {
    path: filePath,
    name: path.basename(filePath),
    displayPath: displayUserPath(filePath),
    url: isImagePath(filePath) ? thumbnailUrlForPath(filePath) : fileUrlForPath(filePath),
    fileUrl: fileUrlForPath(filePath),
    thumbnailUrl: isImagePath(filePath) ? thumbnailUrlForPath(filePath) : null,
  }
}

async function findFiles(roots, options = {}) {
  const limit = options.limit || 100
  const maxDepth = options.depth ?? 2
  const extensions = options.extensions ? new Set(options.extensions.map((ext) => ext.toLowerCase())) : null
  const found = []

  async function walk(dir, depth) {
    if (found.length >= limit) return
    let entries = []
    try {
      entries = await fs.readdir(dir, { withFileTypes: true })
    } catch {
      return
    }

    for (const entry of entries) {
      if (found.length >= limit || entry.name.startsWith('.')) continue
      const fullPath = path.join(dir, entry.name)
      if (entry.isFile()) {
        if (!extensions || extensions.has(path.extname(entry.name).toLowerCase())) {
          found.push(fileToExtensionFile(fullPath))
        }
        continue
      }
      if (entry.isDirectory() && depth > 0) await walk(fullPath, depth - 1)
    }
  }

  await Promise.all(roots.map((root) => walk(expandUserPath(root), maxDepth)))
  return found
}

function dragIconForPath(filePath) {
  const image = nativeImage.createFromPath(filePath)
  if (!image.isEmpty()) return image.resize({ width: 64, height: 64, quality: 'good' })
  return nativeImage.createFromDataURL('data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=')
}

function startFileDrag(event, filePath) {
  const resolvedPath = expandUserPath(filePath)
  if (!resolvedPath || !path.isAbsolute(resolvedPath)) return
  event.sender.startDrag({ file: resolvedPath, icon: dragIconForPath(resolvedPath) })
}

async function selectedInFinder() {
  if (process.platform !== 'darwin') return []
  const script = 'tell application "Finder" to get POSIX path of (selection as alias list)'
  return new Promise((resolve) => {
    execFile('osascript', ['-e', script], (error, stdout) => {
      if (error) return resolve([])
      resolve(stdout.split(', ').map((item) => item.trim()).filter(Boolean).map(fileToExtensionFile))
    })
  })
}

function createExtensionContext(extension, command) {
  return {
    extension,
    command,
    ui: {
      list: (view) => ({ ...view, type: 'list' }),
      grid: (view) => ({ ...view, type: 'grid' }),
      detail: (view) => ({ ...view, type: 'detail' }),
      chat: (view) => ({ ...view, type: 'chat' }),
      form: (view) => ({ ...view, type: 'form' }),
      progress: (view) => ({ ...view, type: 'progress' }),
      item: (item) => item,
      actions: (actions) => actions,
      empty: (title = 'Nothing here', subtitle = '') => ({ type: 'detail', title, content: `# ${title}${subtitle ? `\n\n${subtitle}` : ''}` }),
      loading: (title = 'Loading…') => ({ type: 'progress', title, steps: [{ title, status: 'active' }] }),
      error: (title = 'Something went wrong', message = '') => ({ type: 'detail', title, content: `# ${title}${message ? `\n\n${message}` : ''}` }),
    },
    actions: {
      openPath: (filePath, title = 'Open') => ({ type: 'openPath', title, path: filePath }),
      revealPath: (filePath, title = 'Reveal in Finder') => ({ type: 'revealPath', title, path: filePath }),
      openUrl: (url, title = 'Open URL') => ({ type: 'openUrl', title, url }),
      copyText: (text, title = 'Copy') => ({ type: 'copyText', title, text }),
      copyImage: (image, title = 'Copy image') => String(image || '').startsWith('data:') ? ({ type: 'copyImage', title, imageDataUrl: image }) : ({ type: 'copyImage', title, path: image }),
      push: (title, view) => ({ type: 'pushView', title, view }),
      replace: (title, view) => ({ type: 'replaceView', title, view }),
      pop: (title = 'Back') => ({ type: 'popView', title }),
      run: (title, handler, options = {}) => ({ ...options, type: 'runExtensionAction', title, __handler: handler }),
    },
    clipboard: {
      readText: () => clipboard.readText(),
      writeText: (text) => clipboard.writeText(String(text || '')),
      readImage: () => clipboard.readImage().toDataURL(),
      writeImage: (imageDataUrl) => clipboard.writeImage(nativeImage.createFromDataURL(imageDataUrl)),
    },
    files: {
      find: findFiles,
      findImages: (roots, options) => findFiles(roots, { ...options, extensions: Array.from(IMAGE_EXTENSIONS) }),
      selectedInFinder,
      open: (filePath) => shell.openPath(expandUserPath(filePath)),
      readText: (filePath) => fs.readFile(expandUserPath(filePath), 'utf8'),
      toFileUrl: (filePath) => fileUrlForPath(expandUserPath(filePath)),
    },
    apps: {
      launch: (appPath) => shell.openPath(expandUserPath(appPath)),
      frontmost: () => null,
    },
    shell: {
      openExternal: (url) => shell.openExternal(url),
    },
    cache: new Map(),
    state: {},
    ai: {},
  }
}

function initNevermindAi() {
  nevermindAi = createNevermindAi({
    agentDir: path.join(app.getPath('userData'), 'pi-agent'),
    workspaceDir: path.join(app.getPath('userData'), 'ai-workspace'),
    extensionsDir,
    extensionApiPath: path.join(__dirname, '..', 'docs', 'extension-api.md'),
    skillPath: path.join(__dirname, '..', 'resources', 'skills', 'nevermind-extension-builder', 'SKILL.md'),
    reloadExtensions: loadExtensions,
    getActiveChat: () => activeAiChatId ? userState.aiChats[activeAiChatId] : null,
    markGeneratedExtension: (filePath) => markGeneratedExtensionForActiveChat(filePath),
    onEvent: (event) => {
      const chatId = event.chatId || activeAiChatId
      if (chatId && event.type === 'delta' && event.text) appendAiChatDelta(chatId, event.text)
      if (chatId && event.type === 'tool_start' && event.name) appendAiChatMessage(chatId, 'system', `Using ${event.name}…`)
      if (chatId && event.type === 'error' && event.message) appendAiChatMessage(chatId, 'system', event.message)
      if (chatId && event.type === 'done' && userState.aiChats[chatId]) {
        if (userState.aiChats[chatId].status !== 'ready') userState.aiChats[chatId].status = 'done'
        userState.aiChats[chatId].updatedAt = Date.now()
        scheduleSaveState()
      }
      win?.webContents.send('ai:chat:event', { ...event, chatId })
    },
  })
}

function aiChatPromptWithContext(message, chatId) {
  const chat = userState.aiChats[chatId]
  const messages = (chat?.messages || [])
    .filter((item) => item.role === 'user' || item.role === 'assistant')
    .slice(-12)

  if (!messages.length) return message

  const transcript = messages
    .map((item) => `${item.role === 'user' ? 'User' : 'Assistant'}: ${item.content}`)
    .join('\n\n')

  return `Use this Nevermind AI chat transcript as context. Do not ask questions that the user already answered. If the user has now provided enough details, proceed by calling read_extension_api immediately; do not merely say you will.\n\n${transcript}\n\nNew user message:\n${message}`
}

function markGeneratedExtensionForActiveChat(filePath) {
  const chat = activeAiChatId ? userState.aiChats[activeAiChatId] : null
  if (!chat) return
  chat.generatedExtensionFile = path.basename(filePath)
  chat.status = 'ready'
  chat.updatedAt = Date.now()
  scheduleSaveState()
}

async function sendAiChatMessage(message, chatId) {
  if (!nevermindAi) initNevermindAi()
  activeAiChatId = chatId || activeAiChatId
  const prompt = activeAiChatId ? aiChatPromptWithContext(message, activeAiChatId) : message
  if (activeAiChatId) appendAiChatMessage(activeAiChatId, 'user', message)
  return nevermindAi.send(prompt, activeAiChatId)
}

async function abortAiChat(chatId) {
  return nevermindAi?.abort(chatId || activeAiChatId)
}

async function resetAiChat(chatId) {
  activeAiChatId = chatId || activeAiChatId
  return nevermindAi?.reset(activeAiChatId)
}

async function loadExtensions() {
  extensionRegistry.clear()
  for (const extension of INTERNAL_EXTENSIONS) registerExtension(extension)

  await fs.mkdir(extensionsDir, { recursive: true })
  const entries = await fs.readdir(extensionsDir, { withFileTypes: true }).catch(() => [])
  for (const entry of entries) {
    if (!entry.isFile() || !entry.name.endsWith('.cjs')) continue
    const fullPath = path.join(extensionsDir, entry.name)
    try {
      delete require.cache[require.resolve(fullPath)]
      const extension = require(fullPath)
      extension.__filePath = fullPath
      extension.__generated = true
      const chat = Object.values(userState.aiChats || {}).find((item) => item.generatedExtensionFile === entry.name)
      if (chat) extension.__chatId = chat.id
      registerExtension(extension)
    } catch (error) {
      console.error(`Failed to load extension ${fullPath}`, error)
    }
  }
}

function registerExtension(extension) {
  if (!extension?.id || !Array.isArray(extension.commands)) return
  if (extension.__generated && extension.__chatId) {
    for (const key of Array.from(extensionRegistry.keys())) {
      if (extensionRegistry.get(key)?.extension?.__chatId === extension.__chatId) extensionRegistry.delete(key)
    }
  }
  for (const command of extension.commands) {
    if (!command?.id || !command.title || typeof command.run !== 'function') continue
    extensionRegistry.set(`${extension.id}:${command.id}`, { extension, command })
  }
}

async function launchApp(item) {
  if (!item) return
  if (process.platform === 'darwin') {
    spawn('open', [item.path], { detached: true, stdio: 'ignore' }).unref()
    return
  }
  if (process.platform === 'win32') {
    await shell.openPath(item.path)
    return
  }
  if (item.command) {
    spawn(item.command, { shell: true, detached: true, stdio: 'ignore' }).unref()
    return
  }
  await shell.openPath(item.path)
}

async function scanMacApps() {
  const roots = ['/Applications', '/System/Applications', path.join(os.homedir(), 'Applications')]
  const found = []

  async function walk(dir, depth) {
    let entries = []
    try {
      entries = await fs.readdir(dir, { withFileTypes: true })
    } catch {
      return
    }

    await Promise.all(entries.map(async (entry) => {
      if (!entry.isDirectory() || entry.name.startsWith('.')) return
      const fullPath = path.join(dir, entry.name)
      if (entry.name.endsWith('.app')) {
        found.push({
          id: fullPath,
          name: entry.name.replace(/\.app$/i, ''),
          path: fullPath,
        })
        return
      }
      if (depth > 0) await walk(fullPath, depth - 1)
    }))
  }

  await Promise.all(roots.map((root) => walk(root, 2)))
  return found
}

function displayUserPath(filePath) {
  const home = os.homedir()
  return filePath.startsWith(home) ? `~${filePath.slice(home.length)}` : filePath
}

async function scanFiles() {
  const roots = ['Desktop', 'Documents', 'Downloads'].map((name) => path.join(os.homedir(), name))
  const ignored = new Set(['node_modules', '.git', 'Library', 'Applications'])
  const found = []

  async function walk(dir, depth) {
    let entries = []
    try {
      entries = await fs.readdir(dir, { withFileTypes: true })
    } catch {
      return
    }

    await Promise.all(entries.map(async (entry) => {
      if (entry.name.startsWith('.') || ignored.has(entry.name)) return
      const fullPath = path.join(dir, entry.name)
      if (entry.isFile()) {
        found.push({
          id: fullPath,
          name: entry.name,
          path: fullPath,
          displayPath: displayUserPath(fullPath),
        })
        return
      }
      if (entry.isDirectory() && depth > 0) await walk(fullPath, depth - 1)
    }))
  }

  await Promise.all(roots.map((root) => walk(root, 2)))
  return found.slice(0, 5000)
}

async function scanWindowsApps() {
  const roots = [
    process.env.ProgramData && path.join(process.env.ProgramData, 'Microsoft', 'Windows', 'Start Menu', 'Programs'),
    process.env.APPDATA && path.join(process.env.APPDATA, 'Microsoft', 'Windows', 'Start Menu', 'Programs'),
  ].filter(Boolean)
  const found = []

  async function walk(dir) {
    let entries = []
    try {
      entries = await fs.readdir(dir, { withFileTypes: true })
    } catch {
      return
    }
    await Promise.all(entries.map(async (entry) => {
      const fullPath = path.join(dir, entry.name)
      if (entry.isDirectory()) return walk(fullPath)
      if (entry.name.endsWith('.lnk')) {
        found.push({ id: fullPath, name: entry.name.replace(/\.lnk$/i, ''), path: fullPath })
      }
    }))
  }

  await Promise.all(roots.map(walk))
  return found
}

async function scanLinuxApps() {
  const roots = ['/usr/share/applications', '/usr/local/share/applications', path.join(os.homedir(), '.local/share/applications')]
  const found = []

  await Promise.all(roots.map(async (root) => {
    let entries = []
    try {
      entries = await fs.readdir(root, { withFileTypes: true })
    } catch {
      return
    }

    await Promise.all(entries.map(async (entry) => {
      if (!entry.isFile() || !entry.name.endsWith('.desktop')) return
      const fullPath = path.join(root, entry.name)
      const body = await fs.readFile(fullPath, 'utf8').catch(() => '')
      if (/^(NoDisplay|Hidden)=true$/im.test(body)) return
      const name = body.match(/^Name=(.+)$/m)?.[1]
      const exec = body.match(/^Exec=(.+)$/m)?.[1]
      if (!name || !exec) return
      const command = exec.replace(/\s*%[fFuUdDnNickvm]/g, '').trim()
      found.push({ id: fullPath, name, path: fullPath, command })
    }))
  }))

  return found
}

async function indexApplications() {
  try {
    const apps = process.platform === 'darwin'
      ? await scanMacApps()
      : process.platform === 'win32'
        ? await scanWindowsApps()
        : await scanLinuxApps()

    const deduped = new Map()
    for (const item of apps) deduped.set(normalize(item.name), item)
    appIndex = Array.from(deduped.values()).sort((a, b) => a.name.localeCompare(b.name))
    win?.webContents.send('apps:indexed', appIndex.length)
  } catch (error) {
    console.error('Failed to index applications', error)
  }
}

async function indexFiles() {
  try {
    fileIndex = await scanFiles()
  } catch (error) {
    console.error('Failed to index files', error)
  }
}

async function loadUserState() {
  statePath = path.join(app.getPath('userData'), 'state.json')
  iconCacheDir = path.join(app.getPath('userData'), 'icon-cache')
  extensionsDir = path.join(app.getPath('userData'), 'extensions')

  try {
    const loaded = JSON.parse(await fs.readFile(statePath, 'utf8'))
    userState = {
      recents: loaded.recents || {},
      aliases: loaded.aliases || {},
      shortcuts: loaded.shortcuts || {},
      shortcutActions: loaded.shortcutActions || {},
      overrides: loaded.overrides || {},
      clipboardHistory: loaded.clipboardHistory || [],
      aiChats: loaded.aiChats || {},
    }
  } catch {
    // First run.
  }

  clipboardHistory = normalizeClipboardHistory(userState.clipboardHistory)
}

function normalizeClipboardHistory(items) {
  return (Array.isArray(items) ? items : [])
    .map((item) => {
      if (item?.type === 'image' && item.imageDataUrl) {
        return {
          id: item.id || `image:${hashValue(item.imageDataUrl)}`,
          type: 'image',
          imageDataUrl: item.imageDataUrl,
          thumbnailUrl: item.thumbnailUrl || item.imageDataUrl,
          createdAt: item.createdAt || Date.now(),
        }
      }
      if (item?.text) {
        const text = String(item.text).trim()
        if (!text) return null
        return {
          id: item.id?.startsWith('text:') ? item.id : `text:${hashValue(text)}`,
          type: 'text',
          text,
          createdAt: item.createdAt || Date.now(),
        }
      }
      return null
    })
    .filter(Boolean)
    .slice(0, CLIPBOARD_LIMIT)
}

function clipboardItemTitle(item) {
  if (item.type === 'image') return 'Clipboard image'
  return item.text.length > 72 ? `${item.text.slice(0, 72)}…` : item.text
}

function clipboardItemSubtitle(item) {
  const when = new Date(item.createdAt || Date.now()).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
  return item.type === 'image' ? `Image copied ${when}` : `Copied ${when}`
}

function readClipboardItem() {
  const text = clipboard.readText().trim()
  if (text) {
    return {
      id: `text:${hashValue(text)}`,
      type: 'text',
      text,
      createdAt: Date.now(),
    }
  }

  const image = clipboard.readImage()
  if (image.isEmpty()) return null

  const png = image.toPNG()
  const thumbnail = image.resize({ width: 64, height: 64 })
  return {
    id: `image:${hashValue(png)}`,
    type: 'image',
    imageDataUrl: `data:image/png;base64,${png.toString('base64')}`,
    thumbnailUrl: thumbnail.toDataURL(),
    createdAt: Date.now(),
  }
}

function rememberClipboardItem(item) {
  if (!item) return
  clipboardHistory = [
    item,
    ...clipboardHistory.filter((current) => current.id !== item.id),
  ].slice(0, CLIPBOARD_LIMIT)
  scheduleSaveState()
  win?.webContents.send('clipboard:changed')
}

function scheduleSaveState() {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(saveUserState, 200)
  saveTimer.unref?.()
}

async function saveUserState() {
  userState.clipboardHistory = clipboardHistory
  await fs.mkdir(path.dirname(statePath), { recursive: true })
  await fs.writeFile(statePath, JSON.stringify(userState, null, 2)).catch((error) => {
    console.error('Failed to save state', error)
  })
}

function startClipboardWatcher() {
  let lastId = readClipboardItem()?.id || ''
  setInterval(() => {
    const item = readClipboardItem()
    if (!item || item.id === lastId) return
    lastId = item.id
    rememberClipboardItem(item)
  }, CLIPBOARD_POLL_INTERVAL_MS).unref?.()
}

function normalizeAccelerator(value) {
  return String(value || '')
    .split('+')
    .map((part) => part.trim())
    .filter(Boolean)
    .map((part) => {
      const normalized = part.toLowerCase()
      if (['cmd', 'command', '⌘'].includes(normalized)) return 'Command'
      if (['ctrl', 'control', '^'].includes(normalized)) return 'Control'
      if (['option', 'opt', 'alt', '⌥'].includes(normalized)) return 'Alt'
      if (['shift', '⇧'].includes(normalized)) return 'Shift'
      if (['enter', 'return', '↵'].includes(normalized)) return 'Enter'
      if (['esc', 'escape'].includes(normalized)) return 'Escape'
      if (normalized === 'space') return 'Space'
      return part.length === 1 ? part.toUpperCase() : part
    })
    .join('+')
}

function unregisterShortcutForAction(actionId) {
  const current = userState.shortcuts[actionId]
  if (current) globalShortcut.unregister(current)
}

async function executeShortcutAction(action) {
  const wasVisible = Boolean(win?.isVisible())
  const result = await executeAction(action, { keepPaletteOpen: true })
  if (result?.view) {
    showPalette({ skipShownEvent: true, deferReveal: !wasVisible })
    win?.webContents.send('action:view-open', { ...result, revealWhenReady: !wasVisible })
  }
}

function registerActionShortcut(actionId, accelerator, action) {
  globalShortcut.unregister(accelerator)
  const ok = globalShortcut.register(accelerator, () => executeShortcutAction(action))
  if (!ok) return false
  userState.shortcuts[actionId] = accelerator
  userState.shortcutActions[actionId] = action
  return true
}

function unregisterActionShortcuts() {
  for (const accelerator of Object.values(userState.shortcuts)) globalShortcut.unregister(accelerator)
}

function registerActionShortcuts() {
  unregisterActionShortcuts()
  for (const [actionId, accelerator] of Object.entries(userState.shortcuts)) {
    const action = userState.shortcutActions[actionId]
    if (!action) continue
    const ok = globalShortcut.register(accelerator, () => executeShortcutAction(action))
    if (!ok) console.warn(`Could not register action shortcut ${accelerator} for ${actionId}`)
  }
}

function canCustomizeAction(action) {
  return ['app', 'builtin', 'clipboard-history', 'extension-command'].includes(action?.kind)
}

function getShortcuts() {
  return Object.entries(userState.shortcuts)
    .map(([actionId, accelerator]) => ({
      actionId,
      accelerator,
      action: userState.shortcutActions[actionId],
    }))
    .filter((item) => item.action)
    .sort((a, b) => a.action.title.localeCompare(b.action.title))
}

async function removeShortcut(actionId) {
  if (!actionId || !userState.shortcuts[actionId]) return { ok: false, message: 'Shortcut not found' }
  globalShortcut.unregister(userState.shortcuts[actionId])
  delete userState.shortcuts[actionId]
  delete userState.shortcutActions[actionId]
  scheduleSaveState()
  return { ok: true, message: 'Shortcut removed' }
}

async function setAlias(action, alias) {
  if (!canCustomizeAction(action)) return { ok: false, message: 'Aliases are only available for persistent commands' }
  if (!action?.id || !alias.trim()) return { ok: false, message: 'Missing alias' }
  const aliases = new Set(actionAliases(action.id))
  aliases.add(alias.trim())
  userState.aliases[action.id] = Array.from(aliases)
  scheduleSaveState()
  return { ok: true, message: `Alias set: ${alias.trim()}` }
}

async function setShortcut(action, shortcut) {
  if (!canCustomizeAction(action)) return { ok: false, message: 'Shortcuts are only available for persistent commands' }
  if (!action?.id || !shortcut.trim()) return { ok: false, message: 'Missing shortcut' }
  const accelerator = normalizeAccelerator(shortcut)
  const conflictingActionId = Object.entries(userState.shortcuts)
    .find(([actionId, current]) => actionId !== action.id && current === accelerator)?.[0]
  if (conflictingActionId) {
    const title = userState.shortcutActions[conflictingActionId]?.title || 'another action'
    return { ok: false, message: `${accelerator} is already used by ${title}` }
  }
  unregisterShortcutForAction(action.id)
  const ok = registerActionShortcut(action.id, accelerator, action)
  if (!ok) return { ok: false, message: `Could not register ${accelerator}` }
  scheduleSaveState()
  return { ok: true, message: `Shortcut set: ${accelerator}` }
}

async function setOverride(action, instruction) {
  const defaultActionId = defaultActionIdFor(action)
  if (!defaultActionId) return { ok: false, message: 'This action cannot be overridden yet' }
  if (!instruction.trim()) return { ok: false, message: 'Missing override instructions' }
  userState.overrides[defaultActionId] = {
    instruction: instruction.trim(),
    updatedAt: Date.now(),
    originalTitle: action.title,
  }
  scheduleSaveState()
  return { ok: true, message: 'Override saved' }
}

async function clearOverride(action) {
  const defaultActionId = defaultActionIdFor(action)
  if (!defaultActionId) return { ok: false, message: 'This action has no original to restore' }
  delete userState.overrides[defaultActionId]
  scheduleSaveState()
  return { ok: true, message: 'Original restored' }
}

async function removeCreatedAction(action) {
  if (action?.kind === 'ai-chat' && action.aiChatId) {
    const chat = userState.aiChats[action.aiChatId]
    if (chat?.generatedExtensionFile) {
      await fs.unlink(path.join(extensionsDir, chat.generatedExtensionFile)).catch((error) => {
        if (error?.code !== 'ENOENT') throw error
      })
      await loadExtensions()
    }
    delete userState.aiChats[action.aiChatId]
    delete userState.recents[action.id]
    scheduleSaveState()
    return { ok: true, message: 'AI action removed' }
  }

  if (action?.kind === 'extension-command' && action.removable) {
    const entry = extensionRegistry.get(`${action.extensionId}:${action.commandId}`)
    const filePath = entry?.extension?.__filePath
    if (!filePath) return { ok: false, message: 'This action cannot be removed' }
    const chatId = entry?.extension?.__chatId || action.aiChatId
    await fs.unlink(filePath).catch((error) => {
      if (error?.code !== 'ENOENT') throw error
    })
    if (chatId) delete userState.aiChats[chatId]
    delete userState.recents[action.id]
    scheduleSaveState()
    await loadExtensions()
    return { ok: true, message: 'Generated action removed' }
  }

  return { ok: false, message: 'This action cannot be removed' }
}

app.whenReady().then(async () => {
  nativeTheme.themeSource = 'dark'
  if (process.platform === 'darwin') app.dock.hide()
  registerLocalFileProtocol()

  await loadUserState()
  await loadExtensions()
  initNevermindAi()
  createWindow()
  registerHotkey()
  registerActionShortcuts()
  startClipboardWatcher()
  setTimeout(indexApplications, 100)
  setTimeout(indexFiles, 200)

  ipcMain.handle('actions:search', (_event, query, options) => searchActions(query, options))
  ipcMain.handle('actions:execute', (_event, action) => executeAction(action))
  ipcMain.handle('view-action:execute', (_event, action) => executeViewAction(action))
  ipcMain.on('drag:file', startFileDrag)
  ipcMain.handle('ai:chat:send', (_event, message, chatId) => sendAiChatMessage(message, chatId))
  ipcMain.handle('ai:chat:abort', (_event, chatId) => abortAiChat(chatId))
  ipcMain.handle('ai:chat:reset', (_event, chatId) => resetAiChat(chatId))
  ipcMain.handle('actions:set-alias', (_event, action, alias) => setAlias(action, alias))
  ipcMain.handle('actions:set-shortcut', (_event, action, shortcut) => setShortcut(action, shortcut))
  ipcMain.handle('actions:get-shortcuts', () => getShortcuts())
  ipcMain.handle('actions:remove-shortcut', (_event, actionId) => removeShortcut(actionId))
  ipcMain.handle('actions:suspend-shortcuts', () => unregisterActionShortcuts())
  ipcMain.handle('actions:resume-shortcuts', () => registerActionShortcuts())
  ipcMain.handle('actions:set-override', (_event, action, instruction) => setOverride(action, instruction))
  ipcMain.handle('actions:clear-override', (_event, action) => clearOverride(action))
  ipcMain.handle('actions:remove-created', (_event, action) => removeCreatedAction(action))
  ipcMain.handle('apps:icon', (_event, appPath) => getAppIconDataUrl(appPath))
  ipcMain.handle('palette:set-mode', (_event, mode) => {
    setPaletteSizeForMode(mode)
    centerWindow()
  })
  ipcMain.handle('palette:hide', () => hidePalette())
  ipcMain.handle('palette:shortcut-ready', () => revealPalette())
})

app.on('activate', () => showPalette())
app.on('will-quit', () => {
  app.isQuiting = true
  globalShortcut.unregisterAll()
})

const gotLock = app.requestSingleInstanceLock()
if (!gotLock) app.quit()
else app.on('second-instance', () => showPalette())
