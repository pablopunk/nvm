import { app, globalShortcut, ipcMain, shell, clipboard, nativeImage, nativeTheme, protocol, net } from 'electron'
import electronUpdater from 'electron-updater'
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import crypto from 'node:crypto'
import { spawn, execFile } from 'node:child_process'
import { pathToFileURL } from 'node:url'
import { clipboardFilePath as readClipboardFilePath, clipboardFilePaths, clipboardItemSubtitle, clipboardItemTitle, normalizeClipboardHistory } from './clipboard-utils'
import { expandUserPath, extensionForPath, fileUrlForPath, IMAGE_EXTENSIONS, isImagePath, isVideoPath, LOCAL_FILE_PROTOCOL, LOCAL_THUMB_PROTOCOL, thumbnailUrlForPath, VIDEO_EXTENSIONS } from './file-utils'
import { createRequire } from 'node:module'
import { createNevermindAi } from './ai'
import { createPaletteWindowController, installPermissionHandlers } from './palette-window'
import { settingDefinition, SETTING_DEFINITIONS, settingValue, toggledSettingValue } from './settings'
import { calculate, getUrlFromQuery, hashValue, normalize, score, scoreNormalized } from './search-utils'
import { formatShortcut, isSpotlightAccelerator, normalizeAccelerator } from './shortcut-utils'
import { autoUpdatesUnavailableMessage, executeSystemBuiltin, frontmostApp, hasCapability, launchApp as launchOsApp, pasteIntoFrontmostApp, prepareAppWindowPolicy, quickLookTitle, reservedPaletteShortcutName, revealPathTitle, scanApps, selectedFilePaths, selectedText, settingsTitle, watchApps } from './os'
import { createUpdateManager } from './update-manager'
import { isNewerVersion as isVersionNewerThan } from './version-utils'

const extensionRequire = createRequire(import.meta.url)
const { autoUpdater } = electronUpdater
const updateManager = createUpdateManager(autoUpdater as any)
const paletteWindow = createPaletteWindowController({
  isDev: Boolean(process.env.ELECTRON_RENDERER_URL),
  preloadPath: path.join(__dirname, '..', 'preload', 'preload.cjs'),
  rendererUrl: process.env.ELECTRON_RENDERER_URL,
  rendererIndexPath: path.join(__dirname, '..', 'renderer', 'index.html'),
  userDataPath: app.getPath('userData'),
  getPaletteHotkey: () => String(getPaletteHotkey()),
})

const isDev = Boolean(process.env.ELECTRON_RENDERER_URL)
const CLIPBOARD_LIMIT = 300
const FILE_RESULT_LIMIT = 6
const CLIPBOARD_POLL_INTERVAL_MS = 1000
const APP_REINDEX_DEBOUNCE_MS = 1000
const THUMBNAIL_SIZE = 512
const EXTENSION_ROOT_ITEMS_TTL_MS = 60_000
const EXTENSION_ROOT_ITEMS_TIMEOUT_MS = 10_000

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
let saveTimer: NodeJS.Timeout | undefined
let appIndexTimer: NodeJS.Timeout | undefined
let appWatchers: Array<{ close: () => unknown }> = []
let nevermindAi: any
let activeAiChatId: string | undefined
const draftAiChats = new Map<string, AnyRecord>()
let userState: AnyRecord = {
  recents: {},
  aliases: {},
  shortcuts: {},
  shortcutActions: {},
  overrides: {},
  clipboardHistory: [],
  aiChats: {},
  settings: {},
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
}
const appIconCache = new Map<string, string | null>()
const extensionRegistry = new Map<string, any>()
const extensionModules = new Map<string, any>()
const extensionRootItemsCache = new Map<string, { updatedAt: number; items: any[] }>()
const extensionRootItemsRefreshes = new Map<string, Promise<any[]>>()
const extensionActionHandlers = new Map<string, any>()
const registeredActionAccelerators = new Set<string>()

const INTERNAL_EXTENSIONS: any[] = [createSystemExtension(), createPlacesExtension(), createCalculatorExtension(), createWebSearchExtension(), createClipboardExtension(), createAppsExtension(), createFilesExtension(), createAiBuilderExtension(), createUpdatesExtension(), createKeyboardShortcutsExtension(), createSettingsExtension()]

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
  const map = new Map<string, string>()
  for (const [actionId, storedAction] of Object.entries(userState.shortcutActions) as Array<[string, any]>) {
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

function shortcutForAction(action: any) {
  if (userState.shortcuts[action.id]) return userState.shortcuts[action.id]
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
      console.warn(`Failed to load app icon for ${appPath}`, error)
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
  const action = {
    id: command.actionId || extensionCommandActionId(extension, command),
    kind: 'extension-command',
    extensionId: extension.id,
    commandId: command.id,
    extensionFile: extension.__filePath ? path.basename(extension.__filePath) : undefined,
    removable: Boolean(extension.__generated),
    title: command.title,
    subtitle: command.subtitle || extension.title || 'Extension command',
    aliases: command.aliases || [],
    icon: command.icon || 'sparkles',
    score: command.score || 12,
    dismissAfterRun: command.dismissAfterRun,
    background: command.background,
  }
  const shortcut = shortcutForAction(action) || command.globalShortcut || (command.shortcutScope === 'global' ? command.shortcut : null)
  return shortcut ? { ...action, shortcut } : action
}

function aiChatActionFromItem(item) {
  return {
    id: `ai-chat:${item.id}`,
    kind: 'ai-chat',
    title: item.title || item.query,
    subtitle: item.status === 'ready' ? 'AI builder chat' : 'Continue AI builder chat',
    query: item.query,
    aiChatId: item.id,
    aliases: [item.query],
    icon: 'sparkles',
    score: 13,
    lastUsed: item.updatedAt || item.createdAt || 0,
  }
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
  return draft
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

function aiChatsView() {
  const chats = Object.values(userState.aiChats || {}) as any[]
  return {
    type: 'list',
    id: 'ai-chats',
    title: 'AI Chats',
    searchBarPlaceholder: 'Search AI Chats',
    items: chats
      .sort((a, b) => (b.updatedAt || b.createdAt || 0) - (a.updatedAt || a.createdAt || 0))
      .map((chat) => ({
        id: `ai-chat:${chat.id}`,
        title: chat.title || chat.query || 'AI Chat',
        subtitle: chat.contextExtensionFile || (chat.touchedExtensionFiles || [])[0] || chat.status || 'Builder chat',
        icon: 'sparkles',
        primaryAction: { type: 'nativeAction', title: 'Open Chat', nativeAction: aiChatActionFromItem(chat) },
        actions: [{
          type: 'nativeAction',
          title: 'Remove Chat',
          style: 'destructive',
          requiresConfirmation: true,
          nativeAction: { id: `remove-ai-chat:${chat.id}`, kind: 'remove-ai-chat', aiChatId: chat.id, title: 'Remove Chat', subtitle: chat.title || chat.query || 'AI chat', icon: 'sparkles', score: 0 },
        }],
      })),
  }
}

function chatTouchedExtensionFiles(chat) {
  return Array.from(new Set([...(chat?.touchedExtensionFiles || []), chat?.generatedExtensionFile].filter(Boolean).map((item) => path.basename(item))))
}

function touchExtensionFileForChat(chat, filename) {
  if (!chat || !filename) return
  chat.touchedExtensionFiles = Array.from(new Set([...chatTouchedExtensionFiles(chat), path.basename(filename)]))
  if (!chat.contextExtensionFile) chat.contextExtensionFile = path.basename(filename)
  if (!chat.generatedExtensionFile) chat.generatedExtensionFile = path.basename(filename)
  chat.status = 'ready'
  chat.updatedAt = Date.now()
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
  return item
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

function isNewerVersion(version) {
  return isVersionNewerThan(version, app.getVersion())
}

function updateStatusView() {
  const downloadedInfo = isNewerVersion(updateManager.state.downloadedInfo?.version) ? updateManager.state.downloadedInfo : null
  const availableInfo = isNewerVersion(updateManager.state.availableInfo?.version) ? updateManager.state.availableInfo : null
  const version = downloadedInfo?.version || availableInfo?.version
  const unsupported = updateManager.state.status === 'unsupported' || !updateManager.canUseAutoUpdates()
  const primaryAction = downloadedInfo
    ? { type: 'nativeAction', title: 'Install and Restart', nativeAction: { kind: 'install-update' } }
    : availableInfo
      ? { type: 'nativeAction', title: updateManager.state.downloadInFlight ? 'Downloading…' : 'Download Update', nativeAction: { kind: 'download-update' } }
      : { type: 'nativeAction', title: updateManager.state.checkInFlight ? 'Checking…' : 'Check Again', nativeAction: { kind: 'check-for-updates' } }
  const title = unsupported
    ? 'Updates unavailable'
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
    isLoading: updateManager.state.checkInFlight || updateManager.state.downloadInFlight,
    items: [{
      id: 'update-status',
      title,
      subtitle,
      icon: 'restart',
      accessories: version ? [{ text: version }] : [],
      primaryAction: unsupported ? undefined : primaryAction,
      actionPanel: unsupported ? undefined : { sections: [{ actions: [primaryAction] }] },
    }],
  }
}

function refreshUpdateStatusViewWhenSettled(task: Promise<unknown>) {
  task.finally(() => {
    paletteWindow.win?.webContents.send('action:view-open', { view: updateStatusView() })
  }).catch(() => {})
}

function checkForUpdatesView() {
  refreshUpdateStatusViewWhenSettled(updateManager.checkForUpdates('manual', { download: true }))
  return { view: updateStatusView(), navigation: 'replace' }
}

function downloadUpdateView() {
  refreshUpdateStatusViewWhenSettled(updateManager.downloadAvailableUpdate())
  return { view: updateStatusView(), navigation: 'replace' }
}

function installDownloadedUpdate() {
  if (!updateManager.state.downloadedInfo) return { view: updateStatusView(), navigation: 'replace' }
  nevermindApp.isQuiting = true
  updateManager.quitAndInstall()
}

function settingItemPatch(definition) {
  const value = getSetting(definition.id)
  const accessoryText = definition.type === 'boolean' ? (value ? 'On' : 'Off') : definition.type === 'shortcut' ? '' : String(value)
  const primaryAction = definition.type === 'shortcut'
    ? { type: 'nativeAction', title: 'Change Shortcut', shortcut: String(value || ''), nativeAction: { kind: 'record-palette-hotkey' } }
    : { type: 'nativeAction', title: value ? 'Turn Off' : 'Turn On', nativeAction: { kind: 'toggle-setting', settingId: definition.id } }
  return { id: `setting:${definition.id}`, accessories: accessoryText ? [{ text: accessoryText }] : [], primaryAction, actionPanel: { sections: [{ actions: [primaryAction] }] } }
}

function settingsView(selectedItemId = '') {
  return {
    type: 'list',
    id: 'app-settings',
    title: 'Settings',
    presentation: 'root',
    selectedItemId,
    searchBarPlaceholder: 'Search Settings',
    items: SETTING_DEFINITIONS.map((definition) => ({
      id: `setting:${definition.id}`,
      title: definition.title,
      subtitle: definition.description,
      icon: 'settings',
      ...settingItemPatch(definition),
    })),
  }
}

function updatePromptAction() {
  const downloadedInfo = isNewerVersion(updateManager.state.downloadedInfo?.version) ? updateManager.state.downloadedInfo : null
  const availableInfo = isNewerVersion(updateManager.state.availableInfo?.version) ? updateManager.state.availableInfo : null
  const version = downloadedInfo?.version || availableInfo?.version
  if (downloadedInfo) {
    return {
      id: 'builtin:install-update',
      kind: 'install-update',
      title: `Install Nevermind ${version}`,
      subtitle: 'Restart Nevermind to finish updating',
      icon: 'restart',
      score: 1_000,
    }
  }
  if (availableInfo) {
    return {
      id: 'builtin:download-update',
      kind: 'download-update',
      title: `Download Nevermind ${version}`,
      subtitle: updateManager.state.downloadInFlight ? 'Downloading update…' : 'Update available',
      icon: 'restart',
      score: 1_000,
    }
  }
  return null
}

async function searchActions(query, options: any = {}) {
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
  const contributedItems = q ? await extensionSearchActions(q) : await extensionRootActions()
  for (const item of contributedItems) {
    const ranked = rankAction(withShortcutHint(item), q)
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
    .slice(0, 10)
}

function rootNativeActionCanDismissImmediately(action) {
  return ['open-url', 'web-search', 'app', 'clipboard', 'file', 'calculate', 'builtin', 'open-keyboard-settings'].includes(String(action?.kind))
}

function runInBackground(task) {
  Promise.resolve().then(task).catch((error) => console.error('Background action failed', error))
}

async function executeAction(action, options: any = {}) {
  if (!action) return
  recordRecent(action)

  switch (action.kind) {
    case 'open-url':
      runInBackground(() => shell.openExternal(action.url))
      break
    case 'web-search':
      runInBackground(() => shell.openExternal(`https://www.google.com/search?q=${encodeURIComponent(action.query)}`))
      break
    case 'app':
      runInBackground(() => launchOsApp(action.app))
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
    case 'check-for-updates':
      return checkForUpdatesView()
    case 'download-update':
      return downloadUpdateView()
    case 'install-update':
      return installDownloadedUpdate()
    case 'open-keyboard-settings':
      runInBackground(openSystemKeyboardSettings)
      break
    case 'toggle-setting': {
      const definition = settingDefinition(action.settingId)
      if (!definition) return
      const current = getSetting(definition.id)
      const next = toggledSettingValue(definition, current)
      setSetting(definition.id, next)
      return { patch: { items: [settingItemPatch(definition)] } }
    }
    case 'file':
      runInBackground(() => shell.openPath(action.filePath))
      break
    case 'calculate':
      clipboard.writeText(action.result)
      break
    case 'builtin':
      runInBackground(() => executeBuiltin(action))
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
    case 'ai-chats':
      return { view: aiChatsView() }
    case 'remove-ai-chat':
      return removeAiChat(action.aiChatId)
    case 'ai-chat': {
      const item = userState.aiChats[action.aiChatId]
      if (item) return { view: aiChatView(item) }
      break
    }
    case 'ai-tweak-extension': {
      const item = getOrCreateExtensionChat(action.extensionFile, action.title || action.extensionFile)
      return { view: aiChatView(item) }
    }
    case 'ai-placeholder': {
      const item = createDraftAiChat(action.query)
      return { view: aiChatView(item, { start: item.messages.length <= 1 }) }
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
    console.warn(`Extension command not found: ${action.extensionId}:${action.commandId}`)
    return null
  }
  try {
    const ctx = createExtensionContext(entry.extension, entry.command)
    const result = await entry.command.run(ctx)
    return executeViewActionResult(result, entry)
  } catch (error) {
    console.error(`Extension command failed: ${entry.extension.id}:${entry.command.id}`, error)
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
    console.error(`Extension root item failed: ${record.entry.extension.id}`, error)
    return { view: extensionErrorView(record.entry, error) }
  }
}

async function extensionRootActions() {
  const actionGroups = await Promise.all(Array.from(extensionModules.values()).map(extensionRootActionsForExtension))
  return actionGroups.flat().slice(0, 20)
}

async function extensionSearchActions(query) {
  const actionGroups = await Promise.all(Array.from(extensionModules.values()).map((extension) => extensionSearchActionsForExtension(extension, query)))
  return actionGroups.flat().slice(0, 20)
}

async function extensionSearchActionsForExtension(extension, query) {
  if (typeof extension.searchItems !== 'function') return []
  try {
    const entry = { extension, command: { id: 'search', title: extension.title || extension.id } }
    const items = await withTimeout(extension.searchItems(createExtensionContext(extension, null), query), EXTENSION_ROOT_ITEMS_TIMEOUT_MS)
    const list = Array.isArray(items) ? items : Array.isArray(items?.items) ? items.items : []
    return list.slice(0, 5).map((item) => extensionRootActionFromItem(entry, item)).filter(Boolean)
  } catch (error) {
    if (!String(error?.message || error).includes('Timed out')) console.error(`Extension search items failed: ${extension.id}`, error)
    return []
  }
}

async function extensionRootActionsForExtension(extension) {
  if (typeof extension.rootItems !== 'function') return []
  const cacheKey = extension.__filePath || extension.id
  const cached = extensionRootItemsCache.get(cacheKey)
  if (cached && Date.now() - cached.updatedAt < EXTENSION_ROOT_ITEMS_TTL_MS) return cached.items
  refreshExtensionRootActions(extension, cacheKey)
  return cached?.items || []
}

function refreshExtensionRootActions(extension, cacheKey) {
  const current = extensionRootItemsRefreshes.get(cacheKey)
  if (current) return current
  const promise = (async () => {
    const entry = { extension, command: { id: 'root', title: extension.title || extension.id } }
    const items = await withTimeout(extension.rootItems(createExtensionContext(extension, null)), EXTENSION_ROOT_ITEMS_TIMEOUT_MS)
    const list = Array.isArray(items) ? items : Array.isArray(items?.items) ? items.items : []
    const actions = list.slice(0, 5).map((item) => extensionRootActionFromItem(entry, item)).filter(Boolean)
    extensionRootItemsCache.set(cacheKey, { updatedAt: Date.now(), items: actions })
    return actions
  })().catch((error) => {
    if (!String(error?.message || error).includes('Timed out')) console.error(`Extension root items failed: ${extension.id}`, error)
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
    score: Math.min(Number(item.score || 35), 90),
    lastUsed: Number(item.lastUsed || 0),
    dismissAfterRun: item.dismissAfterRun || primaryAction?.dismissAfterRun,
    actionPanel,
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

function isViewAction(value) {
  return Boolean(value?.type && ['nativeAction', 'openPath', 'revealPath', 'quickLook', 'openWith', 'openUrl', 'copyText', 'pasteText', 'copyImage', 'trash', 'pushView', 'replaceView', 'popView', 'runExtensionAction', 'shellExec', 'shellScript'].includes(value.type))
}

async function executeViewActionResult(result, entry) {
  if (!result) return result
  if (isViewAction(result)) return executeViewAction(normalizeViewAction(result, entry))
  if (isViewAction(result.action)) return executeViewAction(normalizeViewAction(result.action, entry))
  const view = normalizeExtensionView(result, entry)
  return view ? { view, navigation: result?.navigation || 'push', toast: result?.toast, patch: result?.patch } : result
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
  return executeSystemBuiltin(action, () => {
    nevermindApp.isQuiting = true
    app.quit()
  })
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

async function findFiles(roots, options: any = {}) {
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
  if (!hasCapability('quick-look')) return { toast: { message: `${quickLookTitle()} is not available on this OS`, tone: 'error' } }
  const resolvedPath = expandUserPath(filePath)
  if (!resolvedPath || !path.isAbsolute(resolvedPath)) return { toast: { message: `Cannot ${quickLookTitle()} this item`, tone: 'error' } }
  const child = spawn('qlmanage', ['-p', resolvedPath], { detached: true, stdio: 'ignore' })
  child.unref()
}

function execFileText(command, args) {
  return new Promise((resolve, reject) => {
    execFile(command, args, (error, stdout) => error ? reject(error) : resolve(stdout))
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

function extensionStoragePath(extension) {
  const key = extension.__filePath ? path.basename(extension.__filePath, '.cjs') : extension.id || 'extension'
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

function createExtensionAi(extension) {
  const extensionKey = path.basename(extension.__filePath || extension.id || 'extension').replace(/[^a-zA-Z0-9._-]/g, '-')
  return {
    ask: (prompt, options: any = {}) => nevermindAi.ask(String(prompt || ''), { system: options.system }),
    session: (id = 'default', options: any = {}) => nevermindAi.session(`${extensionKey}:${String(id || 'default')}`, { system: options.system }),
  }
}

function systemActions() {
  return [
    { id: 'builtin:lock-screen', kind: 'builtin', builtin: 'lock-screen', title: 'Lock Screen', subtitle: 'Secure this computer', icon: 'lock', score: 22 },
    { id: 'builtin:sleep', kind: 'builtin', builtin: 'sleep', title: 'Sleep', subtitle: 'Put this computer to sleep', icon: 'moon', score: 21 },
    { id: 'builtin:restart', kind: 'builtin', builtin: 'restart', title: 'Restart Computer', subtitle: 'Restart this computer', icon: 'restart', score: 20 },
    { id: 'builtin:settings', kind: 'builtin', builtin: 'settings', title: settingsTitle(), subtitle: 'Open system preferences', icon: 'settings', score: 19 },
    { id: 'builtin:quit', kind: 'builtin', builtin: 'quit', title: 'Quit Nevermind', subtitle: 'Close the app', icon: 'power', score: 15 },
  ]
}

function placesActions() {
  return [
    { id: 'places:downloads', kind: 'file', title: 'Open Downloads', subtitle: '~/Downloads', filePath: path.join(os.homedir(), 'Downloads'), icon: 'folder', score: 18 },
    { id: 'places:documents', kind: 'file', title: 'Open Documents', subtitle: '~/Documents', filePath: path.join(os.homedir(), 'Documents'), icon: 'folder', score: 17 },
    { id: 'places:desktop', kind: 'file', title: 'Open Desktop', subtitle: '~/Desktop', filePath: path.join(os.homedir(), 'Desktop'), icon: 'folder', score: 16 },
  ]
}

function commandFromNativeAction(action) {
  return { id: action.id, actionId: action.id, title: action.title, subtitle: action.subtitle, icon: action.icon, score: action.score, run: (ctx) => ctx.navigation.run(ctx.actions.native(action.title, action)) }
}

function createSystemExtension() {
  return {
    id: 'nevermind.system',
    title: 'System',
    commands: systemActions().map(commandFromNativeAction),
    rootItems: () => systemActions().map(rootItemFromNativeAction),
  }
}

function createPlacesExtension() {
  return {
    id: 'nevermind.places',
    title: 'Places',
    commands: placesActions().map(commandFromNativeAction),
    rootItems: () => placesActions().map(rootItemFromNativeAction),
  }
}

function createCalculatorExtension() {
  return {
    id: 'nevermind.calculator',
    title: 'Calculator',
    commands: [],
    searchItems(_ctx, query) {
      const result = query ? calculate(query) : null
      if (result === null) return []
      const action = { id: `calculate:${query}`, kind: 'calculate', title: `${query} = ${result}`, subtitle: 'Copy result to clipboard', query, result, icon: 'calculator', score: 105 }
      return [{ id: action.id, title: action.title, subtitle: action.subtitle, icon: action.icon, score: action.score, primaryAction: { type: 'nativeAction', title: action.title, nativeAction: action } }]
    },
  }
}

function createWebSearchExtension() {
  return {
    id: 'nevermind.web',
    title: 'Web',
    commands: [],
    searchItems(_ctx, query) {
      const q = String(query || '').trim()
      if (!q) return []
      const url = getUrlFromQuery(q)
      if (url) {
        const action = { id: `open-url:${url}`, kind: 'open-url', title: `Open ${url.replace(/^https?:\/\//, '')}`, subtitle: 'Open website', url, icon: 'globe', score: 100 }
        return [{ id: action.id, title: action.title, subtitle: action.subtitle, icon: action.icon, score: action.score, primaryAction: { type: 'nativeAction', title: action.title, nativeAction: action } }]
      }
      const action = { id: `web-search:${q}`, kind: 'web-search', title: `Search the web for "${q}"`, subtitle: 'Search instead', query: q, icon: 'search', score: 10 + usageBoost(`web-search:${q}`) + recentBoost(`web-search:${q}`) }
      return [{ id: action.id, title: action.title, subtitle: action.subtitle, icon: action.icon, score: action.score, primaryAction: { type: 'nativeAction', title: action.title, nativeAction: action } }]
    },
  }
}

function rootItemFromNativeAction(action) {
  const dismissAfterRun = rootNativeActionCanDismissImmediately(action) ? 'auto' : undefined
  return { id: action.id, title: action.title, subtitle: action.subtitle, icon: action.icon, image: action.thumbnailUrl || action.iconUrl || undefined, score: action.score, lastUsed: action.lastUsed, dismissAfterRun, primaryAction: { type: 'nativeAction', title: action.title, shortcut: action.shortcut, dismissAfterRun, nativeAction: action } }
}

function createClipboardExtension() {
  function historyAction() {
    const latestClipboardTime = clipboardHistory[0]?.createdAt || 0
    return {
      id: 'clipboard-history',
      kind: 'clipboard-history',
      title: 'Clipboard History',
      subtitle: clipboardHistory.length ? `Show all ${clipboardHistory.length} copied items` : 'Show copied items',
      icon: 'clipboard',
      score: 14,
      lastUsed: latestClipboardTime ? latestClipboardTime - 1 : 0,
    }
  }

  return {
    id: 'nevermind.clipboard',
    title: 'Clipboard',
    commands: [{
      id: 'clipboard-history',
      actionId: 'clipboard-history',
      title: 'Clipboard History',
      subtitle: 'Show copied items',
      icon: 'clipboard',
      score: 14,
      run: () => clipboardHistoryView(),
    }],
    rootItems() {
      if (!getSetting('showClipboardInRoot')) return [rootItemFromNativeAction(historyAction())]
      return [rootItemFromNativeAction(historyAction()), ...clipboardHistory.slice(0, 5).map((item) => rootItemFromNativeAction(clipboardActionFromItem(item)))]
    },
    searchItems(_ctx, query) {
      const results = [historyAction(), ...clipboardHistory.map(clipboardActionFromItem)]
      return results.map(rootItemFromNativeAction).filter((item) => rankAction(item.primaryAction.nativeAction, query)).slice(0, 5)
    },
  }
}

function createAppsExtension() {
  return {
    id: 'nevermind.apps',
    title: 'Applications',
    commands: [],
    rootItems() {
      return appIndex.slice(0, 5).map((item) => rootItemFromNativeAction({ id: `app:${item.id}`, kind: 'app', title: item.name, subtitle: 'Launch application', app: item, icon: 'app', score: 30 }))
    },
    async searchItems(_ctx, query) {
      const matches = appIndex.map((item) => rootItemFromNativeAction({ id: `app:${item.id}`, kind: 'app', title: item.name, subtitle: 'Launch application', app: item, icon: 'app', score: 30 })).filter((item) => rankAction(item.primaryAction.nativeAction, query)).slice(0, 5)
      await Promise.all(matches.map(async (item) => {
        const appPath = item.primaryAction.nativeAction.app?.path
        const iconUrl = await getAppIconDataUrl(appPath)
        if (iconUrl) item.image = iconUrl
      }))
      return matches
    },
  }
}

function createFilesExtension() {
  return {
    id: 'nevermind.files',
    title: 'Files',
    commands: [],
    rootItems() {
      return fileIndex.slice(0, FILE_RESULT_LIMIT).map((item) => rootItemFromNativeAction({ id: `file:${item.path}`, kind: 'file', title: item.name, subtitle: item.displayPath, filePath: item.path, icon: 'folder', score: 4 }))
    },
    searchItems(_ctx, query) {
      return fileIndex.map((item) => rootItemFromNativeAction({ id: `file:${item.path}`, kind: 'file', title: item.name, subtitle: item.displayPath, filePath: item.path, icon: 'folder', score: 4 })).filter((item) => rankAction(item.primaryAction.nativeAction, query)).slice(0, FILE_RESULT_LIMIT)
    },
  }
}

function createAiBuilderExtension() {
  function chatsAction() {
    return { id: 'ai-chats', kind: 'ai-chats', title: 'AI Chats', subtitle: `${Object.keys(userState.aiChats || {}).length} builder chats`, icon: 'sparkles', score: 16 }
  }
  function chatItems(query = '') {
    return Object.values(userState.aiChats || {}).map((item: any) => {
      const action = aiChatActionFromItem(item)
      action.lastUsed = Math.max(action.lastUsed || 0, item.updatedAt || item.createdAt || 0)
      return rootItemFromNativeAction(action)
    }).filter((item) => !query || rankAction(item.primaryAction.nativeAction, query))
  }
  return {
    id: 'nevermind.ai-builder',
    title: 'AI Builder',
    commands: [{ id: 'ai-chats', actionId: 'ai-chats', title: 'AI Chats', subtitle: 'Builder chats', icon: 'sparkles', score: 16, run: () => aiChatsView() }],
    rootItems() {
      return [rootItemFromNativeAction(chatsAction()), ...chatItems().slice(0, 4)]
    },
    searchItems(_ctx, query) {
      const q = String(query || '').trim()
      const items = [rootItemFromNativeAction(chatsAction()), ...chatItems(q)]
      if (q && !getUrlFromQuery(q) && calculate(q) === null) items.push(rootItemFromNativeAction({ id: `ai:${q}`, kind: 'ai-placeholder', title: `Press Tab to automate "${q}"`, subtitle: 'Automate with AI', query: q, icon: 'bolt', score: 40 }))
      return items.filter((item) => rankAction(item.primaryAction.nativeAction, q)).slice(0, 5)
    },
  }
}

function createUpdatesExtension() {
  const checkAction = () => ({ id: 'updates:check', kind: 'check-for-updates', title: 'Check for Updates', subtitle: `Current version: ${app.getVersion()}`, icon: 'restart', score: 23 })
  return {
    id: 'nevermind.updates',
    title: 'Updates',
    commands: [commandFromNativeAction(checkAction())],
    rootItems() {
      const action = updatePromptAction() || checkAction()
      return [rootItemFromNativeAction(action)]
    },
  }
}

function keyboardShortcutsView() {
  return {
    type: 'list',
    id: 'keyboard-shortcuts',
    title: 'Keyboard Shortcuts',
    presentation: 'root',
    searchBarPlaceholder: 'Search Keyboard Shortcuts',
    emptyView: { title: 'No shortcuts found.' },
    items: getShortcuts().map((record) => {
      const changeAction = { type: 'nativeAction', title: 'Change shortcut', nativeAction: { kind: 'record-shortcut', action: record.action } }
      const removeAction = record.source === 'user' ? { type: 'nativeAction', title: 'Remove shortcut', style: 'destructive', nativeAction: { kind: 'remove-shortcut', actionId: record.actionId } } : null
      return {
        id: `shortcut:${record.actionId}`,
        title: record.action.title,
        subtitle: record.accelerator,
        icon: 'keyboard',
        primaryAction: changeAction,
        actionPanel: { sections: [{ actions: [changeAction, removeAction].filter(Boolean) }] },
      }
    }),
  }
}

function createKeyboardShortcutsExtension() {
  return {
    id: 'nevermind.shortcuts',
    title: 'Keyboard Shortcuts',
    commands: [{ id: 'keyboard-shortcuts', actionId: 'keyboard-shortcuts', title: 'Keyboard Shortcuts', subtitle: 'View, change, or remove global shortcuts', icon: 'keyboard', score: 16, run: () => keyboardShortcutsView() }],
  }
}

function createSettingsExtension() {
  return {
    id: 'nevermind.settings',
    title: 'Settings',
    commands: [{ id: 'app-settings', actionId: 'app-settings', title: 'Settings', subtitle: 'Configure Nevermind', icon: 'settings', score: 16, run: (ctx) => ctx.ui.list({
      type: 'list',
      id: 'app-settings',
      title: 'Settings',
      presentation: 'root',
      selectedItemId: ctx.state.selectedItemId,
      searchBarPlaceholder: 'Search Settings',
      items: SETTING_DEFINITIONS.map((definition) => {
        const patch = settingItemPatch(definition)
        return { id: `setting:${definition.id}`, title: definition.title, subtitle: definition.description, icon: 'settings', ...patch }
      }),
    }) }],
  }
}

function createExtensionContext(extension, command) {
  return {
    extension: createExtensionRuntimeMetadata(extension, command),
    command,
    ui: {
      list: (view) => ({ ...view, type: 'list' }),
      grid: (view) => ({ ...view, type: 'grid' }),
      preview: (fileOrView, view: any = {}) => {
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
      openPath: (filePath, title = 'Open', options: any = {}) => ({ ...options, type: 'openPath', title, path: filePath }),
      revealPath: (filePath, title = revealPathTitle(), options: any = {}) => ({ ...options, type: 'revealPath', title, path: filePath }),
      quickLook: (filePath, title = quickLookTitle(), options: any = {}) => ({ ...options, type: 'quickLook', title, path: filePath }),
      openWith: (filePath, app, title, options: any = {}) => ({ ...options, type: 'openWith', title: title || `Open with ${app?.name || 'App'}`, path: filePath, app, appPath: app?.path || app }),
      openUrl: (url, title = 'Open URL', options: any = {}) => ({ ...options, type: 'openUrl', title, url }),
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
      toggleSetting: (settingId, title = 'Toggle', options: any = {}) => ({ ...options, type: 'nativeAction', title, nativeAction: { kind: 'toggle-setting', settingId } }),
      setPaletteShortcut: (title = 'Change Shortcut', options: any = {}) => ({ ...options, type: 'nativeAction', title, nativeAction: { kind: 'record-palette-hotkey' } }),
      native: (title, nativeAction, options: any = {}) => ({ ...options, type: 'nativeAction', title, nativeAction }),
    },
    navigation: {
      push: (view) => ({ view, navigation: 'push' }),
      replace: (view) => ({ view, navigation: 'replace' }),
      pop: () => ({ navigation: 'pop' }),
      run: (action) => ({ action }),
    },
    desktop: {
      clipboard: {
        readText: () => clipboard.readText(),
        writeText: (text) => clipboard.writeText(String(text || '')),
        readImage: clipboardImageDataUrl,
        writeImage: (image) => writeDesktopClipboard({ type: 'image', image }),
        readFiles: clipboardFiles,
        read: readDesktopClipboard,
        write: writeDesktopClipboard,
      },
      selection: {
        text: selectedText,
        files: selectedFiles,
        read: readDesktopSelection,
      },
      apps: {
        frontmost: frontmostApp,
        launch: (appPath) => shell.openPath(expandUserPath(appPath)),
      },
      files: {
        find: findFiles,
        findImages: (roots, options) => findFiles(roots, { ...options, kind: 'image' }),
        findVideos: (roots, options) => findFiles(roots, { ...options, kind: 'video' }),
        findMedia: (roots, options) => findFiles(roots, { ...options, kind: 'media' }),
        openWithApps,
        open: (filePath) => shell.openPath(expandUserPath(filePath)),
        reveal: (filePath) => shell.showItemInFolder(expandUserPath(filePath)),
        preview: quickLookPath,
        readText: (filePath) => fs.readFile(expandUserPath(filePath), 'utf8'),
        toFileUrl: (filePath) => fileUrlForPath(expandUserPath(filePath)),
      },
      shell: {
        openExternal: (url) => shell.openExternal(url),
        exec: runShellCommand,
        script: runShellScript,
        appleScript: (script, options: any = {}) => new Promise((resolve) => {
          if (!hasCapability('applescript')) return resolve({ stdout: '', stderr: 'AppleScript is not available on this OS', exitCode: 1 })
          execFile('osascript', ['-e', String(script)], { timeout: Number(options.timeout || 30_000) }, (error, stdout, stderr) => resolve({ stdout: limitedOutput(stdout, options.outputLimit), stderr: limitedOutput(stderr || error?.message || '', options.outputLimit), exitCode: error ? 1 : 0 }))
        }),
        which: (command) => new Promise((resolve) => {
          execFile('/usr/bin/which', [String(command)], (error, stdout, stderr) => resolve({ stdout: stdout.trim(), stderr: stderr || error?.message || '', exitCode: error ? 1 : 0 }))
        }),
      },
    },
    storage: createExtensionStorage(extension),
    settings: {
      definitions: () => SETTING_DEFINITIONS.map((definition) => ({ ...definition, value: getSetting(definition.id) })),
      get: (id) => getSetting(id),
      set: (id, value) => setSetting(id, value),
      toggle: (id) => {
        const definition = settingDefinition(id)
        if (!definition) throw new Error(`Unknown setting: ${id}`)
        const next = toggledSettingValue(definition, getSetting(id))
        setSetting(id, next)
        return next
      },
    },
    cache: new Map(),
    state: {},
    ai: createExtensionAi(extension),
  }
}

function initNevermindAi() {
  nevermindAi = createNevermindAi({
    agentDir: path.join(app.getPath('userData'), 'pi-agent'),
    workspaceDir: path.join(app.getPath('userData'), 'ai-workspace'),
    extensionsDir,
    extensionApiPath: path.join(app.getAppPath(), 'src', 'docs', 'extension-api.md'),
    skillPath: path.join(app.getAppPath(), 'src', 'resources', 'skills', 'nevermind-extension-builder', 'SKILL.md'),
    reloadExtensions: loadExtensions,
    getActiveChat: () => activeAiChatId ? userState.aiChats[activeAiChatId] || draftAiChats.get(activeAiChatId) || null : null,
    getChat: (chatId) => userState.aiChats[chatId] || draftAiChats.get(chatId) || null,
    markGeneratedExtension: (filePath, chatId) => markGeneratedExtensionForActiveChat(filePath, chatId),
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

async function loadExtensions() {
  extensionRegistry.clear()
  extensionModules.clear()
  extensionRootItemsCache.clear()
  extensionRootItemsRefreshes.clear()
  for (const extension of INTERNAL_EXTENSIONS) registerExtension(extension)

  await fs.mkdir(extensionsDir, { recursive: true })
  const entries = await fs.readdir(extensionsDir, { withFileTypes: true }).catch(() => [])
  for (const entry of entries) {
    if (!entry.isFile() || !entry.name.endsWith('.cjs')) continue
    const fullPath = path.join(extensionsDir, entry.name)
    try {
      delete extensionRequire.cache[extensionRequire.resolve(fullPath)]
      const extension = extensionRequire(fullPath)
      extension.__filePath = fullPath
      extension.__generated = true
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
    .catch((error) => console.warn('Failed to persist clipboard image', error))
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
  clipboardHistory = [
    item,
    ...clipboardHistory.filter((current) => current.id !== item.id),
  ].slice(0, CLIPBOARD_LIMIT)
  scheduleSaveState()
  paletteWindow.win?.webContents.send('clipboard:changed')
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

function unregisterShortcutForAction(actionId) {
  const current = userState.shortcuts[actionId]
  if (current) globalShortcut.unregister(current)
}

async function executeShortcutAction(action) {
  const currentAction = currentActionForStoredShortcut(action)
  const wasVisible = Boolean(paletteWindow.win?.isVisible())
  const result = await executeAction(currentAction, { keepPaletteOpen: true })
  if (result?.view) {
    paletteWindow.showPalette({ skipShownEvent: true, deferReveal: !wasVisible })
    paletteWindow.win?.webContents.send('action:view-open', { ...result, revealWhenReady: !wasVisible, asSibling: wasVisible })
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
  const duplicateFile = `${path.basename(filePath, '.cjs')}-copy-${duplicateId.slice(0, 8)}.cjs`
  const sourceFile = path.basename(filePath)
  const sourceCode = `const source = require('./${sourceFile.replace(/'/g, "\\'")}')\n\nmodule.exports = {\n  ...source,\n  id: ${JSON.stringify(`${extension.id}-copy-${duplicateId.slice(0, 8)}`)},\n  title: ${JSON.stringify(duplicateTitle)},\n  commands: (source.commands || []).map((command) => ({ ...command })),\n}\n`

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
  await loadExtensions()
  registerActionShortcuts()
  const duplicateEntry = Array.from(extensionRegistry.values()).find((candidate) => path.basename(candidate.extension?.__filePath || '') === duplicateFile)
  return { ok: true, message: 'Action duplicated', action: duplicateEntry ? extensionActionFromCommand(duplicateEntry.extension, duplicateEntry.command) : { id: `ai-tweak-extension:${duplicateFile}`, kind: 'ai-tweak-extension', extensionFile: duplicateFile, title: duplicateTitle, subtitle: 'Tweak extension with AI', icon: 'sparkles', score: 0 } }
}

async function removeAiChat(chatId) {
  if (!chatId || !userState.aiChats[chatId]) return { toast: { message: 'AI chat not found', tone: 'error' } }
  await nevermindAi?.reset?.(chatId)
  const chat = userState.aiChats[chatId]
  delete userState.aiChats[chatId]
  for (const actionId of Object.keys(userState.recents || {})) {
    if (actionId === `ai-chat:${chatId}`) delete userState.recents[actionId]
  }
  scheduleSaveState()
  return { view: aiChatsView(), navigation: 'replace', toast: { message: `Removed ${chat.title || chat.query || 'AI chat'}` } }
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
    delete userState.recents[action.id]
    scheduleSaveState()
    await loadExtensions()
    registerActionShortcuts()
    return { ok: true, message: 'Generated extension removed' }
  }

  return { ok: false, message: 'This action cannot be removed' }
}

app.whenReady().then(async () => {
  nativeTheme.themeSource = 'dark'
  prepareAppWindowPolicy()
  registerLocalFileProtocol()
  installPermissionHandlers(isDev)
  updateManager.configure()

  await loadUserState()
  await loadExtensions()
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
  ipcMain.handle('apps:icon', (_event, appPath) => getAppIconDataUrl(appPath))
  ipcMain.handle('palette:set-mode', (_event, mode) => {
    paletteWindow.setPaletteSizeForMode(mode)
    paletteWindow.centerWindow()
  })
  ipcMain.handle('palette:hide', () => paletteWindow.hidePalette())
  ipcMain.handle('palette:shortcut-ready', () => paletteWindow.revealPalette())
})

app.on('activate', () => paletteWindow.showPalette())
app.on('before-quit', () => {
  nevermindApp.isQuiting = true
})
app.on('will-quit', () => {
  nevermindApp.isQuiting = true
  if (app.isReady()) globalShortcut.unregisterAll()
  updateManager.clearTimers()
  for (const watcher of appWatchers) watcher.close()
})

if (!isDev) {
  const gotLock = app.requestSingleInstanceLock()
  if (!gotLock) app.quit()
  else app.on('second-instance', () => paletteWindow.showPalette())
}
