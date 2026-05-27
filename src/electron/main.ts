import { app, globalShortcut, ipcMain, shell, clipboard, nativeImage, nativeTheme, protocol, net, systemPreferences } from 'electron'
import electronUpdater from 'electron-updater'
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import crypto from 'node:crypto'
import { spawn, execFile } from 'node:child_process'
import { pathToFileURL } from 'node:url'
import { clipboardFilePath as readClipboardFilePath, clipboardFilePaths, clipboardItemSubtitle, clipboardItemTitle, normalizeClipboardHistory } from './clipboard-utils'
import { expandUserPath, extensionForPath, fileUrlForPath, IMAGE_EXTENSIONS, isImagePath, isVideoPath, LOCAL_FILE_PROTOCOL, LOCAL_THUMB_PROTOCOL, thumbnailUrlForPath, VIDEO_EXTENSIONS } from './file-utils'
import { createNevermindAi } from './ai'
import { createPaletteWindowController, installPermissionHandlers } from './palette-window'
import { settingDefinition, SETTING_DEFINITIONS, settingValue, toggledSettingValue } from './settings'
import { calculate, getUrlFromQuery, hashValue, normalize, score, scoreNormalized } from './search-utils'
import { formatShortcut, isSpotlightAccelerator, normalizeAccelerator } from './shortcut-utils'
import { autoUpdatesUnavailableMessage, executeSystemBuiltin, fileDateAddedMs, frontmostApp, hasCapability, keyboardSettingsSubtitle, launchApp as launchOsApp, osLabel, pasteIntoFrontmostApp, prepareAppWindowPolicy, quickLookTitle, reservedPaletteShortcutName, revealPathTitle, scanApps, selectedFilePaths, selectedText, settingsTitle, watchApps } from './os'
import { createUpdateManager } from './update-manager'
import { isNewerVersion as isVersionNewerThan } from './version-utils'
import { configureLogger, extensionLogger, info as logInfo, warn as logWarn, error as logError, debug as loggerDebug } from './logger'
import { canCustomizeCommandAction } from '../model'

const { autoUpdater } = electronUpdater
const isDev = Boolean(process.env.ELECTRON_RENDERER_URL)
configureLogger(isDev)

const updateManager = createUpdateManager(autoUpdater as any)
const paletteWindow = createPaletteWindowController({
  isDev: Boolean(process.env.ELECTRON_RENDERER_URL),
  preloadPath: path.join(__dirname, '..', 'preload', 'preload.cjs'),
  rendererUrl: process.env.ELECTRON_RENDERER_URL,
  rendererIndexPath: path.join(__dirname, '..', 'renderer', 'index.html'),
  getPaletteHotkey: () => String(getPaletteHotkey()),
})

const CLIPBOARD_LIMIT = 300
const FILE_RESULT_LIMIT = 6
const CLIPBOARD_POLL_INTERVAL_MS = 1000
const APP_REINDEX_DEBOUNCE_MS = 1000
const THUMBNAIL_SIZE = 512
const EXTENSION_ROOT_ITEMS_TTL_MS = 60_000
const EXTENSION_ROOT_ITEMS_TIMEOUT_MS = 10_000
const EXTENSION_ITEMS_PER_PROVIDER_LIMIT = 20
const ITEM_FOREGROUND_COLORS = new Set(['yellow', 'blue', 'purple', 'green', 'red', 'orange', 'pink'])
const EXTENSION_CACHE_MAX_TTL_MS = 24 * 60 * 60_000
const EXTENSION_CACHE_MAX_ENTRIES = 1000
const EXTENSION_REFRESH_MAX_BURST = 5
const EXTENSION_REFRESH_BURST_WINDOW_MS = 2000
const EXTENSION_AI_CALLS_PER_MINUTE = 30
const EXTENSION_AI_RATE_WINDOW_MS = 60_000
const EXTENSION_TYPES_FILENAME = 'nevermind-extension-api.d.ts'

protocol.registerSchemesAsPrivileged([
  { scheme: LOCAL_FILE_PROTOCOL, privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true, bypassCSP: true } },
  { scheme: LOCAL_THUMB_PROTOCOL, privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true, bypassCSP: true } },
])

type AnyRecord = Record<string, any>

type NevermindApp = typeof app & { isQuiting?: boolean }

const nevermindApp = app as NevermindApp

let appIndex: any[] = []
let fileIndex: any[] = []
let clipboardHistory: any[] = []
let statePath = ''
let iconCacheDir = ''
let clipboardImagesDir = ''
let extensionsDir = ''
let extensionStorageDir = ''
let extensionCacheDir = ''
let saveTimer: NodeJS.Timeout | undefined
let appIndexTimer: NodeJS.Timeout | undefined
let appWatchers: Array<{ close: () => unknown }> = []
let nevermindAi: any
let activeAiChatId: string | undefined
const draftAiChats = new Map<string, AnyRecord>()
let didRunQuitCleanup = false
let userState: AnyRecord = {
  recents: {},
  aliases: {},
  shortcuts: {},
  shortcutActions: {},
  removedShortcuts: {},
  overrides: {},
  clipboardHistory: [],
  aiChats: {},
  settings: {},
}

function osCacheRoot() {
  const appName = app.getName()
  if (process.platform === 'darwin') return path.join(os.homedir(), 'Library', 'Caches', appName)
  if (process.platform === 'win32') return path.join(process.env.LOCALAPPDATA || app.getPath('appData'), appName, 'Cache')
  return path.join(process.env.XDG_CACHE_HOME || path.join(os.homedir(), '.cache'), appName)
}

function getPaletteHotkey() {
  return getSetting('paletteHotkey') || 'Alt+Space'
}

function getSetting(id: any) {
  return settingValue(userState.settings, id)
}

function setSetting(id: any, value: any) {
  if (!userState.settings) userState.settings = {}
  userState.settings[id] = value
  scheduleSaveState()
  invalidateExtensionRootItems()
  patchSettingsView(id)
}
const appIconCache = new Map<string, string | null>()
const extensionRegistry = new Map<string, any>()
const extensionModules = new Map<string, any>()
const extensionRootItemsCache = new Map<string, { updatedAt: number; items: any[] }>()
const extensionRootItemsRefreshes = new Map<string, Promise<any[]>>()
const extensionStorageRefreshes = new Map<string, Promise<any>>()
const extensionActionHandlers = new Map<string, any>()
const extensionCaches = new Map<string, Map<string, { value: any; expiresAt: number }>>()

function extensionCacheFor(extensionId) {
  let store = extensionCaches.get(extensionId)
  if (!store) {
    store = new Map()
    extensionCaches.set(extensionId, store)
  }
  return store
}

function enforceExtensionCacheBudget(store: Map<string, { value: any; expiresAt: number }>) {
  if (store.size <= EXTENSION_CACHE_MAX_ENTRIES) return
  const overflow = store.size - EXTENSION_CACHE_MAX_ENTRIES
  const iterator = store.keys()
  for (let i = 0; i < overflow; i++) {
    const next = iterator.next()
    if (next.done) break
    store.delete(next.value)
  }
}

const extensionRefreshBurstWindow = new Map<string, number[]>()
const extensionAiCallWindow = new Map<string, number[]>()

function isInternalExtension(extension: any) {
  return typeof extension?.id === 'string' && extension.id.startsWith('nevermind.')
}

function hasExtensionPermission(extension: any, permission: string) {
  const declared = Array.isArray(extension?.permissions) ? extension.permissions : null
  if (declared) return declared.includes(permission)
  return isInternalExtension(extension)
}

function permissionDeniedError(permission: string) {
  return new Error(`Extension is missing required permission: ${permission}`)
}

function checkRefreshBurst(extension: any) {
  const id = extension?.id || 'unknown'
  const now = Date.now()
  const recent = (extensionRefreshBurstWindow.get(id) || []).filter((time) => now - time < EXTENSION_REFRESH_BURST_WINDOW_MS)
  if (recent.length >= EXTENSION_REFRESH_MAX_BURST) {
    logWarn('extension.refresh.throttled', { count: recent.length }, { source: 'host', scope: 'extension', extensionId: id })
    return false
  }
  recent.push(now)
  extensionRefreshBurstWindow.set(id, recent)
  return true
}

function checkAiRateLimit(extension: any) {
  const id = extension?.id || 'unknown'
  const now = Date.now()
  const recent = (extensionAiCallWindow.get(id) || []).filter((time) => now - time < EXTENSION_AI_RATE_WINDOW_MS)
  if (recent.length >= EXTENSION_AI_CALLS_PER_MINUTE) return false
  recent.push(now)
  extensionAiCallWindow.set(id, recent)
  return true
}


function createExtensionCache(extension) {
  const store = extensionCacheFor(extension.id)
  return {
    get(key) {
      const entry = store.get(key)
      if (!entry) return undefined
      if (entry.expiresAt && Date.now() > entry.expiresAt) return undefined
      return entry.value
    },
    getStale(key) {
      return store.get(key)?.value
    },
    has(key) {
      const entry = store.get(key)
      if (!entry) return false
      return !entry.expiresAt || Date.now() <= entry.expiresAt
    },
    set(key, value, options: any = {}) {
      const rawTtl = Number(options.ttlMs || 0)
      const clampedTtl = rawTtl > 0 ? Math.min(rawTtl, EXTENSION_CACHE_MAX_TTL_MS) : 0
      store.set(key, { value, expiresAt: clampedTtl > 0 ? Date.now() + clampedTtl : 0 })
      enforceExtensionCacheBudget(store)
      return value
    },
    invalidate(key) {
      if (key === undefined) store.clear()
      else store.delete(key)
      invalidateExtensionRootItemsForExtension(extension)
    },
    keys() {
      return Array.from(store.keys())
    },
  }
}
const registeredActionAccelerators = new Set<string>()
const AI_BUILDER_EXTENSION_ID = 'nevermind.ai-builder'

const INTERNAL_EXTENSION_FACTORIES: Array<() => any> = [createSystemExtension, createPlacesExtension, createCalculatorExtension, createWebSearchExtension, createClipboardExtension, createAppsExtension, createFilesExtension, createAiBuilderExtension, createUpdatesExtension, createKeyboardShortcutsExtension, createSettingsExtension]
const REQUIRED_INTERNAL_EXTENSIONS = ['nevermind.system', 'nevermind.places', 'nevermind.calculator', 'nevermind.web', 'nevermind.clipboard', 'nevermind.apps', 'nevermind.files', AI_BUILDER_EXTENSION_ID, 'nevermind.updates', 'nevermind.shortcuts', 'nevermind.settings']
const REQUIRED_INTERNAL_COMMANDS = [{ extensionId: AI_BUILDER_EXTENSION_ID, commandId: 'ai-chats' }]

function actionAliases(actionId: any) {
  const value = userState.aliases[actionId]
  if (!value) return []
  return Array.isArray(value) ? value : [value]
}

function actionSearchScore(action: any, query: any) {
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

function usageBoost(actionId: any) {
  const count = userState.recents[actionId]?.count || 0
  return Math.min(90, count * 6)
}

function recentBoost(actionId: any) {
  const recent = userState.recents[actionId]
  if (!recent) return 0
  const ageHours = Math.max(0, (Date.now() - recent.lastUsed) / 36e5)
  return Math.max(0, 20 - ageHours)
}

function priorityBoost(action: any) {
  return action.kind === 'app' ? 25 : 0
}

function defaultActionIdFor(action: any) {
  if (action.defaultActionId) return action.defaultActionId
  if (action.kind === 'builtin') return action.id
  if (action.kind === 'calculate') return 'default:calculator'
  return null
}

let shortcutByAiChatIdCache: Map<string, string> | null = null
function shortcutByAiChatIdMap() {
  if (shortcutByAiChatIdCache) return shortcutByAiChatIdCache
  const shortcutsByChat = new Map<string, string[]>()
  for (const [actionId, storedAction] of Object.entries(userState.shortcutActions) as Array<[string, any]>) {
    if (storedAction?.aiChatId && userState.shortcuts[actionId]) {
      shortcutsByChat.set(storedAction.aiChatId, [...(shortcutsByChat.get(storedAction.aiChatId) || []), userState.shortcuts[actionId]])
    }
  }
  const map = new Map<string, string>()
  for (const [chatId, shortcuts] of shortcutsByChat) {
    if (shortcuts.length === 1 && chatTouchedExtensionFiles(userState.aiChats[chatId]).length === 1) map.set(chatId, shortcuts[0])
  }
  shortcutByAiChatIdCache = map
  return map
}

function invalidateShortcutCaches() {
  shortcutByAiChatIdCache = null
}

function shortcutForAction(action: any) {
  if (userState.shortcuts[action.id]) return userState.shortcuts[action.id]
  if (userState.removedShortcuts?.[action.id]) return null
  if (!action.aiChatId) return null
  return shortcutByAiChatIdMap().get(action.aiChatId) || null
}

function withShortcutHint(action: any) {
  const shortcut = shortcutForAction(action)
  return shortcut ? { ...action, shortcut } : action
}

function withDefaultOverride(action: any) {
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

function rankAction(action: any, query: any) {
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

function recordRecent(action: any) {
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

async function getAppIconDataUrl(appPath) {
  if (!hasCapability('app-icons') || !appPath || !appPath.endsWith('.app')) return null
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
      logWarn('appIcon.load.failed', { appPath, error }, { source: 'host', scope: 'apps' })
      return null
    }
  })()

  const result = await promise
  if (result) appIconCache.set(appPath, result)
  else appIconCache.delete(appPath)
  return result
}

function extensionCommandActionId(extension, command) {
  return `extension:${extension.id}:${command.id}`
}

function extensionActionFromCommand(extension, command) {
  const extensionFile = extension.__filePath ? path.basename(extension.__filePath) : undefined
  const action = {
    id: command.actionId || extensionCommandActionId(extension, command),
    kind: 'extension-command',
    extensionId: extension.id,
    commandId: command.id,
    extensionFile,
    aiChatId: extensionFile ? aiChatIdForExtensionFile(extensionFile) : undefined,
    removable: Boolean(extension.__generated),
    title: command.title,
    subtitle: command.subtitle || extension.title || 'Extension command',
    aliases: command.aliases || [],
    icon: command.icon || 'sparkles',
    score: command.score || 12,
    dismissAfterRun: command.dismissAfterRun,
    background: command.background,
  }
  const declaredShortcut = userState.removedShortcuts?.[action.id] ? null : command.globalShortcut || (command.shortcutScope === 'global' ? command.shortcut : null)
  const shortcut = shortcutForAction(action) || declaredShortcut
  return shortcut ? { ...action, shortcut } : action
}

function getOrCreateAiChat(query, options: any = {}) {
  const trimmed = query.trim()
  const baseId = hashValue(trimmed)
  const current = userState.aiChats[baseId]
  if (current && !options.fresh) return current
  const id = current && options.fresh ? hashValue(`${trimmed}:${Date.now()}:${crypto.randomUUID()}`) : baseId
  const item = aiChatItem(id, trimmed)
  userState.aiChats[id] = item
  scheduleSaveState()
  invalidateExtensionRootItems()
  return item
}

function aiChatItem(id, query) {
  return {
    id,
    query,
    title: query,
    status: 'running',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    messages: [
      { role: 'assistant', content: `What should “${query}” do? Tell me the exact behavior, inputs, and what UI you want, then I’ll build it.` },
    ],
  }
}

function createDraftAiChat(query) {
  const trimmed = query.trim()
  const id = `draft:${hashValue(`${trimmed}:${Date.now()}:${crypto.randomUUID()}`)}`
  const item = aiChatItem(id, trimmed)
  draftAiChats.set(id, item)
  return item
}

function promoteDraftAiChat(chatId) {
  if (userState.aiChats[chatId]) return userState.aiChats[chatId]
  const draft = draftAiChats.get(chatId)
  if (!draft) return null
  draftAiChats.delete(chatId)
  draft.createdAt = Date.now()
  draft.updatedAt = Date.now()
  userState.aiChats[chatId] = draft
  scheduleSaveState()
  invalidateExtensionRootItems()
  return draft
}

function appendAiChatMessage(chatId, role, content) {
  const chat = userState.aiChats[chatId]
  if (!chat || !content) return
  chat.messages = [...(chat.messages || []), { role, content }].slice(-100)
  chat.updatedAt = Date.now()
  scheduleSaveState()
  patchAiChatsItem(chatId)
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

function aiChatView(item, options: any = {}) {
  return {
    type: 'chat',
    title: `Automate “${item.query}”`,
    aiChat: true,
    chatId: item.id,
    initialPrompt: options.initialPrompt,
    messages: item.messages || [],
  }
}

function aiChatOpenAction(chatId) {
  return buildAiBuilderAction('Open Chat', async () => {
    const item = userState.aiChats[chatId] || draftAiChats.get(chatId)
    if (!item) return { toast: { message: 'AI chat not found', tone: 'error' } }
    return { view: aiChatView(item) }
  })
}

function aiChatRemoveAction(chat) {
  return wrapWithConfirmation(
    buildAiBuilderAction('Remove Chat', () => removeAiChat(chat.id), { style: 'destructive' }),
    { message: `Remove “${chat.title || chat.query || 'AI chat'}” and its history? Generated extension files stay.`, confirmLabel: 'Remove Chat', destructive: true },
  )
}

function aiChatListItem(chat: any) {
  return {
    id: `ai-chat:${chat.id}`,
    title: chat.title || chat.query || 'AI Chat',
    subtitle: chat.contextExtensionFile || (chat.touchedExtensionFiles || [])[0] || chat.status || 'Builder chat',
    icon: 'sparkles',
    primaryAction: aiChatOpenAction(chat.id),
    actions: [aiChatRemoveAction(chat)],
  }
}

function aiChatListItems() {
  const chats = Object.values(userState.aiChats || {}) as any[]
  return chats
    .sort((a, b) => (b.updatedAt || b.createdAt || 0) - (a.updatedAt || a.createdAt || 0))
    .map(aiChatListItem)
}

function aiChatsView() {
  return {
    type: 'list',
    id: 'ai-chats',
    title: 'AI Chats',
    searchBarPlaceholder: 'Search AI Chats',
    items: aiChatListItems(),
  }
}

function patchOpenView(viewId: string, patch: any) {
  paletteWindow.win?.webContents.send('view:patch', { viewId, patch })
}

function aiBuilderRegistryEntry() {
  return extensionRegistry.get(`${AI_BUILDER_EXTENSION_ID}:ai-chats`) || { extension: createAiBuilderExtension(), command: { id: 'ai-chats', title: 'AI Chats' } }
}

function normalizedAiChatListItem(chat: any) {
  return normalizeViewItems([aiChatListItem(chat)], aiBuilderRegistryEntry())[0]
}

function patchAiChatsItem(chatId: string) {
  const chat = userState.aiChats[chatId]
  if (!chat) return
  patchOpenView('ai-chats', { mode: 'patch', items: [normalizedAiChatListItem(chat)] })
}

function patchAiChatsPrepend(chatId: string) {
  const chat = userState.aiChats[chatId]
  if (!chat) return
  patchOpenView('ai-chats', { mode: 'prepend', items: [normalizedAiChatListItem(chat)] })
}

function patchAiChatsRemove(chatId: string) {
  patchOpenView('ai-chats', { removeItemIds: [`ai-chat:${chatId}`] })
}

function chatTouchedExtensionFiles(chat) {
  return Array.from(new Set([...(chat?.touchedExtensionFiles || []), chat?.generatedExtensionFile, chat?.contextExtensionFile].filter(Boolean).map((item) => path.basename(item))))
}

function aiChatIdForExtensionFile(filename) {
  const base = path.basename(filename || '')
  if (!base) return null
  const matches = Object.values(userState.aiChats).filter((chat: any) => chatTouchedExtensionFiles(chat).includes(base))
  return matches.length === 1 ? (matches[0] as any).id : null
}

function touchExtensionFileForChat(chat, filename) {
  if (!chat || !filename) return
  chat.touchedExtensionFiles = Array.from(new Set([...chatTouchedExtensionFiles(chat), path.basename(filename)]))
  if (!chat.contextExtensionFile) chat.contextExtensionFile = path.basename(filename)
  if (!chat.generatedExtensionFile) chat.generatedExtensionFile = path.basename(filename)
  chat.status = 'ready'
  chat.updatedAt = Date.now()
  patchAiChatsItem(chat.id)
}

function getOrCreateExtensionChat(extensionFile, title = extensionFile) {
  const filename = path.basename(extensionFile || '')
  const existing = (Object.values(userState.aiChats || {}) as any[])
    .filter((chat) => chat.contextExtensionFile === filename || chatTouchedExtensionFiles(chat).includes(filename))
    .sort((a, b) => (b.updatedAt || b.createdAt || 0) - (a.updatedAt || a.createdAt || 0))[0]
  if (existing) {
    existing.contextExtensionFile = filename
    existing.updatedAt = Date.now()
    scheduleSaveState()
    patchAiChatsItem(existing.id)
    return existing
  }
  const id = hashValue(`extension-chat:${filename}:${Date.now()}`)
  const item = {
    id,
    query: `Tweak ${title || filename}`,
    title: `Tweak ${title || filename}`,
    status: 'ready',
    contextExtensionFile: filename,
    touchedExtensionFiles: [filename],
    createdAt: Date.now(),
    updatedAt: Date.now(),
    messages: [
      { role: 'assistant', content: `I can tweak “${title || filename}”. I can read this extension as context and inspect any other extension if needed.` },
    ],
  }
  userState.aiChats[id] = item
  scheduleSaveState()
  invalidateExtensionRootItems()
  patchAiChatsPrepend(id)
  return item
}

function clipboardPreviewAction(item) {
  return buildPreviewItemAction({
    kind: item.type === 'image' ? 'image' : item.type === 'video' ? 'video' : item.filePath ? 'file' : 'clipboard',
    clipboardType: item.type,
    text: item.text,
    imageDataUrl: item.imageDataUrl,
    imagePath: item.imagePath,
    videoUrl: item.videoUrl,
    filePath: item.filePath,
    thumbnailUrl: item.thumbnailUrl,
  })
}

function clipboardCopyAction(item) {
  if (item.type === 'image') return { type: 'copyImage', title: 'Copy Image', imageDataUrl: item.imageDataUrl, imagePath: item.imagePath, dismissAfterRun: 'auto' }
  return { type: 'copyText', title: item.type === 'video' ? 'Copy Video Path' : 'Copy Text', text: item.filePath || item.text, dismissAfterRun: 'auto' }
}

function clipboardRootItem(item) {
  return {
    id: `clipboard:${item.id}`,
    title: clipboardItemTitle(item),
    subtitle: clipboardItemSubtitle(item),
    icon: 'clipboard',
    image: item.thumbnailUrl,
    score: 60,
    lastUsed: item.createdAt || 0,
    primaryAction: clipboardCopyAction(item),
    actionPanel: { sections: [{ actions: [clipboardPreviewAction(item), clipboardCopyAction(item)].filter(Boolean) }] },
  }
}

function clipboardHistoryItem(item: any) {
  const isImage = item.type === 'image'
  const isVideo = item.type === 'video'
  const copyAction = clipboardCopyAction(item)
  const previewAction = clipboardPreviewAction(item)
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
}

function clipboardHistoryItems() {
  return clipboardHistory.slice(0, CLIPBOARD_LIMIT).map(clipboardHistoryItem)
}

function clipboardHistorySnapshot(options: any = {}) {
  const { limit, query, types } = options
  let entries = clipboardHistory
  if (Array.isArray(types) && types.length) entries = entries.filter((entry) => types.includes(entry.type))
  if (query) {
    const needle = String(query).toLowerCase()
    entries = entries.filter((entry) => `${entry.text || ''} ${entry.type || ''} ${entry.filePath || ''}`.toLowerCase().includes(needle))
  }
  const max = typeof limit === 'number' ? limit : CLIPBOARD_LIMIT
  return entries.slice(0, max).map((entry) => ({
    id: entry.id,
    type: entry.type,
    text: entry.text,
    imageDataUrl: entry.imageDataUrl,
    imagePath: entry.imagePath,
    videoUrl: entry.videoUrl,
    filePath: entry.filePath,
    thumbnailUrl: entry.thumbnailUrl,
    createdAt: entry.createdAt,
  }))
}

function viewRefreshAction(itemsBuilder) {
  return {
    type: 'runExtensionAction',
    title: 'Refresh',
    __handler: () => ({ patch: { mode: 'replace', items: itemsBuilder() } }),
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
    items: clipboardHistoryItems(),
  }
}

function isNewerVersion(version) {
  return isVersionNewerThan(version, app.getVersion())
}

function updateStatusItems() {
  return updateStatusView().items
}

function updateStatusView(_options: any = {}) {
  const downloadedInfo = isNewerVersion(updateManager.state.downloadedInfo?.version) ? updateManager.state.downloadedInfo : null
  const availableInfo = isNewerVersion(updateManager.state.availableInfo?.version) ? updateManager.state.availableInfo : null
  const version = downloadedInfo?.version || availableInfo?.version
  const unsupported = updateManager.state.status === 'unsupported' || !updateManager.canUseAutoUpdates()
  const installing = updateManager.state.installInFlight || updateManager.state.status === 'installing'
  const primaryAction = installing
    ? undefined
    : downloadedInfo
      ? { type: 'installUpdate', title: 'Install and Restart' }
      : availableInfo
        ? { type: 'downloadUpdate', title: updateManager.state.downloadInFlight ? 'Downloading…' : 'Download Update' }
        : { type: 'checkForUpdates', title: updateManager.state.checkInFlight ? 'Checking…' : 'Check Again' }
  const title = unsupported
    ? 'Updates unavailable'
    : installing
      ? `Installing Nevermind ${version || ''}`.trim()
      : downloadedInfo
        ? `Nevermind ${version} is ready`
        : availableInfo
          ? `Nevermind ${version} is available`
          : updateManager.state.checkInFlight
            ? 'Checking for updates…'
            : updateManager.state.status === 'error'
              ? 'Update check failed'
              : 'No versions available'
  const subtitle = unsupported
    ? autoUpdatesUnavailableMessage()
    : installing
      ? 'Restarting Nevermind to finish updating…'
      : downloadedInfo
        ? 'Install the downloaded update and restart Nevermind'
        : availableInfo
          ? 'Download the update before installing it'
          : updateManager.state.checkInFlight
            ? `Current version: ${app.getVersion()}`
            : updateManager.state.status === 'error'
              ? updateManager.state.errorMessage
              : `Current version: ${app.getVersion()}`
  return {
    type: 'list',
    id: 'app-updates',
    title: 'Updates',
    presentation: 'root',
    searchBarPlaceholder: 'Search Updates',
    isLoading: updateManager.state.checkInFlight || updateManager.state.downloadInFlight || updateManager.state.installInFlight,
    items: [{
      id: 'update-status',
      title,
      subtitle,
      icon: 'restart',
      accessories: version ? [{ text: version }] : [],
      primaryAction: unsupported ? undefined : primaryAction,
      actionPanel: unsupported || !primaryAction ? undefined : { sections: [{ actions: [primaryAction] }] },
    }],
  }
}

function updatesStateSnapshot() {
  const state = updateManager.state
  const downloadedInfo = isNewerVersion(state.downloadedInfo?.version) ? state.downloadedInfo : null
  const availableInfo = isNewerVersion(state.availableInfo?.version) ? state.availableInfo : null
  return {
    currentVersion: app.getVersion(),
    status: String(state.status || 'idle'),
    supported: updateManager.canUseAutoUpdates() && state.status !== 'unsupported',
    checking: Boolean(state.checkInFlight),
    downloading: Boolean(state.downloadInFlight),
    installing: Boolean(state.installInFlight || state.status === 'installing'),
    availableVersion: availableInfo?.version || null,
    downloadedVersion: downloadedInfo?.version || null,
    errorMessage: state.errorMessage || null,
  }
}

function checkForUpdatesView() {
  updateManager.checkForUpdates('manual', { download: true }).catch(() => {})
  return { view: updateStatusView(), navigation: 'replace' }
}

function downloadUpdateView() {
  updateManager.downloadAvailableUpdate().catch(() => {})
  return { view: updateStatusView(), navigation: 'replace' }
}

let updateInstallQuitFallbackTimer: NodeJS.Timeout | null = null

function scheduleUpdateInstallQuitFallback() {
  if (updateInstallQuitFallbackTimer) return
  updateInstallQuitFallbackTimer = setTimeout(() => {
    updateInstallQuitFallbackTimer = null
    logWarn('updater.install.quitFallback', undefined, { source: 'host', scope: 'updater' })
    nevermindApp.isQuiting = true
    app.quit()
    setTimeout(() => {
      logWarn('updater.install.exitFallback', undefined, { source: 'host', scope: 'updater' })
      runQuitCleanup()
      app.exit(0)
    }, 2_000).unref?.()
  }, 5_000)
  updateInstallQuitFallbackTimer.unref?.()
}

function installDownloadedUpdate() {
  if (!updateManager.state.downloadedInfo) return { view: updateStatusView(), navigation: 'replace' }
  nevermindApp.isQuiting = true
  const didStart = updateManager.quitAndInstall()
  if (didStart || updateManager.state.installInFlight) scheduleUpdateInstallQuitFallback()
  return { view: updateStatusView(), navigation: 'replace' }
}

function settingItemPatch(definition) {
  const value = getSetting(definition.id)
  const accessoryText = definition.type === 'boolean' ? (value ? 'On' : 'Off') : definition.type === 'shortcut' ? '' : String(value)
  const primaryAction = definition.type === 'shortcut'
    ? buildRecordShortcutAction({ scope: 'palette', title: 'Change Shortcut', shortcut: String(value || '') }, {})
    : { type: 'toggleSetting', title: value ? 'Turn Off' : 'Turn On', settingId: definition.id }
  return { id: `setting:${definition.id}`, accessories: accessoryText ? [{ text: accessoryText }] : [], primaryAction, actionPanel: { sections: [{ actions: [primaryAction] }] } }
}

function settingsItems() {
  return SETTING_DEFINITIONS.map((definition) => ({
    id: `setting:${definition.id}`,
    title: definition.title,
    subtitle: definition.description,
    icon: 'settings',
    ...settingItemPatch(definition),
  }))
}

function settingsView(selectedItemId = '') {
  return {
    type: 'list',
    id: 'app-settings',
    title: 'Settings',
    presentation: 'root',
    selectedItemId,
    searchBarPlaceholder: 'Search Settings',
    items: settingsItems(),
  }
}

function patchSettingsView(settingId: string, options: any = {}) {
  const definition = SETTING_DEFINITIONS.find((item) => item.id === settingId)
  if (!definition) return
  patchOpenView('app-settings', {
    mode: 'patch',
    items: [{ id: `setting:${definition.id}`, ...settingItemPatch(definition) }],
    ...options,
  })
}

function patchUpdatesView() {
  patchOpenView('app-updates', {
    mode: 'patch',
    items: updateStatusItems(),
    isLoading: updateManager.state.checkInFlight || updateManager.state.downloadInFlight || updateManager.state.installInFlight,
  })
}

function updatePromptAction() {
  const downloadedInfo = isNewerVersion(updateManager.state.downloadedInfo?.version) ? updateManager.state.downloadedInfo : null
  const availableInfo = isNewerVersion(updateManager.state.availableInfo?.version) ? updateManager.state.availableInfo : null
  const version = downloadedInfo?.version || availableInfo?.version
  if (updateManager.state.installInFlight || updateManager.state.status === 'installing') {
    return {
      id: 'updates:installing',
      title: `Installing Nevermind ${version || ''}`.trim(),
      subtitle: 'Restarting Nevermind to finish updating…',
      icon: 'restart',
      score: 1_000,
    }
  }
  if (downloadedInfo) {
    return {
      id: 'updates:install',
      title: `Install Nevermind ${version}`,
      subtitle: 'Restart Nevermind to finish updating',
      icon: 'restart',
      score: 1_000,
      primaryAction: { type: 'installUpdate', title: `Install Nevermind ${version}` },
    }
  }
  if (availableInfo) {
    return {
      id: 'updates:download',
      title: `Download Nevermind ${version}`,
      subtitle: updateManager.state.downloadInFlight ? 'Downloading update…' : 'Update available',
      icon: 'restart',
      score: 1_000,
      primaryAction: { type: 'downloadUpdate', title: `Download Nevermind ${version}` },
    }
  }
  return null
}

async function searchActions(query, options: any = {}) {
  const q = query.trim()

  if (options.clipboardOnly) {
    return clipboardHistory
      .map(clipboardRootItem)
      .filter((item) => q ? rankAction(item, q) : true)
      .sort((a, b) => q ? b.score - a.score || b.lastUsed - a.lastUsed : b.lastUsed - a.lastUsed)
      .slice(0, CLIPBOARD_LIMIT)
  }

  const results = []
  const contributedItems = q ? await extensionSearchActions(q) : await extensionRootActions()
  for (const item of contributedItems) {
    const ranked = item.__ranked ? withShortcutHint(item) : rankAction(withShortcutHint(item), q)
    if (ranked) results.push(ranked)
  }

  for (const command of extensionRegistry.values()) {
    const ranked = rankAction(withShortcutHint(extensionActionFromCommand(command.extension, command.command)), q)
    if (ranked) results.push(ranked)
  }

  return results
    .sort((a, b) => {
      return b.score - a.score || b.lastUsed - a.lastUsed || a.title.localeCompare(b.title)
    })
    .slice(0, 30)
}

function invalidateExtensionRootItems() {
  extensionRootItemsCache.clear()
  paletteWindow.win?.webContents.send('root-items:changed')
}

function invalidateExtensionRootItemsForExtension(extension) {
  const cacheKey = extension.__filePath || extension.id
  extensionRootItemsCache.delete(cacheKey)
  paletteWindow.win?.webContents.send('root-items:changed')
}

function runInBackground(task) {
  Promise.resolve().then(task).catch((error) => logError('backgroundAction.failed', error, { source: 'host' }))
}

async function executeAction(action, options: any = {}) {
  if (!action) return
  recordRecent(action)

  switch (action.kind) {
    case 'open-keyboard-settings':
      runInBackground(openSystemKeyboardSettings)
      break
    case 'extension-root-item': {
      const result = await executeExtensionRootItem(action)
      if (result) return result
      break
    }
    case 'extension-command': {
      const dismissImmediately = action.background || action.dismissAfterRun === 'auto'
      if (dismissImmediately && !options.keepPaletteOpen) paletteWindow.hidePalette()
      const result = await executeExtensionCommand(action)
      if (result?.type) return { view: result }
      if (result) return result
      break
    }
  }

  if (!options.keepPaletteOpen) paletteWindow.hidePalette()
}

function extensionEntryForAction(action) {
  const direct = extensionRegistry.get(`${action.extensionId}:${action.commandId}`)
  if (direct) return direct

  if (action.extensionFile) {
    const fileMatches = Array.from(extensionRegistry.values()).filter((entry) => path.basename(entry.extension.__filePath || '') === action.extensionFile)
    if (fileMatches.length === 1) return fileMatches[0]
  }

  if (action.aiChatId) {
    const files = chatTouchedExtensionFiles(userState.aiChats[action.aiChatId])
    const chatMatches = Array.from(extensionRegistry.values()).filter((entry) => files.includes(path.basename(entry.extension.__filePath || '')))
    if (chatMatches.length === 1) return chatMatches[0]
  }

  const matches = Array.from(extensionRegistry.values()).filter((entry) => entry.extension.id === action.extensionId)
  return matches.length === 1 ? matches[0] : null
}

function extensionModuleForAction(action) {
  const entry = extensionEntryForAction(action)
  if (entry?.extension) return entry.extension
  if (!action?.extensionFile) return null
  return Array.from(extensionModules.values()).find((extension) => path.basename(extension.__filePath || '') === action.extensionFile) || null
}

function currentActionForStoredShortcut(action) {
  if (action?.kind !== 'extension-command') return action
  const entry = extensionEntryForAction(action)
  return entry ? extensionActionFromCommand(entry.extension, entry.command) : action
}

async function executeExtensionCommand(action) {
  const entry = extensionEntryForAction(action)
  if (!entry) {
    logWarn('extension.command.notFound', { extensionId: action.extensionId, commandId: action.commandId }, { source: 'host', scope: 'extension' })
    return null
  }
  try {
    const ctx = createExtensionContext(entry.extension, entry.command)
    const result = await entry.command.run(ctx)
    return executeViewActionResult(result, entry)
  } catch (error) {
    logError('extension.command.failed', error, { source: 'host', scope: 'extension', extensionId: entry.extension.id, commandId: entry.command.id })
    return extensionErrorView(entry, error)
  }
}

async function executeExtensionRootItem(action) {
  if (!action.rootAction) return { view: { type: 'preview', title: action.title || 'Extension item', content: action.subtitle || '' } }
  if (action.rootAction.type !== 'runExtensionAction') return executeViewAction(action.rootAction)
  const record = extensionActionHandlers.get(action.rootAction.handlerId)
  if (!record) return { view: { type: 'preview', title: 'Action unavailable', content: 'This extension item is no longer available.' } }
  try {
    const result = await record.handler(createExtensionContext(record.entry.extension, null), action)
    return executeViewActionResult(result, record.entry)
  } catch (error) {
    logError('extension.rootItem.failed', error, { source: 'host', scope: 'extension', extensionId: record.entry.extension.id })
    return { view: extensionErrorView(record.entry, error) }
  }
}

async function extensionRootActions() {
  const actionGroups = await Promise.all(Array.from(extensionModules.values()).map((extension) => extensionRootActionsForExtension(extension)))
  return actionGroups.flat()
}

async function extensionSearchActions(query) {
  const actionGroups = await Promise.all(Array.from(extensionModules.values()).map((extension) => extensionSearchActionsForExtension(extension, query)))
  return actionGroups.flat()
}

function rankContributionActions(actions, query) {
  return actions
    .map((action) => {
      const ranked = rankAction(action, query)
      return ranked ? { ...ranked, __ranked: true } : null
    })
    .filter(Boolean)
    .sort((a, b) => b.score - a.score || b.lastUsed - a.lastUsed || a.title.localeCompare(b.title))
    .slice(0, EXTENSION_ITEMS_PER_PROVIDER_LIMIT)
}

async function extensionSearchActionsForExtension(extension, query) {
  if (typeof extension.searchItems !== 'function') return []
  try {
    const entry = { extension, command: { id: 'search', title: extension.title || extension.id } }
    const items = await withTimeout(extension.searchItems(createExtensionContext(extension, null), query), EXTENSION_ROOT_ITEMS_TIMEOUT_MS)
    const list = Array.isArray(items) ? items : Array.isArray(items?.items) ? items.items : []
    return rankContributionActions(list.map((item) => extensionRootActionFromItem(entry, item)).filter(Boolean), query)
  } catch (error) {
    if (!String(error?.message || error).includes('Timed out')) logError('extension.searchItems.failed', error, { source: 'host', scope: 'extension', extensionId: extension.id })
    return []
  }
}

async function extensionRootActionsForExtension(extension) {
  if (typeof extension.rootItems !== 'function') return []
  const cacheKey = extension.__filePath || extension.id
  const cached = extensionRootItemsCache.get(cacheKey)
  if (cached && Date.now() - cached.updatedAt < EXTENSION_ROOT_ITEMS_TTL_MS) return cached.items
  const refresh = refreshExtensionRootActions(extension, cacheKey)
  return cached?.items || await refresh
}

function refreshExtensionRootActions(extension, cacheKey) {
  const current = extensionRootItemsRefreshes.get(cacheKey)
  if (current) return current
  const promise = (async () => {
    const entry = { extension, command: { id: 'root', title: extension.title || extension.id } }
    const items = await withTimeout(extension.rootItems(createExtensionContext(extension, null)), EXTENSION_ROOT_ITEMS_TIMEOUT_MS)
    const list = Array.isArray(items) ? items : Array.isArray(items?.items) ? items.items : []
    const actions = rankContributionActions(list.map((item) => extensionRootActionFromItem(entry, item)).filter(Boolean), '')
    extensionRootItemsCache.set(cacheKey, { updatedAt: Date.now(), items: actions })
    return actions
  })().catch((error) => {
    if (!String(error?.message || error).includes('Timed out')) logError('extension.rootItems.failed', error, { source: 'host', scope: 'extension', extensionId: extension.id })
    return extensionRootItemsCache.get(cacheKey)?.items || []
  }).finally(() => {
    extensionRootItemsRefreshes.delete(cacheKey)
  })
  extensionRootItemsRefreshes.set(cacheKey, promise)
  return promise
}

function withTimeout(promise, timeoutMs) {
  return Promise.race([
    Promise.resolve(promise),
    new Promise((_, reject) => setTimeout(() => reject(new Error(`Timed out after ${timeoutMs}ms`)), timeoutMs)),
  ])
}

function normalizeItemAppearance(appearance) {
  const foreground = appearance?.foreground
  if (!ITEM_FOREGROUND_COLORS.has(foreground)) return undefined
  return { foreground }
}

function extensionRootActionFromItem(entry, item) {
  if (!item?.id || !item.title) return null
  const primaryAction = normalizeViewAction(item.primaryAction || item.action, entry)
  const actionPanel = normalizeActionPanel(item.actionPanel, item.actions || [], entry)
  return {
    id: `extension-root:${entry.extension.id}:${item.id}`,
    kind: 'extension-root-item',
    extensionId: entry.extension.id,
    extensionFile: entry.extension.__filePath ? path.basename(entry.extension.__filePath) : undefined,
    rootAction: primaryAction,
    removable: Boolean(entry.extension.__generated),
    title: item.title,
    subtitle: item.subtitle || entry.extension.title || 'Extension item',
    aliases: item.aliases || item.keywords || [],
    icon: item.icon || 'sparkles',
    iconUrl: item.image || item.iconUrl || null,
    thumbnailUrl: item.thumbnailUrl || null,
    score: Math.min(Number(item.score || 35), 90),
    lastUsed: Number(item.lastUsed || 0),
    dismissAfterRun: item.dismissAfterRun || primaryAction?.dismissAfterRun,
    customizable: Boolean(item.customizable),
    actionPanel,
    appearance: normalizeItemAppearance(item.appearance),
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
  const extensionFile = entry.extension.__filePath ? path.basename(entry.extension.__filePath) : null
  if (!extensionFile) return null
  const prompt = `This generated action failed. Please fix the extension.\n\nAction: ${entry.command.title || entry.command.id}\nExtension: ${entry.extension.title || entry.extension.id}\nFile: ${extensionFile}\n\nError:\n\`\`\`\n${message}\n\`\`\``
  return {
    type: 'runExtensionAction',
    title: 'Fix with AI',
    __handler: async () => aiChatView(getOrCreateExtensionChat(extensionFile, entry.extension.title || entry.command.title), { initialPrompt: prompt }),
  }
}

const VIEW_TYPES = new Set(['list', 'grid', 'preview', 'chat', 'form', 'progress', 'webview', 'camera'])

function isView(value) {
  return Boolean(value?.type && VIEW_TYPES.has(value.type))
}

function normalizeExtensionView(result, entry) {
  if (!result) return null
  const view = isView(result) ? result : isView(result.view) ? result.view : null
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
    refresh: view.refresh ? { ...view.refresh, action: normalizeViewAction(view.refresh.action, entry) } : view.refresh,
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
      appearance: normalizeItemAppearance(item.appearance),
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

function runShellCommand(command, args = [], options: any = {}) {
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

function runShellScript(script, options: any = {}) {
  return runShellCommand(options.shell || '/bin/bash', ['-lc', String(script)], { ...options, shell: false })
}

function isAction(value) {
  return Boolean(value?.type && !isView(value))
}

function normalizeViewPatch(patch, entry) {
  if (!patch) return patch
  return {
    ...patch,
    items: normalizeViewItems(patch.items, entry),
  }
}

async function executeViewActionResult(result, entry) {
  if (!result) return result
  if (isAction(result)) return executeViewAction(normalizeViewAction(result, entry))
  if (isAction(result.action)) return executeViewAction(normalizeViewAction(result.action, entry))
  const view = normalizeExtensionView(result, entry)
  return view ? { view, navigation: result?.navigation || 'push', toast: result?.toast, patch: normalizeViewPatch(result?.patch, entry) } : { ...result, patch: normalizeViewPatch(result?.patch, entry) }
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

function runQuitCleanup() {
  if (didRunQuitCleanup) return
  didRunQuitCleanup = true
  if (app.isReady()) globalShortcut.unregisterAll()
  updateManager.clearTimers()
  for (const watcher of appWatchers) watcher.close()
}

function requestQuitApp(reason = 'action') {
  nevermindApp.isQuiting = true
  logInfo('app.quit.requested', { reason }, { source: 'host', scope: 'app' })
  app.quit()
  setTimeout(() => {
    logWarn('app.quit.fallbackExit', { reason }, { source: 'host', scope: 'app' })
    runQuitCleanup()
    app.exit(0)
  }, 250).unref?.()
}

async function executeViewAction(action) {
  switch (action?.type) {
    case 'nativeAction':
      return executeAction(action.nativeAction, { keepPaletteOpen: true })
    case 'openPath':
      runInBackground(() => shell.openPath(action.path))
      break
    case 'revealPath':
      shell.showItemInFolder(action.path)
      break
    case 'quickLook':
      return quickLookPath(action.path)
    case 'openWith':
      runInBackground(() => openPathWithApp(action.path, action.appPath || action.app?.path))
      break
    case 'openUrl':
      runInBackground(() => shell.openExternal(action.url))
      break
    case 'copyText':
      clipboard.writeText(action.text || '')
      break
    case 'pasteText':
      clipboard.writeText(action.text || '')
      pasteIntoFrontmostApp()
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
    case 'lockScreen':
      runInBackground(() => executeSystemBuiltin({ builtin: 'lock-screen' }, () => {}))
      break
    case 'sleepSystem':
      runInBackground(() => executeSystemBuiltin({ builtin: 'sleep' }, () => {}))
      break
    case 'restartSystem':
      runInBackground(() => executeSystemBuiltin({ builtin: 'restart' }, () => {}))
      break
    case 'openSystemSettings':
      runInBackground(() => executeSystemBuiltin({ builtin: 'settings' }, () => {}))
      break
    case 'openKeyboardSettings':
      runInBackground(openSystemKeyboardSettings)
      break
    case 'quitApp':
      requestQuitApp('view-action')
      break
    case 'checkForUpdates':
      return checkForUpdatesView()
    case 'downloadUpdate':
      return downloadUpdateView()
    case 'installUpdate':
      return installDownloadedUpdate()
    case 'toggleSetting': {
      const definition = settingDefinition(action.settingId)
      if (!definition || definition.type !== 'boolean') return { toast: { message: 'Setting not found', tone: 'error' } }
      setSetting(definition.id, toggledSettingValue(definition, getSetting(definition.id)))
      return { patch: { items: [settingItemPatch(definition)] } }
    }
    case 'setActionShortcut': {
      const result = await setShortcut(action.targetAction || action.action, action.accelerator || action.shortcut)
      return { toast: { message: result.message, tone: result.ok ? 'default' : 'error' }, ok: result.ok }
    }
    case 'removeShortcut': {
      const result = await removeShortcut(action.actionId)
      if (!result.ok) return { toast: { message: result.message, tone: 'error' } }
      return { patch: { removeItemIds: [`shortcut:${action.actionId}`] }, toast: { message: result.message }, ok: true }
    }
    case 'setActionAlias': {
      const result = await setAlias(action.targetAction || action.action, action.alias)
      return { toast: { message: result.message, tone: result.ok ? 'default' : 'error' }, ok: result.ok }
    }
    case 'removeActionAlias': {
      const result = await removeAlias(action.targetAction || action.action, action.alias)
      return { toast: { message: result.message, tone: result.ok ? 'default' : 'error' }, ok: result.ok }
    }
    case 'duplicateCreatedAction': {
      const result = await duplicateCreatedAction(action.targetAction || action.action)
      if (!result.ok) return { toast: { message: result.message, tone: 'error' } }
      return { toast: { message: result.message }, action: result.action }
    }
    case 'removeCreatedAction': {
      const result = await removeCreatedAction(action.targetAction || action.action)
      return { toast: { message: result.message, tone: result.ok ? 'default' : 'error' }, ok: result.ok }
    }
    case 'clearActionOverride': {
      const result = await clearOverride(action.targetAction || action.action)
      return { toast: { message: result.message, tone: result.ok ? 'default' : 'error' }, ok: result.ok }
    }
    case 'recordShortcut':
      return { toast: { message: 'Shortcut recording is handled by the palette' } }
    case 'runExtensionAction': {
      const record = extensionActionHandlers.get(action.handlerId)
      if (!record) return { toast: { message: 'Action is no longer available', tone: 'error' } }
      try {
        const result = await record.handler(createExtensionContext(record.entry.extension, record.entry.command), action)
        return executeViewActionResult(result, record.entry)
      } catch (error) {
        logError('extension.action.failed', error, { source: 'host', scope: 'extension', extensionId: record.entry.extension.id, commandId: record.entry.command.id })
        return { view: extensionErrorView(record.entry, error), navigation: 'push' }
      }
    }
    default:
      throw new Error(`Unsupported action type: ${String(action?.type || 'unknown')}`)
  }
}

async function executeBuiltin(action) {
  return executeSystemBuiltin(action, () => requestQuitApp('builtin'))
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
      logError('thumbnail.create.failed', { requestPath, error }, { source: 'host', scope: 'thumbnail' })
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
    dateAdded: null,
    dateAddedMs: 0,
    size: stat?.size || 0,
  }
}

function normalizeFindRoots(roots) {
  if (Array.isArray(roots)) return roots
  if (typeof roots === 'string') return [roots]
  return []
}

function extensionsForFindOptions(options: any = {}) {
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

function sortFoundFiles(files, options: any = {}) {
  const sortBy = options.sortBy || options.sort || null
  if (!sortBy) return files
  const direction = options.order === 'asc' ? 1 : -1
  const field = sortBy === 'recent' || sortBy === 'modified' ? 'mtimeMs'
    : sortBy === 'added' ? 'dateAddedMs'
      : sortBy === 'created' ? 'birthtimeMs'
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

async function enrichDateAdded(files) {
  const dates = await fileDateAddedMs(files.map((file) => file.path))
  return files.map((file) => {
    const addedMs = dates.get(file.path) || file.birthtimeMs || 0
    return { ...file, dateAdded: addedMs ? new Date(addedMs).toISOString() : null, dateAddedMs: addedMs }
  })
}

async function findFiles(roots, options: any = {}) {
  const limit = options.limit || 100
  const maxDepth = options.depth ?? 2
  const extensions = extensionsForFindOptions(options)
  let found = []

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
  if ((options.sortBy || options.sort) === 'added') found = await enrichDateAdded(found)
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
  if (!hasCapability('quick-look')) return { toast: { message: `${quickLookTitle()} is not available on this OS`, tone: 'error' } }
  const resolvedPath = expandUserPath(filePath)
  if (!resolvedPath || !path.isAbsolute(resolvedPath)) return { toast: { message: `Cannot ${quickLookTitle()} this item`, tone: 'error' } }
  const child = spawn('qlmanage', ['-p', resolvedPath], { detached: true, stdio: 'ignore' })
  child.unref()
}

function execFileText(command, args, options = {}) {
  return new Promise((resolve, reject) => {
    execFile(command, args, options, (error, stdout) => error ? reject(error) : resolve(stdout))
  })
}

async function contentTypesForPath(filePath) {
  if (!hasCapability('quick-look')) return []
  try {
    const stdout = await execFileText('mdls', ['-raw', '-name', 'kMDItemContentTypeTree', filePath]) as string
    return stdout.match(/"([^"]+)"/g)?.map((item) => item.slice(1, -1)) || []
  } catch {
    return []
  }
}

async function documentTypesForApp(appPath) {
  try {
    const stdout = await execFileText('/usr/bin/plutil', ['-convert', 'json', '-o', '-', path.join(appPath, 'Contents', 'Info.plist')]) as string
    return JSON.parse(stdout).CFBundleDocumentTypes || []
  } catch {
    return []
  }
}

async function openWithApps(filePath) {
  const resolvedPath = expandUserPath(filePath)
  if (!resolvedPath || !path.isAbsolute(resolvedPath)) return []
  if (!hasCapability('open-with')) return appIndex
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
  if (hasCapability('open-with')) spawn('open', ['-a', resolvedAppPath, resolvedPath], { detached: true, stdio: 'ignore' }).unref()
  else await shell.openPath(resolvedPath)
}

async function selectedFiles() {
  const paths = await selectedFilePaths()
  return Promise.all(paths.map(fileToExtensionFile))
}

async function clipboardFiles() {
  return Promise.all(clipboardFilePaths(clipboard).map(fileToExtensionFile))
}

function clipboardImageDataUrl() {
  const image = clipboard.readImage()
  return image.isEmpty() ? null : image.toDataURL()
}

async function readDesktopClipboard() {
  const files = await clipboardFiles()
  if (files.length) return { type: 'files', files }
  const image = clipboardImageDataUrl()
  if (image) return { type: 'image', image }
  const text = clipboard.readText()
  return text ? { type: 'text', text } : { type: 'empty' }
}

function writeDesktopClipboard(item) {
  if (typeof item === 'string') return clipboard.writeText(item)
  if (item?.type === 'text' || item?.text != null) return clipboard.writeText(String(item.text || ''))
  const image = item?.image || item?.imageDataUrl || item?.path
  if (item?.type === 'image' || image) {
    return String(image || '').startsWith('data:') ? clipboard.writeImage(nativeImage.createFromDataURL(image)) : clipboard.writeImage(nativeImage.createFromPath(expandUserPath(image)))
  }
}

async function readDesktopSelection() {
  const [text, files, app] = await Promise.all([selectedText(), selectedFiles(), frontmostApp()])
  return { text, files, sourceApp: app }
}

function extensionSourceBasename(filePath) {
  const base = path.basename(filePath || '')
  if (base.endsWith('.d.ts')) return base.slice(0, -5)
  return base.replace(/\.(cjs|ts)$/i, '')
}

function isExtensionSourceFile(entry) {
  return entry.isFile() && entry.name.endsWith('.ts') && !entry.name.endsWith('.d.ts')
}

function safeExtensionStorageKey(extension) {
  const key = extension.__filePath ? extensionSourceBasename(extension.__filePath) : extension.id || 'extension'
  return String(key).replace(/[^a-zA-Z0-9._-]/g, '-')
}

function extensionStoragePath(extension) {
  return path.join(extensionStorageDir, `${safeExtensionStorageKey(extension)}.json`)
}

function extensionCachePath(extension) {
  return path.join(extensionCacheDir, `${safeExtensionStorageKey(extension)}.json`)
}

async function readJsonFile(filePath) {
  try {
    return JSON.parse(await fs.readFile(filePath, 'utf8'))
  } catch {
    return {}
  }
}

async function writeJsonFile(filePath, data) {
  await fs.mkdir(path.dirname(filePath), { recursive: true })
  await fs.writeFile(filePath, JSON.stringify(data, null, 2))
}

async function readExtensionStorage(extension) {
  return readJsonFile(extensionStoragePath(extension))
}

async function writeExtensionStorage(extension, data) {
  await writeJsonFile(extensionStoragePath(extension), data)
}

async function readExtensionCache(extension) {
  return readJsonFile(extensionCachePath(extension))
}

async function writeExtensionCache(extension, data) {
  await writeJsonFile(extensionCachePath(extension), data)
}

function extensionStorageRefreshKey(extension, key) {
  return `${extensionCachePath(extension)}:${String(key)}`
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
      const data = await readExtensionCache(extension)
      const cached = data[key]
      if (cached && typeof cached === 'object' && Date.now() - Number(cached.updatedAt || 0) < Number(ttlMs || 0)) return cached.value
      const value = await loader()
      data[key] = { value, updatedAt: Date.now() }
      await writeExtensionCache(extension, data)
      return value
    },
    async memoStale(key, ttlMs, staleTtlMs, loader) {
      const data = await readExtensionCache(extension)
      const cached = data[key]
      const age = cached && typeof cached === 'object' ? Date.now() - Number(cached.updatedAt || 0) : Infinity
      if (age < Number(ttlMs || 0)) return cached.value
      const refreshKey = extensionStorageRefreshKey(extension, key)
      const refresh = extensionStorageRefreshes.get(refreshKey) || (async () => {
        const value = await loader()
        const latest = await readExtensionCache(extension)
        latest[key] = { value, updatedAt: Date.now() }
        await writeExtensionCache(extension, latest)
        return value
      })().finally(() => extensionStorageRefreshes.delete(refreshKey))
      extensionStorageRefreshes.set(refreshKey, refresh)
      if (cached && age < Number(staleTtlMs || 0)) return cached.value
      return refresh
    },
  }
}

function createExtensionAi(extension) {
  const extensionKey = path.basename(extension.__filePath || extension.id || 'extension').replace(/[^a-zA-Z0-9._-]/g, '-')
  const enforceAiQuota = () => {
    if (!checkAiRateLimit(extension)) throw Object.assign(new Error('AI rate limit exceeded'), { code: 'ai-rate-limit-exceeded', extensionId: extension?.id })
  }
  return {
    ask: (prompt, options: any = {}) => {
      enforceAiQuota()
      return nevermindAi.ask(String(prompt || ''), { system: options.system })
    },
    session: (id = 'default', options: any = {}) => {
      const session = nevermindAi.session(`${extensionKey}:${String(id || 'default')}`, { system: options.system })
      return {
        ...session,
        ask: (prompt: any) => {
          enforceAiQuota()
          return session.ask(prompt)
        },
      }
    },
  }
}

function systemItems(ctx) {
  const system = ctx.actions.system
  return [
    { id: 'builtin:lock-screen', title: 'Lock Screen', subtitle: 'Secure this computer', icon: 'lock', score: 22, dismissAfterRun: 'auto', primaryAction: system.lockScreen('Lock Screen') },
    { id: 'builtin:sleep', title: 'Sleep', subtitle: 'Put this computer to sleep', icon: 'moon', score: 21, dismissAfterRun: 'auto', primaryAction: system.sleep('Sleep') },
    { id: 'builtin:restart', title: 'Restart Computer', subtitle: 'Restart this computer', icon: 'restart', score: 20, dismissAfterRun: 'auto', primaryAction: system.restart('Restart Computer') },
    { id: 'builtin:settings', title: settingsTitle(), subtitle: 'Open system preferences', icon: 'settings', score: 19, dismissAfterRun: 'auto', primaryAction: system.openSystemSettings(settingsTitle()) },
    { id: 'builtin:quit', title: 'Quit Nevermind', subtitle: 'Close the app', icon: 'power', score: 15, dismissAfterRun: 'auto', primaryAction: system.quit('Quit Nevermind') },
  ]
}

function placesItems() {
  return [
    { id: 'places:downloads', title: 'Open Downloads', subtitle: '~/Downloads', icon: 'folder', score: 18, dismissAfterRun: 'auto', primaryAction: { type: 'openPath', title: 'Open Downloads', path: path.join(os.homedir(), 'Downloads'), dismissAfterRun: 'auto' } },
    { id: 'places:documents', title: 'Open Documents', subtitle: '~/Documents', icon: 'folder', score: 17, dismissAfterRun: 'auto', primaryAction: { type: 'openPath', title: 'Open Documents', path: path.join(os.homedir(), 'Documents'), dismissAfterRun: 'auto' } },
    { id: 'places:desktop', title: 'Open Desktop', subtitle: '~/Desktop', icon: 'folder', score: 16, dismissAfterRun: 'auto', primaryAction: { type: 'openPath', title: 'Open Desktop', path: path.join(os.homedir(), 'Desktop'), dismissAfterRun: 'auto' } },
  ]
}

function commandFromItem(item) {
  return { ...item, run: (ctx) => ctx.navigation.run(item.primaryAction) }
}

function createSystemExtension() {
  const extension = { id: 'nevermind.system', title: 'System', permissions: ['system'] as const }
  const commands = systemItems(createExtensionContext(extension, null)).map(commandFromItem)
  return {
    ...extension,
    commands,
    rootItems: (ctx) => systemItems(ctx),
  }
}

function createPlacesExtension() {
  return {
    id: 'nevermind.places',
    title: 'Places',
    permissions: ['places'] as const,
    commands: placesItems().map(commandFromItem),
    rootItems: () => placesItems(),
  }
}

function createCalculatorExtension() {
  return {
    id: 'nevermind.calculator',
    title: 'Calculator',
    permissions: [] as const,
    commands: [],
    searchItems(_ctx, query) {
      const result = query ? calculate(query) : null
      if (result === null) return []
      return [{ id: `calculate:${query}`, title: `${query} = ${result}`, subtitle: 'Copy result to clipboard', icon: 'calculator', score: 105, dismissAfterRun: 'auto', primaryAction: { type: 'copyText', title: 'Copy Result', text: String(result), dismissAfterRun: 'auto' } }]
    },
  }
}

function createWebSearchExtension() {
  return {
    id: 'nevermind.web',
    title: 'Web',
    permissions: [] as const,
    commands: [],
    searchItems(_ctx, query) {
      const q = String(query || '').trim()
      if (!q) return []
      const url = getUrlFromQuery(q)
      if (url) {
        return [{ id: `open-url:${url}`, title: `Open ${url.replace(/^https?:\/\//, '')}`, subtitle: 'Open website', icon: 'globe', score: 100, dismissAfterRun: 'auto', primaryAction: { type: 'openUrl', title: 'Open Website', url, dismissAfterRun: 'auto' } }]
      }
      return [{ id: `web-search:${q}`, title: `Search the web for "${q}"`, subtitle: 'Search instead', icon: 'search', score: 10 + usageBoost(`web-search:${q}`) + recentBoost(`web-search:${q}`), dismissAfterRun: 'auto', primaryAction: { type: 'openUrl', title: 'Search the Web', url: `https://www.google.com/search?q=${encodeURIComponent(q)}`, dismissAfterRun: 'auto' } }]
    },
  }
}

function appRootItem(item) {
  const id = `app:${item.id}`
  return { id, title: item.name, subtitle: 'Launch application', aliases: actionAliases(`extension-root:nevermind.apps:${id}`), icon: 'app', image: undefined as string | undefined, score: 30, dismissAfterRun: 'auto', customizable: true, primaryAction: { type: 'openPath', title: `Open ${item.name}`, path: item.path, dismissAfterRun: 'auto' } }
}

async function attachAppIcons(items, iconFor = getAppIconDataUrl) {
  await Promise.all(items.map(async (item) => {
    const iconUrl = await iconFor(item.primaryAction?.path)
    if (iconUrl) item.image = iconUrl
  }))
}

function fileRootItem(item) {
  return { id: `file:${item.path}`, title: item.name, subtitle: item.displayPath, icon: 'folder', score: 4, dismissAfterRun: 'auto', primaryAction: { type: 'openPath', title: `Open ${item.name}`, path: item.path, dismissAfterRun: 'auto' } }
}

function createClipboardExtension() {
  function historyItem() {
    const latestClipboardTime = clipboardHistory[0]?.createdAt || 0
    return {
      id: 'clipboard-history',
      title: 'Clipboard History',
      subtitle: clipboardHistory.length ? `Show all ${clipboardHistory.length} copied items` : 'Show copied items',
      icon: 'clipboard',
      score: 14,
      lastUsed: latestClipboardTime ? latestClipboardTime - 1 : 0,
      primaryAction: { type: 'pushView', title: 'Clipboard History', view: clipboardHistoryView() },
    }
  }

  return {
    id: 'nevermind.clipboard',
    title: 'Clipboard',
    permissions: ['clipboard.history'] as const,
    commands: [{
      id: 'clipboard-history',
      actionId: 'clipboard-history',
      title: 'Clipboard History',
      subtitle: 'Show copied items',
      icon: 'clipboard',
      score: 14,
      run: () => clipboardHistoryView(),
    }],
    rootItems(ctx) {
      if (!getSetting('showClipboardInRoot')) return []
      return ctx.clipboard.history.list({ limit: 10 }).map(clipboardRootItem)
    },
    searchItems(ctx, query) {
      return ctx.clipboard.history.list().map(clipboardRootItem).filter((item) => rankAction(item, query)).slice(0, 5)
    },
  }
}

function createAppsExtension() {
  return {
    id: 'nevermind.apps',
    title: 'Applications',
    permissions: ['desktop.apps'] as const,
    commands: [],
    async rootItems(ctx) {
      const items = ctx.desktop.apps.list().map(appRootItem)
      await attachAppIcons(items, ctx.desktop.apps.icon)
      return items
    },
    async searchItems(ctx, query) {
      const matches = ctx.desktop.apps.list().map(appRootItem).filter((item) => rankAction(item, query))
      await attachAppIcons(matches.slice(0, EXTENSION_ITEMS_PER_PROVIDER_LIMIT), ctx.desktop.apps.icon)
      return matches
    },
  }
}

function createFilesExtension() {
  return {
    id: 'nevermind.files',
    title: 'Files',
    permissions: ['desktop.files'] as const,
    commands: [],
    rootItems() {
      return fileIndex.slice(0, FILE_RESULT_LIMIT).map(fileRootItem)
    },
    searchItems(_ctx, query) {
      return fileIndex.map(fileRootItem).filter((item) => rankAction(item, query)).slice(0, FILE_RESULT_LIMIT)
    },
  }
}

function createAiBuilderExtension() {
  function chatsSubtitle() {
    const count = Object.keys(userState.aiChats || {}).length
    return `${count} builder ${count === 1 ? 'chat' : 'chats'}`
  }
  function chatItems(ctx, query = '') {
    return Object.values(userState.aiChats || {}).map((item: any) => ({
      id: `ai-chat:${item.id}`,
      title: item.title || item.query,
      subtitle: item.status === 'ready' ? 'AI builder chat' : 'Continue AI builder chat',
      aliases: [item.query],
      icon: 'sparkles',
      score: 13,
      lastUsed: Math.max(item.updatedAt || 0, item.createdAt || 0),
      primaryAction: ctx.aiBuilder.openChat(item.id),
      appearance: { foreground: 'yellow' },
    })).filter((item) => !query || rankAction(item, query))
  }
  return {
    id: AI_BUILDER_EXTENSION_ID,
    title: 'AI Builder',
    permissions: ['ai', 'extensions.ownership'] as const,
    commands: [{ id: 'ai-chats', actionId: 'ai-chats', title: 'AI Chats', get subtitle() { return chatsSubtitle() }, icon: 'sparkles', score: 16, run: () => aiChatsView() }],
    rootItems(ctx) {
      return chatItems(ctx).slice(0, 4)
    },
    searchItems(ctx, query) {
      const q = String(query || '').trim()
      const items: any[] = chatItems(ctx, q)
      if (q && !getUrlFromQuery(q) && calculate(q) === null) items.push({ id: `ai:${q}`, title: `Press Tab to automate "${q}"`, subtitle: 'Automate with AI', query: q, icon: 'bolt', score: 40, primaryAction: ctx.aiBuilder.startChat({ prompt: q, title: `Automate "${q}"` }) })
      return items.filter((item) => rankAction(item, q)).slice(0, 5)
    },
  }
}

function createUpdatesExtension() {
  const extension = { id: 'nevermind.updates', title: 'Updates', permissions: ['updates'] as const }
  const ctx: any = createExtensionContext(extension, null)
  const checkItem = () => ({ id: 'updates:check', title: 'Check for Updates', subtitle: `Current version: ${app.getVersion()}`, icon: 'restart', score: 23, primaryAction: ctx.actions.updates.check('Check for Updates') })
  return {
    ...extension,
    commands: [{ ...checkItem(), run: () => checkForUpdatesView() }],
    rootItems() {
      return [updatePromptAction() || checkItem()]
    },
  }
}

function keyboardShortcutItem(record: any) {
  const changeAction = buildRecordShortcutAction({ actionId: record.actionId, action: record.action, title: 'Change shortcut' }, {})
  const removeAction = record.source === 'user' ? buildRemoveShortcutAction({ actionId: record.actionId, title: 'Remove shortcut' }, {}) : null
  return {
    id: `shortcut:${record.actionId}`,
    title: record.action.title,
    subtitle: record.accelerator,
    icon: 'keyboard',
    primaryAction: changeAction,
    actionPanel: { sections: [{ actions: [changeAction, removeAction].filter(Boolean) }] },
  }
}

function keyboardShortcutItems() {
  return getShortcuts().map(keyboardShortcutItem)
}

function keyboardShortcutsView() {
  return {
    type: 'list',
    id: 'keyboard-shortcuts',
    title: 'Keyboard Shortcuts',
    presentation: 'root',
    searchBarPlaceholder: 'Search Keyboard Shortcuts',
    emptyView: { title: 'No shortcuts found.' },
    items: keyboardShortcutItems(),
  }
}

function patchKeyboardShortcutsView() {
  patchOpenView('keyboard-shortcuts', { mode: 'replace', items: keyboardShortcutItems() })
}

function createKeyboardShortcutsExtension() {
  return {
    id: 'nevermind.shortcuts',
    title: 'Keyboard Shortcuts',
    permissions: ['shortcuts'] as const,
    commands: [{ id: 'keyboard-shortcuts', actionId: 'keyboard-shortcuts', title: 'Keyboard Shortcuts', subtitle: 'View, change, or remove global shortcuts', icon: 'keyboard', score: 16, run: () => keyboardShortcutsView() }],
  }
}

function createSettingsExtension() {
  return {
    id: 'nevermind.settings',
    title: 'Settings',
    permissions: ['settings.write'] as const,
    commands: [{ id: 'app-settings', actionId: 'app-settings', title: 'Settings', subtitle: 'Configure Nevermind', icon: 'settings', score: 16, run: (ctx) => ctx.ui.list({
      type: 'list',
      id: 'app-settings',
      title: 'Settings',
      presentation: 'root',
      selectedItemId: ctx.state.selectedItemId,
      searchBarPlaceholder: 'Search Settings',
      items: settingsItems(),
    }) }],
  }
}

const PALETTE_HOTKEY_ACTION_ID = '__palette-hotkey__'

function resolveShortcutTargetAction(input: any) {
  if (input?.action) return input.action
  const actionId = input?.actionId
  if (!actionId) return null
  if (actionId === PALETTE_HOTKEY_ACTION_ID || input?.scope === 'palette') return { id: PALETTE_HOTKEY_ACTION_ID }
  const record = getShortcuts().find((item) => item.actionId === actionId)
  return record?.action || { id: actionId }
}

function buildRecordShortcutAction(input: any, options: any) {
  const scope = input?.scope === 'palette' || input?.actionId === PALETTE_HOTKEY_ACTION_ID ? 'palette' : 'action'
  const targetAction = scope === 'palette' ? { id: PALETTE_HOTKEY_ACTION_ID } : resolveShortcutTargetAction(input)
  const title = input?.title || options?.title || (scope === 'palette' ? 'Change Shortcut' : 'Record shortcut')
  const shortcut = input?.shortcut !== undefined ? String(input.shortcut) : undefined
  return { ...options, type: 'recordShortcut', title, action: targetAction, ...(shortcut !== undefined ? { shortcut } : {}) }
}

function buildRemoveShortcutAction(input: any, options: any) {
  const actionId = input?.actionId || input?.action?.id
  const title = input?.title || options?.title || 'Remove shortcut'
  return { style: 'destructive', ...options, type: 'removeShortcut', title, actionId }
}

function wrapWithConfirmation(action: any, input: any) {
  if (!action) return action
  const destructive = input?.destructive ?? (action.style === 'destructive')
  return {
    ...action,
    title: input?.title || action.title,
    requiresConfirmation: true,
    ...(destructive ? { style: 'destructive' } : {}),
    ...(input?.message !== undefined ? { confirmMessage: String(input.message) } : {}),
    ...(input?.confirmLabel !== undefined ? { confirmLabel: String(input.confirmLabel) } : {}),
    ...(input?.cancelLabel !== undefined ? { cancelLabel: String(input.cancelLabel) } : {}),
  }
}

function buildConfirmAction(input: any) {
  const inner = input?.onConfirm || input?.action
  if (!inner) throw new Error('ctx.ui.confirm requires onConfirm action')
  return wrapWithConfirmation(inner, input)
}

function buildPreviewItemAction(input: any) {
  const kind = input?.kind || (input?.text ? 'text' : input?.imageDataUrl || input?.imagePath ? 'image' : input?.videoUrl ? 'video' : input?.filePath ? 'file' : 'clipboard')
  return {
    type: 'previewClipboardItem',
    title: input?.title || 'Preview',
    shortcut: input?.shortcut || 'Command+Y',
    clipboardType: input?.clipboardType || kind,
    text: input?.text,
    imageDataUrl: input?.imageDataUrl,
    imagePath: input?.imagePath,
    videoUrl: input?.videoUrl,
    filePath: input?.filePath,
    thumbnailUrl: input?.thumbnailUrl,
  }
}

function progressView(input: any = {}) {
  const hasSteps = Array.isArray(input.steps) && input.steps.length
  const steps = hasSteps ? input.steps : [{ title: input.label || input.title || 'Loading…', status: input.status || 'active' }]
  return {
    ...input,
    type: 'progress',
    title: input.title || input.label || 'Loading…',
    steps,
    ...(input.id !== undefined ? { id: String(input.id) } : {}),
    ...(typeof input.value === 'number' ? { value: input.value } : {}),
    ...(typeof input.total === 'number' ? { total: input.total } : {}),
  }
}

function createExtensionContext(extension, command) {
  const canUseDesktopApps = hasExtensionPermission(extension, 'desktop.apps')
  const canUseDesktopFiles = hasExtensionPermission(extension, 'desktop.files')
  const canUseClipboard = hasExtensionPermission(extension, 'clipboard.history')
  const canUseSystem = hasExtensionPermission(extension, 'system')
  const canUseUpdates = hasExtensionPermission(extension, 'updates')
  const canUseShortcuts = hasExtensionPermission(extension, 'shortcuts')
  const canUseAi = hasExtensionPermission(extension, 'ai')
  const canWriteSettings = hasExtensionPermission(extension, 'settings.write')
  const denyShortcut = (name: string) => () => { throw permissionDeniedError(`shortcuts (${name})`) }
  return {
    extension: createExtensionRuntimeMetadata(extension, command),
    command,
    ui: {
      list: (view) => ({ ...view, type: 'list' }),
      grid: (view) => ({ ...view, type: 'grid' }),
      preview: (fileOrView, view: any = {}) => {
        if (fileOrView?.kind && ['clipboard', 'image', 'video', 'file', 'text'].includes(fileOrView.kind)) return buildPreviewItemAction(fileOrView)
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
      progress: (input: any = {}) => progressView(input),
      confirm: (input: any = {}) => buildConfirmAction(input),
      toast: (input: any = {}) => ({ toast: { message: String(input?.message || ''), tone: input?.tone || 'default' } }),
      webview: (view) => ({ ...view, type: 'webview' }),
      camera: (view = {}) => ({ title: 'Camera', size: 'large', muted: true, ...view, type: 'camera' }),
      item: (item) => item,
      actions: (actions) => actions,
      empty: (title = 'Nothing here', subtitle = '') => ({ type: 'preview', title, content: `# ${title}${subtitle ? `\n\n${subtitle}` : ''}` }),
      loading: (title = 'Loading…') => progressView({ title, label: title }),
      error: (title = 'Something went wrong', message = '') => ({ type: 'preview', title, content: `# ${title}${message ? `\n\n${message}` : ''}` }),
    },
    actions: {
      openPath: (filePath, title = 'Open', options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'openPath', title, path: filePath }),
      revealPath: (filePath, title = revealPathTitle(), options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'revealPath', title, path: filePath }),
      quickLook: (filePath, title = quickLookTitle(), options: any = {}) => ({ ...options, type: 'quickLook', title, path: filePath }),
      openWith: (filePath, app, title, options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'openWith', title: title || `Open with ${app?.name || 'App'}`, path: filePath, app, appPath: app?.path || app }),
      openUrl: (url, title = 'Open URL', options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'openUrl', title, url }),
      copyText: (text, title = 'Copy', options: any = {}) => ({ ...options, type: 'copyText', title, text }),
      pasteText: (text, title = 'Paste', options: any = {}) => ({ ...options, type: 'pasteText', title, text }),
      copyImage: (image, title = 'Copy image', options: any = {}) => String(image || '').startsWith('data:') ? ({ ...options, type: 'copyImage', title, imageDataUrl: image }) : ({ ...options, type: 'copyImage', title, path: image }),
      trash: (paths, title = 'Move to Trash', options: any = {}) => ({ ...options, type: 'trash', title, paths: Array.isArray(paths) ? paths : [paths], style: options.style || 'destructive', requiresConfirmation: options.requiresConfirmation ?? true }),
      push: (title, view, options: any = {}) => ({ ...options, type: 'pushView', title, view }),
      replace: (title, view, options: any = {}) => ({ ...options, type: 'replaceView', title, view }),
      pop: (title = 'Back', options: any = {}) => ({ ...options, type: 'popView', title }),
      run: (title, handler, options: any = {}) => ({ ...options, type: 'runExtensionAction', title, __handler: handler }),
      background: (title, handler, options: any = {}) => ({ ...options, type: 'runExtensionAction', title, __handler: handler, dismissAfterRun: options.dismissAfterRun || 'auto' }),
      shellExec: (title, command, args = [], options: any = {}) => ({ ...options, type: 'shellExec', title, command, args, options, requiresConfirmation: options.requiresConfirmation ?? true }),
      shellScript: (title, script, options: any = {}) => ({ ...options, type: 'shellScript', title, script, options, requiresConfirmation: options.requiresConfirmation ?? true }),
      toggleSetting: canWriteSettings
        ? (settingId, title = 'Toggle', options: any = {}) => ({ ...options, type: 'toggleSetting', title, settingId })
        : denyShortcut('settings.write'),
      recordShortcut: canUseShortcuts
        ? (input: any = {}, options: any = {}) => buildRecordShortcutAction(input, options)
        : denyShortcut('recordShortcut'),
      removeShortcut: canUseShortcuts
        ? (input: any = {}, options: any = {}) => buildRemoveShortcutAction(input, options)
        : denyShortcut('removeShortcut'),
      setPaletteShortcut: canUseShortcuts
        ? (title = 'Change Shortcut', options: any = {}) => buildRecordShortcutAction({ scope: 'palette', title }, options)
        : denyShortcut('setPaletteShortcut'),
      native: (title, nativeAction, options: any = {}) => ({ ...options, type: 'nativeAction', title, nativeAction }),
      system: canUseSystem ? {
        lockScreen: (title = 'Lock Screen', options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'lockScreen', title }),
        sleep: (title = 'Sleep', options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'sleepSystem', title }),
        restart: (title = 'Restart Computer', options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'restartSystem', title }),
        openSystemSettings: (title = settingsTitle(), options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'openSystemSettings', title }),
        openKeyboardSettings: (title = 'Keyboard Settings', options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'openKeyboardSettings', title }),
        quit: (title = 'Quit Nevermind', options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'quitApp', title }),
      } : denyShortcut('system'),
      updates: canUseUpdates ? {
        check: (title = 'Check for Updates', options: any = {}) => ({ ...options, type: 'checkForUpdates', title }),
        download: (title = 'Download Update', options: any = {}) => ({ ...options, type: 'downloadUpdate', title }),
        install: (title = 'Install and Restart', options: any = {}) => ({ ...options, type: 'installUpdate', title }),
      } : denyShortcut('updates'),
      camera: {
        switchDevice: (title = 'Switch Camera', options: any = {}) => ({ ...options, type: 'nativeAction', title, nativeAction: { kind: 'camera.switchDevice' } }),
        nextDevice: (title = 'Next Camera', options: any = {}) => ({ ...options, type: 'nativeAction', title, nativeAction: { kind: 'camera.nextDevice' } }),
        previousDevice: (title = 'Previous Camera', options: any = {}) => ({ ...options, type: 'nativeAction', title, nativeAction: { kind: 'camera.previousDevice' } }),
        toggleMuted: (title = 'Toggle Camera Audio', options: any = {}) => ({ ...options, type: 'nativeAction', title, nativeAction: { kind: 'camera.toggleMuted' } }),
        toggleControls: (title = 'Toggle Camera Controls', options: any = {}) => ({ ...options, type: 'nativeAction', title, nativeAction: { kind: 'camera.toggleControls' } }),
      },
    },
    navigation: {
      push: (view) => ({ view, navigation: 'push' }),
      replace: (view) => ({ view, navigation: 'replace' }),
      pop: () => ({ navigation: 'pop' }),
      run: (action) => ({ action }),
    },
    system: {
      os: osLabel(),
      capabilities: { has: (id) => hasCapability(String(id)) },
      labels: {
        revealInFileManager: revealPathTitle(),
        previewFile: quickLookTitle(),
        openSystemSettings: settingsTitle(),
        keyboardSettings: keyboardSettingsSubtitle(),
      },
    },
    clipboard: {
      history: canUseClipboard ? {
        list: (options: any = {}) => clipboardHistorySnapshot(options),
        search: (query, options: any = {}) => clipboardHistorySnapshot({ ...options, query }),
      } : undefined,
    },
    desktop: {
      clipboard: canUseClipboard ? {
        readText: () => clipboard.readText(),
        writeText: (text) => clipboard.writeText(String(text || '')),
        readImage: clipboardImageDataUrl,
        writeImage: (image) => writeDesktopClipboard({ type: 'image', image }),
        readFiles: clipboardFiles,
        read: readDesktopClipboard,
        write: writeDesktopClipboard,
      } : undefined,
      selection: {
        text: selectedText,
        files: selectedFiles,
        read: readDesktopSelection,
      },
      apps: canUseDesktopApps ? {
        frontmost: frontmostApp,
        launch: (appPath) => runInBackground(() => shell.openPath(expandUserPath(appPath))),
        list: () => appIndex.map((entry) => ({ id: entry.id, name: entry.name, path: entry.path })),
        search: (query) => {
          const needle = String(query || '').toLowerCase()
          return appIndex
            .filter((entry) => !needle || String(entry.name || '').toLowerCase().includes(needle))
            .map((entry) => ({ id: entry.id, name: entry.name, path: entry.path }))
        },
        icon: (appPath) => getAppIconDataUrl(appPath),
      } : undefined,
      files: canUseDesktopFiles ? {
        find: findFiles,
        findImages: (roots, options) => findFiles(roots, { ...options, kind: 'image' }),
        findVideos: (roots, options) => findFiles(roots, { ...options, kind: 'video' }),
        findMedia: (roots, options) => findFiles(roots, { ...options, kind: 'media' }),
        openWithApps,
        open: (filePath) => runInBackground(() => shell.openPath(expandUserPath(filePath))),
        reveal: (filePath) => runInBackground(() => shell.showItemInFolder(expandUserPath(filePath))),
        preview: quickLookPath,
        readText: (filePath) => fs.readFile(expandUserPath(filePath), 'utf8'),
        toFileUrl: (filePath) => fileUrlForPath(expandUserPath(filePath)),
      } : undefined,
      shell: canUseSystem ? {
        openExternal: (url) => runInBackground(() => shell.openExternal(url)),
        exec: runShellCommand,
        script: runShellScript,
        appleScript: (script, options: any = {}) => new Promise((resolve) => {
          if (!hasCapability('applescript')) return resolve({ stdout: '', stderr: 'AppleScript is not available on this OS', exitCode: 1 })
          execFile('osascript', ['-e', String(script)], { timeout: Number(options.timeout || 30_000) }, (error, stdout, stderr) => resolve({ stdout: limitedOutput(stdout, options.outputLimit), stderr: limitedOutput(stderr || error?.message || '', options.outputLimit), exitCode: error ? 1 : 0 }))
        }),
        which: (command) => new Promise((resolve) => {
          execFile('/usr/bin/which', [String(command)], (error, stdout, stderr) => resolve({ stdout: stdout.trim(), stderr: stderr || error?.message || '', exitCode: error ? 1 : 0 }))
        }),
      } : undefined,
    },
    storage: createExtensionStorage(extension),
    settings: {
      definitions: () => SETTING_DEFINITIONS.map((definition) => ({ ...definition, value: getSetting(definition.id) })),
      get: (id) => getSetting(id),
      set: canWriteSettings
        ? (id, value) => setSetting(id, value)
        : () => { throw permissionDeniedError('settings.write') },
      toggle: canWriteSettings
        ? (id) => {
            const definition = settingDefinition(id)
            if (!definition) throw new Error(`Unknown setting: ${id}`)
            const next = toggledSettingValue(definition, getSetting(id))
            setSetting(id, next)
            return next
          }
        : () => { throw permissionDeniedError('settings.write') },
    },
    logs: extensionLogger(extension.id, command?.id),
    cache: createExtensionCache(extension),
    views: createExtensionViewsApi(extension, command),
    updates: canUseUpdates ? { getState: () => updatesStateSnapshot() } : undefined,
    state: {},
    ai: canUseAi ? createExtensionAi(extension) : undefined,
    aiBuilder: createAiBuilderApi(extension),
    extensions: { ownership: hasExtensionPermission(extension, 'extensions.ownership') ? createExtensionOwnershipApi(extension) : undefined },
  }
}

function assertAiBuilderPrivilege(extension) {
  if (extension?.id !== AI_BUILDER_EXTENSION_ID) {
    throw new Error('ctx.aiBuilder is only available to the built-in AI Builder extension')
  }
}

function buildAiBuilderAction(title, handler, options: any = {}) {
  return { ...options, type: 'runExtensionAction', title, __handler: handler }
}

function createAiBuilderApi(extension) {
  const privileged = extension?.id === AI_BUILDER_EXTENSION_ID
  if (!privileged) return undefined
  return {
    startChat: (input: any = {}) => {
      const prompt = String(input.prompt || input.query || '')
      return buildAiBuilderAction(input.title || `Automate "${prompt}"`, async () => {
        const item = createDraftAiChat(prompt)
        return { view: aiChatView(item, { start: item.messages.length <= 1 }) }
      }, input.options)
    },
    openChat: (chatId, input: any = {}) => buildAiBuilderAction(input.title || 'Open Chat', async () => {
      const item = userState.aiChats[chatId] || draftAiChats.get(chatId)
      if (!item) return { toast: { message: 'AI chat not found', tone: 'error' } }
      return { view: aiChatView(item) }
    }, input.options),
    removeChat: (chatId, input: any = {}) => buildAiBuilderAction(input.title || 'Remove Chat', async () => {
      return removeAiChat(chatId)
    }, { style: 'destructive', ...(input.options || {}) }),
    tweakExtension: (input: any = {}) => buildAiBuilderAction(input.title || 'Tweak with AI', async () => {
      const file = input.extensionFile || input.extensionId
      if (!file) return { toast: { message: 'No extension specified', tone: 'error' } }
      const item = getOrCreateExtensionChat(file, input.title || file)
      return { view: aiChatView(item, { initialPrompt: input.prompt }) }
    }, input.options),
    openChatsList: (input: any = {}) => buildAiBuilderAction(input.title || 'AI Chats', async () => ({ view: aiChatsView() }), input.options),
    listChats: () => Object.values(userState.aiChats || {}).map((chat: any) => ({
      id: chat.id,
      title: chat.title || chat.query,
      query: chat.query,
      status: chat.status,
      createdAt: chat.createdAt,
      updatedAt: chat.updatedAt,
      extensionFiles: chatTouchedExtensionFiles(chat),
    })),
    getChat: (chatId) => {
      const chat = userState.aiChats[chatId] || draftAiChats.get(chatId)
      if (!chat) return null
      return {
        id: chat.id,
        title: chat.title || chat.query,
        query: chat.query,
        status: chat.status,
        messages: chat.messages,
        extensionFiles: chatTouchedExtensionFiles(chat),
      }
    },
  }
}

function createExtensionOwnershipApi(extension) {
  const privileged = extension?.id === AI_BUILDER_EXTENSION_ID
  const readOnly = {
    ownerOf: (extensionFile) => aiChatIdForExtensionFile(extensionFile),
    filesForChat: (chatId) => {
      const chat = userState.aiChats[chatId] || draftAiChats.get(chatId)
      return chat ? chatTouchedExtensionFiles(chat) : []
    },
    canWrite: (extensionFile, chatId) => chatCanWriteExtension(extensionFile, chatId),
  }
  if (!privileged) return readOnly
  return {
    ...readOnly,
    claim: (extensionFile, chatId) => {
      const chat = chatId ? userState.aiChats[chatId] || draftAiChats.get(chatId) : null
      if (!chat) return false
      touchExtensionFileForChat(chat, extensionFile)
      scheduleSaveState()
      return true
    },
    reload: async () => {
      await loadExtensions()
      registerActionShortcuts()
    },
  }
}

function createExtensionViewsApi(extension, command) {
  return {
    refresh: () => ({
      type: 'runExtensionAction',
      title: 'Refresh',
      __handler: async (innerCtx) => {
        if (!command || typeof command.run !== 'function') return { toast: { message: 'View cannot refresh', tone: 'error' } }
        if (!checkRefreshBurst(extension)) return { skipped: true }
        const result = await command.run(innerCtx)
        const view = result?.type ? result : result?.view?.type ? result.view : null
        if (view?.items) return { patch: { mode: 'replace', items: view.items } }
        if (view) return { view, navigation: 'replace' }
        return result
      },
    }),
    invalidate: () => {
      invalidateExtensionRootItemsForExtension(extension)
      extensionCacheFor(extension.id).clear()
    },
  }
}

function initNevermindAi() {
  nevermindAi = createNevermindAi({
    agentDir: path.join(app.getPath('userData'), 'pi-agent'),
    workspaceDir: path.join(app.getPath('userData'), 'ai-workspace'),
    extensionsDir,
    extensionApiPath: path.join(app.getAppPath(), 'src', 'resources', EXTENSION_TYPES_FILENAME),
    extensionTypesPath: path.join(app.getAppPath(), 'src', 'resources', EXTENSION_TYPES_FILENAME),
    skillPath: path.join(app.getAppPath(), 'src', 'resources', 'skills', 'nevermind-extension-builder', 'SKILL.md'),
    reloadExtensions: loadExtensions,
    getActiveChat: () => activeAiChatId ? userState.aiChats[activeAiChatId] || draftAiChats.get(activeAiChatId) || null : null,
    getChat: (chatId) => userState.aiChats[chatId] || draftAiChats.get(chatId) || null,
    markGeneratedExtension: (filePath, chatId) => markGeneratedExtensionForActiveChat(filePath, chatId),
    canWriteExtension: (filename, chatId) => chatCanWriteExtension(filename, chatId),
    addAliasForChat: (chatId) => addAliasForGeneratedAction(chatId),
    onEvent: (event) => {
      const chatId = event.chatId || activeAiChatId
      if (chatId && event.type === 'delta' && event.text) appendAiChatDelta(chatId, event.text)
      if (chatId && event.type === 'tool_start' && event.name) appendAiChatMessage(chatId, 'system', event.name)
      if (chatId && event.type === 'error' && event.message) appendAiChatMessage(chatId, 'system', event.message)
      if (chatId && event.type === 'done' && userState.aiChats[chatId]) {
        if (userState.aiChats[chatId].status !== 'ready') userState.aiChats[chatId].status = 'done'
        userState.aiChats[chatId].updatedAt = Date.now()
        patchAiChatsItem(chatId)
        scheduleSaveState()
      }
      paletteWindow.win?.webContents.send('ai:chat:event', { ...event, chatId })
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

  const focused = chat?.contextExtensionFile ? `\n\nFocused extension file: ${chat.contextExtensionFile}. Use read_current_extension before editing it. You may list/read other extensions if needed.` : ''

  return `Use this Nevermind AI chat transcript as context. Do not ask questions that the user already answered. If the user has now provided enough details, proceed by calling read_extension_api immediately; do not merely say you will.${focused}\n\n${transcript}\n\nNew user message:\n${message}`
}

function markGeneratedExtensionForActiveChat(filePath, chatId = activeAiChatId) {
  const chat = chatId ? userState.aiChats[chatId] : null
  if (!chat) return
  touchExtensionFileForChat(chat, path.basename(filePath))
  scheduleSaveState()
}

function chatCanWriteExtension(filename, chatId = activeAiChatId) {
  const base = path.basename(filename || '')
  if (!base) return false
  const chat = chatId ? userState.aiChats[chatId] || draftAiChats.get(chatId) : null
  if (!chat) return false
  const ownedFiles = chatTouchedExtensionFiles(chat)
  if (ownedFiles.includes(base)) return true
  const owner = (Object.values(userState.aiChats || {}) as any[]).find((item) => chatTouchedExtensionFiles(item).includes(base))
  if (owner) return false
  return !Array.from(extensionModules.values()).some((extension) => path.basename(extension.__filePath || '') === base)
}

function addAliasForGeneratedAction(chatId) {
  const chat = userState.aiChats[chatId]
  if (!chat?.query) return
  const files = chatTouchedExtensionFiles(chat)
  const entry = Array.from(extensionRegistry.values()).find((e) => files.includes(path.basename(e.extension?.__filePath || '')))
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
  if (activeAiChatId?.startsWith('draft:')) promoteDraftAiChat(activeAiChatId)
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

async function ensureExtensionTypeDefinitions() {
  if (!extensionsDir) return
  const sourcePath = path.join(app.getAppPath(), 'src', 'resources', EXTENSION_TYPES_FILENAME)
  const targetPath = path.join(extensionsDir, EXTENSION_TYPES_FILENAME)
  await fs.copyFile(sourcePath, targetPath).catch((error) => {
    logWarn('extension.types.copy.failed', { error: error?.message || String(error) }, { source: 'host', scope: 'extension' })
  })
  await fs.writeFile(path.join(extensionsDir, 'package.json'), `${JSON.stringify({ type: 'module' }, null, 2)}\n`).catch((error) => {
    logWarn('extension.packageJson.write.failed', { error: error?.message || String(error) }, { source: 'host', scope: 'extension' })
  })
}

async function loadExtensionModule(fullPath) {
  const url = pathToFileURL(fullPath)
  url.searchParams.set('reload', String(Date.now()))
  const imported = await import(url.href)
  return imported.default || imported
}

async function loadExtensions() {
  extensionRegistry.clear()
  extensionModules.clear()
  extensionRootItemsCache.clear()
  extensionRootItemsRefreshes.clear()
  registerInternalExtensions()

  await fs.mkdir(extensionsDir, { recursive: true })
  await ensureExtensionTypeDefinitions()
  const entries = await fs.readdir(extensionsDir, { withFileTypes: true }).catch(() => [])
  for (const entry of entries) {
    if (!isExtensionSourceFile(entry)) continue
    const fullPath = path.join(extensionsDir, entry.name)
    try {
      const extension = await loadExtensionModule(fullPath)
      extension.__filePath = fullPath
      extension.__generated = true
      await applyExtensionMetadataOverrides(extension)
      registerExtension(extension)
    } catch (error) {
      logError('extension.load.failed', error, { source: 'host', scope: 'extension', extensionId: path.basename(fullPath) })
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

function registerInternalExtensions() {
  for (const createExtension of INTERNAL_EXTENSION_FACTORIES) registerExtension(createExtension())
  assertInternalExtensionsRegistered()
}

function assertInternalExtensionsRegistered() {
  const missingExtensions = REQUIRED_INTERNAL_EXTENSIONS.filter((extensionId) => !extensionModules.has(extensionId))
  const missingCommands = REQUIRED_INTERNAL_COMMANDS.filter(({ extensionId, commandId }) => !extensionRegistry.has(`${extensionId}:${commandId}`))
  if (missingExtensions.length || missingCommands.length) {
    const details = [
      missingExtensions.length ? `extensions: ${missingExtensions.join(', ')}` : '',
      missingCommands.length ? `commands: ${missingCommands.map(({ extensionId, commandId }) => `${extensionId}:${commandId}`).join(', ')}` : '',
    ].filter(Boolean).join('; ')
    throw new Error(`Missing required internal extensions (${details})`)
  }
}

function registerExtension(extension) {
  if (!extension?.id) return
  extensionModules.set(extension.id, extension)
  for (const command of extension.commands || []) {
    if (!command?.id || !command.title || typeof command.run !== 'function') continue
    extensionRegistry.set(`${extension.id}:${command.id}`, { extension, command })
  }
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
  appWatchers = watchApps(scheduleIndexApplications)
}

async function indexApplications() {
  try {
    const apps = await scanApps()
    const deduped = new Map()
    for (const item of apps) deduped.set(normalize(item.name), item)
    appIndex = Array.from(deduped.values()).sort((a, b) => a.name.localeCompare(b.name))
    paletteWindow.win?.webContents.send('apps:indexed', appIndex.length)
  } catch (error) {
    logError('applications.index.failed', error, { source: 'host', scope: 'apps' })
  }
}

async function indexFiles() {
  try {
    fileIndex = await scanFiles()
  } catch (error) {
    logError('files.index.failed', error, { source: 'host', scope: 'files' })
  }
}

async function loadUserState() {
  const cacheRoot = osCacheRoot()
  statePath = path.join(app.getPath('userData'), 'state.json')
  iconCacheDir = path.join(cacheRoot, 'icons')
  clipboardImagesDir = path.join(app.getPath('userData'), 'clipboard-images')
  extensionsDir = path.join(app.getPath('userData'), 'extensions')
  extensionStorageDir = path.join(app.getPath('userData'), 'extension-storage')
  extensionCacheDir = path.join(cacheRoot, 'extension-storage')
  await fs.rm(path.join(app.getPath('userData'), 'icon-cache'), { recursive: true, force: true }).catch(() => {})

  try {
    const loaded = JSON.parse(await fs.readFile(statePath, 'utf8'))
    userState = {
      recents: loaded.recents || {},
      aliases: loaded.aliases || {},
      shortcuts: loaded.shortcuts || {},
      shortcutActions: loaded.shortcutActions || {},
      removedShortcuts: loaded.removedShortcuts || {},
      overrides: loaded.overrides || {},
      clipboardHistory: loaded.clipboardHistory || [],
      aiChats: loaded.aiChats || {},
      settings: loaded.settings || {},
    }
  } catch {
    // First run.
  }

  migrateAiChats()
  clipboardHistory = normalizeClipboardHistory(userState.clipboardHistory, CLIPBOARD_LIMIT, persistClipboardImage)
}

function migrateAiChats() {
  for (const chat of Object.values(userState.aiChats || {}) as any[]) {
    if (chat.generatedExtensionFile && !chat.touchedExtensionFiles) chat.touchedExtensionFiles = [chat.generatedExtensionFile]
    if (chat.generatedExtensionFile && !chat.contextExtensionFile) chat.contextExtensionFile = chat.generatedExtensionFile
  }
}

function persistClipboardImage(png, hash) {
  const imagePath = path.join(clipboardImagesDir, `${hash}.png`)
  fs.mkdir(clipboardImagesDir, { recursive: true })
    .then(() => fs.writeFile(imagePath, png))
    .catch((error) => logWarn('clipboard.image.persist.failed', error, { source: 'host', scope: 'clipboard' }))
  return imagePath
}

function readClipboardItem() {
  const filePath = readClipboardFilePath(clipboard)
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
  const previousIds = new Set(clipboardHistory.map((entry) => entry.id))
  clipboardHistory = [
    item,
    ...clipboardHistory.filter((current) => current.id !== item.id),
  ].slice(0, CLIPBOARD_LIMIT)
  scheduleSaveState()
  invalidateExtensionRootItems()
  paletteWindow.win?.webContents.send('clipboard:changed')
  const currentIds = new Set(clipboardHistory.map((entry) => entry.id))
  const removeItemIds = [...previousIds].filter((id) => !currentIds.has(id)).map((id) => `clipboard:${id}`)
  patchOpenView('clipboard-history', {
    mode: 'prepend',
    items: [clipboardHistoryItem(item)],
    removeItemIds,
  })
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
    logError('state.save.failed', error, { source: 'host', scope: 'state' })
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

function unregisterShortcutForAction(actionId) {
  const current = userState.shortcuts[actionId]
  if (current) globalShortcut.unregister(current)
}

async function executeShortcutAction(action) {
  const currentAction = currentActionForStoredShortcut(action)
  const wasVisible = Boolean(paletteWindow.win?.isVisible())
  if (!wasVisible) {
    paletteWindow.showPalette({ skipShownEvent: true, deferReveal: true })
    paletteWindow.win?.webContents.send('action:view-open', {
      view: progressView({ title: currentAction?.title || 'Opening…', label: 'Opening…' }),
      revealWhenReady: true,
      asSibling: false,
    })
  }
  const result = await executeAction(currentAction, { keepPaletteOpen: true })
  if (result?.view) {
    if (wasVisible) paletteWindow.showPalette({ skipShownEvent: true })
    paletteWindow.win?.webContents.send('action:view-open', { ...result, revealWhenReady: false, asSibling: wasVisible })
  } else if (!wasVisible) {
    paletteWindow.hidePalette()
  }
}

function bindGlobalActionShortcut(actionId, accelerator, action) {
  if (accelerator === getPaletteHotkey()) return false
  globalShortcut.unregister(accelerator)
  registeredActionAccelerators.add(accelerator)
  const ok = globalShortcut.register(accelerator, () => executeShortcutAction(action))
  paletteWindow.debugLog('registerActionShortcut', { actionId, accelerator, title: action?.title, ok, isRegistered: globalShortcut.isRegistered(accelerator) })
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
    else logWarn('actionShortcut.register.failed', { actionId, accelerator }, { source: 'host', scope: 'shortcuts' })
  }
  for (const { actionId, accelerator, action } of declaredGlobalShortcuts()) {
    if (userState.shortcuts[actionId] || userState.removedShortcuts?.[actionId] || bound.has(accelerator)) continue
    const ok = bindGlobalActionShortcut(actionId, accelerator, action)
    if (ok) bound.add(accelerator)
    else logWarn('declaredActionShortcut.register.failed', { actionId, accelerator }, { source: 'host', scope: 'shortcuts' })
  }
}

function canCustomizeAction(action) {
  return canCustomizeCommandAction(action)
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
    .filter((item) => !userState.shortcuts[item.actionId] && !userState.removedShortcuts?.[item.actionId])
    .map((item) => ({ ...item, scope: 'global', source: 'extension' }))
  return [...configured, ...declared]
    .sort((a, b) => a.action.title.localeCompare(b.action.title))
}

async function removeShortcut(actionId) {
  if (!actionId) return { ok: false, message: 'Shortcut not found' }
  if (userState.shortcuts[actionId]) {
    globalShortcut.unregister(userState.shortcuts[actionId])
    delete userState.shortcuts[actionId]
    delete userState.shortcutActions[actionId]
  } else {
    const declared = declaredGlobalShortcuts().find((item) => item.actionId === actionId)
    if (!declared) return { ok: false, message: 'Shortcut not found' }
    globalShortcut.unregister(declared.accelerator)
    if (!userState.removedShortcuts) userState.removedShortcuts = {}
    userState.removedShortcuts[actionId] = declared.accelerator
  }
  invalidateShortcutCaches()
  scheduleSaveState()
  patchKeyboardShortcutsView()
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
    const conflictingAction = userState.shortcutActions[conflictingActionId]
    if (action.aiChatId && conflictingAction?.aiChatId === action.aiChatId && chatTouchedExtensionFiles(userState.aiChats[action.aiChatId]).length === 1) {
      await removeShortcut(conflictingActionId)
    } else {
      const title = conflictingAction?.title || 'another action'
      return { ok: false, message: `${accelerator} is already used by ${title}` }
    }
  }
  unregisterShortcutForAction(action.id)
  delete userState.removedShortcuts?.[action.id]
  const ok = registerActionShortcut(action.id, accelerator, action)
  if (!ok) return { ok: false, message: `Could not register ${accelerator}` }
  scheduleSaveState()
  patchKeyboardShortcutsView()
  return { ok: true, message: `Shortcut set: ${accelerator}` }
}

async function setPaletteHotkey(accelerator) {
  if (!accelerator?.trim()) return { ok: false, message: 'Missing shortcut' }
  const normalized = normalizeAccelerator(accelerator)
  const current = String(getPaletteHotkey())
  if (normalized === current) return { ok: true, message: `Shortcut unchanged: ${normalized}`, spotlightConflict: isSpotlightAccelerator(normalized) }
  const conflictingActionId = Object.entries(userState.shortcuts).find(([, value]) => value === normalized)?.[0]
  if (conflictingActionId) {
    const title = userState.shortcutActions[conflictingActionId]?.title || 'another action'
    return { ok: false, message: `${normalized} is already used by ${title}` }
  }
  globalShortcut.unregister(current)
  const ok = globalShortcut.register(normalized, paletteWindow.togglePalette)
  if (!ok) {
    globalShortcut.register(current, paletteWindow.togglePalette)
    const spotlightConflict = isSpotlightAccelerator(normalized)
    return { ok: false, message: spotlightConflict ? `${normalized} is used by ${reservedPaletteShortcutName()}` : `Could not register ${normalized}`, spotlightConflict }
  }
  setSetting('paletteHotkey', normalized)
  return { ok: true, message: `Shortcut set: ${normalized}`, spotlightConflict: isSpotlightAccelerator(normalized) }
}

async function openSystemKeyboardSettings() {
  await executeSystemBuiltin({ builtin: 'open-keyboard-settings' }, () => {})
  return { ok: true }
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
  if (!['extension-command', 'extension-root-item'].includes(action?.kind) || !action.removable) return { ok: false, message: 'Only generated extensions can be duplicated' }
  const extension = extensionModuleForAction(action)
  const filePath = extension?.__filePath
  if (!filePath) return { ok: false, message: 'Generated extension not found' }

  const duplicateId = hashValue(`${filePath}:${Date.now()}`)
  const duplicateTitle = `Copy of ${extension.title || action.title}`
  const duplicateFile = `${extensionSourceBasename(filePath)}-copy-${duplicateId.slice(0, 8)}.ts`
  const sourceFile = path.basename(filePath)
  const sourceCode = `import type { NevermindExtension } from './${EXTENSION_TYPES_FILENAME.replace(/\.d\.ts$/, '')}'\nimport source from './${sourceFile.replace(/'/g, "\\'")}'\n\nexport default {\n  ...source,\n  id: ${JSON.stringify(`${extension.id}-copy-${duplicateId.slice(0, 8)}`)},\n  title: ${JSON.stringify(duplicateTitle)},\n  commands: (source.commands || []).map((command) => ({ ...command })),\n} satisfies NevermindExtension\n`

  await fs.writeFile(path.join(extensionsDir, duplicateFile), sourceCode)
  userState.aiChats[duplicateId] = {
    id: duplicateId,
    query: `Tweak ${duplicateTitle}`,
    title: `Tweak ${duplicateTitle}`,
    status: 'ready',
    createdAt: Date.now(),
    updatedAt: Date.now(),
    contextExtensionFile: duplicateFile,
    touchedExtensionFiles: [duplicateFile],
    messages: [
      { role: 'system', content: `Duplicated from “${extension.title || action.title}”. Tweak this copy without changing the original.` },
    ],
  }
  scheduleSaveState()
  invalidateExtensionRootItems()
  await loadExtensions()
  registerActionShortcuts()
  const duplicateEntry = Array.from(extensionRegistry.values()).find((candidate) => path.basename(candidate.extension?.__filePath || '') === duplicateFile)
  return { ok: true, message: 'Action duplicated', action: duplicateEntry ? extensionActionFromCommand(duplicateEntry.extension, duplicateEntry.command) : { id: `ai-tweak-extension:${duplicateFile}`, kind: 'ai-tweak-extension', extensionFile: duplicateFile, title: duplicateTitle, subtitle: 'Tweak extension with AI', icon: 'sparkles', score: 0 } }
}

async function removeAiChat(chatId) {
  if (!chatId || !userState.aiChats[chatId]) return { toast: { message: 'AI chat not found', tone: 'error' } }
  await nevermindAi?.reset?.(chatId)
  const chat = userState.aiChats[chatId]
  // INVARIANT: removing a chat deletes only conversation history and AI session state.
  // It must NEVER unlink generated extension files. Generated extensions are durable
  // artifacts owned by chats via touchedExtensionFiles; chat removal preserves them so
  // the user can keep the extension after discarding the conversation that built it.
  delete userState.aiChats[chatId]
  for (const actionId of Object.keys(userState.recents || {})) {
    if (actionId === `ai-chat:${chatId}`) delete userState.recents[actionId]
  }
  scheduleSaveState()
  invalidateExtensionRootItems()
  patchAiChatsRemove(chatId)
  return { toast: { message: `Removed ${chat.title || chat.query || 'AI chat'}` } }
}

async function removeAiChatReferencesToExtensionFile(extensionFile) {
  const removedFile = path.basename(extensionFile || '')
  if (!removedFile) return
  for (const chat of Object.values(userState.aiChats || {}) as any[]) {
    const touchedFiles = chatTouchedExtensionFiles(chat)
    if (!touchedFiles.includes(removedFile)) continue
    const remainingFiles = [] as string[]
    for (const filename of touchedFiles.filter((item) => item !== removedFile)) {
      const exists = await fs.stat(path.join(extensionsDir, filename)).then(() => true).catch(() => false)
      if (exists) remainingFiles.push(filename)
    }
    if (remainingFiles.length === 0) {
      await removeAiChat(chat.id)
      continue
    }
    chat.touchedExtensionFiles = remainingFiles
    if (chat.contextExtensionFile === removedFile) chat.contextExtensionFile = remainingFiles[0]
    if (chat.generatedExtensionFile === removedFile) chat.generatedExtensionFile = remainingFiles[0]
    chat.updatedAt = Date.now()
    scheduleSaveState()
  }
}

async function removeCreatedAction(action) {
  if (action?.kind === 'ai-chat' && action.aiChatId) {
    await removeAiChat(action.aiChatId)
    delete userState.recents[action.id]
    return { ok: true, message: 'AI chat removed' }
  }

  if (['extension-command', 'extension-root-item'].includes(action?.kind) && action.removable) {
    const extension = extensionModuleForAction(action)
    const filePath = extension?.__filePath
    if (!filePath) return { ok: false, message: 'This extension cannot be removed' }
    await fs.unlink(filePath).catch((error) => {
      if (error?.code !== 'ENOENT') throw error
    })
    await removeAiChatReferencesToExtensionFile(path.basename(filePath))
    delete userState.recents[action.id]
    scheduleSaveState()
    await loadExtensions()
    registerActionShortcuts()
    return { ok: true, message: 'Generated extension removed' }
  }

  return { ok: false, message: 'This action cannot be removed' }
}

async function runPaletteDebugCli() {
  await indexApplications()
  await indexFiles()
  const query = String(process.env.NVM_PALETTE_QUERY || '')
  const actions = await searchActions(query)
  const selected = process.env.NVM_PALETTE_EXECUTE
    ? actions.find((action) => action.id === process.env.NVM_PALETTE_EXECUTE || action.title === process.env.NVM_PALETTE_EXECUTE)
    : null
  const result = selected ? await executeActionForIpc(selected) : undefined
  console.log(JSON.stringify({ query, count: actions.length, actions, selected, result }, null, 2))
}

app.whenReady().then(async () => {
  nativeTheme.themeSource = 'dark'
  prepareAppWindowPolicy()
  registerLocalFileProtocol()
  installPermissionHandlers(isDev)
  updateManager.configure()
  updateManager.onStateChange(() => patchUpdatesView())

  await loadUserState()
  await loadExtensions()
  if (process.env.NVM_PALETTE_DEBUG) {
    await runPaletteDebugCli()
    app.quit()
    return
  }
  initNevermindAi()
  paletteWindow.createWindow()
  paletteWindow.registerHotkey()
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
  ipcMain.handle('ai-builder:tweak-extension', (_event, input: any = {}) => {
    const file = input?.extensionFile || input?.extensionId
    if (!file) return { toast: { message: 'No extension specified', tone: 'error' } }
    const item = getOrCreateExtensionChat(file, input.title || file)
    return { view: aiChatView(item, { initialPrompt: input.prompt }) }
  })
  ipcMain.handle('ai-builder:start-chat', (_event, input: any = {}) => {
    const item = createDraftAiChat(String(input?.prompt || input?.query || ''))
    return { view: aiChatView(item, { start: item.messages.length <= 1 }) }
  })
  ipcMain.handle('apps:icon', (_event, appPath) => getAppIconDataUrl(appPath))
  ipcMain.handle('palette:set-mode', (_event, mode) => {
    paletteWindow.setPaletteSizeForMode(mode)
    paletteWindow.centerWindow()
  })
  ipcMain.handle('palette:hide', () => paletteWindow.hidePalette())
  ipcMain.handle('app:quit', () => {
    requestQuitApp('ipc')
    return { ok: true }
  })
  ipcMain.handle('palette:shortcut-ready', () => paletteWindow.revealPalette())
  ipcMain.handle('camera:request-access', async () => {
    if (!hasCapability('camera')) return { ok: false, status: 'unsupported' }
    if (process.platform !== 'darwin') return { ok: true, status: 'unknown' }
    const status = systemPreferences.getMediaAccessStatus('camera')
    if (status === 'granted') return { ok: true, status }
    if (status === 'denied' || status === 'restricted') return { ok: false, status }
    return { ok: true, status }
  })
  ipcMain.handle('logs:write', (_event, level, message, data) => {
    const method = level === 'error' ? logError : level === 'warn' ? logWarn : level === 'debug' ? loggerDebug : logInfo
    method(String(message || ''), data, { source: 'renderer', scope: 'renderer' })
  })
})

app.on('activate', () => paletteWindow.showPalette())
app.on('before-quit', () => {
  nevermindApp.isQuiting = true
})
app.on('will-quit', () => {
  nevermindApp.isQuiting = true
  runQuitCleanup()
})

if (!isDev) {
  const gotLock = app.requestSingleInstanceLock()
  if (!gotLock) app.quit()
  else app.on('second-instance', () => paletteWindow.showPalette())
}
