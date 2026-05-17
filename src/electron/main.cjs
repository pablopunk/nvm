// @ts-nocheck
const { app, BrowserWindow, globalShortcut, ipcMain, shell, screen, clipboard, nativeImage, nativeTheme, protocol, net, session, dialog } = require('electron')
const { autoUpdater } = require('electron-updater')
const fs = require('node:fs/promises')
const fsSync = require('node:fs')
const path = require('node:path')
const os = require('node:os')
const crypto = require('node:crypto')
const { spawn, execFile } = require('node:child_process')
const { pathToFileURL } = require('node:url')
const { createNevermindAi } = require('./ai.cjs')

let updateCheckInFlight = false
let updateDownloadInFlight = false
let updateStartupTimer = null
let updatePollTimer = null
const AUTO_UPDATE_STARTUP_DELAY_MS = 15_000
const AUTO_UPDATE_POLL_INTERVAL_MS = 4 * 60 * 60 * 1000

function canUseAutoUpdates() {
  if (!app.isPackaged) return false
  if (process.platform === 'darwin') return true
  if (process.platform === 'linux') return Boolean(process.env.APPIMAGE)
  return false
}

async function checkForUpdates(trigger = 'manual') {
  if (!canUseAutoUpdates() || updateCheckInFlight || updateDownloadInFlight) return
  updateCheckInFlight = true
  try {
    console.info(`[nvm-updater] checking for updates (${trigger})`)
    await autoUpdater.checkForUpdates()
  } catch (error) {
    console.error('[nvm-updater] update check failed', error)
  } finally {
    updateCheckInFlight = false
  }
}

function clearUpdateTimers() {
  if (updateStartupTimer) {
    clearTimeout(updateStartupTimer)
    updateStartupTimer = null
  }
  if (updatePollTimer) {
    clearInterval(updatePollTimer)
    updatePollTimer = null
  }
}

function configureAutoUpdater() {
  if (!canUseAutoUpdates()) {
    console.info('[nvm-updater] disabled (development build or unsupported platform)')
    return
  }

  autoUpdater.autoDownload = false
  autoUpdater.autoInstallOnAppQuit = false

  autoUpdater.on('checking-for-update', () => {
    console.info('[nvm-updater] checking for update')
  })

  autoUpdater.on('update-available', async (info) => {
    if (updateDownloadInFlight) return
    const { response } = await dialog.showMessageBox({
      type: 'info',
      title: 'Update available',
      message: `Nevermind ${info.version} is available.`,
      detail: 'Do you want to download it now?',
      buttons: ['Download', 'Later'],
      defaultId: 0,
      cancelId: 1,
    })
    if (response !== 0) return
    updateDownloadInFlight = true
    try {
      await autoUpdater.downloadUpdate()
    } catch (error) {
      console.error('[nvm-updater] update download failed', error)
    } finally {
      updateDownloadInFlight = false
    }
  })

  autoUpdater.on('update-not-available', () => {
    console.info('[nvm-updater] no updates available')
  })

  autoUpdater.on('download-progress', (progress) => {
    console.info(`[nvm-updater] download progress ${Math.floor(progress.percent)}%`)
  })

  autoUpdater.on('update-downloaded', async (info) => {
    const { response } = await dialog.showMessageBox({
      type: 'info',
      title: 'Update ready',
      message: `Nevermind ${info.version} has been downloaded.`,
      detail: 'Restart Nevermind now to install the update?',
      buttons: ['Restart and Install', 'Later'],
      defaultId: 0,
      cancelId: 1,
    })
    if (response === 0) autoUpdater.quitAndInstall()
  })

  autoUpdater.on('error', (error) => {
    console.error('[nvm-updater] updater error', error)
  })

  updateStartupTimer = setTimeout(() => {
    updateStartupTimer = null
    void checkForUpdates('startup')
  }, AUTO_UPDATE_STARTUP_DELAY_MS)
  updateStartupTimer.unref?.()

  updatePollTimer = setInterval(() => {
    void checkForUpdates('poll')
  }, AUTO_UPDATE_POLL_INTERVAL_MS)
  updatePollTimer.unref?.()
}

const isDev = Boolean(process.env.ELECTRON_RENDERER_URL)
const DEFAULT_PALETTE_HOTKEY = 'Alt+Space'
const CLIPBOARD_LIMIT = 300
const FILE_RESULT_LIMIT = 6
const CLIPBOARD_POLL_INTERVAL_MS = 1000
const APP_REINDEX_DEBOUNCE_MS = 1000
const WINDOW_BLUR_MARGIN = 96
const DEFAULT_PALETTE_SIZE = { width: 600, height: 400 }
const AI_CHAT_PALETTE_SIZE = { width: 760, height: 560 }
const STACKED_PALETTE_SIZE = { width: 760, height: 720 }
const PREVIEW_PALETTE_SIZE = { width: 1080, height: 760 }
const DEFAULT_WINDOW_SIZE = addWindowBlurMargin(DEFAULT_PALETTE_SIZE)
const AI_CHAT_WINDOW_SIZE = addWindowBlurMargin(AI_CHAT_PALETTE_SIZE)
const STACKED_WINDOW_SIZE = addWindowBlurMargin(STACKED_PALETTE_SIZE)
const PREVIEW_WINDOW_SIZE = addWindowBlurMargin(PREVIEW_PALETTE_SIZE)
const IMAGE_EXTENSIONS = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'tiff', 'tif', 'heic'])
const VIDEO_EXTENSIONS = new Set(['mp4', 'mov', 'avi', 'mkv', 'webm', 'wmv', 'flv', 'm4v'])
const LOCAL_FILE_PROTOCOL = 'nvm-file'
const LOCAL_THUMB_PROTOCOL = 'nvm-thumb'
const THUMBNAIL_SIZE = 512

protocol.registerSchemesAsPrivileged([
  { scheme: LOCAL_FILE_PROTOCOL, privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true, bypassCSP: true } },
  { scheme: LOCAL_THUMB_PROTOCOL, privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true, bypassCSP: true } },
])

let win
let ignorePaletteBlurUntil = 0
let appIndex = []
let fileIndex = []
let clipboardHistory = []
let statePath
let iconCacheDir
let clipboardImagesDir
let extensionsDir
let extensionStorageDir
let saveTimer
let appIndexTimer
let appWatchers = []
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
  settings: {},
}

const SETTING_DEFINITIONS = [
  {
    id: 'paletteHotkey',
    title: 'Open Nevermind Shortcut',
    description: 'Global keyboard shortcut that toggles the palette',
    type: 'shortcut',
    default: DEFAULT_PALETTE_HOTKEY,
  },
  {
    id: 'showClipboardInRoot',
    title: 'Show Clipboard Items in Main List',
    description: 'Show copied items inline in the root list',
    type: 'boolean',
    default: true,
  },
]

function getPaletteHotkey() {
  return getSetting('paletteHotkey') || DEFAULT_PALETTE_HOTKEY
}

const SHORTCUT_SYMBOLS = { Command: '⌘', Cmd: '⌘', Control: '⌃', Ctrl: '⌃', Alt: '⌥', Option: '⌥', Shift: '⇧', Enter: '↵', Return: '↵', Escape: 'Esc', Tab: 'Tab' }

function formatShortcut(accelerator) {
  return String(accelerator || '').split('+').map((part) => SHORTCUT_SYMBOLS[part] || part).join('')
}

function isSpotlightAccelerator(accelerator) {
  if (process.platform !== 'darwin') return false
  return normalizeAccelerator(accelerator) === 'Command+Space'
}

function getSetting(id) {
  const definition = SETTING_DEFINITIONS.find((entry) => entry.id === id)
  if (!definition) return undefined
  const stored = userState.settings?.[id]
  return stored === undefined ? definition.default : stored
}

function setSetting(id, value) {
  if (!userState.settings) userState.settings = {}
  userState.settings[id] = value
  scheduleSaveState()
}
const appIconCache = new Map()
const extensionRegistry = new Map()
const extensionActionHandlers = new Map()
const registeredActionAccelerators = new Set()

function addWindowBlurMargin(size) {
  return {
    width: size.width + WINDOW_BLUR_MARGIN * 2,
    height: size.height + WINDOW_BLUR_MARGIN * 2,
  }
}

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
    id: 'app-settings',
    kind: 'app-settings',
    title: 'Settings',
    subtitle: 'Configure Nevermind',
    icon: 'settings',
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

function installPermissionHandlers() {
  session.defaultSession.setPermissionRequestHandler((webContents, permission, callback) => {
    const url = webContents.getURL()
    const isAppPage = url.startsWith('file:') || (isDev && url.startsWith(process.env.ELECTRON_RENDERER_URL || ''))
    if (isAppPage && ['media', 'display-capture', 'clipboard-read', 'clipboard-sanitized-write'].includes(permission)) return callback(true)
    callback(false)
  })
}

function createWindow() {
  win = new BrowserWindow({
    width: DEFAULT_WINDOW_SIZE.width,
    height: DEFAULT_WINDOW_SIZE.height,
    show: false,
    frame: false,
    transparent: true,
    resizable: false,
    movable: true,
    type: process.platform === 'darwin' ? 'panel' : undefined,
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

  win.on('blur', () => {
    if (Date.now() < ignorePaletteBlurUntil) return
    hidePalette()
  })
  win.on('close', (event) => {
    if (!app.isQuiting) {
      event.preventDefault()
      hidePalette()
    }
  })

  win.webContents.on('did-fail-load', (_event, errorCode, errorDescription, validatedURL) => {
    debugLog('renderer.didFailLoad', { errorCode, errorDescription, validatedURL })
  })
  win.webContents.on('render-process-gone', (_event, details) => {
    debugLog('renderer.gone', details)
  })
  win.webContents.on('console-message', (_event, level, message, line, sourceId) => {
    debugLog('renderer.console', { level, message, line, sourceId })
  })
  win.webContents.once('did-finish-load', () => {
    debugLog('renderer.didFinishLoad', { url: win.webContents.getURL() })
    if (isDev || pendingShowOnReady) {
      pendingShowOnReady = false
      showPalette()
    }
  })

  if (isDev) {
    win.loadURL(process.env.ELECTRON_RENDERER_URL)
  } else {
    win.loadFile(path.join(__dirname, '..', '..', 'dist', 'index.html'))
  }
}

function debugLog(message, data) {
  try {
    const line = `${new Date().toISOString()} ${message}${data ? ` ${JSON.stringify(data)}` : ''}\n`
    fs.appendFile(path.join(app.getPath('userData'), 'debug.log'), line).catch(() => {})
  } catch {}
}

function setPaletteSizeForMode(mode = 'default') {
  if (!win) return
  const size = mode === 'preview' ? PREVIEW_WINDOW_SIZE : mode === 'stacked' ? STACKED_WINDOW_SIZE : mode === 'ai-chat' ? AI_CHAT_WINDOW_SIZE : DEFAULT_WINDOW_SIZE
  win.setSize(size.width, size.height, false)
  if (win.isVisible()) centerWindow()
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
  ignorePaletteBlurUntil = Date.now() + 500
  debugLog('showPalette', { options, visible: win.isVisible(), bounds: win.getBounds() })
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
  win.moveTop()
  win.focus()
  win.webContents.focus()
  debugLog('showPalette.after', { visible: win.isVisible(), focused: win.isFocused(), bounds: win.getBounds(), opacity: win.getOpacity() })
  setTimeout(() => debugLog('showPalette.later', { visible: win?.isVisible(), focused: win?.isFocused(), bounds: win?.getBounds(), opacity: win?.getOpacity() }), 300)
}

function revealPalette() {
  win?.setOpacity(1)
}

function hidePalette() {
  if (!win) return
  debugLog('hidePalette', { visible: win.isVisible(), focused: win.isFocused(), bounds: win.getBounds(), opacity: win.getOpacity() })
  win.webContents.send('palette:hidden')
  win.hide()
}

let pendingShowOnReady = false

function showPaletteWhenReady() {
  if (!win) {
    pendingShowOnReady = true
    return
  }
  if (win.webContents.isLoading()) pendingShowOnReady = true
  else showPalette()
}

function togglePalette() {
  if (win?.isVisible()) hidePalette()
  else showPalette()
}

function registerHotkey() {
  const hotkey = getPaletteHotkey()
  const ok = globalShortcut.register(hotkey, togglePalette)
  debugLog('registerHotkey', { accelerator: hotkey, ok, isRegistered: globalShortcut.isRegistered(hotkey) })
  if (ok) console.log(`Registered global shortcut: ${hotkey}`)
  else {
    console.warn(`Could not register global shortcut: ${hotkey}`)
    showPaletteWhenReady()
  }

  win.webContents.on('before-input-event', (_event, input) => {
    if (!(input.meta || input.control) || !input.alt || input.key.toLowerCase() !== 'i') return
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

function scoreNormalized(value, q) {
  if (!q) return 0
  const v = normalize(value)
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

function score(value, query) {
  return scoreNormalized(value, normalize(query))
}

function actionAliases(actionId) {
  const value = userState.aliases[actionId]
  if (!value) return []
  return Array.isArray(value) ? value : [value]
}

function actionSearchScore(action, query) {
  const q = normalize(query)
  if (!q) return action.score || 0
  let best = scoreNormalized(action.title, q)
  if (best === 100) return best
  const subtitleScore = scoreNormalized(action.subtitle, q)
  if (subtitleScore > best) best = subtitleScore
  if (best === 100) return best
  const aliases = action.aliases
  if (aliases) {
    for (const alias of aliases) {
      const s = scoreNormalized(alias, q)
      if (s > best) best = s
      if (best === 100) return best
    }
  }
  for (const alias of actionAliases(action.id)) {
    const s = scoreNormalized(alias, q)
    if (s > best) best = s
    if (best === 100) return best
  }
  return best
}

function usageBoost(actionId) {
  const count = userState.recents[actionId]?.count || 0
  return Math.min(90, count * 6)
}

function recentBoost(actionId) {
  const recent = userState.recents[actionId]
  if (!recent) return 0
  const ageHours = Math.max(0, (Date.now() - recent.lastUsed) / 36e5)
  return Math.max(0, 20 - ageHours)
}

function priorityBoost(action) {
  return action.kind === 'app' ? 25 : 0
}

function defaultActionIdFor(action) {
  if (action.defaultActionId) return action.defaultActionId
  if (action.kind === 'builtin') return action.id
  if (action.kind === 'calculate') return 'default:calculator'
  return null
}

let shortcutByAiChatIdCache = null
function shortcutByAiChatIdMap() {
  if (shortcutByAiChatIdCache) return shortcutByAiChatIdCache
  const map = new Map()
  for (const [actionId, storedAction] of Object.entries(userState.shortcutActions)) {
    if (storedAction?.aiChatId && userState.shortcuts[actionId]) {
      map.set(storedAction.aiChatId, userState.shortcuts[actionId])
    }
  }
  shortcutByAiChatIdCache = map
  return map
}

function invalidateShortcutCaches() {
  shortcutByAiChatIdCache = null
}

function shortcutForAction(action) {
  if (userState.shortcuts[action.id]) return userState.shortcuts[action.id]
  if (!action.aiChatId) return null
  return shortcutByAiChatIdMap().get(action.aiChatId) || null
}

function withShortcutHint(action) {
  const shortcut = shortcutForAction(action)
  return shortcut ? { ...action, shortcut } : action
}

function withDefaultOverride(action) {
  const defaultActionId = defaultActionIdFor(action)
  if (!defaultActionId) return withShortcutHint(action)
  const override = userState.overrides[defaultActionId]
  return {
    ...action,
    defaultActionId,
    isOverridden: Boolean(override),
    overrideSummary: override?.instruction,
    shortcut: shortcutForAction(action),
  }
}

function rankAction(action, query) {
  const base = actionSearchScore(action, query)
  if (query.trim() && base <= 0) return null
  return {
    ...action,
    aliases: [...(action.aliases || []), ...actionAliases(action.id)],
    userAliases: actionAliases(action.id),
    score: base + priorityBoost(action) + usageBoost(action.id) + recentBoost(action.id),
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
  if (result) appIconCache.set(appPath, result)
  else appIconCache.delete(appPath)
  return result
}

function extensionCommandActionId(extension, command) {
  return `extension:${extension.id}:${command.id}`
}

function extensionActionFromCommand(extension, command) {
  const action = {
    id: extensionCommandActionId(extension, command),
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
  const shortcut = shortcutForAction(action) || command.globalShortcut || (command.shortcutScope === 'global' ? command.shortcut : null)
  return shortcut ? { ...action, shortcut } : action
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
    initialPrompt: options.initialPrompt,
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
    imagePath: item.imagePath,
    videoUrl: item.videoUrl,
    filePath: item.filePath,
    thumbnailUrl: item.thumbnailUrl,
    icon: 'clipboard',
    score: 3,
    lastUsed: item.createdAt || 0,
  }
}

function clipboardHistoryView() {
  return {
    type: 'list',
    id: 'clipboard-history',
    title: 'Clipboard History',
    presentation: 'root',
    searchBarPlaceholder: 'Search Clipboard History',
    emptyView: { title: 'No clipboard items found.', subtitle: 'Copy text or images and they will appear here.' },
    items: clipboardHistory.slice(0, CLIPBOARD_LIMIT).map((item) => {
      const isImage = item.type === 'image'
      const isVideo = item.type === 'video'
      const copyAction = isImage
        ? { type: 'copyImage', title: 'Copy Image', imageDataUrl: item.imageDataUrl, imagePath: item.imagePath, dismissAfterRun: 'auto' }
        : { type: 'copyText', title: isVideo ? 'Copy Video Path' : 'Copy Text', text: item.filePath || item.text, dismissAfterRun: 'auto' }
      const previewAction = { type: 'nativeAction', title: 'Preview', nativeAction: clipboardActionFromItem(item) }
      const pasteAction = isImage || isVideo
        ? null
        : { type: 'pasteText', title: 'Paste Text', text: item.text, dismissAfterRun: 'auto' }
      return {
        id: `clipboard:${item.id}`,
        title: clipboardItemTitle(item),
        subtitle: clipboardItemSubtitle(item),
        icon: 'clipboard',
        image: item.thumbnailUrl,
        keywords: [item.text || '', item.type || '', `clipboard ${item.type || ''}`, isImage ? 'image photo picture screenshot' : '', isVideo ? 'video movie recording' : ''].filter(Boolean),
        primaryAction: copyAction,
        actionPanel: {
          sections: [
            { actions: [previewAction, copyAction, pasteAction].filter(Boolean) },
          ],
        },
      }
    }),
  }
}

function settingsView() {
  return {
    type: 'list',
    id: 'app-settings',
    title: 'Settings',
    presentation: 'root',
    searchBarPlaceholder: 'Search Settings',
    items: SETTING_DEFINITIONS.map((definition) => {
      const value = getSetting(definition.id)
      const accessoryText = definition.type === 'boolean' ? (value ? 'On' : 'Off') : definition.type === 'shortcut' ? '' : String(value)
      const primaryAction = definition.type === 'shortcut'
        ? { type: 'nativeAction', title: 'Change Shortcut', shortcut: value, nativeAction: { kind: 'record-palette-hotkey' } }
        : { type: 'nativeAction', title: value ? 'Turn Off' : 'Turn On', nativeAction: { kind: 'toggle-setting', settingId: definition.id } }
      return {
        id: `setting:${definition.id}`,
        title: definition.title,
        subtitle: definition.description,
        icon: 'settings',
        accessories: accessoryText ? [{ text: accessoryText }] : [],
        primaryAction,
        actionPanel: { sections: [{ actions: [primaryAction] }] },
      }
    }),
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
    const ranked = rankAction(withShortcutHint(extensionActionFromCommand(command.extension, command.command)), q)
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
  const clipboardHistoryAction = rankAction(withShortcutHint({
    id: 'clipboard-history',
    kind: 'clipboard-history',
    title: 'Clipboard History',
    subtitle: clipboardHistory.length ? `Show all ${clipboardHistory.length} copied items` : 'Show copied items',
    icon: 'clipboard',
    score: 14,
    lastUsed: latestClipboardTime ? latestClipboardTime - 1 : 0,
  }), q)
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
      score: 30,
    }
    const ranked = rankAction(withShortcutHint(action), q)
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

  if (getSetting('showClipboardInRoot')) {
    for (const item of clipboardHistory) {
      const ranked = rankAction(clipboardActionFromItem(item), q)
      if (ranked) {
        ranked.lastUsed = Math.max(ranked.lastUsed || 0, item.createdAt || 0)
        results.push(ranked)
      }
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
      score: 10 + usageBoost(`web-search:${q}`) + recentBoost(`web-search:${q}`),
    })
  }

  return results
    .sort((a, b) => {
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
      if (action.clipboardType === 'image' && (action.imagePath || action.imageDataUrl)) {
        const image = action.imagePath
          ? nativeImage.createFromPath(action.imagePath)
          : nativeImage.createFromDataURL(action.imageDataUrl)
        clipboard.writeImage(image)
      } else {
        clipboard.writeText(action.text || '')
      }
      break
    case 'clipboard-history':
      return { view: clipboardHistoryView() }
    case 'app-settings':
      return { view: settingsView() }
    case 'open-keyboard-settings':
      await openSystemKeyboardSettings()
      return
    case 'toggle-setting': {
      const definition = SETTING_DEFINITIONS.find((entry) => entry.id === action.settingId)
      if (!definition) return
      const current = getSetting(definition.id)
      const next = definition.type === 'boolean' ? !current : current
      setSetting(definition.id, next)
      return { view: settingsView(), navigation: 'replace' }
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

  if (action.aiChatId) {
    const chatMatches = Array.from(extensionRegistry.values()).filter((entry) => entry.extension.__chatId === action.aiChatId)
    if (chatMatches.length === 1) return chatMatches[0]
  }

  const matches = Array.from(extensionRegistry.values()).filter((entry) => entry.extension.id === action.extensionId)
  return matches.length === 1 ? matches[0] : null
}

function currentActionForStoredShortcut(action) {
  if (action?.kind !== 'extension-command') return action
  const entry = extensionEntryForAction(action)
  return entry ? extensionActionFromCommand(entry.extension, entry.command) : action
}

async function executeExtensionCommand(action) {
  const entry = extensionEntryForAction(action)
  if (!entry) {
    console.warn(`Extension command not found: ${action.extensionId}:${action.commandId}`)
    return null
  }
  try {
    const ctx = createExtensionContext(entry.extension, entry.command)
    const result = await entry.command.run(ctx)
    return normalizeExtensionView(result, entry)
  } catch (error) {
    console.error(`Extension command failed: ${entry.extension.id}:${entry.command.id}`, error)
    return extensionErrorView(entry, error)
  }
}

async function executeActionForIpc(action) {
  try {
    const result = await executeAction(action)
    structuredClone(result)
    return result
  } catch (error) {
    if (action?.kind === 'extension-command') {
      const entry = extensionEntryForAction(action)
      if (entry) return { view: extensionErrorView(entry, error) }
    }
    return { view: { type: 'preview', title: 'Action failed', content: `# Something went wrong\n\n\`\`\`\n${extensionErrorMessage(error)}\n\`\`\`` } }
  }
}

function extensionErrorMessage(error) {
  if (!error) return 'Unknown error'
  if (error.stack) return error.stack
  if (error.message) return error.message
  return String(error)
}

function extensionErrorView(entry, error) {
  const message = extensionErrorMessage(error)
  return normalizeView({
    type: 'preview',
    title: `${entry.command.title || entry.extension.title || 'Extension'} failed`,
    content: `# Something went wrong\n\n\`\`\`\n${message}\n\`\`\``,
    actions: [extensionErrorAiAction(entry, message)].filter(Boolean),
  }, entry)
}

function extensionErrorAiAction(entry, message) {
  const chatId = entry.extension.__chatId
  if (!chatId) return null
  const prompt = `This generated action failed. Please fix the extension.\n\nAction: ${entry.command.title || entry.command.id}\nExtension: ${entry.extension.title || entry.extension.id}\n\nError:\n\`\`\`\n${message}\n\`\`\``
  return {
    type: 'runExtensionAction',
    title: 'Fix with AI',
    __handler: async () => aiChatView(userState.aiChats[chatId], { initialPrompt: prompt }),
  }
}

function normalizeExtensionView(result, entry) {
  if (!result) return null
  const view = result.type ? result : result.view?.type ? result.view : null
  return view ? normalizeView(view, entry) : null
}

function normalizeView(view, entry) {
  const actions = normalizeViewActions(view.actions, entry)
  return {
    ...view,
    actions,
    actionPanel: normalizeActionPanel(view.actionPanel, actions, entry),
    onSelectionChange: normalizeViewAction(view.onSelectionChange, entry),
    submitAction: normalizeViewAction(view.submitAction, entry),
    searchAccessory: view.searchAccessory ? { ...view.searchAccessory, onChange: normalizeViewAction(view.searchAccessory.onChange, entry) } : view.searchAccessory,
    items: normalizeViewItems(view.items, entry),
    sections: Array.isArray(view.sections) ? view.sections.map((section) => ({ ...section, items: normalizeViewItems(section.items, entry) })) : view.sections,
  }
}

function normalizeViewItems(items, entry) {
  return Array.isArray(items) ? items.map((item) => {
    const itemActions = normalizeViewActions(item.actions, entry)
    return {
      ...item,
      actions: itemActions,
      actionPanel: normalizeActionPanel(item.actionPanel, itemActions, entry),
      primaryAction: normalizeViewAction(item.primaryAction, entry),
    }
  }) : items
}

function normalizeActionPanel(panel, fallbackActions, entry) {
  if (panel?.sections) return {
    ...panel,
    sections: panel.sections.map((section) => ({
      ...section,
      actions: normalizeViewActions([...(section.actions || []), ...(section.lazyActions || [])], entry),
    })),
  }
  if (Array.isArray(fallbackActions) && fallbackActions.length) return { sections: [{ actions: normalizeViewActions(fallbackActions, entry) }] }
  return panel
}

function normalizeViewActions(actions, entry) {
  return Array.isArray(actions) ? actions.map((action) => normalizeViewAction(action, entry)).filter(Boolean) : []
}

function normalizeViewAction(action, entry) {
  if (!action) return null
  const handler = typeof action.__handler === 'function' ? action.__handler : typeof action.run === 'function' ? action.run : null
  if (handler) {
    const handlerId = crypto.randomUUID()
    extensionActionHandlers.set(handlerId, { entry, handler })
    const { __handler, run, ...rest } = action
    return normalizeViewAction({ ...rest, type: 'runExtensionAction', handlerId }, entry)
  }
  const normalized = action.submenu ? { ...action, submenu: normalizeActionPanel(action.submenu, [], entry) } : action
  if ((normalized.type === 'pushView' || normalized.type === 'replaceView') && normalized.view) {
    return { ...normalized, view: normalizeView(normalized.view, entry) }
  }
  return normalized
}

function runShellCommand(command, args = [], options = {}) {
  return new Promise((resolve) => {
    const child = spawn(String(command), Array.isArray(args) ? args.map(String) : [], {
      cwd: options.cwd ? expandUserPath(options.cwd) : undefined,
      env: { ...process.env, ...(options.env || {}) },
      shell: Boolean(options.shell),
      timeout: Number(options.timeout || 30_000),
    })
    let stdout = ''
    let stderr = ''
    child.stdout?.on('data', (chunk) => { stdout = limitedOutput(stdout + chunk.toString(), options.outputLimit) })
    child.stderr?.on('data', (chunk) => { stderr = limitedOutput(stderr + chunk.toString(), options.outputLimit) })
    child.on('error', (error) => resolve({ stdout, stderr: stderr || error.message, exitCode: 1 }))
    child.on('close', (exitCode) => resolve({ stdout, stderr, exitCode }))
  })
}

function runShellScript(script, options = {}) {
  return runShellCommand(options.shell || '/bin/bash', ['-lc', String(script)], { ...options, shell: false })
}

function isViewAction(value) {
  return Boolean(value?.type && ['nativeAction', 'openPath', 'revealPath', 'quickLook', 'openWith', 'openUrl', 'copyText', 'pasteText', 'copyImage', 'trash', 'pushView', 'replaceView', 'popView', 'runExtensionAction', 'shellExec', 'shellScript'].includes(value.type))
}

async function executeViewActionResult(result, entry) {
  if (!result) return result
  if (isViewAction(result)) return executeViewAction(normalizeViewAction(result, entry))
  if (isViewAction(result.action)) return executeViewAction(normalizeViewAction(result.action, entry))
  const view = normalizeExtensionView(result, entry)
  return view ? { view, navigation: result?.navigation || 'push', toast: result?.toast } : result
}

async function executeViewActionForIpc(action) {
  try {
    const result = await executeViewAction(action)
    structuredClone(result)
    return result
  } catch (error) {
    const record = action?.type === 'runExtensionAction' ? extensionActionHandlers.get(action.handlerId) : null
    if (record) return { view: extensionErrorView(record.entry, error), navigation: 'push' }
    return { view: { type: 'preview', title: 'Action failed', content: `# Something went wrong\n\n\`\`\`\n${extensionErrorMessage(error)}\n\`\`\`` }, navigation: 'push' }
  }
}

function shellResultView(title, result) {
  return {
    type: 'preview',
    title,
    content: `Exit code: ${result.exitCode ?? 0}\n\nSTDOUT\n${result.stdout || '—'}\n\nSTDERR\n${result.stderr || '—'}`,
  }
}

async function executeViewAction(action) {
  switch (action?.type) {
    case 'nativeAction':
      return executeAction(action.nativeAction, { keepPaletteOpen: true })
    case 'openPath':
      await shell.openPath(action.path)
      break
    case 'revealPath':
      shell.showItemInFolder(action.path)
      break
    case 'quickLook':
      return quickLookPath(action.path)
    case 'openWith':
      return openPathWithApp(action.path, action.appPath || action.app?.path)
    case 'openUrl':
      await shell.openExternal(action.url)
      break
    case 'copyText':
      clipboard.writeText(action.text || '')
      break
    case 'pasteText':
      clipboard.writeText(action.text || '')
      if (process.platform === 'darwin') execFile('osascript', ['-e', 'tell application "System Events" to keystroke "v" using command down'], () => {})
      return { toast: { message: 'Pasted' } }
    case 'copyImage':
      if (action.path) clipboard.writeImage(nativeImage.createFromPath(expandUserPath(action.path)))
      else if (action.imagePath) clipboard.writeImage(nativeImage.createFromPath(action.imagePath))
      else clipboard.writeImage(nativeImage.createFromDataURL(action.imageDataUrl))
      break
    case 'trash':
      for (const itemPath of action.paths || [action.path]) {
        if (itemPath) await shell.trashItem(expandUserPath(itemPath))
      }
      return { toast: { message: 'Moved to Trash' } }
    case 'pushView':
      return { view: action.view, navigation: 'push' }
    case 'replaceView':
      return { view: action.view, navigation: 'replace' }
    case 'popView':
      return { navigation: 'pop' }
    case 'shellExec': {
      const result = await runShellCommand(action.command, action.args || [], action.options || {})
      return { view: shellResultView(action.title || action.command || 'Command', result), navigation: 'push' }
    }
    case 'shellScript': {
      const result = await runShellScript(action.script, action.options || {})
      return { view: shellResultView(action.title || 'Script', result), navigation: 'push' }
    }
    case 'runExtensionAction': {
      const record = extensionActionHandlers.get(action.handlerId)
      if (!record) return { toast: { message: 'Action is no longer available', tone: 'error' } }
      try {
        const result = await record.handler(createExtensionContext(record.entry.extension, record.entry.command), action)
        return executeViewActionResult(result, record.entry)
      } catch (error) {
        console.error(`Extension action failed: ${record.entry.extension.id}:${record.entry.command.id}`, error)
        return { view: extensionErrorView(record.entry, error), navigation: 'push' }
      }
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

function extensionForPath(filePath) {
  return path.extname(filePath).toLowerCase().replace(/^\./, '')
}

function isImagePath(filePath) {
  return IMAGE_EXTENSIONS.has(extensionForPath(filePath))
}

function isVideoPath(filePath) {
  return VIDEO_EXTENSIONS.has(extensionForPath(filePath))
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

async function fileToExtensionFile(filePath) {
  const stat = await fs.stat(filePath).catch(() => null)
  return {
    path: filePath,
    name: path.basename(filePath),
    displayPath: displayUserPath(filePath),
    url: isImagePath(filePath) || isVideoPath(filePath) ? thumbnailUrlForPath(filePath) : fileUrlForPath(filePath),
    fileUrl: fileUrlForPath(filePath),
    videoUrl: isVideoPath(filePath) ? fileUrlForPath(filePath) : null,
    thumbnailUrl: isImagePath(filePath) || isVideoPath(filePath) ? thumbnailUrlForPath(filePath) : null,
    kind: isImagePath(filePath) ? 'image' : isVideoPath(filePath) ? 'video' : 'file',
    extension: extensionForPath(filePath),
    mtime: stat ? new Date(stat.mtimeMs).toISOString() : null,
    mtimeMs: stat?.mtimeMs || 0,
    birthtime: stat ? new Date(stat.birthtimeMs).toISOString() : null,
    birthtimeMs: stat?.birthtimeMs || 0,
    size: stat?.size || 0,
  }
}

function normalizeFindRoots(roots) {
  if (Array.isArray(roots)) return roots
  if (typeof roots === 'string') return [roots]
  return []
}

function extensionsForFindOptions(options = {}) {
  const kinds = Array.isArray(options.kind) ? options.kind : options.kind ? [options.kind] : []
  const kindExtensions = kinds.flatMap((kind) => {
    if (kind === 'image') return Array.from(IMAGE_EXTENSIONS)
    if (kind === 'video') return Array.from(VIDEO_EXTENSIONS)
    if (kind === 'media') return [...IMAGE_EXTENSIONS, ...VIDEO_EXTENSIONS]
    return []
  })
  const patterns = Array.isArray(options.patterns) ? options.patterns : options.pattern ? [options.pattern] : []
  const patternExtensions = patterns.map((pattern) => String(pattern).match(/\.([a-z0-9]+)$/i)?.[1]).filter(Boolean)
  const requestedExtensions = [...(options.extensions || []), ...kindExtensions, ...patternExtensions]
  return requestedExtensions.length ? new Set(requestedExtensions.map((ext) => String(ext).toLowerCase().replace(/^\./, ''))) : null
}

function sortFoundFiles(files, options = {}) {
  const sortBy = options.sortBy || options.sort || null
  if (!sortBy) return files
  const direction = options.order === 'asc' ? 1 : -1
  const field = sortBy === 'recent' || sortBy === 'modified' ? 'mtimeMs'
    : sortBy === 'added' || sortBy === 'created' ? 'birthtimeMs'
      : sortBy === 'name' ? 'name'
        : sortBy === 'size' ? 'size'
          : sortBy
  return [...files].sort((a, b) => {
    const av = a[field] || 0
    const bv = b[field] || 0
    if (typeof av === 'string' || typeof bv === 'string') return direction * String(av).localeCompare(String(bv))
    return direction * (av - bv)
  })
}

async function findFiles(roots, options = {}) {
  const limit = options.limit || 100
  const maxDepth = options.depth ?? 2
  const extensions = extensionsForFindOptions(options)
  const found = []

  async function walk(dir, depth) {
    let entries = []
    try {
      entries = await fs.readdir(dir, { withFileTypes: true })
    } catch {
      return
    }

    for (const entry of entries) {
      if (entry.name.startsWith('.')) continue
      const fullPath = path.join(dir, entry.name)
      if (entry.isFile()) {
        const ext = extensionForPath(entry.name)
        if (!extensions || extensions.has(ext)) found.push(await fileToExtensionFile(fullPath))
        continue
      }
      if (entry.isDirectory() && depth > 0) await walk(fullPath, depth - 1)
    }
  }

  await Promise.all(normalizeFindRoots(roots).map((root) => walk(expandUserPath(root), maxDepth)))
  return sortFoundFiles(found, options).slice(0, limit)
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

function quickLookPath(filePath) {
  if (process.platform !== 'darwin') return { toast: { message: 'Quick Look is only available on macOS', tone: 'error' } }
  const resolvedPath = expandUserPath(filePath)
  if (!resolvedPath || !path.isAbsolute(resolvedPath)) return { toast: { message: 'Cannot Quick Look this item', tone: 'error' } }
  const child = spawn('qlmanage', ['-p', resolvedPath], { detached: true, stdio: 'ignore' })
  child.unref()
}

function execFileText(command, args) {
  return new Promise((resolve, reject) => {
    execFile(command, args, (error, stdout) => error ? reject(error) : resolve(stdout))
  })
}

async function contentTypesForPath(filePath) {
  if (process.platform !== 'darwin') return []
  try {
    const stdout = await execFileText('mdls', ['-raw', '-name', 'kMDItemContentTypeTree', filePath])
    return stdout.match(/"([^"]+)"/g)?.map((item) => item.slice(1, -1)) || []
  } catch {
    return []
  }
}

async function documentTypesForApp(appPath) {
  try {
    const stdout = await execFileText('/usr/bin/plutil', ['-convert', 'json', '-o', '-', path.join(appPath, 'Contents', 'Info.plist')])
    return JSON.parse(stdout).CFBundleDocumentTypes || []
  } catch {
    return []
  }
}

async function openWithApps(filePath) {
  const resolvedPath = expandUserPath(filePath)
  if (!resolvedPath || !path.isAbsolute(resolvedPath)) return []
  if (process.platform !== 'darwin') return appIndex
  const extension = path.extname(resolvedPath).replace(/^\./, '').toLowerCase()
  const contentTypes = new Set(await contentTypesForPath(resolvedPath))
  const scored = []
  await Promise.all(appIndex.map(async (item) => {
    if (!item.path?.endsWith('.app')) return
    const documentTypes = await documentTypesForApp(item.path)
    let score = 0
    for (const type of documentTypes) {
      const extensions = (type.CFBundleTypeExtensions || []).map((value) => String(value).toLowerCase())
      const itemTypes = type.LSItemContentTypes || []
      if (extension && extensions.includes(extension)) score = Math.max(score, 3)
      if (itemTypes.some((itemType) => contentTypes.has(itemType))) score = Math.max(score, 2)
      if (extensions.includes('*') || itemTypes.includes('public.data') || itemTypes.includes('public.item')) score = Math.max(score, 1)
    }
    if (score) scored.push({ ...item, score })
  }))
  return scored.sort((a, b) => b.score - a.score || a.name.localeCompare(b.name)).map(({ score, ...item }) => item)
}

async function openPathWithApp(filePath, appPath) {
  const resolvedPath = expandUserPath(filePath)
  const resolvedAppPath = expandUserPath(appPath)
  if (!resolvedPath || !resolvedAppPath || !path.isAbsolute(resolvedPath) || !path.isAbsolute(resolvedAppPath)) return { toast: { message: 'Cannot open this file with that app', tone: 'error' } }
  if (process.platform === 'darwin') spawn('open', ['-a', resolvedAppPath, resolvedPath], { detached: true, stdio: 'ignore' }).unref()
  else await shell.openPath(resolvedPath)
}

async function selectedInFinder() {
  if (process.platform !== 'darwin') return []
  const script = 'tell application "Finder" to get POSIX path of (selection as alias list)'
  return new Promise((resolve) => {
    execFile('osascript', ['-e', script], (error, stdout) => {
      if (error) return resolve([])
      Promise.all(stdout.split(', ').map((item) => item.trim()).filter(Boolean).map(fileToExtensionFile)).then(resolve)
    })
  })
}

function extensionStoragePath(extension) {
  const key = extension.__chatId || extension.id || 'extension'
  const safeKey = String(key).replace(/[^a-zA-Z0-9._-]/g, '-')
  return path.join(extensionStorageDir, `${safeKey}.json`)
}

async function readExtensionStorage(extension) {
  try {
    return JSON.parse(await fs.readFile(extensionStoragePath(extension), 'utf8'))
  } catch {
    return {}
  }
}

async function writeExtensionStorage(extension, data) {
  await fs.mkdir(extensionStorageDir, { recursive: true })
  await fs.writeFile(extensionStoragePath(extension), JSON.stringify(data, null, 2))
}

function limitedOutput(value, limit = 200_000) {
  const text = String(value || '')
  return text.length > limit ? `${text.slice(0, limit)}\n… output truncated …` : text
}

function createExtensionStorage(extension) {
  return {
    async get(key, fallback = null) {
      const data = await readExtensionStorage(extension)
      return Object.prototype.hasOwnProperty.call(data, key) ? data[key] : fallback
    },
    async set(key, value) {
      const data = await readExtensionStorage(extension)
      data[key] = value
      await writeExtensionStorage(extension, data)
      return value
    },
    async delete(key) {
      const data = await readExtensionStorage(extension)
      delete data[key]
      await writeExtensionStorage(extension, data)
    },
    async clear() {
      await writeExtensionStorage(extension, {})
    },
    async memo(key, ttlMs, loader) {
      const data = await readExtensionStorage(extension)
      const cached = data[key]
      if (cached && typeof cached === 'object' && Date.now() - Number(cached.updatedAt || 0) < Number(ttlMs || 0)) return cached.value
      const value = await loader()
      data[key] = { value, updatedAt: Date.now() }
      await writeExtensionStorage(extension, data)
      return value
    },
  }
}

function createExtensionContext(extension, command) {
  return {
    extension: createExtensionRuntimeMetadata(extension, command),
    command,
    ui: {
      list: (view) => ({ ...view, type: 'list' }),
      grid: (view) => ({ ...view, type: 'grid' }),
      preview: (fileOrView, view = {}) => {
        const isFile = fileOrView?.path || fileOrView?.fileUrl || fileOrView?.videoUrl || fileOrView?.thumbnailUrl
        if (!isFile) return { ...fileOrView, type: 'preview' }
        const file = fileOrView
        return {
          ...view,
          type: 'preview',
          presentation: view.presentation || 'preview',
          title: view.title || file.name || 'Preview',
          subtitle: view.subtitle || file.displayPath,
          content: view.content || file.displayPath || '',
          image: file.thumbnailUrl || file.url,
          video: file.videoUrl || undefined,
        }
      },
      chat: (view) => ({ ...view, type: 'chat' }),
      form: (view) => ({ ...view, type: 'form' }),
      progress: (view) => ({ ...view, type: 'progress' }),
      webview: (view) => ({ ...view, type: 'webview' }),
      item: (item) => item,
      actions: (actions) => actions,
      empty: (title = 'Nothing here', subtitle = '') => ({ type: 'preview', title, content: `# ${title}${subtitle ? `\n\n${subtitle}` : ''}` }),
      loading: (title = 'Loading…') => ({ type: 'progress', title, steps: [{ title, status: 'active' }] }),
      error: (title = 'Something went wrong', message = '') => ({ type: 'preview', title, content: `# ${title}${message ? `\n\n${message}` : ''}` }),
    },
    actions: {
      openPath: (filePath, title = 'Open', options = {}) => ({ ...options, type: 'openPath', title, path: filePath }),
      revealPath: (filePath, title = 'Reveal in Finder', options = {}) => ({ ...options, type: 'revealPath', title, path: filePath }),
      quickLook: (filePath, title = 'Quick Look', options = {}) => ({ ...options, type: 'quickLook', title, path: filePath }),
      openWith: (filePath, app, title, options = {}) => ({ ...options, type: 'openWith', title: title || `Open with ${app?.name || 'App'}`, path: filePath, app, appPath: app?.path || app }),
      openUrl: (url, title = 'Open URL', options = {}) => ({ ...options, type: 'openUrl', title, url }),
      copyText: (text, title = 'Copy', options = {}) => ({ ...options, type: 'copyText', title, text }),
      pasteText: (text, title = 'Paste', options = {}) => ({ ...options, type: 'pasteText', title, text }),
      copyImage: (image, title = 'Copy image', options = {}) => String(image || '').startsWith('data:') ? ({ ...options, type: 'copyImage', title, imageDataUrl: image }) : ({ ...options, type: 'copyImage', title, path: image }),
      trash: (paths, title = 'Move to Trash', options = {}) => ({ ...options, type: 'trash', title, paths: Array.isArray(paths) ? paths : [paths], style: options.style || 'destructive', requiresConfirmation: options.requiresConfirmation ?? true }),
      push: (title, view, options = {}) => ({ ...options, type: 'pushView', title, view }),
      replace: (title, view, options = {}) => ({ ...options, type: 'replaceView', title, view }),
      pop: (title = 'Back', options = {}) => ({ ...options, type: 'popView', title }),
      run: (title, handler, options = {}) => ({ ...options, type: 'runExtensionAction', title, __handler: handler }),
      shellExec: (title, command, args = [], options = {}) => ({ ...options, type: 'shellExec', title, command, args, options, requiresConfirmation: options.requiresConfirmation ?? true }),
      shellScript: (title, script, options = {}) => ({ ...options, type: 'shellScript', title, script, options, requiresConfirmation: options.requiresConfirmation ?? true }),
    },
    navigation: {
      push: (view) => ({ view, navigation: 'push' }),
      replace: (view) => ({ view, navigation: 'replace' }),
      pop: () => ({ navigation: 'pop' }),
      run: (action) => ({ action }),
    },
    clipboard: {
      readText: () => clipboard.readText(),
      writeText: (text) => clipboard.writeText(String(text || '')),
      readImage: () => clipboard.readImage().toDataURL(),
      writeImage: (imageDataUrl) => clipboard.writeImage(nativeImage.createFromDataURL(imageDataUrl)),
    },
    files: {
      find: findFiles,
      findImages: (roots, options) => findFiles(roots, { ...options, kind: 'image' }),
      findVideos: (roots, options) => findFiles(roots, { ...options, kind: 'video' }),
      findMedia: (roots, options) => findFiles(roots, { ...options, kind: 'media' }),
      selectedInFinder,
      openWithApps,
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
      exec: (command, args = [], options = {}) => new Promise((resolve) => {
        const child = spawn(String(command), Array.isArray(args) ? args.map(String) : [], {
          cwd: options.cwd ? expandUserPath(options.cwd) : undefined,
          env: { ...process.env, ...(options.env || {}) },
          shell: Boolean(options.shell),
          timeout: Number(options.timeout || 30_000),
        })
        let stdout = ''
        let stderr = ''
        child.stdout?.on('data', (chunk) => { stdout = limitedOutput(stdout + chunk.toString(), options.outputLimit) })
        child.stderr?.on('data', (chunk) => { stderr = limitedOutput(stderr + chunk.toString(), options.outputLimit) })
        child.on('error', (error) => resolve({ stdout, stderr: stderr || error.message, exitCode: 1 }))
        child.on('close', (exitCode) => resolve({ stdout, stderr, exitCode }))
      }),
      script: (script, options = {}) => new Promise((resolve) => {
        const child = spawn(options.shell || '/bin/bash', ['-lc', String(script)], {
          cwd: options.cwd ? expandUserPath(options.cwd) : undefined,
          env: { ...process.env, ...(options.env || {}) },
          timeout: Number(options.timeout || 30_000),
        })
        let stdout = ''
        let stderr = ''
        child.stdout?.on('data', (chunk) => { stdout = limitedOutput(stdout + chunk.toString(), options.outputLimit) })
        child.stderr?.on('data', (chunk) => { stderr = limitedOutput(stderr + chunk.toString(), options.outputLimit) })
        child.on('error', (error) => resolve({ stdout, stderr: stderr || error.message, exitCode: 1 }))
        child.on('close', (exitCode) => resolve({ stdout, stderr, exitCode }))
      }),
      appleScript: (script, options = {}) => new Promise((resolve) => {
        if (process.platform !== 'darwin') return resolve({ stdout: '', stderr: 'AppleScript is only available on macOS', exitCode: 1 })
        execFile('osascript', ['-e', String(script)], { timeout: Number(options.timeout || 30_000) }, (error, stdout, stderr) => resolve({ stdout: limitedOutput(stdout, options.outputLimit), stderr: limitedOutput(stderr || error?.message || '', options.outputLimit), exitCode: error ? 1 : 0 }))
      }),
      which: (command) => new Promise((resolve) => {
        execFile('/usr/bin/which', [String(command)], (error, stdout, stderr) => resolve({ stdout: stdout.trim(), stderr: stderr || error?.message || '', exitCode: error ? 1 : 0 }))
      }),
    },
    storage: createExtensionStorage(extension),
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
    addAliasForChat: (chatId) => addAliasForGeneratedAction(chatId),
    onEvent: (event) => {
      const chatId = event.chatId || activeAiChatId
      if (chatId && event.type === 'delta' && event.text) appendAiChatDelta(chatId, event.text)
      if (chatId && event.type === 'tool_start' && event.name) appendAiChatMessage(chatId, 'system', event.name)
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

function addAliasForGeneratedAction(chatId) {
  const chat = userState.aiChats[chatId]
  if (!chat?.query) return
  const entry = Array.from(extensionRegistry.values()).find((e) => e.extension?.__chatId === chatId)
  if (!entry) return
  const action = extensionActionFromCommand(entry.extension, entry.command)
  if (!action?.id) return
  const aliases = new Set(actionAliases(action.id))
  aliases.add(chat.query.trim())
  userState.aliases[action.id] = Array.from(aliases)
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
      await applyExtensionMetadataOverrides(extension)
      registerExtension(extension)
    } catch (error) {
      console.error(`Failed to load extension ${fullPath}`, error)
    }
  }
}

function createExtensionRuntimeMetadata(extension, command) {
  return {
    ...extension,
    rename: (metadata) => renameExtension(extension, command, metadata),
  }
}

function normalizedExtensionMetadata(metadata) {
  const value = typeof metadata === 'string' ? { title: metadata, commandTitle: metadata } : metadata || {}
  return {
    title: value.title == null ? undefined : String(value.title).trim(),
    subtitle: value.subtitle == null ? undefined : String(value.subtitle).trim(),
    commandTitle: value.commandTitle == null ? undefined : String(value.commandTitle).trim(),
    commandSubtitle: value.commandSubtitle == null ? undefined : String(value.commandSubtitle).trim(),
  }
}

async function applyExtensionMetadataOverrides(extension) {
  const metadata = (await readExtensionStorage(extension)).__metadata
  if (!metadata || typeof metadata !== 'object') return
  applyExtensionMetadata(extension, metadata)
}

function applyExtensionMetadata(extension, metadata) {
  const normalized = normalizedExtensionMetadata(metadata)
  if (normalized.title) extension.title = normalized.title
  if (normalized.subtitle) extension.subtitle = normalized.subtitle
  if (!Array.isArray(extension.commands)) return
  for (const command of extension.commands) {
    if (normalized.commandTitle) command.title = normalized.commandTitle
    if (normalized.commandSubtitle) command.subtitle = normalized.commandSubtitle
  }
}

async function renameExtension(extension, command, metadata) {
  const normalized = normalizedExtensionMetadata(metadata)
  if (!normalized.title && !normalized.subtitle && !normalized.commandTitle && !normalized.commandSubtitle) throw new Error('rename requires a title, subtitle, commandTitle, or commandSubtitle')
  const data = await readExtensionStorage(extension)
  data.__metadata = { ...(data.__metadata || {}), ...normalized }
  await writeExtensionStorage(extension, data)
  applyExtensionMetadata(extension, normalized)
  if (command && normalized.commandTitle) command.title = normalized.commandTitle
  if (command && normalized.commandSubtitle) command.subtitle = normalized.commandSubtitle
  await loadExtensions()
  registerActionShortcuts()
  return { ok: true, title: extension.title, commandTitle: command?.title }
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

function appScanRoots() {
  if (process.platform === 'darwin') return ['/Applications', '/System/Applications', path.join(os.homedir(), 'Applications')]
  if (process.platform === 'win32') {
    return [
      process.env.ProgramData && path.join(process.env.ProgramData, 'Microsoft', 'Windows', 'Start Menu', 'Programs'),
      process.env.APPDATA && path.join(process.env.APPDATA, 'Microsoft', 'Windows', 'Start Menu', 'Programs'),
    ].filter(Boolean)
  }
  return ['/usr/share/applications', '/usr/local/share/applications', path.join(os.homedir(), '.local/share/applications')]
}

async function scanMacApps() {
  const roots = appScanRoots()
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
  const roots = appScanRoots()
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
  const roots = appScanRoots()
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

function scheduleIndexApplications() {
  if (appIndexTimer) clearTimeout(appIndexTimer)
  appIndexTimer = setTimeout(() => {
    appIndexTimer = null
    void indexApplications()
  }, APP_REINDEX_DEBOUNCE_MS)
  appIndexTimer.unref?.()
}

async function startAppWatcher() {
  for (const watcher of appWatchers) watcher.close()
  appWatchers = []
  for (const root of appScanRoots()) {
    if (!fsSync.existsSync(root)) continue
    try {
      const watcher = fsSync.watch(root, { recursive: process.platform === 'darwin' || process.platform === 'win32' }, scheduleIndexApplications)
      watcher.on('error', () => {})
      appWatchers.push(watcher)
    } catch {}
  }
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
  clipboardImagesDir = path.join(app.getPath('userData'), 'clipboard-images')
  extensionsDir = path.join(app.getPath('userData'), 'extensions')
  extensionStorageDir = path.join(app.getPath('userData'), 'extension-storage')

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
      settings: loaded.settings || {},
    }
  } catch {
    // First run.
  }

  clipboardHistory = normalizeClipboardHistory(userState.clipboardHistory)
}

function normalizeClipboardHistory(items) {
  return (Array.isArray(items) ? items : [])
    .map((item) => {
      if (item?.type === 'image' && (item.imagePath || item.imageDataUrl)) {
        let imagePath = item.imagePath
        if (!imagePath && typeof item.imageDataUrl === 'string' && item.imageDataUrl.startsWith('data:')) {
          const base64 = item.imageDataUrl.split(',', 2)[1] || ''
          try {
            const png = Buffer.from(base64, 'base64')
            imagePath = persistClipboardImage(png, hashValue(png))
          } catch {}
        }
        const id = item.id || (imagePath ? `image:${path.basename(imagePath, '.png')}` : `image:${hashValue(item.imageDataUrl)}`)
        return {
          id,
          type: 'image',
          imagePath,
          imageDataUrl: imagePath ? fileUrlForPath(imagePath) : item.imageDataUrl,
          thumbnailUrl: item.thumbnailUrl || (imagePath ? fileUrlForPath(imagePath) : item.imageDataUrl),
          createdAt: item.createdAt || Date.now(),
        }
      }
      if (item?.type === 'video' && item.filePath) {
        const filePath = expandUserPath(item.filePath)
        if (!isVideoPath(filePath)) return null
        return {
          id: item.id || `video:${hashValue(filePath)}`,
          type: 'video',
          filePath,
          videoUrl: item.videoUrl || fileUrlForPath(filePath),
          thumbnailUrl: item.thumbnailUrl || thumbnailUrlForPath(filePath),
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
  if (item.type === 'video') return path.basename(item.filePath || 'Clipboard video')
  return item.text.length > 72 ? `${item.text.slice(0, 72)}…` : item.text
}

function clipboardItemSubtitle(item) {
  const when = new Date(item.createdAt || Date.now()).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
  if (item.type === 'image') return `Image copied ${when}`
  if (item.type === 'video') return `Video copied ${when}`
  return `Copied ${when}`
}

function clipboardFilePath() {
  const candidates = [clipboard.readBuffer('public.file-url').toString('utf8'), clipboard.readText()]
  for (const candidate of candidates) {
    const value = String(candidate || '').replace(/\0/g, '').trim().split(/\r?\n/)[0]
    if (!value) continue
    if (value.startsWith('file://')) return decodeURIComponent(new URL(value).pathname)
    if (path.isAbsolute(value)) return value
  }
  return null
}

function persistClipboardImage(png, hash) {
  const imagePath = path.join(clipboardImagesDir, `${hash}.png`)
  fs.mkdir(clipboardImagesDir, { recursive: true })
    .then(() => fs.writeFile(imagePath, png))
    .catch((error) => console.warn('Failed to persist clipboard image', error))
  return imagePath
}

function readClipboardItem() {
  const filePath = clipboardFilePath()
  if (filePath && isVideoPath(filePath)) {
    return {
      id: `video:${hashValue(filePath)}`,
      type: 'video',
      filePath,
      videoUrl: fileUrlForPath(filePath),
      thumbnailUrl: thumbnailUrlForPath(filePath),
      createdAt: Date.now(),
    }
  }

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
  const hash = hashValue(png)
  const imagePath = persistClipboardImage(png, hash)
  const thumbnail = image.resize({ width: 64, height: 64 })
  return {
    id: `image:${hash}`,
    type: 'image',
    imagePath,
    imageDataUrl: fileUrlForPath(imagePath),
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
  const currentAction = currentActionForStoredShortcut(action)
  const wasVisible = Boolean(win?.isVisible())
  const result = await executeAction(currentAction, { keepPaletteOpen: true })
  if (result?.view) {
    showPalette({ skipShownEvent: true, deferReveal: !wasVisible })
    win?.webContents.send('action:view-open', { ...result, revealWhenReady: !wasVisible, asSibling: wasVisible })
  }
}

function bindGlobalActionShortcut(actionId, accelerator, action) {
  if (accelerator === getPaletteHotkey()) return false
  globalShortcut.unregister(accelerator)
  registeredActionAccelerators.add(accelerator)
  const ok = globalShortcut.register(accelerator, () => executeShortcutAction(action))
  debugLog('registerActionShortcut', { actionId, accelerator, title: action?.title, ok, isRegistered: globalShortcut.isRegistered(accelerator) })
  return ok
}

function registerActionShortcut(actionId, accelerator, action) {
  const ok = bindGlobalActionShortcut(actionId, accelerator, action)
  if (!ok) return false
  userState.shortcuts[actionId] = accelerator
  userState.shortcutActions[actionId] = action
  invalidateShortcutCaches()
  return true
}

function declaredGlobalShortcuts() {
  return Array.from(extensionRegistry.values()).map(({ extension, command }) => {
    const accelerator = command.globalShortcut || (command.shortcutScope === 'global' ? command.shortcut : null)
    if (!accelerator) return null
    const action = extensionActionFromCommand(extension, command)
    return { actionId: action.id, accelerator: normalizeAccelerator(accelerator), action }
  }).filter(Boolean)
}

function unregisterActionShortcuts() {
  for (const accelerator of registeredActionAccelerators) globalShortcut.unregister(accelerator)
  registeredActionAccelerators.clear()
}

function registerActionShortcuts() {
  unregisterActionShortcuts()
  const bound = new Set()
  for (const [actionId, accelerator] of Object.entries(userState.shortcuts)) {
    const action = currentActionForStoredShortcut(userState.shortcutActions[actionId])
    if (!action) continue
    const ok = bindGlobalActionShortcut(actionId, accelerator, action)
    if (ok) bound.add(accelerator)
    else console.warn(`Could not register action shortcut ${accelerator} for ${actionId}`)
  }
  for (const { actionId, accelerator, action } of declaredGlobalShortcuts()) {
    if (userState.shortcuts[actionId] || bound.has(accelerator)) continue
    const ok = bindGlobalActionShortcut(actionId, accelerator, action)
    if (ok) bound.add(accelerator)
    else console.warn(`Could not register declared action shortcut ${accelerator} for ${actionId}`)
  }
}

function canCustomizeAction(action) {
  return ['app', 'builtin', 'clipboard-history', 'extension-command'].includes(action?.kind)
}

function getShortcuts() {
  const configured = Object.entries(userState.shortcuts)
    .map(([actionId, accelerator]) => ({
      actionId,
      accelerator,
      scope: 'global',
      source: 'user',
      action: currentActionForStoredShortcut(userState.shortcutActions[actionId]),
    }))
    .filter((item) => item.action)
  const declared = declaredGlobalShortcuts()
    .filter((item) => !userState.shortcuts[item.actionId])
    .map((item) => ({ ...item, scope: 'global', source: 'extension' }))
  return [...configured, ...declared]
    .sort((a, b) => a.action.title.localeCompare(b.action.title))
}

async function removeShortcut(actionId) {
  if (!actionId || !userState.shortcuts[actionId]) return { ok: false, message: 'Shortcut not found' }
  globalShortcut.unregister(userState.shortcuts[actionId])
  delete userState.shortcuts[actionId]
  delete userState.shortcutActions[actionId]
  invalidateShortcutCaches()
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

async function removeAlias(action, alias) {
  if (!action?.id || !alias) return { ok: false, message: 'Missing alias' }
  const current = actionAliases(action.id).filter((value) => value !== alias)
  if (current.length) userState.aliases[action.id] = current
  else delete userState.aliases[action.id]
  scheduleSaveState()
  return { ok: true, message: `Alias removed: ${alias}` }
}

async function setShortcut(action, shortcut) {
  if (!canCustomizeAction(action)) return { ok: false, message: 'Shortcuts are only available for persistent commands' }
  if (!action?.id || !shortcut.trim()) return { ok: false, message: 'Missing shortcut' }
  const accelerator = normalizeAccelerator(shortcut)
  if (accelerator === getPaletteHotkey()) return { ok: false, message: `${accelerator} is reserved for opening Nevermind` }
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

async function setPaletteHotkey(accelerator) {
  if (!accelerator?.trim()) return { ok: false, message: 'Missing shortcut' }
  const normalized = normalizeAccelerator(accelerator)
  const current = getPaletteHotkey()
  if (normalized === current) return { ok: true, message: `Shortcut unchanged: ${normalized}`, spotlightConflict: isSpotlightAccelerator(normalized) }
  const conflictingActionId = Object.entries(userState.shortcuts).find(([, value]) => value === normalized)?.[0]
  if (conflictingActionId) {
    const title = userState.shortcutActions[conflictingActionId]?.title || 'another action'
    return { ok: false, message: `${normalized} is already used by ${title}` }
  }
  globalShortcut.unregister(current)
  const ok = globalShortcut.register(normalized, togglePalette)
  if (!ok) {
    globalShortcut.register(current, togglePalette)
    const spotlightConflict = isSpotlightAccelerator(normalized)
    return { ok: false, message: spotlightConflict ? `${normalized} is used by Spotlight` : `Could not register ${normalized}`, spotlightConflict }
  }
  setSetting('paletteHotkey', normalized)
  return { ok: true, message: `Shortcut set: ${normalized}`, spotlightConflict: isSpotlightAccelerator(normalized) }
}

async function openSystemKeyboardSettings() {
  if (process.platform === 'darwin') {
    const urls = [
      'x-apple.systempreferences:com.apple.Keyboard-Settings.extension?Shortcuts',
      'x-apple.systempreferences:com.apple.Keyboard-Settings.extension',
      'x-apple.systempreferences:com.apple.preference.keyboard?Shortcuts',
    ]
    for (const url of urls) {
      try { await shell.openExternal(url); return { ok: true } } catch {}
    }
  }
  return { ok: false }
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

async function duplicateCreatedAction(action) {
  if (action?.kind !== 'extension-command' || !action.removable) return { ok: false, message: 'Only generated extension actions can be duplicated' }
  const entry = extensionEntryForAction(action)
  const filePath = entry?.extension?.__filePath
  const chatId = entry?.extension?.__chatId || action.aiChatId
  const chat = chatId ? userState.aiChats[chatId] : null
  if (!filePath || !chat) return { ok: false, message: 'Generated action not found' }

  const duplicateId = hashValue(`${chat.id}:${Date.now()}`)
  const duplicateTitle = `Copy of ${chat.title || chat.query || action.title}`
  const duplicateQuery = duplicateTitle
  const duplicateFile = `${path.basename(filePath, '.cjs')}-copy-${duplicateId.slice(0, 8)}.cjs`
  const sourceFile = path.basename(filePath)
  const sourceCode = `const source = require('./${sourceFile.replace(/'/g, "\\'")}')\n\nmodule.exports = {\n  ...source,\n  id: ${JSON.stringify(`${entry.extension.id}-copy-${duplicateId.slice(0, 8)}`)},\n  title: ${JSON.stringify(duplicateTitle)},\n  commands: (source.commands || []).map((command) => ({ ...command })),\n}\n`

  await fs.writeFile(path.join(extensionsDir, duplicateFile), sourceCode)
  userState.aiChats[duplicateId] = {
    id: duplicateId,
    query: duplicateQuery,
    title: duplicateTitle,
    status: 'ready',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    generatedExtensionFile: duplicateFile,
    messages: [
      ...(chat.messages || []),
      { role: 'system', content: `Duplicated from “${chat.title || chat.query || action.title}”. Tweak this copy without changing the original.` },
    ].slice(-100),
  }
  scheduleSaveState()
  await loadExtensions()
  registerActionShortcuts()
  const duplicateEntry = Array.from(extensionRegistry.values()).find((candidate) => candidate.extension?.__chatId === duplicateId)
  return { ok: true, message: 'Action duplicated', action: duplicateEntry ? extensionActionFromCommand(duplicateEntry.extension, duplicateEntry.command) : { kind: 'ai-chat', aiChatId: duplicateId, title: duplicateTitle } }
}

async function removeCreatedAction(action) {
  if (action?.kind === 'ai-chat' && action.aiChatId) {
    const chat = userState.aiChats[action.aiChatId]
    if (chat?.generatedExtensionFile) {
      await fs.unlink(path.join(extensionsDir, chat.generatedExtensionFile)).catch((error) => {
        if (error?.code !== 'ENOENT') throw error
      })
      await loadExtensions()
      registerActionShortcuts()
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
    registerActionShortcuts()
    return { ok: true, message: 'Generated action removed' }
  }

  return { ok: false, message: 'This action cannot be removed' }
}

app.whenReady().then(async () => {
  nativeTheme.themeSource = 'dark'
  if (process.platform === 'darwin') {
    app.setActivationPolicy('accessory')
    app.dock.hide()
  }
  registerLocalFileProtocol()
  installPermissionHandlers()
  configureAutoUpdater()

  await loadUserState()
  await loadExtensions()
  initNevermindAi()
  createWindow()
  registerHotkey()
  registerActionShortcuts()
  startClipboardWatcher()
  setTimeout(indexApplications, 100)
  setTimeout(indexFiles, 200)
  await startAppWatcher()

  ipcMain.handle('actions:search', (_event, query, options) => searchActions(query, options))
  ipcMain.handle('actions:execute', (_event, action) => executeActionForIpc(action))
  ipcMain.handle('view-action:execute', (_event, action) => executeViewActionForIpc(action))
  ipcMain.on('drag:file', startFileDrag)
  ipcMain.handle('ai:chat:send', (_event, message, chatId) => sendAiChatMessage(message, chatId))
  ipcMain.handle('ai:chat:abort', (_event, chatId) => abortAiChat(chatId))
  ipcMain.handle('ai:chat:reset', (_event, chatId) => resetAiChat(chatId))
  ipcMain.handle('actions:set-alias', (_event, action, alias) => setAlias(action, alias))
  ipcMain.handle('actions:remove-alias', (_event, action, alias) => removeAlias(action, alias))
  ipcMain.handle('actions:set-shortcut', (_event, action, shortcut) => setShortcut(action, shortcut))
  ipcMain.handle('palette:set-hotkey', (_event, accelerator) => setPaletteHotkey(accelerator))
  ipcMain.handle('system:open-keyboard-settings', () => openSystemKeyboardSettings())
  ipcMain.handle('actions:get-shortcuts', () => getShortcuts())
  ipcMain.handle('actions:remove-shortcut', (_event, actionId) => removeShortcut(actionId))
  ipcMain.handle('actions:suspend-shortcuts', () => unregisterActionShortcuts())
  ipcMain.handle('actions:resume-shortcuts', () => registerActionShortcuts())
  ipcMain.handle('actions:set-override', (_event, action, instruction) => setOverride(action, instruction))
  ipcMain.handle('actions:clear-override', (_event, action) => clearOverride(action))
  ipcMain.handle('actions:duplicate-created', (_event, action) => duplicateCreatedAction(action))
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
  if (app.isReady()) globalShortcut.unregisterAll()
  clearUpdateTimers()
  for (const watcher of appWatchers) watcher.close()
})

const gotLock = app.requestSingleInstanceLock()
if (!gotLock) app.quit()
else app.on('second-instance', () => showPalette())
