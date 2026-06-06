import { app, BrowserWindow, globalShortcut, ipcMain, shell, clipboard, nativeImage, nativeTheme, protocol, net, systemPreferences, screen, dialog, powerMonitor } from 'electron'
import electronUpdater from 'electron-updater'
import fsSync from 'node:fs'
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import crypto from 'node:crypto'
import { spawn, execFile } from 'node:child_process'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { Readable } from 'node:stream'
import { clipboardFilePath as readClipboardFilePath, clipboardFilePaths, clipboardItemSubtitle, clipboardItemTitle, normalizeClipboardHistory } from './clipboard-utils'
import { configureLocalFileUrlSecret, expandUserPath, extensionForPath, fileUrlForPath, IMAGE_EXTENSIONS, isImagePath, isVideoPath, LOCAL_FILE_PROTOCOL, LOCAL_THUMB_PROTOCOL, thumbnailUrlForPath, verifyLocalFileToken, VIDEO_EXTENSIONS } from './file-utils'
import { createNevermindAi } from './ai'
import { signInToNevermind, getNevermindAuth, signOutFromNevermind } from './nevermind-auth'
import { currentNevermindCompatibilityManifest, onNevermindCompatibilityChanged, warmNevermindCompatibilityCache } from './nevermind-compatibility'
import { initSentry } from './sentry'

initSentry()
import { createPaletteWindowController, installPermissionHandlers } from './palette-window'
import { settingDefinition, SETTING_DEFINITIONS, settingValue, toggledSettingValue } from './settings'
import { calculate, getUrlFromQuery, hashValue, normalize, score, scoreNormalized } from './search-utils'
import { isSpotlightAccelerator, normalizeAccelerator } from './shortcut-utils'
import { autoUpdatesUnavailableMessage, captureScreenImage, executeSystemBuiltin, fileDateAddedMs, frontmostApp, getLaunchAtLoginEnabled, hasCapability, keyboardSettingsSubtitle, launchApp as launchOsApp, osLabel, pasteIntoFrontmostApp, prepareAppWindowPolicy, quickLookTitle, recognizeTextInImage, reservedPaletteShortcutName, revealPathTitle, scanApps, selectedFilePaths, selectedText, setLaunchAtLoginEnabled, settingsTitle, typeTextIntoFrontmostApp, watchApps } from './os'
import { createUpdateManager } from './update-manager'
import { JobRegistry, type JobSnapshot } from './jobs'
import { isNewerVersion as isVersionNewerThan } from './version-utils'
import { configureLogger, extensionLogger, info as logInfo, warn as logWarn, error as logError, debug as loggerDebug } from './logger'
import { LocalLearningStore, type LearningKind } from './learning-store'
import { measureDebugPerformance, measureDebugPerformanceSync, markDebugPerformance, summarizeDebugValue } from './debug-performance'
import { canCustomizeCommandAction } from '../model'

const { autoUpdater } = electronUpdater
const isDev = Boolean(process.env.ELECTRON_RENDERER_URL)
configureLogger(isDev)

const updateManager = createUpdateManager(autoUpdater as any)
const rendererUrl = process.env.ELECTRON_RENDERER_URL
const preloadPath = path.join(__dirname, '..', 'preload', 'preload.cjs')
const rendererIndexPath = path.join(__dirname, '..', 'renderer', 'index.html')
const paletteWindow = createPaletteWindowController({
  isDev: Boolean(rendererUrl),
  preloadPath,
  rendererUrl,
  rendererIndexPath,
  getPaletteHotkey: () => String(getPaletteHotkey()),
})

const CLIPBOARD_LIMIT = 300
const FILE_RESULT_LIMIT = 6
const CLIPBOARD_POLL_INTERVAL_MS = 1000
const CLIPBOARD_LAST_HOUR_MS = 60 * 60_000
const CLIPBOARD_LAST_DAY_MS = 24 * CLIPBOARD_LAST_HOUR_MS
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
const LEARNING_RULES_FILENAME = 'ai-learnings.md'
const LEGACY_LEARNING_RULES_FILENAME = 'ai-learnings.json'
const LEARNING_TRACES_FILENAME = 'ai-learning-traces.json'

protocol.registerSchemesAsPrivileged([
  { scheme: LOCAL_FILE_PROTOCOL, privileges: { standard: true, secure: true, supportFetchAPI: true } },
  { scheme: LOCAL_THUMB_PROTOCOL, privileges: { standard: true, secure: true, supportFetchAPI: true } },
])

type AnyRecord = Record<string, any>

type NevermindApp = typeof app & { isQuiting?: boolean }

const nevermindApp = app as NevermindApp

function bundledResourcePath(...relativePath) {
  const candidates = [
    path.join(app.getAppPath(), 'src', 'resources', ...relativePath),
    path.join(process.resourcesPath, 'app.asar', 'src', 'resources', ...relativePath),
    path.join(process.resourcesPath, 'src', 'resources', ...relativePath),
  ]
  return candidates.find((candidate) => fsSync.existsSync(candidate)) || candidates[0]
}

let appIndex: any[] = []
let fileIndex: any[] = []
let clipboardHistory: any[] = []
const suppressedClipboardItemIds = new Map<string, number>()
let statePath = ''
let iconCacheDir = ''
let clipboardImagesDir = ''
let extensionsDir = ''
let extensionStorageDir = ''
let extensionCacheDir = ''
let learningRulesPath = ''
let legacyLearningRulesPath = ''
let learningTracesPath = ''
let saveTimer: NodeJS.Timeout | undefined
let appIndexTimer: NodeJS.Timeout | undefined
let appWatchers: Array<{ close: () => unknown }> = []
let extensionFileWatchers: Array<{ close: () => unknown }> = []
let clipboardWatcherLastId = ''
let frontmostWatcherLastId = ''
const jobRegistry = new JobRegistry()
let nevermindAi: any
let learningStore: LocalLearningStore | null = null
const learningReviewJobs = new Map<string, Promise<void>>()
let activeAiChatId: string | undefined
const draftAiChats = new Map<string, AnyRecord>()
type ExtensionWindowRecord = { id: string; win: BrowserWindow; view: any; options: any }
const extensionWindows = new Map<string, ExtensionWindowRecord>()
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
  jobSettings: {},
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

function settingIsAvailable(definition: any) {
  return !definition?.capability || hasCapability(definition.capability)
}

function availableSettingDefinitions() {
  return SETTING_DEFINITIONS.filter(settingIsAvailable)
}

function getSetting(id: any) {
  const definition = settingDefinition(String(id))
  if (!definition || !settingIsAvailable(definition)) return undefined
  if (definition.id === 'startAtLogin') return getLaunchAtLoginEnabled()
  return settingValue(userState.settings, definition.id)
}

function setSetting(id: any, value: any) {
  const definition = settingDefinition(String(id))
  if (!definition) return { ok: false, message: 'Setting not found' }
  if (!settingIsAvailable(definition)) return { ok: false, message: `${definition.title} is not available on ${osLabel()}` }
  if (definition.id === 'startAtLogin') {
    const result = setLaunchAtLoginEnabled(Boolean(value))
    if (!result.ok) return result
  }
  if (!userState.settings) userState.settings = {}
  userState.settings[definition.id] = value
  scheduleSaveState()
  invalidateExtensionRootItems()
  patchSettingsView(definition.id)
  return { ok: true, message: `${definition.title} updated` }
}

function aiLearningMetadata(chatId: string) {
  const chat = userState.aiChats[chatId] || draftAiChats.get(chatId)
  return {
    query: chat?.query,
    title: chat?.title,
    contextExtensionFile: chat?.contextExtensionFile,
    extensionFiles: chatTouchedExtensionFiles(chat),
  }
}

function relevantLearningContext(message: string, chatId: string) {
  if (!learningStore) return ''
  const metadata = aiLearningMetadata(chatId)
  const learnings = learningStore.relevantLearnings({
    message,
    query: metadata.query,
    contextExtensionFile: metadata.contextExtensionFile,
    limit: 4,
  })
  if (!learnings.length) return ''
  const lines = learnings.map((learning) => `- ${learning.summary}${learning.appliesWhen ? ` When relevant: ${learning.appliesWhen}.` : ''}`)
  return `\n\nLocal machine learnings for future extension builds:\n${lines.join('\n')}`
}

function normalizedLearningReview(response: string) {
  const raw = String(response || '').trim()
  if (!raw) return []
  const fencedMatch = raw.match(/```(?:json)?\s*([\s\S]*?)```/i)
  const candidate = (fencedMatch?.[1] || raw).trim()
  const parsed = JSON.parse(candidate) as { learnings?: Array<{ kind?: string; summary?: string; appliesWhen?: string; keywords?: string[]; confidence?: string; evidence?: string }> }
  return (parsed.learnings || []).map((learning) => ({
    kind: learning.kind === 'workflow' || learning.kind === 'preference' ? learning.kind : 'environment',
    summary: String(learning.summary || '').trim(),
    appliesWhen: learning.appliesWhen ? String(learning.appliesWhen).trim() : undefined,
    keywords: Array.isArray(learning.keywords) ? learning.keywords.map(String) : [],
    confidence: learning.confidence === 'low' || learning.confidence === 'high' ? learning.confidence : 'medium',
    evidence: learning.evidence ? String(learning.evidence).trim() : undefined,
  })).filter((learning) => learning.summary)
}

function learningReviewPrompt(snapshot: any) {
  return `You are curating the canonical user learning rules for Nevermind's extension-building AI on this machine.

Analyze the conversation and trace for friction patterns, including misunderstandings, too many iterations, repeated back-and-forth, retries, environment quirks, tool misuse, or anything else that would help future extension-building chats go better.

Your job is to update the current learning set, not append blindly. Merge overlapping rules, rewrite weak rules into stronger generic ones, remove stale or redundant rules, and keep the final set small. Prefer a tiny set of generic durable rules over many specific ones.

Generic means durable principles for future extension builds, not exact API/class workarounds, one extension's domain model, one bug's workaround, or transient host/client behavior. If a finding only applies to the current extension or technology, lift it to a broader principle or omit it.

Do not include product feedback about improving the extension API or builder prompt. Do not include volatile facts like absolute file paths, specific chat titles, or one-off task details unless they are stable machine conventions.

Return the full resulting rule set as strict JSON only with this shape:
{"learnings":[{"kind":"environment|workflow|preference","summary":"short generic rule","appliesWhen":"optional short trigger","keywords":["keyword"],"confidence":"low|medium|high","evidence":"brief rationale"}]}

Keep the final list as small as possible. Returning {"learnings":[]} is valid when no durable user learnings should exist.

Trace:
${JSON.stringify(snapshot, null, 2)}`
}

function recordLearningReview(chatId: string) {
  const jobId = `ai.learning.review.${chatId}`
  if (!learningStore?.shouldReview(chatId) || learningReviewJobs.has(chatId) || jobRegistry.snapshot().some((job) => job.id === jobId && job.running) || !nevermindAi?.ask) return
  const snapshot = learningStore.reviewSnapshot(chatId)
  if (!snapshot) return
  jobRegistry.register({
    id: jobId,
    title: `AI Learning Review: ${chatId}`,
    owner: 'host',
    scope: 'ai',
    timeoutMs: 60_000,
    run: async () => {
      try {
        const response = await nevermindAi.ask(learningReviewPrompt(snapshot), {
          system: 'You maintain a tiny canonical set of generic user learnings for future Nevermind extension-building chats. Merge and rewrite rules instead of appending; omit one-off implementation details. Return strict JSON and keep the final set minimal.',
        })
        const learnings = normalizedLearningReview(response)
        learningStore?.replaceLearningsFromReview(chatId, learnings as Array<{ kind: LearningKind; summary: string; appliesWhen?: string; keywords?: string[]; confidence?: "low" | "medium" | "high"; evidence?: string }>)
      } catch (error) {
        logWarn('ai.learning.review.failed', { chatId, error: error instanceof Error ? error.message : String(error) }, { source: 'host', scope: 'ai' })
        throw error
      } finally {
        learningReviewJobs.delete(chatId)
      }
    },
  })
  const job = jobRegistry.run(jobId, 'ai-chat-exited').then(() => undefined).catch(() => undefined)
  learningReviewJobs.set(chatId, job)
}

const appIconCache = new Map<string, string | null>()
const appIconLoadPromises = new Map<string, Promise<string | null>>()
const pendingAppIconPaths = new Set<string>()
const appIconWaiters = new Map<string, Array<(result: string | null) => void>>()
const pendingThumbnailPaths = new Map<string, string>()
const extensionActionRegistry = new Map<string, any>()
const extensionModules = new Map<string, any>()
let fixtureExtensions: any[] = []
const extensionRootItemsCache = new Map<string, { updatedAt: number; items: any[] }>()
const extensionRootItemsRefreshes = new Map<string, Promise<any[]>>()
const extensionStorageRefreshes = new Map<string, Promise<any>>()
const extensionActionHandlers = new Map<string, any>()
const viewActionExecutionRecords = new Map<string, { action: any; createdAt: number }>()
const rootActionExecutionRecords = new Map<string, { action: any; createdAt: number }>()
const viewRefreshRecords = new Map<string, { entry: any; action: any | null; viewId?: string; mode?: any; createdAt: number; running?: Promise<any> | null; failureCount: number; backoffUntil?: number }>()
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

const ACTION_EXECUTION_TTL_MS = 30 * 60_000
const ACTION_EXECUTION_MAX_RECORDS = 2_000
const RENDERER_ONLY_VIEW_ACTION_TYPES = new Set(['recordShortcut', 'previewClipboardItem'])
const TOKEN_REQUIRED_VIEW_ACTION_TYPES = new Set([
  'runExtensionAction',
  'openPath',
  'openWith',
  'pasteText',
  'copyImage',
  'trash',
  'removeClipboardHistory',
  'shellExec',
  'shellScript',
  'lockScreen',
  'sleepSystem',
  'restartSystem',
  'openSystemSettings',
  'openKeyboardSettings',
  'quitApp',
  'checkForUpdates',
  'downloadUpdate',
  'installUpdate',
  'toggleSetting',
])

function clonePlain(value) {
  if (!value) return value
  return structuredClone(value)
}

function withoutExecutionId(value) {
  if (!value || typeof value !== 'object') return value
  const { executionId, ...rest } = value
  return rest
}

function pruneExecutionRecords(store: Map<string, { createdAt: number }>) {
  const now = Date.now()
  for (const [id, record] of store) {
    if (now - record.createdAt > ACTION_EXECUTION_TTL_MS) store.delete(id)
  }
  while (store.size > ACTION_EXECUTION_MAX_RECORDS) {
    const oldest = store.keys().next().value
    if (!oldest) break
    store.delete(oldest)
  }
}

function registerViewActionForRenderer(action) {
  if (!action || typeof action !== 'object') return action
  if (RENDERER_ONLY_VIEW_ACTION_TYPES.has(String(action.type || ''))) return action
  pruneExecutionRecords(viewActionExecutionRecords)
  const executionId = crypto.randomUUID()
  const stored = clonePlain(withoutExecutionId(action))
  viewActionExecutionRecords.set(executionId, { action: stored, createdAt: Date.now() })
  return { ...action, executionId }
}

function registerRootActionForRenderer(action) {
  if (!action || typeof action !== 'object') return action
  pruneExecutionRecords(rootActionExecutionRecords)
  const executionId = crypto.randomUUID()
  const stored = clonePlain(withoutExecutionId(action))
  rootActionExecutionRecords.set(executionId, { action: stored, createdAt: Date.now() })
  return { ...action, executionId }
}

function pruneViewRefreshRecords() {
  const now = Date.now()
  for (const [id, record] of viewRefreshRecords) {
    if (now - record.createdAt > ACTION_EXECUTION_TTL_MS) viewRefreshRecords.delete(id)
  }
  while (viewRefreshRecords.size > ACTION_EXECUTION_MAX_RECORDS) {
    const oldest = viewRefreshRecords.keys().next().value
    if (!oldest) break
    viewRefreshRecords.delete(oldest)
  }
}

function registerViewRefreshForRenderer(refresh, entry, view) {
  if (!refresh || typeof refresh !== 'object') return refresh
  if (refresh.id && !refresh.action) return refresh
  pruneViewRefreshRecords()
  const { action, ...safeRefresh } = refresh
  const refreshId = crypto.randomUUID()
  const normalizedAction = action ? normalizeViewAction(action, entry) : null
  viewRefreshRecords.set(refreshId, {
    entry,
    action: normalizedAction ? withoutExecutionId(normalizedAction) : null,
    viewId: view?.id,
    mode: refresh.mode,
    createdAt: Date.now(),
    running: null,
    failureCount: 0,
  })
  return { ...safeRefresh, id: refreshId }
}

function mergeRendererActionInput(storedAction, rendererAction) {
  const merged = { ...storedAction }
  if (rendererAction && typeof rendererAction === 'object') {
    if ('formValues' in rendererAction) merged.formValues = rendererAction.formValues
    if ('selectedItemId' in rendererAction) merged.selectedItemId = rendererAction.selectedItemId
    if ('value' in rendererAction) merged.value = rendererAction.value
    if ('text' in rendererAction && !('text' in storedAction)) merged.text = rendererAction.text
  }
  return merged
}

function resolveRootActionForIpc(action) {
  if (!action || typeof action !== 'object') return action
  const record = action.executionId ? rootActionExecutionRecords.get(String(action.executionId)) : null
  if (record) return clonePlain(record.action)
  const fallback = withoutExecutionId(action)
  if (fallback.kind === 'extension-root-item' && fallback.rootAction) throw new Error('Untrusted extension root action')
  return fallback
}

function resolveViewActionForIpc(action) {
  if (!action || typeof action !== 'object') return action
  const record = action.executionId ? viewActionExecutionRecords.get(String(action.executionId)) : null
  if (record) return mergeRendererActionInput(clonePlain(record.action), action)
  const fallback = withoutExecutionId(action)
  if (fallback.type === 'nativeAction') return { ...fallback, nativeAction: resolveRootActionForIpc(fallback.nativeAction) }
  if (TOKEN_REQUIRED_VIEW_ACTION_TYPES.has(String(fallback.type || ''))) throw new Error(`Untrusted ${fallback.type} action`)
  return fallback
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

const INTERNAL_EXTENSION_FACTORIES: Array<() => any> = [createSystemExtension, createPlacesExtension, createCalculatorExtension, createWebSearchExtension, createClipboardExtension, createAppsExtension, createFilesExtension, createAiBuilderExtension, createUpdatesExtension, createKeyboardShortcutsExtension, createSettingsExtension, createBackgroundTasksExtension, createAccountExtension]
const REQUIRED_INTERNAL_EXTENSIONS = ['nevermind.system', 'nevermind.places', 'nevermind.calculator', 'nevermind.web', 'nevermind.clipboard', 'nevermind.apps', 'nevermind.files', AI_BUILDER_EXTENSION_ID, 'nevermind.updates', 'nevermind.shortcuts', 'nevermind.settings', 'nevermind.background-tasks', 'nevermind.account']
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

async function loadAppIconDataUrl(appPath) {
  return measureDebugPerformance('apps.icon.load', { appPath }, async () => {
    try {
      const cachePath = path.join(iconCacheDir, `${hashValue(appPath)}.png`)
      const cached = await fs.readFile(cachePath).catch(() => null)
      if (cached) {
        markDebugPerformance('apps.icon.cache-hit', { appPath })
        return `data:image/png;base64,${cached.toString('base64')}`
      }

      const { fileIconToBuffer } = await import('file-icon')
      const png = Buffer.from(await fileIconToBuffer(appPath, { size: 64 }))
      await fs.mkdir(iconCacheDir, { recursive: true })
      await fs.writeFile(cachePath, png).catch(() => {})
      return `data:image/png;base64,${png.toString('base64')}`
    } catch (error) {
      logWarn('appIcon.load.failed', { appPath, error }, { source: 'host', scope: 'apps' })
      return null
    }
  })
}

async function processPendingAppIcons() {
  const paths = Array.from(pendingAppIconPaths).slice(0, 20)
  for (const appPath of paths) pendingAppIconPaths.delete(appPath)
  await Promise.all(paths.map(async (appPath) => {
    const result = await loadAppIconDataUrl(appPath)
    appIconCache.set(appPath, result)
    for (const resolve of appIconWaiters.get(appPath) || []) resolve(result)
    appIconWaiters.delete(appPath)
    appIconLoadPromises.delete(appPath)
  }))
  if (pendingAppIconPaths.size) jobRegistry.schedule('cache.app-icons', 'icon-backlog', 50)
}

async function getAppIconDataUrl(appPath) {
  return measureDebugPerformance('apps.icon.get', { appPath, alwaysLog: true }, async () => {
    if (!hasCapability('app-icons') || !appPath || !appPath.endsWith('.app')) return null
    if (appIconCache.has(appPath)) {
      markDebugPerformance('apps.icon.memory-cache-hit', { appPath })
      return appIconCache.get(appPath)
    }
    const inFlight = appIconLoadPromises.get(appPath)
    if (inFlight) {
      markDebugPerformance('apps.icon.in-flight-hit', { appPath })
      return inFlight
    }

    pendingAppIconPaths.add(appPath)
    const promise = new Promise<string | null>((resolve) => {
      const waiters = appIconWaiters.get(appPath) || []
      waiters.push(resolve)
      appIconWaiters.set(appPath, waiters)
    })
    appIconLoadPromises.set(appPath, promise)
    jobRegistry.schedule('cache.app-icons', 'icon-request', 0)
    return promise
  })
}

function isFixtureExtension(extension) {
  return Boolean(extension?.__fixture)
}

function visibleExtensions() {
  return Array.from(extensionModules.values()).filter((extension) => !isFixtureExtension(extension))
}

function visibleExtensionActionEntries() {
  return Array.from(extensionActionRegistry.values()).filter((entry) => !isFixtureExtension(entry.extension))
}

function extensionCommandActionId(extension, command) {
  return `extension:${extension.id}:${command.id}`
}


function getOrCreateAiChat(query, options: any = {}) {
  const trimmed = query.trim()
  const baseId = hashValue(trimmed)
  const current = userState.aiChats[baseId]
  if (current && !options.fresh) return current
  const id = current && options.fresh ? hashValue(`${trimmed}:${Date.now()}:${crypto.randomUUID()}`) : baseId
  const item = aiChatItem(id, trimmed)
  userState.aiChats[id] = item
   learningStore?.upsertTraceMetadata(id, aiLearningMetadata(id))
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
   learningStore?.upsertTraceMetadata(chatId, aiLearningMetadata(chatId))
  scheduleSaveState()
  invalidateExtensionRootItems()
  return draft
}

function appendAiChatMessage(chatId, role, content) {
  const chat = userState.aiChats[chatId]
  if (!chat || !content) return
  chat.messages = [...(chat.messages || []), { role, content }].slice(-100)
  chat.updatedAt = Date.now()
   learningStore?.appendMessage(chatId, role, content, aiLearningMetadata(chatId))
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
   learningStore?.appendAssistantDelta(chatId, text, aiLearningMetadata(chatId))
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
  const normalizedPatch = normalizeViewPatch(patch, null)
  structuredClone(normalizedPatch)
  paletteWindow.win?.webContents.send('view:patch', { viewId, patch: normalizedPatch })
}

function aiBuilderRegistryEntry() {
  return extensionActionRegistry.get(`${AI_BUILDER_EXTENSION_ID}:ai-chats`) || { extension: createAiBuilderExtension(), command: { id: 'ai-chats', title: 'AI Chats' } }
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
  learningStore?.upsertTraceMetadata(chat.id, aiLearningMetadata(chat.id))
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
    learningStore?.upsertTraceMetadata(existing.id, aiLearningMetadata(existing.id))
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
  learningStore?.upsertTraceMetadata(id, aiLearningMetadata(id))
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

function clipboardHistoryRemovalAction(range, title, message, itemId = '') {
  return {
    type: 'removeClipboardHistory',
    title,
    clipboardHistoryRange: range,
    clipboardHistoryItemId: itemId,
    style: 'destructive',
    requiresConfirmation: true,
    confirmLabel: title,
    confirmMessage: message,
  }
}

function clipboardHistoryRemovalActions(item: any = null) {
  if (clipboardHistory.length === 0) return []
  const actions: any[] = []
  if (item?.id) actions.push(clipboardHistoryRemovalAction('item', 'Remove Item', `Remove “${clipboardItemTitle(item)}” from clipboard history?`, item.id))
  actions.push(
    clipboardHistoryRemovalAction('last-hour', 'Remove Entries from Last Hour', 'Remove clipboard history entries copied in the last hour?'),
    clipboardHistoryRemovalAction('last-day', 'Remove Entries from Last Day', 'Remove clipboard history entries copied in the last day?'),
    clipboardHistoryRemovalAction('all', 'Remove All Entries', 'Remove all clipboard history entries?'),
  )
  return actions
}

const CLIPBOARD_ITEM_APPEARANCE = { foreground: 'blue' } as const

function clipboardRootItem(item) {
  return {
    id: `clipboard:${item.id}`,
    title: clipboardItemTitle(item),
    subtitle: clipboardItemSubtitle(item),
    icon: 'clipboard',
    image: item.thumbnailUrl,
    score: 60,
    lastUsed: item.createdAt || 0,
    appearance: CLIPBOARD_ITEM_APPEARANCE,
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
    appearance: CLIPBOARD_ITEM_APPEARANCE,
    primaryAction: copyAction,
    actionPanel: {
      sections: [
        { actions: [previewAction, copyAction, pasteAction].filter(Boolean) },
        { title: 'Manage History', actions: clipboardHistoryRemovalActions(item) },
      ],
    },
  }
}

function clipboardHistoryItems() {
  return clipboardHistory.slice(0, CLIPBOARD_LIMIT).map(clipboardHistoryItem)
}

function clipboardHistorySnapshot(options: any = {}) {
  return measureDebugPerformanceSync('clipboard.snapshot', { queryLength: String(options.query || '').length, clipboardCount: clipboardHistory.length, limit: options.limit }, () => {
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
  })
}

function clipboardHistoryRemovalEntries(action) {
  const range = action?.clipboardHistoryRange || 'item'
  const now = Date.now()
  const types = new Set(Array.isArray(action?.types) ? action.types.map(String) : [])
  const typeMatches = (entry) => !types.size || types.has(entry.type)
  if (range === 'item') return clipboardHistory.filter((entry) => entry.id === action?.clipboardHistoryItemId && typeMatches(entry))
  if (range === 'ids') {
    const ids = new Set(Array.isArray(action?.clipboardHistoryItemIds) ? action.clipboardHistoryItemIds : [action?.clipboardHistoryItemId].filter(Boolean))
    return clipboardHistory.filter((entry) => ids.has(entry.id) && typeMatches(entry))
  }
  if (range === 'last-hour') return clipboardHistory.filter((entry) => (entry.createdAt || 0) >= now - CLIPBOARD_LAST_HOUR_MS && typeMatches(entry))
  if (range === 'last-day') return clipboardHistory.filter((entry) => (entry.createdAt || 0) >= now - CLIPBOARD_LAST_DAY_MS && typeMatches(entry))
  if (range === 'older-than') return clipboardHistory.filter((entry) => (entry.createdAt || 0) < now - Math.max(0, Number(action?.olderThanMs || 0)) && typeMatches(entry))
  if (range === 'all') return clipboardHistory.filter(typeMatches)
  return []
}

function clipboardHistoryGet(id) {
  return clipboardHistorySnapshot().find((entry) => entry.id === id) || null
}

function removeClipboardHistoryByAction(action) {
  const removed = clipboardHistoryRemovalEntries(action)
  if (removed.length === 0) return 0
  const removedIds = new Set(removed.map((entry) => entry.id))
  clipboardHistory = clipboardHistory.filter((entry) => !removedIds.has(entry.id))
  scheduleSaveState()
  invalidateExtensionRootItems()
  paletteWindow.win?.webContents.send('clipboard:changed')
  return removed.length
}

function clipboardHistoryRemovedMessage(count) {
  return count === 1 ? 'Removed 1 clipboard item' : `Removed ${count} clipboard items`
}

function removeClipboardHistoryEntries(action) {
  const removed = removeClipboardHistoryByAction(action)
  if (removed === 0) return { toast: { message: 'No matching clipboard items to remove' } }
  return { view: clipboardHistoryView(), navigation: 'replace', toast: { message: clipboardHistoryRemovedMessage(removed) } }
}

function viewRefreshAction(itemsBuilder) {
  return {
    type: 'runExtensionAction',
    title: 'Refresh',
    __handler: () => ({ patch: { mode: 'replace', items: itemsBuilder() } }),
  }
}

function clipboardHistoryView() {
  const actions = clipboardHistoryRemovalActions()
  return {
    type: 'list',
    id: 'clipboard-history',
    title: 'Clipboard History',
    presentation: 'root',
    searchBarPlaceholder: 'Search Clipboard History',
    emptyView: { title: 'No clipboard items found.', subtitle: 'Copy text or images and they will appear here.' },
    actions,
    actionPanel: actions.length ? { sections: [{ title: 'Manage History', actions }] } : undefined,
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
  const accessoryText = definition.type === 'boolean' ? (value ? 'On' : 'Off') : definition.type === 'shortcut' ? String(value || '') : String(value)
  const shortcutInput = definition.id === 'paletteHotkey'
    ? { scope: 'palette', title: 'Change Shortcut', shortcut: String(value || '') }
    : { action: { id: '__hyper-key__' }, title: 'Change Hyper Key', shortcut: String(value || '') }
  const primaryAction = definition.type === 'shortcut'
    ? buildRecordShortcutAction(shortcutInput, {})
    : { type: 'toggleSetting', title: value ? 'Turn Off' : 'Turn On', settingId: definition.id }
  return { id: `setting:${definition.id}`, accessories: accessoryText ? [{ text: accessoryText }] : [], primaryAction, actionPanel: { sections: [{ actions: [primaryAction] }] } }
}

function settingsItems() {
  return availableSettingDefinitions().map((definition) => ({
    id: `setting:${definition.id}`,
    title: definition.title,
    subtitle: definition.description,
    icon: definition.icon || 'settings',
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
  if (!definition || !settingIsAvailable(definition)) return
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

let activeNevermindBaseUrl: string | null = null

function safeExternalUpdateUrl(raw?: string) {
  if (!raw) return null
  try {
    const parsed = new URL(raw)
    return parsed.protocol === 'https:' || parsed.protocol === 'http:' ? parsed.toString() : null
  } catch {
    return null
  }
}

function updateActionForCompatibilityPrompt(updateUrl?: string) {
  const safeUrl = safeExternalUpdateUrl(updateUrl)
  const downloadedInfo = isNewerVersion(updateManager.state.downloadedInfo?.version) ? updateManager.state.downloadedInfo : null
  const availableInfo = isNewerVersion(updateManager.state.availableInfo?.version) ? updateManager.state.availableInfo : null
  if (downloadedInfo) return { type: 'installUpdate', title: `Install Nevermind ${downloadedInfo.version || ''}`.trim() }
  if (availableInfo) return { type: 'downloadUpdate', title: `Download Nevermind ${availableInfo.version || ''}`.trim() }
  if (updateManager.canUseAutoUpdates()) return { type: 'checkForUpdates', title: 'Check for Update' }
  return safeUrl ? { type: 'openUrl', title: 'Download Update', url: safeUrl } : undefined
}

function compatibilityPromptAction() {
  if (!activeNevermindBaseUrl) return null
  const manifest = currentNevermindCompatibilityManifest(activeNevermindBaseUrl)
  if (manifest?.client?.compatible !== false) return null
  const version = manifest.desktop?.latestVersion || manifest.desktop?.minimumSupportedVersion || ''
  const primaryAction = updateActionForCompatibilityPrompt(manifest.desktop?.updateUrl)
  return {
    id: 'updates:compatibility-required',
    title: 'Update Nevermind',
    subtitle: version ? `Nevermind ${version} or newer is required for backend compatibility` : 'This version is no longer supported by the backend',
    icon: 'restart',
    score: 1_100,
    primaryAction,
    actionPanel: primaryAction ? { sections: [{ actions: [primaryAction] }] } : undefined,
  }
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
  return measureDebugPerformance('search.actions', { queryLength: String(query || '').length, clipboardOnly: Boolean(options.clipboardOnly), alwaysLog: true }, async () => {
    const q = query.trim()

    if (options.clipboardOnly) {
      return measureDebugPerformanceSync('search.clipboard-only', { queryLength: q.length, clipboardCount: clipboardHistory.length }, () => clipboardHistory
        .map(clipboardRootItem)
        .filter((item) => q ? rankAction(item, q) : true)
        .sort((a, b) => q ? b.score - a.score || b.lastUsed - a.lastUsed : b.lastUsed - a.lastUsed)
        .slice(0, CLIPBOARD_LIMIT)
        .map(prepareRootActionForRenderer))
    }

    const results = []
    const contributedItems = await measureDebugPerformance(q ? 'search.extensions.query' : 'search.extensions.root', { queryLength: q.length, extensionCount: visibleExtensions().length, alwaysLog: true }, () => q ? extensionSearchActions(q) : extensionRootActions())
    for (const item of contributedItems) {
      const ranked = item.__ranked ? withShortcutHint(item) : rankAction(withShortcutHint(item), q)
      if (ranked) results.push(ranked)
    }

    const entries = visibleExtensionActionEntries()
    measureDebugPerformanceSync('search.rank-registered-actions', { queryLength: q.length, actionCount: entries.length }, () => {
      for (const entry of entries) {
        const action = extensionActionFromContribution(entry)
        const ranked = action ? rankAction(withShortcutHint(action), q) : null
        if (ranked) results.push(ranked)
      }
    })

    const sorted = measureDebugPerformanceSync('search.sort-prepare-clone', { queryLength: q.length, resultCount: results.length }, () => results
      .sort((a, b) => {
        return b.score - a.score || b.lastUsed - a.lastUsed || a.title.localeCompare(b.title)
      })
      .slice(0, 30)
      .map(prepareRootActionForRenderer))
    structuredClone(sorted)
    markDebugPerformance('search.actions.result', { queryLength: q.length, contributedCount: contributedItems.length, rankedCount: results.length, resultCount: sorted.length })
    return sorted
  })
}

function invalidateExtensionRootItems() {
  extensionRootItemsCache.clear()
  paletteWindow.win?.webContents.send('root-items:changed')
}

function broadcastAuthChanged(status: { authed: boolean; email?: string }) {
  paletteWindow.win?.webContents.send('nevermind:auth-changed', status)
}

function prepareActionPanelForRenderer(panel) {
  if (!panel?.sections) return panel
  return {
    ...panel,
    sections: panel.sections.map((section) => {
      const { lazyActions, ...rest } = section
      return {
        ...rest,
        actions: [...(section.actions || []), ...(section.lazyActions || [])].map((action) => normalizeViewAction(action, null)).filter(Boolean),
      }
    }),
  }
}

function prepareRootActionForRenderer(action) {
  if (!action || typeof action !== 'object') return action
  return registerRootActionForRenderer({
    ...action,
    primaryAction: normalizeViewAction(action.primaryAction, null),
    rootAction: normalizeViewAction(action.rootAction, null),
    actionPanel: prepareActionPanelForRenderer(action.actionPanel),
  })
}

function invalidateExtensionRootItemsForExtension(extension) {
  const cacheKey = extension.__filePath || extension.id
  extensionRootItemsCache.delete(cacheKey)
  paletteWindow.win?.webContents.send('root-items:changed')
}

function runInBackground(task) {
  setImmediate(() => {
    Promise.resolve().then(task).catch((error) => logError('backgroundAction.failed', error, { source: 'host' }))
  })
}

async function executeAction(action, options: any = {}) {
  if (!action) return
  recordRecent(action)

  switch (action.kind) {
    case 'open-keyboard-settings':
      runInBackground(openSystemKeyboardSettings)
      break
    case 'extension-root-item':
    case 'extension-action': {
      const result = await executeExtensionRootItem(action)
      if (result) return result
      break
    }
    case 'extension-command': {
      const upgraded = currentActionForStoredShortcut(action)
      if (upgraded !== action) return executeAction(upgraded, options)
      break
    }
  }

  if (!options.keepPaletteOpen) paletteWindow.hidePalette()
}

function extensionActionEntryForAction(action) {
  const registeredActionId = action?.registeredActionId || action?.commandId
  const direct = extensionActionRegistry.get(`${action.extensionId}:${registeredActionId}`)
  if (direct) return direct

  if (action?.extensionFile) {
    const fileMatches = Array.from(extensionActionRegistry.values()).filter((entry) => path.basename(entry.extension.__filePath || '') === action.extensionFile)
    if (fileMatches.length === 1) return fileMatches[0]
  }

  if (action?.aiChatId) {
    const files = chatTouchedExtensionFiles(userState.aiChats[action.aiChatId])
    const chatMatches = Array.from(extensionActionRegistry.values()).filter((entry) => files.includes(path.basename(entry.extension.__filePath || '')))
    if (chatMatches.length === 1) return chatMatches[0]
  }

  const matches = Array.from(extensionActionRegistry.values()).filter((entry) => entry.extension.id === action?.extensionId)
  return matches.length === 1 ? matches[0] : null
}

function extensionEntryForAction(action) {
  return extensionActionEntryForAction(action)
}

function extensionModuleForAction(action) {
  const entry = extensionActionEntryForAction(action)
  if (entry?.extension) return entry.extension
  if (!action?.extensionFile) return null
  return Array.from(extensionModules.values()).find((extension) => path.basename(extension.__filePath || '') === action.extensionFile) || null
}

function currentActionForStoredShortcut(action) {
  if (action?.kind === 'extension-command') {
    const entry = extensionActionEntryForAction(action)
    return entry ? extensionActionFromContribution(entry) : action
  }
  if (action?.kind === 'extension-action') {
    const entry = extensionActionEntryForAction(action)
    return entry ? extensionActionFromContribution(entry) : action
  }
  return action
}

async function executeExtensionRootItem(action) {
  return measureDebugPerformance('extension.root-item.execute', { action: summarizeDebugValue(action), alwaysLog: true }, async () => {
    if (!action.rootAction) return { view: { type: 'preview', title: action.title || 'Extension item', content: action.subtitle || '' } }
    if (action.rootAction.type !== 'runExtensionAction') return executeViewAction(action.rootAction)
    const record = extensionActionHandlers.get(action.rootAction.handlerId)
    if (!record) return { view: { type: 'preview', title: 'Action unavailable', content: 'This extension item is no longer available.' } }
    try {
      const result = await measureDebugPerformance('extension.root-item.handler', { extensionId: record.entry.extension.id, commandId: record.entry.command?.id, alwaysLog: true }, () => record.handler(createExtensionContext(record.entry.extension, record.entry.command || null), action))
      return executeViewActionResult(result, record.entry)
    } catch (error) {
      logError('extension.rootItem.failed', error, { source: 'host', scope: 'extension', extensionId: record.entry.extension.id })
      return { view: extensionErrorView(record.entry, error) }
    }
  })
}

async function extensionRootActions() {
  const actionGroups = await Promise.all(visibleExtensions().map((extension) => extensionRootActionsForExtension(extension)))
  return actionGroups.flat()
}

async function extensionSearchActions(query) {
  const extensions = visibleExtensions()
  const actionGroups = await measureDebugPerformance('extension.search.all', { extensionCount: extensions.length, queryLength: String(query || '').length }, () => Promise.all(extensions.map((extension) => extensionSearchActionsForExtension(extension, query))))
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
  return measureDebugPerformance('extension.search.provider', { extensionId: extension.id, queryLength: String(query || '').length }, async () => {
    try {
      const entry = { extension, command: { id: 'search', title: extension.title || extension.id } }
      const items = await withTimeout(extension.searchItems(createExtensionContext(extension, null), query), EXTENSION_ROOT_ITEMS_TIMEOUT_MS)
      const list = Array.isArray(items) ? items : Array.isArray(items?.items) ? items.items : []
      return measureDebugPerformanceSync('extension.search.provider.rank', { extensionId: extension.id, itemCount: list.length, queryLength: String(query || '').length }, () => rankContributionActions(list.map((item) => extensionRootActionFromItem(entry, item)).filter(Boolean), query))
    } catch (error) {
      if (!String(error?.message || error).includes('Timed out')) logError('extension.searchItems.failed', error, { source: 'host', scope: 'extension', extensionId: extension.id })
      return []
    }
  })
}

async function extensionRootActionsForExtension(extension) {
  if (typeof extension.rootItems !== 'function') return []
  const cacheKey = extension.__filePath || extension.id
  const cached = extensionRootItemsCache.get(cacheKey)
  if (cached && Date.now() - cached.updatedAt < EXTENSION_ROOT_ITEMS_TTL_MS) {
    markDebugPerformance('extension.root.cache-hit', { extensionId: extension.id, itemCount: cached.items.length })
    return cached.items
  }
  const refresh = refreshExtensionRootActions(extension, cacheKey)
  return cached?.items || await refresh
}

function refreshExtensionRootActions(extension, cacheKey) {
  const current = extensionRootItemsRefreshes.get(cacheKey)
  if (current) return current
  const promise = measureDebugPerformance('extension.root.provider', { extensionId: extension.id }, async () => {
    const entry = { extension, command: { id: 'root', title: extension.title || extension.id } }
    const items = await withTimeout(extension.rootItems(createExtensionContext(extension, null)), EXTENSION_ROOT_ITEMS_TIMEOUT_MS)
    const list = Array.isArray(items) ? items : Array.isArray(items?.items) ? items.items : []
    const actions = measureDebugPerformanceSync('extension.root.provider.rank', { extensionId: extension.id, itemCount: list.length }, () => rankContributionActions(list.map((item) => extensionRootActionFromItem(entry, item)).filter(Boolean), ''))
    extensionRootItemsCache.set(cacheKey, { updatedAt: Date.now(), items: actions })
    return actions
  }).catch((error) => {
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

function extensionActionFromContribution(entry) {
  const item = entry.item
  if (!item?.id || !item.title) return null
  const extensionFile = entry.extension.__filePath ? path.basename(entry.extension.__filePath) : undefined
  const primaryAction = normalizeViewAction(item.primaryAction || item.action, entry)
  const actionId = item.actionId || `extension-action:${entry.extension.id}:${item.id}`
  const action = {
    id: actionId,
    kind: 'extension-action',
    extensionId: entry.extension.id,
    registeredActionId: item.id,
    extensionFile,
    aiChatId: extensionFile ? aiChatIdForExtensionFile(extensionFile) : undefined,
    rootAction: primaryAction,
    removable: Boolean(entry.extension.__generated),
    customizable: item.customizable !== false,
    title: item.title,
    subtitle: item.subtitle || entry.extension.title || 'Extension action',
    aliases: item.aliases || item.keywords || [],
    icon: item.icon || 'sparkles',
    iconUrl: item.image || item.iconUrl || null,
    thumbnailUrl: item.thumbnailUrl || null,
    score: item.score || 12,
    dismissAfterRun: item.dismissAfterRun || primaryAction?.dismissAfterRun,
    background: item.background,
    actionPanel: normalizeActionPanel(item.actionPanel, item.actions || [], entry),
    appearance: normalizeItemAppearance(item.appearance),
  }
  const declaredShortcut = userState.removedShortcuts?.[action.id] ? null : item.globalShortcut || (item.shortcutScope === 'global' ? item.shortcut : null)
  const shortcut = shortcutForAction(action) || declaredShortcut
  return shortcut ? { ...action, shortcut } : action
}

async function executeActionForIpc(action) {
  return measureDebugPerformance('ipc.actions.execute', { action: summarizeDebugValue(action), alwaysLog: true }, async () => {
    let trustedAction: any = null
    try {
      trustedAction = resolveRootActionForIpc(action)
      const result = normalizeHostViewResult(await measureDebugPerformance('action.execute', { action: summarizeDebugValue(trustedAction), alwaysLog: true }, () => executeAction(trustedAction)))
      structuredClone(result)
      return result
    } catch (error) {
      if (trustedAction?.kind === 'extension-command') {
        const entry = extensionEntryForAction(trustedAction)
        if (entry) return { view: extensionErrorView(entry, error) }
      }
      if (action?.kind === 'extension-action') {
        const entry = extensionActionEntryForAction(action)
        if (entry) return { view: extensionErrorView(entry, error) }
      }
      return { view: { type: 'preview', title: 'Action failed', content: `# Something went wrong\n\n\`\`\`\n${extensionErrorMessage(error)}\n\`\`\`` } }
    }
  })
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

const VIEW_TYPES = new Set(['list', 'grid', 'preview', 'chat', 'form', 'editor', 'progress', 'webview', 'camera'])

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
    refresh: registerViewRefreshForRenderer(view.refresh, entry, view),
    items: normalizeViewItems(view.items, entry),
    sections: Array.isArray(view.sections) ? view.sections.map((section) => ({ ...section, items: normalizeViewItems(section.items, entry) })) : view.sections,
  }
}

function persistentActionForRef(action, entry) {
  if (action?.type !== 'runExtensionRegisteredAction') return null
  const extensionId = action.extensionId || entry?.extension?.id
  const registeredActionId = action.registeredActionId || action.actionId
  const registered = extensionActionRegistry.get(`${extensionId}:${registeredActionId}`)
  return registered ? extensionActionFromContribution(registered) : null
}

function normalizeViewItems(items, entry) {
  return Array.isArray(items) ? items.map((item) => {
    const itemActions = normalizeViewActions(item.actions, entry)
    const primaryAction = normalizeViewAction(item.primaryAction || item.action, entry)
    const { run, __handler, action, ...safeItem } = item
    const detailActions = normalizeViewActions(item.detail?.actions, entry)
    return {
      ...safeItem,
      ...(item.detail ? { detail: { ...item.detail, actions: detailActions } } : {}),
      actions: itemActions,
      actionPanel: normalizeActionPanel(item.actionPanel, itemActions, entry),
      primaryAction,
      persistentAction: item.persistentAction || persistentActionForRef(primaryAction, entry),
      appearance: normalizeItemAppearance(item.appearance),
    }
  }) : items
}

function normalizeActionPanel(panel, fallbackActions, entry) {
  if (panel?.sections) return {
    ...panel,
    sections: panel.sections.map((section) => {
      const { lazyActions, ...safeSection } = section
      return {
        ...safeSection,
        actions: normalizeViewActions([...(section.actions || []), ...(lazyActions || [])], entry),
      }
    }),
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
  if ((normalized.type === 'rootView' || normalized.type === 'pushView' || normalized.type === 'replaceView') && normalized.view) {
    return registerViewActionForRenderer({ ...normalized, view: normalizeView(normalized.view, entry) })
  }
  if (normalized.type === 'promptAction' && normalized.targetAction) {
    return registerViewActionForRenderer({ ...normalized, targetAction: normalizeViewAction(normalized.targetAction, entry) })
  }
  if ((normalized.type === 'createWindow' || normalized.type === 'toggleWindow') && normalized.view) {
    return registerViewActionForRenderer({ ...normalized, view: normalizeView(normalized.view, entry) })
  }
  return registerViewActionForRenderer(normalized)
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

function normalizeHostViewResult(result) {
  if (!result) return result
  return {
    ...result,
    ...(result.view ? { view: normalizeView(result.view, null) } : {}),
    ...(result.patch ? { patch: normalizeViewPatch(result.patch, null) } : {}),
  }
}

async function executeViewActionResult(result, entry, launchContext?: any) {
  if (!result) return result
  if (isAction(result)) return executeViewAction(normalizeViewAction(result, entry), launchContext)
  if (isAction(result.action)) return executeViewAction(normalizeViewAction(result.action, entry), launchContext)
  const view = normalizeExtensionView(result, entry)
  return view ? { view, navigation: result?.navigation || 'push', toast: result?.toast, patch: normalizeViewPatch(result?.patch, entry) } : { ...result, patch: normalizeViewPatch(result?.patch, entry) }
}

async function executeHostRefreshAction(record, launchContext?: any) {
  if (record.action?.type === 'runExtensionAction') {
    const handlerRecord = extensionActionHandlers.get(record.action.handlerId)
    if (!handlerRecord) return { skipped: true }
    const result = await handlerRecord.handler(createExtensionContext(handlerRecord.entry.extension, handlerRecord.entry.command, launchContext), record.action)
    return executeViewActionResult(result, handlerRecord.entry, launchContext)
  }
  if (record.action) return executeViewAction(record.action, launchContext)
  if (!record.entry?.command || typeof record.entry.command.run !== 'function') return { skipped: true }
  const result = await record.entry.command.run(createExtensionContext(record.entry.extension, record.entry.command, launchContext))
  const view = result?.type ? result : result?.view?.type ? result.view : null
  if (view?.items) return { patch: { mode: record.mode || 'replace', items: normalizeViewItems(view.items, record.entry) } }
  return executeViewActionResult(result, record.entry, launchContext)
}

function refreshBackoffDelay(failureCount: number) {
  return Math.min(30_000, 1_000 * Math.max(1, 2 ** Math.max(0, failureCount - 1)))
}

async function refreshViewForIpc(input: any = {}) {
  return measureDebugPerformance('ipc.view.refresh', { input: summarizeDebugValue(input), alwaysLog: true }, async () => {
    pruneViewRefreshRecords()
    const refreshId = typeof input === 'string' ? input : String(input?.id || '')
    const record = refreshId ? viewRefreshRecords.get(refreshId) : null
    if (!record) return { skipped: true }
    if (input?.viewId && record.viewId && input.viewId !== record.viewId) return { skipped: true }
    const now = Date.now()
    if (record.backoffUntil && record.backoffUntil > now) return { skipped: true }
    if (record.running) return { skipped: true }
    record.running = measureDebugPerformance('view.refresh.host-action', { refreshId, viewId: record.viewId, extensionId: record.entry?.extension?.id, commandId: record.entry?.command?.id, alwaysLog: true }, async () => {
      try {
        const result = normalizeHostViewResult(await executeHostRefreshAction(record, { refresh: true }))
        structuredClone(result)
        record.failureCount = 0
        record.backoffUntil = undefined
        return result
      } catch (error) {
        record.failureCount += 1
        record.backoffUntil = Date.now() + refreshBackoffDelay(record.failureCount)
        logWarn('extension.viewRefresh.failed', { viewId: record.viewId, error: error instanceof Error ? error.message : String(error) }, { source: 'host', scope: 'extension', extensionId: record.entry?.extension?.id, commandId: record.entry?.command?.id })
        return { skipped: true }
      } finally {
        record.running = null
      }
    })
    return record.running
  })
}

async function executeViewActionForIpc(action) {
  return measureDebugPerformance('ipc.view-action.execute', { action: summarizeDebugValue(action), alwaysLog: true }, async () => {
    let trustedAction: any = null
    try {
      trustedAction = resolveViewActionForIpc(action)
      const result = normalizeHostViewResult(await measureDebugPerformance('view-action.execute', { action: summarizeDebugValue(trustedAction), alwaysLog: true }, () => executeViewAction(trustedAction)))
      structuredClone(result)
      return result
    } catch (error) {
      const record = trustedAction?.type === 'runExtensionAction' ? extensionActionHandlers.get(trustedAction.handlerId) : null
      if (record) return { view: extensionErrorView(record.entry, error), navigation: 'push' }
      return { view: { type: 'preview', title: 'Action failed', content: `# Something went wrong\n\n\`\`\`\n${extensionErrorMessage(error)}\n\`\`\`` }, navigation: 'push' }
    }
  })
}

function clipboardSnapshot() {
  const image = clipboard.readImage()
  return {
    text: clipboard.readText(),
    html: clipboard.readHTML(),
    rtf: clipboard.readRTF(),
    bookmark: clipboard.readBookmark(),
    image: image.isEmpty() ? null : image,
  }
}

function restoreClipboardSnapshot(snapshot: ReturnType<typeof clipboardSnapshot>) {
  if (!snapshot) return
  const data: any = {}
  if (snapshot.text) data.text = snapshot.text
  if (snapshot.html) data.html = snapshot.html
  if (snapshot.rtf) data.rtf = snapshot.rtf
  if (snapshot.bookmark?.title || snapshot.bookmark?.url) data.bookmark = snapshot.bookmark
  if (snapshot.image && !snapshot.image.isEmpty()) data.image = snapshot.image
  if (Object.keys(data).length === 0) clipboard.clear()
  else clipboard.write(data)
}

function clipboardHistoryIdForText(text: string) {
  const value = String(text || '').trim()
  return value ? `text:${hashValue(value)}` : ''
}

function suppressClipboardHistoryId(id: string, durationMs = 2_000) {
  if (id) suppressedClipboardItemIds.set(id, Date.now() + durationMs)
}

function pasteTextAction(action: any) {
  const text = String(action.text || '')
  const restoreClipboard = Boolean(action.restoreClipboard)
  const concealed = Boolean(action.concealed || restoreClipboard)
  const snapshot = restoreClipboard ? clipboardSnapshot() : null
  const suppressedId = clipboardHistoryIdForText(text)
  if (concealed) suppressClipboardHistoryId(suppressedId)
  if (action.plainText === false && action.html) clipboard.write({ text, html: String(action.html) })
  else clipboard.writeText(text)
  pasteIntoFrontmostApp()
  if (restoreClipboard && snapshot) {
    const delay = Math.max(50, Math.min(5_000, Number(action.restoreDelayMs || 250)))
    setTimeout(() => {
      suppressClipboardHistoryId(clipboardHistoryIdForText(snapshot.text))
      restoreClipboardSnapshot(snapshot)
    }, delay).unref?.()
  }
}

function extensionWindowSize(options: any = {}) {
  const large = options.size === 'large'
  return {
    width: Math.max(320, Math.min(1600, Number(options.width || (large ? 900 : 560)))),
    height: Math.max(240, Math.min(1200, Number(options.height || (large ? 680 : 420)))),
  }
}

function extensionWindowId(view: any, options: any = {}) {
  return String(options.id || view?.id || `window:${hashValue(`${view?.title || 'Extension Window'}:${JSON.stringify(view || {})}`)}`)
}

function loadExtensionWindow(win: BrowserWindow, id: string) {
  if (isDev && rendererUrl) return win.loadURL(`${rendererUrl}?extensionWindowId=${encodeURIComponent(id)}`)
  return win.loadFile(rendererIndexPath, { query: { extensionWindowId: id } })
}

function centerExtensionWindow(win: BrowserWindow) {
  const cursor = screen.getCursorScreenPoint()
  const display = screen.getDisplayNearestPoint(cursor)
  const bounds = win.getBounds()
  const { x, y, width, height } = display.workArea
  win.setBounds({ x: Math.round(x + (width - bounds.width) / 2), y: Math.round(y + (height - bounds.height) / 2), width: bounds.width, height: bounds.height })
}

function applyExtensionWindowOptions(win: BrowserWindow, options: any = {}) {
  const alwaysOnTop = options.alwaysOnTop !== false
  win.setAlwaysOnTop(alwaysOnTop, alwaysOnTop ? 'floating' : 'normal')
  if (options.visibleOnAllSpaces) win.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true })
}

function createOrUpdateExtensionWindow(view: any, options: any = {}) {
  const normalizedView = normalizeView(view, null)
  structuredClone(normalizedView)
  const id = extensionWindowId(normalizedView, options)
  const existing = extensionWindows.get(id)
  if (existing && !existing.win.isDestroyed()) {
    existing.view = normalizedView
    existing.options = { ...existing.options, ...options, id }
    existing.win.setTitle(String(options.title || normalizedView.title || 'Nevermind'))
    existing.win.webContents.send('extension-window:view', { id, view: normalizedView, options: existing.options })
    existing.win.show()
    existing.win.focus()
    return existing
  }

  const size = extensionWindowSize(options)
  const hiddenTitleBar = options.titleBar === 'hidden'
  const win = new BrowserWindow({
    width: size.width,
    height: size.height,
    minWidth: 320,
    minHeight: 240,
    show: false,
    frame: true,
    ...(hiddenTitleBar ? { titleBarStyle: 'hiddenInset', trafficLightPosition: { x: 12, y: 12 } } : {}),
    title: String(options.title || normalizedView.title || 'Nevermind'),
    backgroundColor: nativeTheme.shouldUseDarkColors ? '#111111' : '#f7f7f7',
    webPreferences: { preload: preloadPath, contextIsolation: true, nodeIntegration: false, sandbox: false },
  } satisfies Electron.BrowserWindowConstructorOptions)
  const record = { id, win, view: normalizedView, options: { ...options, id } }
  extensionWindows.set(id, record)
  applyExtensionWindowOptions(win, options)
  win.once('ready-to-show', () => { centerExtensionWindow(win); win.show() })
  if (options.hideOnBlur) win.on('blur', () => win.hide())
  win.on('closed', () => { if (extensionWindows.get(id)?.win === win) extensionWindows.delete(id) })
  win.webContents.on('did-fail-load', (_event, errorCode, errorDescription, validatedURL) => loggerDebug('extensionWindow.didFailLoad', { id, errorCode, errorDescription, validatedURL }, { source: 'host', scope: 'extensions' }))
  loadExtensionWindow(win, id)
  return record
}

function executeWindowAction(action: any) {
  const id = String(action.windowId || action.id || '')
  if (action.type === 'createWindow') {
    createOrUpdateExtensionWindow(action.view, action.windowOptions || {})
    return { toast: { message: 'Opened window' } }
  }
  const record = extensionWindows.get(id)
  if (!record) {
    if (action.type === 'toggleWindow' && action.view) {
      createOrUpdateExtensionWindow(action.view, { ...(action.windowOptions || {}), id })
      return { toast: { message: 'Opened window' } }
    }
    return { toast: { message: 'Window is not open', tone: 'error' } }
  }
  if (action.type === 'showWindow') { record.win.show(); record.win.focus(); return { toast: { message: 'Shown window' } } }
  if (action.type === 'hideWindow') { record.win.hide(); return { toast: { message: 'Hidden window' } } }
  if (action.type === 'toggleWindow') {
    if (action.view || action.windowOptions) createOrUpdateExtensionWindow(action.view || record.view, { ...(record.options || {}), ...(action.windowOptions || {}), id })
    if (record.win.isVisible()) record.win.hide(); else { record.win.show(); record.win.focus() }
    return { toast: { message: 'Toggled window' } }
  }
  if (action.type === 'closeWindow') { record.win.close(); return { toast: { message: 'Closed window' } } }
  return null
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
  jobRegistry.clear()
  for (const record of extensionWindows.values()) record.win.close()
  extensionWindows.clear()
  for (const watcher of appWatchers) watcher.close()
  for (const watcher of extensionFileWatchers) watcher.close()
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

async function executeViewAction(action, launchContext?: any) {
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
      pasteTextAction(action)
      return { toast: { message: 'Pasted' } }
    case 'pasteClipboard':
      pasteClipboardAction(action)
      return { toast: { message: 'Pasted' } }
    case 'typeText': {
      const result = await typeTextIntoFrontmostApp(action.text || '', { delayMs: action.delayMs })
      return result?.ok ? { toast: { message: 'Typed' } } : { toast: { message: result?.error || 'Unable to type text', tone: 'error' } }
    }
    case 'createWindow':
    case 'showWindow':
    case 'hideWindow':
    case 'toggleWindow':
    case 'closeWindow':
      return executeWindowAction(action)
    case 'copyImage':
      if (action.path) clipboard.writeImage(nativeImage.createFromPath(expandUserPath(action.path)))
      else if (action.imagePath) clipboard.writeImage(nativeImage.createFromPath(action.imagePath))
      else clipboard.writeImage(nativeImage.createFromDataURL(action.imageDataUrl))
      break
    case 'removeClipboardHistory':
      return removeClipboardHistoryEntries(action)
    case 'trash':
      for (const itemPath of action.paths || [action.path]) {
        if (itemPath) await shell.trashItem(expandUserPath(itemPath))
      }
      return { toast: { message: 'Moved to Trash' } }
    case 'rootView':
      return { view: action.view, navigation: 'root' }
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
      const result = setSetting(definition.id, toggledSettingValue(definition, getSetting(definition.id)))
      if (!result.ok) return { toast: { message: result.message, tone: 'error' } }
      return { patch: { items: [settingItemPatch(definition)] } }
    }
    case 'setActionShortcut': {
      const result = await setShortcut(action.targetAction || action.action, action.accelerator || action.shortcut)
      return { toast: { message: result.message, tone: result.ok ? 'default' : 'error' }, ok: result.ok }
    }
    case 'setSettingShortcut': {
      const result = setShortcutSetting(action.settingId, action.accelerator || action.shortcut)
      return { patch: { items: result.ok ? [settingItemPatch(settingDefinition(action.settingId))] : [] }, toast: { message: result.message, tone: result.ok ? 'default' : 'error' }, ok: result.ok }
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
    case 'runExtensionRegisteredAction': {
      const resolved = resolveRegisteredActionRef(action)
      if (resolved.error) return { toast: { message: resolved.error, tone: 'error' } }
      return resolved.rootAction ? executeExtensionRootItem(resolved.rootAction) : { toast: { message: 'Action unavailable', tone: 'error' } }
    }
    case 'promptAction':
      return { view: promptActionView(action), navigation: 'push' }
    case 'runExtensionAction': {
      const record = extensionActionHandlers.get(action.handlerId)
      if (!record) return { toast: { message: 'Action is no longer available', tone: 'error' } }
      try {
        const result = await measureDebugPerformance('extension.action.handler', { extensionId: record.entry.extension.id, commandId: record.entry.command.id, actionTitle: action.title, alwaysLog: true }, () => record.handler(createExtensionContext(record.entry.extension, record.entry.command, launchContext), action))
        return executeViewActionResult(result, record.entry, launchContext)
      } catch (error) {
        logError('extension.action.failed', error, { source: 'host', scope: 'extension', extensionId: record.entry.extension.id, commandId: record.entry.command.id })
        return { view: extensionErrorView(record.entry, error), navigation: 'push' }
      }
    }
    default:
      throw new Error(`Unsupported action type: ${String(action?.type || 'unknown')}`)
  }
}

function resolveRegisteredActionRef(action) {
  let current = action
  const visited = new Set<string>()
  while (current?.type === 'runExtensionRegisteredAction') {
    const extensionId = current.extensionId
    const registeredActionId = current.registeredActionId || current.actionId
    const key = `${extensionId}:${registeredActionId}`
    if (visited.has(key)) return { error: 'Action reference cycle detected' }
    visited.add(key)
    const entry = extensionActionRegistry.get(key)
    const rootAction = entry ? extensionActionFromContribution(entry) : null
    if (!rootAction) return { error: 'Action unavailable' }
    if (rootAction.rootAction?.type !== 'runExtensionRegisteredAction') return { rootAction }
    current = rootAction.rootAction
  }
  return { error: 'Action unavailable' }
}

async function executeBuiltin(action) {
  return executeSystemBuiltin(action, () => requestQuitApp('builtin'))
}

async function thumbnailCachePath(filePath) {
  const stat = await fs.stat(filePath)
  const key = crypto.createHash('sha1').update(`${filePath}:${stat.mtimeMs}:${stat.size}:${THUMBNAIL_SIZE}`).digest('hex')
  return path.join(iconCacheDir, 'thumbs', `${key}.png`)
}

async function generateQuickLookThumbnail(filePath, cachedPath) {
  if (!hasCapability('quick-look')) return false
  const outputDir = await fs.mkdtemp(path.join(os.tmpdir(), 'nevermind-thumb-'))
  try {
    await execFileText('qlmanage', ['-t', '-s', String(THUMBNAIL_SIZE), '-o', outputDir, filePath], { timeout: 10_000 })
    const generatedPath = path.join(outputDir, `${path.basename(filePath)}.png`)
    await fs.mkdir(path.dirname(cachedPath), { recursive: true })
    await fs.copyFile(generatedPath, cachedPath)
    return true
  } catch {
    return false
  } finally {
    await fs.rm(outputDir, { recursive: true, force: true }).catch(() => {})
  }
}

async function generateThumbnail(filePath, cachedPath) {
  return measureDebugPerformance('thumbnail.generate', { filePath, alwaysLog: true }, async () => {
    let image = null
    if (typeof nativeImage.createThumbnailFromPath === 'function') {
      image = await nativeImage.createThumbnailFromPath(filePath, { width: THUMBNAIL_SIZE, height: THUMBNAIL_SIZE })
    }
    if (!image || image.isEmpty()) image = nativeImage.createFromPath(filePath).resize({ width: THUMBNAIL_SIZE, quality: 'good' })
    if (!image || image.isEmpty()) return generateQuickLookThumbnail(filePath, cachedPath)
    const png = image.toPNG()
    await fs.mkdir(path.dirname(cachedPath), { recursive: true })
    await fs.writeFile(cachedPath, png).catch(() => {})
    return true
  })
}

async function processPendingThumbnails() {
  const entries = Array.from(pendingThumbnailPaths.entries()).slice(0, 4)
  for (const [filePath] of entries) pendingThumbnailPaths.delete(filePath)
  for (const [filePath, cachedPath] of entries) {
    await generateThumbnail(filePath, cachedPath).catch((error) => {
      logWarn('thumbnail.generate.failed', { filePath, error }, { source: 'host', scope: 'cache' })
    })
  }
  if (pendingThumbnailPaths.size) jobRegistry.schedule('cache.thumbnails', 'thumbnail-backlog', 250)
}

async function thumbnailResponseForPath(filePath) {
  return measureDebugPerformance('thumbnail.response', { filePath, alwaysLog: true }, async () => {
    const cachedPath = await thumbnailCachePath(filePath)
    const cached = await fs.readFile(cachedPath).catch(() => null)
    if (cached) {
      markDebugPerformance('thumbnail.cache-hit', { filePath })
      return new Response(cached, { headers: { 'content-type': 'image/png', 'cache-control': 'public, max-age=31536000, immutable' } })
    }

    pendingThumbnailPaths.set(filePath, cachedPath)
    await jobRegistry.run('cache.thumbnails', 'thumbnail-request').catch(() => {})
    const generated = await fs.readFile(cachedPath).catch(() => null)
    if (generated) return new Response(generated, { headers: { 'content-type': 'image/png', 'cache-control': 'public, max-age=31536000, immutable' } })
    return net.fetch(pathToFileURL(filePath).href)
  })
}

async function localFileResponse(requestPath: string, request: Request) {
  const stat = await fs.stat(requestPath).catch(() => null)
  if (!stat?.isFile()) return new Response('File not found', { status: 404 })

  const headers = new Headers({
    'accept-ranges': 'bytes',
    'cache-control': 'private, max-age=3600',
    'content-type': mimeTypeForPath(requestPath),
  })
  const range = request.headers.get('range')
  let start = 0
  let end = stat.size - 1
  let status = 200

  if (range) {
    const match = range.match(/^bytes=(\d*)-(\d*)$/)
    if (!match || (!match[1] && !match[2])) return new Response('Invalid range', { status: 416, headers: { 'content-range': `bytes */${stat.size}` } })
    if (!match[1]) {
      start = Math.max(0, stat.size - Number(match[2]))
      end = stat.size - 1
    } else {
      start = Number(match[1])
      end = match[2] ? Number(match[2]) : stat.size - 1
    }
    if (Number.isNaN(start) || Number.isNaN(end) || start > end || start >= stat.size) return new Response('Invalid range', { status: 416, headers: { 'content-range': `bytes */${stat.size}` } })
    end = Math.min(end, stat.size - 1)
    status = 206
    headers.set('content-range', `bytes ${start}-${end}/${stat.size}`)
  }

  headers.set('content-length', String(Math.max(0, end - start + 1)))
  return new Response(Readable.toWeb(fsSync.createReadStream(requestPath, { start, end })) as BodyInit, { status, headers })
}

function registerLocalFileProtocol() {
  protocol.handle(LOCAL_FILE_PROTOCOL, (request) => {
    const url = new URL(request.url)
    const encodedPath = url.host ? `/${url.host}${url.pathname}` : url.pathname
    const requestPath = path.resolve(decodeURIComponent(encodedPath))
    if (!path.isAbsolute(requestPath)) return new Response('Invalid file path', { status: 400 })
    if (!verifyLocalFileToken('file', requestPath, url.searchParams.get('token'))) return new Response('Forbidden', { status: 403 })
    return localFileResponse(requestPath, request)
  })

  protocol.handle(LOCAL_THUMB_PROTOCOL, async (request) => {
    const url = new URL(request.url)
    const requestPath = path.resolve(decodeURIComponent(url.searchParams.get('path') || ''))
    if (!path.isAbsolute(requestPath)) return new Response('Invalid file path', { status: 400 })
    if (!verifyLocalFileToken('thumb', requestPath, url.searchParams.get('token'))) return new Response('Forbidden', { status: 403 })
    try {
      return await thumbnailResponseForPath(requestPath)
    } catch (error) {
      logError('thumbnail.create.failed', { requestPath, error }, { source: 'host', scope: 'thumbnail' })
      return new Response('Thumbnail not found', { status: 404 })
    }
  })
}

function mimeTypeForPath(filePath) {
  const extension = extensionForPath(filePath)
  const types = {
    png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', gif: 'image/gif', webp: 'image/webp', bmp: 'image/bmp', tif: 'image/tiff', tiff: 'image/tiff', heic: 'image/heic',
    mp4: 'video/mp4', mov: 'video/quicktime', webm: 'video/webm', m4v: 'video/x-m4v', mkv: 'video/x-matroska', avi: 'video/x-msvideo',
    txt: 'text/plain', md: 'text/markdown', json: 'application/json', pdf: 'application/pdf', html: 'text/html', htm: 'text/html', csv: 'text/csv',
  }
  return types[extension] || 'application/octet-stream'
}

async function imageDimensionsForPath(filePath) {
  if (!isImagePath(filePath)) return {}
  const image = nativeImage.createFromPath(filePath)
  if (!image || image.isEmpty()) return {}
  const size = image.getSize()
  return size.width && size.height ? { width: size.width, height: size.height } : {}
}

function thumbnailUrlForPreviewablePath(filePath) {
  const expandedPath = expandUserPath(filePath)
  return isImagePath(expandedPath) || isVideoPath(expandedPath) ? thumbnailUrlForPath(expandedPath) : null
}

function dataUrlExtension(dataUrl: string) {
  const mime = dataUrl.match(/^data:([^;,]+)/)?.[1] || 'image/png'
  if (mime.includes('jpeg')) return 'jpg'
  if (mime.includes('webp')) return 'webp'
  if (mime.includes('gif')) return 'gif'
  if (mime.includes('tiff')) return 'tiff'
  if (mime.includes('heic')) return 'heic'
  return 'png'
}

async function writeOcrDataUrl(dataUrl: string) {
  const match = dataUrl.match(/^data:([^;,]+)?(;base64)?,(.*)$/)
  if (!match) throw new Error('Invalid OCR data URL')
  const outputPath = path.join(os.tmpdir(), `nevermind-ocr-input-${crypto.randomUUID()}.${dataUrlExtension(dataUrl)}`)
  const body = decodeURIComponent(match[3] || '')
  await fs.writeFile(outputPath, match[2] ? Buffer.from(match[3] || '', 'base64') : Buffer.from(body))
  return outputPath
}

function filePathFromLocalUrl(value: string) {
  if (value.startsWith(`${LOCAL_FILE_PROTOCOL}:`)) return fileURLToPath(`file:${value.slice(`${LOCAL_FILE_PROTOCOL}:`.length)}`)
  if (value.startsWith('file:')) return fileURLToPath(value)
  return null
}

async function ocrInputPath(input: any) {
  const value = typeof input === 'string' ? input : input?.path || input?.filePath || input?.fileUrl || input?.url
  if (!value) throw new Error('OCR requires an image path, file URL, data URL, or ExtensionFile')
  const text = String(value)
  if (text.startsWith('data:')) return { path: await writeOcrDataUrl(text), cleanup: true }
  const urlPath = filePathFromLocalUrl(text)
  return { path: expandUserPath(urlPath || text), cleanup: false }
}

async function ocrImage(input: any, options: any = {}) {
  const resolved = await ocrInputPath(input)
  try {
    if (!isImagePath(resolved.path)) throw new Error('OCR currently supports image files only')
    return await recognizeTextInImage(resolved.path, options)
  } finally {
    if (resolved.cleanup) await fs.rm(resolved.path, { force: true }).catch(() => {})
  }
}

async function ocrScreen(options: any = {}) {
  const imagePath = await captureScreenImage(options)
  try { return await recognizeTextInImage(imagePath, options) }
  finally { await fs.rm(imagePath, { force: true }).catch(() => {}) }
}

async function fileToExtensionFile(filePath) {
  const expandedPath = expandUserPath(filePath)
  const stat = await fs.stat(expandedPath).catch(() => null)
  const dimensions = await imageDimensionsForPath(expandedPath)
  return {
    path: expandedPath,
    name: path.basename(expandedPath),
    displayPath: displayUserPath(expandedPath),
    url: thumbnailUrlForPreviewablePath(expandedPath) || fileUrlForPath(expandedPath),
    fileUrl: fileUrlForPath(expandedPath),
    videoUrl: isVideoPath(expandedPath) ? fileUrlForPath(expandedPath) : null,
    thumbnailUrl: thumbnailUrlForPreviewablePath(expandedPath),
    kind: isImagePath(expandedPath) ? 'image' : isVideoPath(expandedPath) ? 'video' : 'file',
    extension: extensionForPath(expandedPath),
    mimeType: mimeTypeForPath(expandedPath),
    ...dimensions,
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
  const limit = Math.max(1, Math.min(Number(options.limit || 100), MAX_FILE_INDEX_LIMIT))
  const maxDepth = options.depth ?? 2
  const extensions = extensionsForFindOptions(options)
  const ignored = normalizedIgnorePatterns(options.ignore)
  const includeHidden = Boolean(options.includeHidden)

  async function walk(dir, depth, found) {
    if (found.length >= limit) return
    let entries = []
    try {
      entries = await fs.readdir(dir, { withFileTypes: true })
    } catch {
      return
    }

    for (const entry of entries) {
      if (found.length >= limit) return
      if (!includeHidden && entry.name.startsWith('.')) continue
      const fullPath = path.join(dir, entry.name)
      if (ignoredByPattern(fullPath, entry.name, ignored)) continue
      if (entry.isFile()) {
        const ext = extensionForPath(entry.name)
        if (!extensions || extensions.has(ext)) found.push(await fileToExtensionFile(fullPath))
        continue
      }
      if (entry.isDirectory() && depth > 0) await walk(fullPath, depth - 1, found)
    }
  }

  const findRoots = normalizeFindRoots(roots).map(expandUserPath).filter((root) => root && path.isAbsolute(root))
  const rootResults = await Promise.all(findRoots.map(async (root) => {
    const found = []
    await walk(root, maxDepth, found)
    return found
  }))
  let found = rootResults.flat()
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
  return selectedFilePaths()
}

async function selectedExtensionFiles() {
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

function clipboardFormats(options: any = {}) {
  const formats = Array.isArray(options.formats) ? options.formats.map(String) : []
  return formats.length ? new Set(formats) : null
}

async function readDesktopClipboard(options: any = {}) {
  const formats = clipboardFormats(options)
  if (!formats || formats.has('files')) {
    const files = await clipboardFiles()
    if (files.length) return { type: 'files', files, paths: files.map((file: any) => file.path) }
  }
  if (!formats || formats.has('image')) {
    const image = clipboardImageDataUrl()
    if (image) return { type: 'image', imageDataUrl: image, image }
  }
  if (!formats || formats.has('html')) {
    const html = clipboard.readHTML()
    if (html) return { type: 'html', html, text: clipboard.readText() }
  }
  if (!formats || formats.has('text')) {
    const text = clipboard.readText()
    if (text) return { type: 'text', text, html: clipboard.readHTML() || undefined }
  }
  return { type: 'empty' }
}

function clipboardImageForContent(item: any) {
  const image = item?.image || item?.imageDataUrl || item?.path
  if (!image) return null
  return String(image).startsWith('data:') ? nativeImage.createFromDataURL(String(image)) : nativeImage.createFromPath(expandUserPath(String(image)))
}

function suppressClipboardHistoryForContent(item: any) {
  if (!item) return
  if (typeof item === 'string') return suppressClipboardHistoryId(clipboardHistoryIdForText(item))
  const text = item.text || (item.type === 'html' ? item.html : '')
  if (text) suppressClipboardHistoryId(clipboardHistoryIdForText(String(text)))
  const image = clipboardImageForContent(item)
  if (image && !image.isEmpty()) suppressClipboardHistoryId(`image:${hashValue(image.toPNG())}`)
  const paths = Array.isArray(item.paths) ? item.paths : Array.isArray(item.files) ? item.files.map((file) => file.path || file).filter(Boolean) : []
  for (const filePath of paths) if (isVideoPath(String(filePath))) suppressClipboardHistoryId(`video:${hashValue(expandUserPath(String(filePath)))}`)
}

function writeDesktopClipboardFiles(paths) {
  const resolvedPaths = (Array.isArray(paths) ? paths : [paths]).map((filePath) => expandUserPath(String(filePath))).filter(Boolean)
  const fileUrls = resolvedPaths.map((filePath) => pathToFileURL(filePath).href).join('\n')
  clipboard.write({ text: resolvedPaths.join('\n') })
  if (fileUrls) clipboard.writeBuffer('public.file-url', Buffer.from(fileUrls, 'utf8'))
}

function writeDesktopClipboard(item, options: any = {}) {
  const content = typeof item === 'string' ? { type: 'text', text: item } : item || {}
  if (content.concealed || options.concealed) suppressClipboardHistoryForContent(content)
  if (content.type === 'files' || Array.isArray(content.paths) || Array.isArray(content.files)) return writeDesktopClipboardFiles(content.paths || content.files)
  if (content.type === 'html' || content.html != null) return clipboard.write({ text: String(content.text || ''), html: String(content.html || '') })
  if (content.type === 'text' || content.text != null) return content.html ? clipboard.write({ text: String(content.text || ''), html: String(content.html) }) : clipboard.writeText(String(content.text || ''))
  const image = clipboardImageForContent(content)
  if (content.type === 'image' || image) return clipboard.writeImage(image || nativeImage.createEmpty())
}

function pasteClipboardAction(action: any) {
  const restoreClipboard = Boolean(action.restoreClipboard)
  const snapshot = restoreClipboard ? clipboardSnapshot() : null
  const content = action.content || action.clipboard || action
  writeDesktopClipboard(content, { concealed: action.concealed || restoreClipboard })
  pasteIntoFrontmostApp()
  if (restoreClipboard && snapshot) {
    const delay = Math.max(50, Math.min(5_000, Number(action.restoreDelayMs || 250)))
    setTimeout(() => restoreClipboardSnapshot(snapshot), delay).unref?.()
  }
}

async function readDesktopSelection() {
  const [text, files, app] = await Promise.all([selectedText(), selectedExtensionFiles(), frontmostApp()])
  return { text, files, sourceApp: app }
}

function extensionSourceBasename(filePath) {
  const base = path.basename(filePath || '')
  if (base.endsWith('.d.ts')) return base.slice(0, -5)
  return base.replace(/\.(cjs|ts)$/i, '')
}

function isExtensionSourceFile(filename) {
  return typeof filename === 'string' && filename.endsWith('.ts') && !filename.endsWith('.d.ts')
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

const EXTENSION_AI_ATTACHMENT_LIMIT = 8
const EXTENSION_AI_TEXT_ATTACHMENT_LIMIT = 80_000
const EXTENSION_AI_IMAGE_ATTACHMENT_MAX_BYTES = 8 * 1024 * 1024

function extensionAiDataUrlImage(dataUrl) {
  const match = String(dataUrl || '').match(/^data:([^;,]+);base64,(.*)$/s)
  if (!match) throw new Error('AI image attachment must be a base64 data URL')
  return { type: 'image' as const, mimeType: match[1], data: match[2] }
}

async function extensionAiPathImage(filePath) {
  const resolvedPath = expandUserPath(filePath)
  const stat = await fs.stat(resolvedPath)
  if (stat.size > EXTENSION_AI_IMAGE_ATTACHMENT_MAX_BYTES) throw new Error(`AI image attachment is too large: ${displayUserPath(resolvedPath)}`)
  return { type: 'image' as const, mimeType: mimeTypeForPath(resolvedPath) || 'image/png', data: (await fs.readFile(resolvedPath)).toString('base64') }
}

function isTextLikeAttachmentPath(filePath) {
  const mime = mimeTypeForPath(filePath)
  const ext = extensionForPath(filePath)
  return mime.startsWith('text/') || ['md', 'markdown', 'txt', 'json', 'csv', 'tsv', 'xml', 'html', 'css', 'js', 'ts', 'tsx', 'jsx', 'yml', 'yaml', 'log'].includes(ext)
}

async function extensionAiFileContext(filePath, options: any = {}) {
  const resolvedPath = expandUserPath(filePath)
  const stat = await fs.stat(resolvedPath).catch(() => null)
  const title = options.title || path.basename(resolvedPath)
  if (!stat) return `### ${title}\nMissing file: ${displayUserPath(resolvedPath)}`
  const metadata = await fileToExtensionFile(resolvedPath)
  const header = `### ${title}\nPath: ${metadata.displayPath}\nMIME: ${metadata.mimeType || 'unknown'}\nSize: ${metadata.size} bytes`
  if (options.as === 'metadata') return header
  if (!isTextLikeAttachmentPath(resolvedPath)) return header
  const text = await fs.readFile(resolvedPath, 'utf8')
  return `${header}\n\n${limitedOutput(text, EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`
}

async function resolveExtensionAiAttachmentList(input) {
  const output: any[] = []
  async function visit(value) {
    const resolved = await value
    if (resolved == null || resolved === false) return
    if (Array.isArray(resolved)) {
      for (const item of resolved) await visit(item)
      return
    }
    output.push(resolved)
  }
  await visit(input)
  return output.slice(0, EXTENSION_AI_ATTACHMENT_LIMIT)
}

function extensionAiOcrText(result) {
  if (typeof result === 'string') return result
  if (Array.isArray(result?.blocks)) return result.blocks.map((block) => block.text).filter(Boolean).join('\n')
  if (Array.isArray(result?.observations)) return result.observations.map((item) => item.text || item.transcript).filter(Boolean).join('\n')
  return result?.text || result?.transcript || JSON.stringify(result)
}

async function normalizeExtensionAiAttachments(extension, attachments, capabilities) {
  const textSections: string[] = []
  const images: Array<{ type: 'image'; data: string; mimeType: string }> = []
  for (const attachment of await resolveExtensionAiAttachmentList(attachments || [])) {
    if (typeof attachment === 'string') {
      textSections.push(limitedOutput(attachment, EXTENSION_AI_TEXT_ATTACHMENT_LIMIT))
      continue
    }
    const type = attachment?.type || (attachment?.text != null ? 'text' : attachment?.path || attachment?.file ? 'file' : attachment?.dataUrl || attachment?.imageDataUrl ? 'image' : '')
    if (type === 'text') {
      textSections.push(`${attachment.title ? `### ${attachment.title}\n` : ''}${limitedOutput(attachment.text || '', EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`)
      continue
    }
    if (type === 'image') {
      const source = attachment.dataUrl || attachment.imageDataUrl || attachment.data || attachment.path || attachment.file?.path || attachment.filePath
      if (attachment.data && attachment.mimeType && !String(source || '').startsWith('data:')) images.push({ type: 'image' as const, data: String(attachment.data), mimeType: String(attachment.mimeType) })
      else if (String(source || '').startsWith('data:')) images.push(extensionAiDataUrlImage(source))
      else {
        if (!capabilities.files) throw permissionDeniedError('desktop.files')
        images.push(await extensionAiPathImage(source))
      }
      if (attachment.ocr) {
        if (!capabilities.ocr) throw permissionDeniedError('ocr')
        textSections.push(`### ${attachment.title || 'OCR'}\n${limitedOutput(extensionAiOcrText(await ocrImage(source)), EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`)
      }
      continue
    }
    if (type === 'file') {
      if (!capabilities.files) throw permissionDeniedError('desktop.files')
      const filePath = attachment.path || attachment.file?.path || attachment.filePath
      if (!filePath) continue
      const resolvedPath = expandUserPath(filePath)
      const shouldAttachImage = (attachment.as === 'image' || (!attachment.as && isImagePath(resolvedPath))) && attachment.as !== 'text'
      if (shouldAttachImage) images.push(await extensionAiPathImage(resolvedPath))
      if (!shouldAttachImage || attachment.as === 'text' || attachment.as === 'metadata') textSections.push(await extensionAiFileContext(resolvedPath, attachment))
      if (attachment.ocr || attachment.as === 'ocr') {
        if (!capabilities.ocr) throw permissionDeniedError('ocr')
        textSections.push(`### ${attachment.title || `OCR ${path.basename(resolvedPath)}`}\n${limitedOutput(extensionAiOcrText(await ocrImage(resolvedPath)), EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`)
      }
    }
  }
  return { context: textSections.filter(Boolean).join('\n\n'), images }
}

function createExtensionAi(extension) {
  const extensionKey = path.basename(extension.__filePath || extension.id || 'extension').replace(/[^a-zA-Z0-9._-]/g, '-')
  const capabilities = {
    files: hasExtensionPermission(extension, 'desktop.files'),
    clipboard: hasExtensionPermission(extension, 'clipboard.history'),
    ocr: hasExtensionPermission(extension, 'ocr'),
  }
  const enforceAiQuota = () => {
    if (!checkAiRateLimit(extension)) throw Object.assign(new Error('AI rate limit exceeded'), { code: 'ai-rate-limit-exceeded', extensionId: extension?.id })
  }
  const normalizeOptions = async (options: any = {}) => {
    const normalized = await normalizeExtensionAiAttachments(extension, options.attachments || [], capabilities)
    return {
      system: options.system,
      signal: options.signal,
      context: normalized.context,
      images: normalized.images,
      onEvent: (event) => {
        if (event.type === 'delta' && event.text) options.onDelta?.(event.text)
        options.onEvent?.(event)
      },
    }
  }
  const stream = (prompt, options: any = {}, session: any = nevermindAi) => {
    enforceAiQuota()
    const controller = new AbortController()
    const removeExternalAbortListener = options.signal?.addEventListener ? (() => {
      const listener = () => controller.abort()
      options.signal.addEventListener('abort', listener, { once: true })
      return () => options.signal.removeEventListener?.('abort', listener)
    })() : () => {}
    let inner: any = null
    const result = (async () => {
      const normalized = await normalizeOptions({ ...options, signal: controller.signal })
      inner = session.stream(String(prompt || ''), normalized)
      return inner.result
    })().finally(removeExternalAbortListener)
    return { result, abort: () => { controller.abort(); inner?.abort?.() } }
  }
  return {
    ask: async (prompt, options: any = {}) => {
      enforceAiQuota()
      return nevermindAi.ask(String(prompt || ''), await normalizeOptions(options))
    },
    stream,
    session: (id = 'default', options: any = {}) => {
      const session = nevermindAi.session(`${extensionKey}:${String(id || 'default')}`, { system: options.system })
      return {
        ...session,
        ask: async (prompt: any, askOptions: any = {}) => {
          enforceAiQuota()
          return session.ask(prompt, await normalizeOptions({ ...options, ...askOptions }))
        },
        stream: (prompt: any, streamOptions: any = {}) => stream(prompt, { ...options, ...streamOptions }, session),
      }
    },
    attachments: {
      text: (text, title) => ({ type: 'text', text: String(text || ''), title }),
      image: (input, options: any = {}) => ({ ...options, type: 'image', ...(String(input || '').startsWith('data:') ? { dataUrl: input } : { path: input }) }),
      file: (input, options: any = {}) => ({ ...options, type: 'file', path: typeof input === 'string' ? input : input?.path || input?.filePath }),
      selectedText: async (title = 'Selected Text') => ({ type: 'text', title, text: await selectedText() }),
      selectedFiles: async (options: any = {}) => {
        if (!capabilities.files) throw permissionDeniedError('desktop.files')
        return (await selectedExtensionFiles()).map((file) => ({ ...options, type: 'file', file }))
      },
      clipboard: async (options: any = {}) => {
        if (!capabilities.clipboard) throw permissionDeniedError('clipboard.history')
        const item: any = await readDesktopClipboard()
        if (item.type === 'text') return { type: 'text', title: options.title || 'Clipboard', text: item.text }
        if (item.type === 'image') return { ...options, type: 'image', title: options.title || 'Clipboard Image', dataUrl: item.image }
        if (item.type === 'files') return item.files.map((file) => ({ ...options, type: 'file', file }))
        return null
      },
      ocrImage: async (input, options: any = {}) => {
        if (!capabilities.ocr) throw permissionDeniedError('ocr')
        return { type: 'text', title: options.title || 'OCR Text', text: extensionAiOcrText(await ocrImage(input, options)) }
      },
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

function fileRootItem(item) {
  return { id: `file:${item.path}`, title: item.name, subtitle: item.displayPath, icon: 'folder', score: 4, dismissAfterRun: 'auto', primaryAction: { type: 'openPath', title: `Open ${item.name}`, path: item.path, dismissAfterRun: 'auto' } }
}

function fileIndexSnapshot(options: any = {}) {
  return measureDebugPerformanceSync('files.index-snapshot', { queryLength: String(options.query || '').length, indexedCount: fileIndex.length, limit: options.limit }, () => {
    const { limit, query } = options
    const roots = normalizeFindRoots(options.roots).map(expandUserPath).filter(Boolean)
    const extensions = extensionsForFindOptions(options)
    const ignored = normalizedIgnorePatterns(options.ignore)
    let entries = fileIndex
    if (roots.length) entries = entries.filter((entry) => roots.some((root) => entry.path === root || entry.path.startsWith(`${root}${path.sep}`)))
    if (extensions) entries = entries.filter((entry) => extensions.has(entry.extension || extensionForPath(entry.path)))
    if (options.ignore) entries = entries.filter((entry) => !ignoredByPattern(entry.path, entry.name, ignored))
    if (query) {
      const needle = String(query).toLowerCase()
      entries = entries.filter((entry) => `${entry.name || ''} ${entry.displayPath || ''}`.toLowerCase().includes(needle))
    }
    const max = typeof limit === 'number' ? Math.max(0, Math.min(limit, entries.length)) : entries.length
    return entries.slice(0, max).map((entry) => ({ id: entry.id, name: entry.name, path: entry.path, displayPath: entry.displayPath, extension: entry.extension, kind: entry.kind }))
  })
}

async function reindexFiles(options: any = {}) {
  return measureDebugPerformance('files.reindex', { roots: normalizedIndexRoots(options).map(displayUserPath), alwaysLog: true }, async () => {
    fileIndex = await scanFiles(options)
    paletteWindow.win?.webContents.send('root-items:changed')
    return { count: fileIndex.length, roots: normalizedIndexRoots(options).map(displayUserPath) }
  })
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
    rootItems(ctx) {
      return ctx.desktop.apps.list().map(appRootItem)
    },
    searchItems(ctx, query) {
      return ctx.desktop.apps.list().map(appRootItem).filter((item) => rankAction(item, query))
    },
  }
}

function createFilesExtension() {
  return {
    id: 'nevermind.files',
    title: 'Files',
    permissions: ['desktop.files'] as const,
    commands: [],
    rootItems(ctx) {
      return ctx.desktop.files.recent({ limit: FILE_RESULT_LIMIT }).map(fileRootItem)
    },
    searchItems(ctx, query) {
      return ctx.desktop.files.recent().map(fileRootItem).filter((item) => rankAction(item, query)).slice(0, FILE_RESULT_LIMIT)
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
      return [compatibilityPromptAction() || updatePromptAction() || checkItem()]
    },
  }
}

function keyboardShortcutItem(record: any) {
  const changeAction = buildRecordShortcutAction({ actionId: record.actionId, title: 'Change shortcut' }, {})
  const removeAction = record.source === 'user' ? buildRemoveShortcutAction({ actionId: record.actionId, title: 'Remove shortcut' }, {}) : null
  return {
    id: `shortcut:${record.actionId}`,
    title: record.title,
    subtitle: record.subtitle,
    shortcut: record.accelerator,
    icon: 'keyboard',
    primaryAction: changeAction,
    actionPanel: { sections: [{ actions: [changeAction, removeAction].filter(Boolean) }] },
  }
}

function keyboardShortcutItems() {
  return extensionShortcutRecords().map(keyboardShortcutItem)
}

function patchKeyboardShortcutsView() {
  patchOpenView('keyboard-shortcuts', { mode: 'replace', items: keyboardShortcutItems() })
}

function createKeyboardShortcutsExtension() {
  return {
    id: 'nevermind.shortcuts',
    title: 'Keyboard Shortcuts',
    permissions: ['shortcuts'] as const,
    commands: [{
      id: 'keyboard-shortcuts',
      actionId: 'keyboard-shortcuts',
      title: 'Keyboard Shortcuts',
      subtitle: 'View, change, or remove global shortcuts',
      icon: 'keyboard',
      score: 16,
      run: (ctx) => ctx.ui.list({
        id: 'keyboard-shortcuts',
        title: 'Keyboard Shortcuts',
        presentation: 'root',
        searchBarPlaceholder: 'Search Keyboard Shortcuts',
        emptyView: { title: 'No shortcuts found.' },
        items: ctx.shortcuts.list().map(keyboardShortcutItem),
      }),
    }],
  }
}

function jobTime(value?: number) {
  return value ? new Date(value).toLocaleTimeString() : 'Never'
}

function jobTriggerSummary(job: JobSnapshot) {
  if (!job.triggers.length) return 'Manual'
  return job.triggers.map((trigger: any) => {
    if (trigger.type === 'startup') return 'Startup'
    if (trigger.type === 'interval') return `Every ${Math.round(trigger.everyMs / 1000)}s`
    if (trigger.type === 'event') return `On ${trigger.event}`
    return 'Manual'
  }).join(' · ')
}

function jobSubtitle(job: JobSnapshot) {
  const status = job.status === 'running' ? 'Running' : job.status === 'failed' ? `Failed: ${job.lastError || 'unknown error'}` : job.status === 'backing-off' ? `Backing off until ${jobTime(job.backoffUntil)}` : job.status
  return `${status} · ${jobTriggerSummary(job)} · Last: ${jobTime(job.lastFinishedAt)}`
}

function jobHistoryMarkdown(job: JobSnapshot) {
  if (!job.history.length) return '_No runs recorded yet._'
  return job.history.map((entry) => `- ${new Date(entry.finishedAt).toLocaleTimeString()} · ${entry.status} · ${entry.reason} · ${entry.durationMs}ms${entry.error ? ` · ${entry.error}` : ''}`).join('\n')
}

function jobDetailsMarkdown(job: JobSnapshot) {
  return [
    `# ${job.title}`,
    '',
    `- ID: ${job.id}`,
    `- Owner: ${job.owner}`,
    job.scope ? `- Scope: ${job.scope}` : '',
    `- Status: ${job.status}`,
    `- Enabled: ${job.enabled ? 'yes' : 'no'}`,
    `- Running: ${job.running ? 'yes' : 'no'}`,
    `- Triggers: ${jobTriggerSummary(job)}`,
    `- Runs: ${job.runCount}`,
    `- Failures: ${job.failureCount}`,
    `- Last reason: ${job.lastReason || '—'}`,
    `- Last started: ${jobTime(job.lastStartedAt)}`,
    `- Last finished: ${jobTime(job.lastFinishedAt)}`,
    job.lastDurationMs != null ? `- Last duration: ${job.lastDurationMs}ms` : '',
    job.nextRunAt ? `- Next run: ${jobTime(job.nextRunAt)}` : '',
    job.backoffUntil ? `- Backoff until: ${jobTime(job.backoffUntil)}` : '',
    `- Consecutive failures: ${job.consecutiveFailures}`,
    job.lastError ? `\n## Last error\n\n${job.lastError}` : '',
    `\n## Recent runs\n\n${jobHistoryMarkdown(job)}`,
  ].filter(Boolean).join('\n')
}

function jobItem(ctx, job: JobSnapshot) {
  const runNow = ctx.actions.run('Run Now', async () => {
    try {
      await jobRegistry.run(job.id, 'manual')
      return { toast: { message: `Ran ${job.title}` }, view: backgroundTasksView(ctx), navigation: 'replace' }
    } catch (error) {
      return { view: { type: 'preview', title: `${job.title} Failed`, content: `# ${job.title} Failed\n\n${error instanceof Error ? error.message : String(error)}` } }
    }
  })
  const toggle = ctx.actions.run(job.enabled ? 'Disable Job' : 'Enable Job', async () => {
    jobRegistry.setEnabled(job.id, !job.enabled)
    await saveUserState()
    return { toast: { message: `${job.enabled ? 'Disabled' : 'Enabled'} ${job.title}` }, view: backgroundTasksView(ctx), navigation: 'replace' }
  })
  const clearError = ctx.actions.run('Clear Error', () => {
    jobRegistry.clearError(job.id)
    return { toast: { message: `Cleared ${job.title}` }, view: backgroundTasksView(ctx), navigation: 'replace' }
  })
  const showDetails = ctx.actions.push('Show Details', { type: 'preview', title: job.title, content: jobDetailsMarkdown(job) })
  return {
    id: `job:${job.id}`,
    title: job.title,
    subtitle: jobSubtitle(job),
    icon: job.status === 'failed' || job.status === 'backing-off' ? 'circle-alert' : job.running ? 'loader' : job.enabled ? 'activity' : 'circle-pause',
    accessories: [{ text: job.owner }, { text: job.status }],
    primaryAction: showDetails,
    actionPanel: { sections: [{ actions: [showDetails, runNow, toggle, job.lastError ? clearError : null].filter(Boolean) }] },
  }
}

function backgroundTasksView(ctx) {
  const jobs = jobRegistry.snapshot()
  return ctx.ui.list({
    id: 'background-tasks',
    title: 'Background Tasks',
    subtitle: `${jobs.length} host-managed jobs`,
    presentation: 'root',
    searchBarPlaceholder: 'Search background tasks',
    emptyView: { title: 'No background tasks registered.' },
    items: jobs.map((job) => jobItem(ctx, job)),
  })
}

function createBackgroundTasksExtension() {
  return {
    id: 'nevermind.background-tasks',
    title: 'Background Tasks',
    permissions: [] as const,
    commands: [{ id: 'background-tasks', actionId: 'background-tasks', title: 'Background Tasks', subtitle: 'Inspect and run host-managed background jobs', icon: 'activity', score: 16, run: (ctx) => backgroundTasksView(ctx) }],
  }
}

function createAccountExtension() {
  const extensionId = 'nevermind.account'

  async function accountItem() {
    const existing = await getNevermindAuth()
    if (existing) {
      return {
        id: 'account-logout',
        actionId: 'account-logout',
        title: 'Log out of Nevermind',
        subtitle: `Signed in as ${existing.email}`,
        icon: 'person',
        score: 18,
        aliases: ['logout', 'sign out', 'nevermind', 'account', 'disconnect'],
        primaryAction: {
          type: 'runExtensionAction',
          title: 'Log out',
          __handler: async () => {
            const { revoked } = await signOutFromNevermind()
            activeNevermindBaseUrl = null
            await nevermindAi?.disposeAllSessions?.()
            invalidateExtensionRootItems()
            broadcastAuthChanged({ authed: false })
            const suffix = revoked ? '' : ' (token revoke failed — check connection)'
            return { toast: { message: `Logged out of ${existing.email}${suffix}`, tone: revoked ? 'default' as const : 'error' as const } }
          },
        },
      }
    }
    return {
      id: 'account-login',
      actionId: 'account-login',
      title: 'Log in to Nevermind',
      subtitle: 'Connect this device to your Nevermind account',
      icon: 'person',
      score: 18,
      aliases: ['login', 'sign in', 'nevermind', 'account', 'connect'],
      primaryAction: {
        type: 'runExtensionAction',
        title: 'Log in',
        __handler: async () => {
          const result = await signInToNevermind()
          invalidateExtensionRootItems()
          if (result.ok) broadcastAuthChanged({ authed: true, email: result.auth.email })
          const message = result.ok ? `Logged in as ${result.auth.email}` : `Log-in failed: ${'error' in result ? result.error : 'unknown'}`
          return { toast: { message, tone: result.ok ? 'default' as const : 'error' as const } }
        },
      },
    }
  }

  return {
    id: extensionId,
    title: 'Nevermind Account',
    permissions: [] as const,
    searchItems: async () => [await accountItem()],
    rootItems: async () => [await accountItem()],
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

function buildPromptAction(input: any = {}, options: any = {}) {
  const targetAction = input?.action || input?.onSubmit
  if (!targetAction) throw new Error('ctx.input.prompt requires action')
  const title = String(input?.title || targetAction.title || 'Prompt')
  return {
    ...options,
    type: 'promptAction',
    title,
    subtitle: input?.message !== undefined ? String(input.message) : options.subtitle,
    promptMessage: input?.message !== undefined ? String(input.message) : undefined,
    fields: Array.isArray(input?.fields) ? input.fields : [],
    targetAction,
    submitTitle: input?.submitTitle !== undefined ? String(input.submitTitle) : undefined,
  }
}

function promptActionView(action: any = {}) {
  const targetAction = action.targetAction || action.action
  if (!targetAction) throw new Error('Prompt action is missing its target action')
  const fields = Array.isArray(action.fields) ? action.fields : []
  const promptMessage = action.promptMessage || action.subtitle
  return {
    type: 'form',
    title: action.title || 'Prompt',
    fields: promptMessage ? [{ id: '__prompt_message', type: 'description', description: String(promptMessage) }, ...fields] : fields,
    submitAction: { ...targetAction, title: action.submitTitle || targetAction.title || 'Submit' },
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

async function safeSelectedText() {
  try { return String(await selectedText()) } catch { return '' }
}

function templateDateParts(now = new Date()) {
  return {
    date: now.toLocaleDateString(),
    time: now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    datetime: now.toLocaleString(),
  }
}

function normalizeTemplateOptions(input: any = {}) {
  const looksLikeOptions = input && typeof input === 'object' && !Array.isArray(input) && ['variables', 'cursorToken', 'returnCursor', 'returnResult', 'includeClipboard', 'includeSelectedText', 'promptMissing'].some((key) => key in input)
  return looksLikeOptions ? { ...input, variables: input.variables || {} } : { variables: input || {} }
}

async function expandTextTemplate(input: string, variablesOrOptions: Record<string, unknown> = {}, hostOptions: { includeClipboard?: boolean } = {}) {
  const options: any = normalizeTemplateOptions(variablesOrOptions)
  const cursorToken = String(options.cursorToken || '\uE000NEVERMIND_CURSOR\uE000')
  const missingVariables = new Set<string>()
  const builtins = {
    ...templateDateParts(),
    uuid: crypto.randomUUID(),
    clipboard: hostOptions.includeClipboard && options.includeClipboard !== false ? clipboard.readText() : '',
    selectedText: options.includeSelectedText === false ? '' : await safeSelectedText(),
    cursor: cursorToken,
  }
  const values = { ...builtins, ...(options.variables || {}) }
  const textWithCursor = String(input || '').replace(/\{\{\s*([^{}]+?)\s*\}\}|\{\s*([^{}]+?)\s*\}/g, (_match, doubleName, singleName) => {
    const rawName = String(doubleName || singleName || '').trim()
    if (rawName.startsWith('calculator:')) {
      const result = calculate(rawName.slice('calculator:'.length).trim())
      return result == null ? '' : String(result)
    }
    if (rawName.startsWith('argument:')) {
      const argumentName = rawName.slice('argument:'.length).trim()
      const value = values[argumentName]
      if (value == null) missingVariables.add(argumentName)
      return value == null ? '' : String(value)
    }
    const value = values[rawName]
    if (value == null) missingVariables.add(rawName)
    return value == null ? '' : String(value)
  })
  const cursor = textWithCursor.indexOf(cursorToken)
  const text = cursor >= 0 ? textWithCursor.replace(cursorToken, '') : textWithCursor
  if (options.returnCursor || options.returnResult || options.promptMissing) return { text, cursor: cursor >= 0 ? cursor : undefined, missingVariables: Array.from(missingVariables) }
  return text
}

function createExtensionContext(extension, command, launchContext?: any) {
  const canUseDesktopApps = hasExtensionPermission(extension, 'desktop.apps')
  const canUseDesktopFiles = hasExtensionPermission(extension, 'desktop.files')
  const canUseClipboard = hasExtensionPermission(extension, 'clipboard.history')
  const canUseSystem = hasExtensionPermission(extension, 'system')
  const canUseOcr = hasExtensionPermission(extension, 'ocr')
  const canUseUpdates = hasExtensionPermission(extension, 'updates')
  const canUseShortcuts = hasExtensionPermission(extension, 'shortcuts')
  const canUseAi = hasExtensionPermission(extension, 'ai')
  const canWriteSettings = hasExtensionPermission(extension, 'settings.write')
  const denyShortcut = (name: string) => () => { throw permissionDeniedError(`shortcuts (${name})`) }
  return {
    extension: createExtensionRuntimeMetadata(extension, command),
    command,
    launch: launchContext ? structuredClone(launchContext) : undefined,
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
      editor: (view) => ({ ...view, type: 'editor' }),
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
      paste: (content, title = 'Paste', options: any = {}) => ({ ...options, type: 'pasteClipboard', title, content }),
      ref: (registeredActionId, title = 'Run Action', options: any = {}) => ({ ...options, type: 'runExtensionRegisteredAction', title, extensionId: extension.id, registeredActionId }),
      typeText: (text, title = 'Type Text', options: any = {}) => ({ ...options, type: 'typeText', title, text }),
      copyImage: (image, title = 'Copy image', options: any = {}) => String(image || '').startsWith('data:') ? ({ ...options, type: 'copyImage', title, imageDataUrl: image }) : ({ ...options, type: 'copyImage', title, path: image }),
      trash: (paths, title = 'Move to Trash', options: any = {}) => ({ ...options, type: 'trash', title, paths: Array.isArray(paths) ? paths : [paths], style: options.style || 'destructive', requiresConfirmation: options.requiresConfirmation ?? true }),
      root: (title, view, options: any = {}) => ({ ...options, type: 'rootView', title, view }),
      push: (title, view, options: any = {}) => ({ ...options, type: 'pushView', title, view }),
      replace: (title, view, options: any = {}) => ({ ...options, type: 'replaceView', title, view }),
      pop: (title = 'Back', options: any = {}) => ({ ...options, type: 'popView', title }),
      run: (title, handler, options: any = {}) => ({ ...options, type: 'runExtensionAction', title, __handler: handler }),
      background: (title, handler, options: any = {}) => ({ ...options, type: 'runExtensionAction', title, __handler: handler, dismissAfterRun: options.dismissAfterRun || 'auto' }),
      shellExec: canUseSystem
        ? (title, command, args = [], options: any = {}) => ({ ...options, type: 'shellExec', title, command, args, options, requiresConfirmation: options.requiresConfirmation ?? true })
        : () => { throw permissionDeniedError('system') },
      shellScript: canUseSystem
        ? (title, script, options: any = {}) => ({ ...options, type: 'shellScript', title, script, options, requiresConfirmation: options.requiresConfirmation ?? true })
        : () => { throw permissionDeniedError('system') },
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
    action: (input) => input,
    navigation: {
      root: (view) => ({ view, navigation: 'root' }),
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
    text: {
      template: (input, variables) => expandTextTemplate(input, variables, { includeClipboard: canUseClipboard }),
    },
    input: {
      prompt: buildPromptAction,
    },
    windows: {
      create: (view, options: any = {}) => ({ dismissAfterRun: 'auto', type: 'createWindow', title: options.title || view?.title || 'Open Window', view, windowOptions: options, windowId: options.id || view?.id }),
      show: (id, title = 'Show Window', options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'showWindow', title, windowId: id }),
      hide: (id, title = 'Hide Window', options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'hideWindow', title, windowId: id }),
      toggle: (idOrView, titleOrOptions: any = 'Toggle Window', options: any = {}) => {
        if (typeof idOrView === 'string') return { dismissAfterRun: 'auto', ...options, type: 'toggleWindow', title: typeof titleOrOptions === 'string' ? titleOrOptions : titleOrOptions.title || 'Toggle Window', windowId: idOrView, windowOptions: typeof titleOrOptions === 'string' ? options : titleOrOptions }
        const windowOptions = typeof titleOrOptions === 'string' ? options : titleOrOptions || {}
        return { dismissAfterRun: 'auto', type: 'toggleWindow', title: typeof titleOrOptions === 'string' ? titleOrOptions : windowOptions.title || idOrView?.title || 'Toggle Window', view: idOrView, windowOptions, windowId: windowOptions.id || idOrView?.id }
      },
      close: (id, title = 'Close Window', options: any = {}) => ({ dismissAfterRun: 'auto', ...options, type: 'closeWindow', title, windowId: id }),
    },
    clipboard: {
      history: canUseClipboard ? {
        list: (options: any = {}) => clipboardHistorySnapshot(options),
        search: (query, options: any = {}) => clipboardHistorySnapshot({ ...options, query }),
        get: (id) => clipboardHistoryGet(String(id || '')),
        remove: async (idOrIds) => ({ removed: removeClipboardHistoryByAction({ clipboardHistoryRange: 'ids', clipboardHistoryItemIds: Array.isArray(idOrIds) ? idOrIds : [idOrIds] }) }),
        clear: async (options: any = {}) => ({ removed: removeClipboardHistoryByAction({ clipboardHistoryRange: options.olderThanMs ? 'older-than' : 'all', olderThanMs: options.olderThanMs, types: options.types }) }),
      } : undefined,
    },
    desktop: {
      keyboard: {
        typeText: (text, options: any = {}) => typeTextIntoFrontmostApp(String(text || ''), options),
      },
      clipboard: canUseClipboard ? {
        readText: () => clipboard.readText(),
        writeText: (text, options: any = {}) => writeDesktopClipboard({ type: 'text', text }, options),
        readHtml: () => clipboard.readHTML(),
        writeHtml: (html, text = '', options: any = {}) => writeDesktopClipboard({ type: 'html', html, text }, options),
        readImage: clipboardImageDataUrl,
        writeImage: (image, options: any = {}) => writeDesktopClipboard({ type: 'image', image }, options),
        readFiles: clipboardFiles,
        writeFiles: (paths, options: any = {}) => writeDesktopClipboard({ type: 'files', paths }, options),
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
        thumbnail: (filePath) => thumbnailUrlForPreviewablePath(filePath),
        metadata: (filePath) => fileToExtensionFile(filePath),
        indexedRoots: () => defaultFileIndexRoots().map(displayUserPath),
        indexSnapshot: (options: any = {}) => fileIndexSnapshot(options),
        reindex: (options: any = {}) => reindexFiles(options),
        recent: (options: any = {}) => fileIndexSnapshot(options),
        searchIndex: (query, options: any = {}) => fileIndexSnapshot({ ...options, query }),
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
    ocr: canUseOcr ? {
      image: ocrImage,
      screen: (options: any = {}) => ocrScreen(options),
      region: (rect, options: any = {}) => ocrScreen({ ...options, region: rect }),
    } : undefined,
    storage: createExtensionStorage(extension),
    settings: {
      definitions: () => availableSettingDefinitions().map((definition) => ({ ...definition, value: getSetting(definition.id) })),
      get: (id) => getSetting(id),
      set: canWriteSettings
        ? (id, value) => {
            const result = setSetting(id, value)
            if (!result.ok) throw new Error(result.message)
            return value
          }
        : () => { throw permissionDeniedError('settings.write') },
      toggle: canWriteSettings
        ? (id) => {
            const definition = settingDefinition(id)
            if (!definition) throw new Error(`Unknown setting: ${id}`)
            const next = toggledSettingValue(definition, getSetting(id))
            const result = setSetting(id, next)
            if (!result.ok) throw new Error(result.message)
            return next
          }
        : () => { throw permissionDeniedError('settings.write') },
    },
    shortcuts: canUseShortcuts ? {
      list: () => extensionShortcutRecords(),
      palette: () => ({ title: 'Open Nevermind', accelerator: String(getPaletteHotkey()), scope: 'palette' as const }),
    } : {
      list: denyShortcut('list'),
      palette: denyShortcut('palette'),
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
    remove: async (extensionFile, chatId) => {
      if (!chatCanWriteExtension(extensionFile, chatId)) return false
      const { removed } = await removeGeneratedExtensionForChat(extensionFile, chatId)
      return removed
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
    extensionApiPath: bundledResourcePath(EXTENSION_TYPES_FILENAME),
    extensionTypesPath: bundledResourcePath(EXTENSION_TYPES_FILENAME),
    skillPath: bundledResourcePath('skills', 'nevermind-extension-builder', 'SKILL.md'),
    reloadExtensions: loadExtensions,
    getShortcuts: () => extensionShortcutRecords(),
    getPaletteShortcut: () => ({ title: 'Open Nevermind', accelerator: String(getPaletteHotkey()), scope: 'palette' as const }),
    getExtensionRuntimeState: (filename) => extensionRuntimeStateForFile(filename),
    getActiveChat: () => activeAiChatId ? userState.aiChats[activeAiChatId] || draftAiChats.get(activeAiChatId) || null : null,
    getChat: (chatId) => userState.aiChats[chatId] || draftAiChats.get(chatId) || null,
    markGeneratedExtension: (filePath, chatId) => markGeneratedExtensionForActiveChat(filePath, chatId),
    canWriteExtension: (filename, chatId) => chatCanWriteExtension(filename, chatId),
    removeExtension: (filename, chatId) => removeGeneratedExtensionForChat(filename, chatId),
    addAliasForChat: (chatId) => addAliasForGeneratedAction(chatId),
    onEvent: (event) => {
      const chatId = event.chatId || activeAiChatId
      const metadata = chatId ? aiLearningMetadata(chatId) : undefined
      if (chatId && event.type === 'start') learningStore?.recordStatus(chatId, 'start', metadata)
      if (chatId && event.type === 'delta' && event.text) appendAiChatDelta(chatId, event.text)
      if (chatId && event.type === 'tool_start' && event.name) appendAiChatMessage(chatId, 'system', event.name)
      if (chatId && event.type === 'tool_trace_start' && event.name) learningStore?.recordToolStart(chatId, event.name, (event.data as any)?.input, metadata, (event.data as any)?.toolCallId)
      if (chatId && event.type === 'tool_trace_end' && event.name) {
        const toolData = event.data as any
        learningStore?.recordToolEnd(chatId, event.name, { ok: !event.isError, outputSummary: toolData?.output, error: toolData?.error, toolCallId: toolData?.toolCallId }, metadata)
        if (['write_extension', 'validate_extension', 'remove_extension', 'install_extension'].includes(event.name)) {
          learningStore?.recordExtensionEvent(chatId, {
            kind: event.name as 'write_extension' | 'validate_extension' | 'remove_extension' | 'install_extension',
            filename: toolData?.output?.filePath || toolData?.input?.filename,
            extensionId: toolData?.output?.extensionId,
            commandIds: Array.isArray(toolData?.output?.commandIds) ? toolData.output.commandIds : undefined,
            ok: !event.isError,
            error: toolData?.error,
            details: toolData?.output,
          }, metadata)
        }
      }
      if (chatId && event.type === 'error' && event.message) appendAiChatMessage(chatId, 'system', event.message)
      if (chatId && event.type === 'done' && userState.aiChats[chatId]) {
        if (userState.aiChats[chatId].status !== 'ready') userState.aiChats[chatId].status = 'done'
        userState.aiChats[chatId].updatedAt = Date.now()
        learningStore?.recordStatus(chatId, 'done', metadata)
        patchAiChatsItem(chatId)
        scheduleSaveState()
      }
      if (chatId && event.type === 'error' && event.message) {
        learningStore?.recordStatus(chatId, 'error', metadata, event.message)
      }
      if (chatId && event.type === 'aborted') {
        learningStore?.recordStatus(chatId, 'aborted', metadata)
      }
      paletteWindow.win?.webContents.send('ai:chat:event', { ...event, chatId })
    },
  })
}

function extensionRuntimeStateForFile(filename) {
  const base = path.basename(filename || '')
  const matches = Array.from(extensionActionRegistry.values()).filter((entry) => path.basename(entry.extension?.__filePath || '') === base)
  return {
    loaded: matches.length > 0,
    extensionId: matches[0]?.extension?.id,
    commandIds: matches.map((entry) => entry.command?.id).filter(Boolean),
  }
}

function aiChatPromptWithContext(message, chatId) {
  const chat = userState.aiChats[chatId]
  const focused = chat?.contextExtensionFile ? `\n\nFocused extension file: ${chat.contextExtensionFile}. Use read_current_extension before editing it. You may list/read other extensions if needed.` : ''
  const learnings = relevantLearningContext(message, chatId)
  const messages = (chat?.messages || [])
    .filter((item) => item.role === 'user' || item.role === 'assistant')
    .slice(-12)

  if (!messages.length) return `Use this Nevermind AI chat transcript as context. If the user has provided enough details, proceed by calling read_extension_api immediately; do not merely say you will.${focused}${learnings}\n\nNew user message:\n${message}`

  const transcript = messages
    .map((item) => `${item.role === 'user' ? 'User' : 'Assistant'}: ${item.content}`)
    .join('\n\n')

  return `Use this Nevermind AI chat transcript as context. Do not ask questions that the user already answered. If the user has now provided enough details, proceed by calling read_extension_api immediately; do not merely say you will.${focused}${learnings}\n\n${transcript}\n\nNew user message:\n${message}`
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

async function removeGeneratedExtensionForChat(filename, chatId = activeAiChatId) {
  const base = path.basename(filename || '')
  if (!base || !isExtensionSourceFile(base)) return { removed: false }
  if (!chatCanWriteExtension(base, chatId)) throw new Error(`Refusing to remove ${base}: this AI chat does not own that extension.`)
  const filePath = path.join(extensionsDir, base)
  const existed = await fs.stat(filePath).then(() => true).catch(() => false)
  if (!existed) return { removed: false, filePath }
  await fs.unlink(filePath)
  await removeAiChatReferencesToExtensionFile(base, chatId)
  scheduleSaveState()
  await loadExtensions()
  registerActionShortcuts()
  return { removed: true, filePath }
}

function addAliasForGeneratedAction(chatId) {
  const chat = userState.aiChats[chatId]
  if (!chat?.query) return
  const files = chatTouchedExtensionFiles(chat)
  const entry = Array.from(extensionActionRegistry.values()).find((e) => files.includes(path.basename(e.extension?.__filePath || '')))
  if (!entry) return
  const action = extensionActionFromContribution(entry)
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

async function noteAiChatExited(chatId) {
  if (!chatId) return
  recordLearningReview(chatId)
}

async function ensureExtensionTypeDefinitions() {
  if (!extensionsDir) return
  const sourcePath = bundledResourcePath(EXTENSION_TYPES_FILENAME)
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
  await measureDebugPerformance('extensions.load-all', { alwaysLog: true }, async () => {
    extensionActionRegistry.clear()
    extensionModules.clear()
    fixtureExtensions = []
    extensionRootItemsCache.clear()
    extensionRootItemsRefreshes.clear()
    extensionActionHandlers.clear()
    viewActionExecutionRecords.clear()
    rootActionExecutionRecords.clear()
    viewRefreshRecords.clear()
    jobRegistry.unregisterWhere((job) => job.owner === 'extension')
    for (const watcher of extensionFileWatchers) watcher.close()
    extensionFileWatchers = []
    registerInternalExtensions()
    if (isDev) await measureDebugPerformance('extensions.load-dev', undefined, () => loadDevExtensions())

    await fs.mkdir(extensionsDir, { recursive: true })
    await ensureExtensionTypeDefinitions()
    const entries = await fs.readdir(extensionsDir, { withFileTypes: true }).catch(() => [])
    for (const entry of entries) {
      if (!isExtensionSourceFile(entry.name)) continue
      const fullPath = path.join(extensionsDir, entry.name)
      try {
        await measureDebugPerformance('extension.load-file', { file: entry.name }, async () => {
          const extension = await loadExtensionModule(fullPath)
          extension.__filePath = fullPath
          extension.__generated = true
          await applyExtensionMetadataOverrides(extension)
          registerExtension(extension)
        })
      } catch (error) {
        logError('extension.load.failed', error, { source: 'host', scope: 'extension', extensionId: path.basename(fullPath) })
      }
    }
    markDebugPerformance('extensions.load-all.result', { extensionCount: extensionModules.size, actionCount: extensionActionRegistry.size })
  })
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

function fixturePersistentActionItems(fixture) {
  const registeredEntries = Array.from(extensionActionRegistry.values()).filter((entry) => entry.extension.id === fixture.id)
  return registeredEntries.map((entry) => {
    const item = entry.item
    const persistentAction = extensionActionFromContribution(entry)
    return {
      ...item,
      id: `fixture-action:${fixture.id}:${item.id}`,
      persistentAction,
      primaryAction: persistentAction?.rootAction || item.primaryAction,
      subtitle: item.subtitle || `Persistent action · ${fixture.title || fixture.id}`,
      accessories: [...(item.accessories || []), { text: entry.source === 'command' ? 'command' : 'action' }],
    }
  })
}

function fixturesIndexView(ctx) {
  return ctx.ui.list({
    id: 'extension-api-fixtures',
    title: 'Extension API Fixtures',
    subtitle: 'Dev-only runnable fixtures for host-rendered extension UI',
    searchBarPlaceholder: 'Search fixture commands and persistent actions',
    emptyView: { title: 'No fixtures found', subtitle: 'Add fixture extensions under src/fixtures.' },
    sections: fixtureExtensions.map((fixture) => ({
      title: fixture.title || fixture.id,
      subtitle: fixture.subtitle || 'Extension API fixture',
      items: fixturePersistentActionItems(fixture),
    })),
  })
}

function fixturesRootItem(ctx) {
  const persistentItems = fixtureExtensions.flatMap((fixture) => fixturePersistentActionItems(fixture))
  const runnableCount = persistentItems.length
  return {
    id: 'fixtures',
    title: 'Fixtures',
    subtitle: `${fixtureExtensions.length} dev-only extension API ${fixtureExtensions.length === 1 ? 'fixture' : 'fixtures'} · ${runnableCount} ${runnableCount === 1 ? 'item' : 'items'}`,
    icon: 'wrench',
    aliases: ['fixture', 'fixtures', 'dev fixtures', 'extension fixtures', ...fixtureExtensions.map((fixture) => fixture.title || fixture.id), ...persistentItems.map((item) => item.title)].filter(Boolean),
    score: 100,
    primaryAction: ctx.actions.push('Open Fixtures', fixturesIndexView(ctx)),
  }
}

function createFixturesExtension() {
  return {
    id: 'dev.fixtures',
    title: 'Fixtures',
    subtitle: 'Dev-only extension API fixtures',
    rootItems(ctx) {
      return [fixturesRootItem(ctx)]
    },
    searchItems(ctx) {
      return [fixturesRootItem(ctx)]
    },
  }
}

async function loadDevExtensions() {
  const fixturesDir = path.join(app.getAppPath(), 'src', 'fixtures')
  const entries = await fs.readdir(fixturesDir, { withFileTypes: true }).catch(() => [])
  for (const entry of entries) {
    if (!isExtensionSourceFile(entry.name)) continue
    const fullPath = path.join(fixturesDir, entry.name)
    try {
      const extension = await loadExtensionModule(fullPath)
      extension.__filePath = fullPath
      extension.__dev = true
      extension.__fixture = true
      fixtureExtensions.push(extension)
      registerExtension(extension)
    } catch (error) {
      logError('extension.dev.load.failed', error, { source: 'host', scope: 'extension', extensionId: path.basename(fullPath) })
    }
  }
  if (fixtureExtensions.length) registerExtension(createFixturesExtension())
}

function registerInternalExtensions() {
  for (const createExtension of INTERNAL_EXTENSION_FACTORIES) registerExtension(createExtension())
  assertInternalExtensionsRegistered()
}

function durationMs(value: any) {
  if (typeof value === 'number') return Math.max(1000, value)
  const match = String(value || '').trim().match(/^(\d+(?:\.\d+)?)\s*(ms|s|m|h|d)?$/i)
  if (!match) return 0
  const amount = Number(match[1])
  const unit = (match[2] || 'ms').toLowerCase()
  const multiplier = unit === 'd' ? 86_400_000 : unit === 'h' ? 3_600_000 : unit === 'm' ? 60_000 : unit === 's' ? 1000 : 1
  return Math.max(1000, Math.round(amount * multiplier))
}

function triggerPermission(extension, trigger: any) {
  if (trigger?.type === 'clipboard.changed') return hasExtensionPermission(extension, 'clipboard.history')
  if (trigger?.type === 'files.changed') return hasExtensionPermission(extension, 'desktop.files')
  if (trigger?.type === 'app.frontmost.changed') return hasExtensionPermission(extension, 'desktop.apps')
  return true
}

function normalizedFileTrigger(trigger: any) {
  const roots = normalizeFindRoots(trigger.roots).map(expandUserPath).filter((root) => root && path.isAbsolute(root))
  return {
    ...trigger,
    roots,
    includeHidden: Boolean(trigger.includeHidden),
    extensions: extensionsForFindOptions(trigger) || null,
    ignored: normalizedIgnorePatterns(trigger.ignore),
  }
}

function fileWatchChangedPath(root: string, filename: string | Buffer | null) {
  if (!filename) return root
  const value = String(filename)
  return path.isAbsolute(value) ? value : path.join(root, value)
}

function fileWatchPathMatches(filePath: string, trigger: any) {
  const name = path.basename(filePath)
  if (!trigger.includeHidden && name.startsWith('.')) return false
  if (ignoredByPattern(filePath, name, trigger.ignored || [])) return false
  const ext = extensionForPath(filePath)
  if (trigger.extensions && !trigger.extensions.has(ext)) return false
  if (trigger.kind === 'image' && !isImagePath(filePath)) return false
  if (trigger.kind === 'video' && !isVideoPath(filePath)) return false
  if (trigger.kind === 'media' && !isImagePath(filePath) && !isVideoPath(filePath)) return false
  return true
}

function watchExtensionFileTrigger(trigger: any, event: string) {
  const normalized = normalizedFileTrigger(trigger)
  for (const root of normalized.roots) {
    if (!fsSync.existsSync(root)) continue
    try {
      const watcher = fsSync.watch(root, { recursive: process.platform === 'darwin' || process.platform === 'win32' }, (_eventType, filename) => {
        const changedPath = fileWatchChangedPath(root, filename)
        if (!fileWatchPathMatches(changedPath, normalized)) return
        jobRegistry.emit(event, { trigger: normalizedFileTriggerForLaunch(trigger), root: displayUserPath(root), changedPaths: [changedPath] })
      })
      watcher.on('error', () => {})
      extensionFileWatchers.push(watcher)
    } catch {}
  }
}

function normalizedFileTriggerForLaunch(trigger: any) {
  return {
    type: 'files.changed',
    roots: normalizeFindRoots(trigger.roots).map(displayUserPath),
    debounceMs: trigger.debounceMs || 0,
    includeHidden: Boolean(trigger.includeHidden),
    extensions: trigger.extensions,
    kind: trigger.kind,
    ignore: trigger.ignore,
  }
}

function extensionTriggerForLaunch(trigger: any) {
  if (!trigger) return undefined
  if (trigger.type === 'files.changed') return normalizedFileTriggerForLaunch(trigger)
  return structuredClone(trigger)
}

function jobTriggersFromExtensionTriggers(extension, triggers: any[] = [], jobId = '') {
  return triggers.map((trigger, index) => {
    if (!triggerPermission(extension, trigger)) {
      logWarn('extension.jobTrigger.permissionDenied', { trigger: trigger?.type }, { source: 'host', scope: 'extension', extensionId: extension.id })
      return null
    }
    if (trigger?.type === 'startup') return { type: 'startup' as const, delayMs: trigger.delayMs || 0, payload: { trigger: extensionTriggerForLaunch(trigger) } }
    if (trigger?.type === 'login') return { type: 'event' as const, event: 'login', debounceMs: trigger.debounceMs || 0, payload: { trigger: extensionTriggerForLaunch(trigger) } }
    if (trigger?.type === 'wake') return { type: 'event' as const, event: 'wake', debounceMs: trigger.debounceMs || 0, payload: { trigger: extensionTriggerForLaunch(trigger) } }
    if (trigger?.type === 'interval') {
      const everyMs = durationMs(trigger.every)
      return everyMs ? { type: 'interval' as const, everyMs, delayMs: trigger.delayMs, payload: { trigger: extensionTriggerForLaunch(trigger) } } : null
    }
    if (trigger?.type === 'clipboard.changed') return { type: 'event' as const, event: 'clipboard.changed', debounceMs: trigger.debounceMs || 0, payload: { trigger: extensionTriggerForLaunch(trigger) } }
    if (trigger?.type === 'app.frontmost.changed') return { type: 'event' as const, event: 'app.frontmost.changed', debounceMs: trigger.debounceMs || 0, payload: { trigger: extensionTriggerForLaunch(trigger) } }
    if (trigger?.type === 'files.changed') {
      const event = `files.changed:${jobId}:${index}`
      watchExtensionFileTrigger(trigger, event)
      return { type: 'event' as const, event, debounceMs: trigger.debounceMs || 0, payload: { trigger: extensionTriggerForLaunch(trigger) } }
    }
    return null
  }).filter(Boolean)
}

async function extensionLaunchContextFromJob(context: any) {
  const payload = context?.payload && typeof context.payload === 'object' ? context.payload : {}
  const changedPaths = Array.isArray(payload.changedPaths) ? payload.changedPaths.map((value) => expandUserPath(String(value))).filter(Boolean).slice(-100) : []
  const files = changedPaths.length ? await Promise.all(changedPaths.map((filePath) => fileToExtensionFile(filePath))) : []
  return structuredClone({
    trigger: payload.trigger,
    files,
    changedPaths,
    reason: context?.reason || 'manual',
    event: context?.event,
    startedAt: context?.startedAt || Date.now(),
  })
}

function registerExtensionBackgroundJob(entry, item) {
  const mode = item.mode || (item.background ? 'background' : 'view')
  const id = `extension.${entry.extension.id}.${item.id}`
  const triggers = jobTriggersFromExtensionTriggers(entry.extension, item.triggers || [], id)
  if (mode === 'view' && triggers.length === 0) return
  jobRegistry.register({
    id,
    title: item.title,
    owner: 'extension',
    scope: entry.extension.id,
    triggers,
    timeoutMs: Number(item.timeoutMs || EXTENSION_ROOT_ITEMS_TIMEOUT_MS),
    run: async (context) => {
      const action = item.primaryAction || item.action
      if (!action) return
      const launchContext = await extensionLaunchContextFromJob(context)
      const result = await executeViewAction(action, launchContext)
      if (result?.view) logInfo('extension.background.viewIgnored', { jobId: id, title: item.title }, { source: 'host', scope: 'extension', extensionId: entry.extension.id, commandId: item.id })
    },
  })
}

function assertInternalExtensionsRegistered() {
  const missingExtensions = REQUIRED_INTERNAL_EXTENSIONS.filter((extensionId) => !extensionModules.has(extensionId))
  const missingCommands = REQUIRED_INTERNAL_COMMANDS.filter(({ extensionId, commandId }) => !extensionActionRegistry.has(`${extensionId}:${commandId}`))
  if (missingExtensions.length || missingCommands.length) {
    const details = [
      missingExtensions.length ? `extensions: ${missingExtensions.join(', ')}` : '',
      missingCommands.length ? `commands: ${missingCommands.map(({ extensionId, commandId }) => `${extensionId}:${commandId}`).join(', ')}` : '',
    ].filter(Boolean).join('; ')
    throw new Error(`Missing required internal extensions (${details})`)
  }
}

function registerExtension(extension) {
  measureDebugPerformanceSync('extension.register', { extensionId: extension?.id, commandCount: extension?.commands?.length || 0 }, () => {
  if (!extension?.id) return
  extensionModules.set(extension.id, extension)
  for (const command of extension.commands || []) {
    if (!command?.id || !command.title || typeof command.run !== 'function') continue
    const entry = { extension, command, source: 'command' }
    const action = { type: 'runExtensionAction', title: command.title, __handler: async (ctx, actionArg) => command.run(ctx, actionArg) }
    const item = {
      id: command.id,
      actionId: command.actionId || extensionCommandActionId(extension, command),
      title: command.title,
      subtitle: command.subtitle || extension.title || 'Extension command',
      aliases: command.aliases || [],
      icon: command.icon || 'sparkles',
      score: command.score || 12,
      shortcut: command.shortcut,
      shortcutScope: command.shortcutScope,
      globalShortcut: command.globalShortcut,
      dismissAfterRun: command.dismissAfterRun,
      appearance: normalizeItemAppearance(command.appearance),
      background: command.background || command.mode === 'background' || command.mode === 'noView',
      mode: command.mode,
      triggers: command.triggers,
      primaryAction: action,
    }
    const normalizedItem = normalizeViewItems([item], entry)[0]
    extensionActionRegistry.set(`${extension.id}:${command.id}`, { ...entry, item: normalizedItem })
    registerExtensionBackgroundJob(entry, normalizedItem)
  }
  if (typeof extension.actions === 'function') {
    try {
      const result = extension.actions(createExtensionContext(extension, null))
      const items = Array.isArray(result) ? result : Array.isArray(result?.actions) ? result.actions : []
      const entry = { extension, command: { id: 'actions', title: extension.title || extension.id }, source: 'action' }
      for (const item of items) {
        if (!item?.id || !item.title) continue
        const action = item.run ? { type: 'runExtensionAction', title: item.title, __handler: item.run } : item.action
        const normalizedItem = normalizeViewItems([{ ...item, background: item.background || item.mode === 'background' || item.mode === 'noView', primaryAction: action }], entry)[0]
        extensionActionRegistry.set(`${extension.id}:${item.id}`, { ...entry, item: normalizedItem })
        registerExtensionBackgroundJob(entry, normalizedItem)
      }
    } catch (error) {
      logError('extension.actions.failed', error, { source: 'host', scope: 'extension', extensionId: extension.id })
    }
  }
  })
}

function displayUserPath(filePath) {
  const home = os.homedir()
  return filePath.startsWith(home) ? `~${filePath.slice(home.length)}` : filePath
}

const DEFAULT_FILE_INDEX_IGNORES = ['node_modules', '.git', 'Library', 'Applications']
const DEFAULT_FILE_INDEX_LIMIT = 5_000
const MAX_FILE_INDEX_LIMIT = 20_000

function defaultFileIndexRoots() {
  return ['Desktop', 'Documents', 'Downloads'].map((name) => path.join(os.homedir(), name))
}

function normalizedIndexRoots(options: any = {}) {
  const roots = normalizeFindRoots(options.roots)
  return (roots.length ? roots : defaultFileIndexRoots()).map(expandUserPath).filter((root) => root && path.isAbsolute(root))
}

function normalizedIgnorePatterns(ignore) {
  const values = Array.isArray(ignore) ? ignore : ignore ? [ignore] : []
  return [...DEFAULT_FILE_INDEX_IGNORES, ...values].map((value) => String(value).trim()).filter(Boolean)
}

function wildcardPatternMatches(pattern, value) {
  if (!pattern.includes('*')) return false
  const escaped = pattern.split('*').map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('.*')
  return new RegExp(`^${escaped}$`, 'i').test(value)
}

function ignoredByPattern(fullPath, name, patterns) {
  return patterns.some((pattern) => pattern === name || wildcardPatternMatches(pattern, name) || fullPath.includes(pattern))
}

async function scanFiles(options: any = {}) {
  return measureDebugPerformance('files.scan', { roots: normalizedIndexRoots(options).map(displayUserPath), depth: options.depth ?? 2, limit: options.limit || DEFAULT_FILE_INDEX_LIMIT, alwaysLog: true }, async () => {
  const roots = normalizedIndexRoots(options)
  const ignored = normalizedIgnorePatterns(options.ignore)
  const includeHidden = Boolean(options.includeHidden)
  const maxDepth = options.depth ?? 2
  const limit = Math.max(1, Math.min(Number(options.limit || DEFAULT_FILE_INDEX_LIMIT), MAX_FILE_INDEX_LIMIT))
  const extensions = extensionsForFindOptions(options)
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
      if (found.length >= limit) return
      if (!includeHidden && entry.name.startsWith('.')) continue
      const fullPath = path.join(dir, entry.name)
      if (ignoredByPattern(fullPath, entry.name, ignored)) continue
      if (entry.isFile()) {
        const ext = extensionForPath(entry.name)
        if (extensions && !extensions.has(ext)) continue
        found.push({
          id: fullPath,
          name: entry.name,
          path: fullPath,
          displayPath: displayUserPath(fullPath),
          extension: ext,
          kind: isImagePath(fullPath) ? 'image' : isVideoPath(fullPath) ? 'video' : 'file',
        })
        continue
      }
      if (entry.isDirectory() && depth > 0) await walk(fullPath, depth - 1)
    }
  }

  await Promise.all(roots.map((root) => walk(root, maxDepth)))
  markDebugPerformance('files.scan.result', { foundCount: found.length, limit })
  return found.slice(0, limit)
  })
}

function scheduleIndexApplications() {
  jobRegistry.emit('apps.changed')
}

async function startAppWatcher() {
  for (const watcher of appWatchers) watcher.close()
  appWatchers = watchApps(scheduleIndexApplications)
}

async function indexApplications() {
  await measureDebugPerformance('apps.index', { alwaysLog: true }, async () => {
    try {
      const apps = await scanApps()
      const deduped = new Map()
      for (const item of apps) deduped.set(normalize(item.name), item)
      appIndex = Array.from(deduped.values()).sort((a, b) => a.name.localeCompare(b.name))
      markDebugPerformance('apps.index.result', { scannedCount: apps.length, indexedCount: appIndex.length })
      paletteWindow.win?.webContents.send('apps:indexed', appIndex.length)
    } catch (error) {
      logError('applications.index.failed', error, { source: 'host', scope: 'apps' })
    }
  })
}

async function indexFiles() {
  await measureDebugPerformance('files.index', { alwaysLog: true }, async () => {
    try {
      fileIndex = await scanFiles()
      markDebugPerformance('files.index.result', { indexedCount: fileIndex.length })
    } catch (error) {
      logError('files.index.failed', error, { source: 'host', scope: 'files' })
    }
  })
}

async function pollFrontmostAppChange() {
  if (!hasCapability('frontmost-app')) return
  const current: any = await frontmostApp()
  const currentId = current?.bundleId || current?.path || current?.name || ''
  if (!currentId || currentId === frontmostWatcherLastId) return
  frontmostWatcherLastId = currentId
  jobRegistry.emit('app.frontmost.changed')
}

function registerHostJobs() {
  jobRegistry.register({
    id: 'state.save',
    title: 'Save User State',
    owner: 'host',
    scope: 'state',
    timeoutMs: 5_000,
    run: saveUserState,
  })
  jobRegistry.register({
    id: 'apps.index',
    title: 'Application Index',
    owner: 'host',
    scope: 'apps',
    triggers: [{ type: 'startup', delayMs: 100 }, { type: 'event', event: 'apps.changed', debounceMs: APP_REINDEX_DEBOUNCE_MS }],
    timeoutMs: 30_000,
    run: indexApplications,
  })
  jobRegistry.register({
    id: 'files.index',
    title: 'File Index',
    owner: 'host',
    scope: 'files',
    triggers: [{ type: 'startup', delayMs: 200 }],
    timeoutMs: 30_000,
    run: indexFiles,
  })
  jobRegistry.register({
    id: 'frontmost-app.poll',
    title: 'Frontmost App Poll',
    owner: 'host',
    scope: 'apps',
    triggers: [{ type: 'interval', everyMs: 5_000, delayMs: 5_000 }],
    timeoutMs: 3_000,
    run: pollFrontmostAppChange,
  })
  jobRegistry.register({
    id: 'cache.app-icons',
    title: 'App Icon Cache',
    owner: 'host',
    scope: 'cache',
    timeoutMs: 15_000,
    run: processPendingAppIcons,
  })
  jobRegistry.register({
    id: 'cache.thumbnails',
    title: 'Thumbnail Cache',
    owner: 'host',
    scope: 'cache',
    timeoutMs: 20_000,
    run: processPendingThumbnails,
  })
}

async function ensureLocalFileUrlSecret() {
  const secretPath = path.join(app.getPath('userData'), 'local-file-url-secret')
  const existing = await fs.readFile(secretPath, 'utf8').catch(() => '')
  const secret = existing.trim() || crypto.randomBytes(32).toString('base64url')
  if (!existing.trim()) {
    await fs.mkdir(path.dirname(secretPath), { recursive: true })
    await fs.writeFile(secretPath, secret, { mode: 0o600 })
  }
  configureLocalFileUrlSecret(secret)
}

async function loadUserState() {
  const cacheRoot = osCacheRoot()
  await ensureLocalFileUrlSecret()
  statePath = path.join(app.getPath('userData'), 'state.json')
  learningRulesPath = path.join(app.getPath('userData'), LEARNING_RULES_FILENAME)
  legacyLearningRulesPath = path.join(app.getPath('userData'), LEGACY_LEARNING_RULES_FILENAME)
  learningTracesPath = path.join(app.getPath('userData'), LEARNING_TRACES_FILENAME)
  iconCacheDir = path.join(cacheRoot, 'icons')
  clipboardImagesDir = path.join(app.getPath('userData'), 'clipboard-images')
  extensionsDir = path.join(app.getPath('userData'), 'extensions')
  extensionStorageDir = path.join(app.getPath('userData'), 'extension-storage')
  extensionCacheDir = path.join(cacheRoot, 'extension-storage')
  learningStore = new LocalLearningStore({ tracesPath: learningTracesPath, learningsPath: learningRulesPath, legacyLearningsPath: legacyLearningRulesPath })
  await learningStore.load()
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
      jobSettings: loaded.jobSettings || {},
    }
  } catch {
    // First run.
  }

  migrateAiChats()
  clipboardHistory = normalizeClipboardHistory(userState.clipboardHistory, CLIPBOARD_LIMIT, persistClipboardImage)
  jobRegistry.hydrateEnabled(userState.jobSettings?.enabled || {})
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
  jobRegistry.emit('clipboard.changed')
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
  if (jobRegistry.has('state.save')) {
    jobRegistry.schedule('state.save', 'state.changed', 200)
    return
  }
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(saveUserState, 200)
  saveTimer.unref?.()
}

async function saveUserState() {
  userState.clipboardHistory = clipboardHistory
  userState.jobSettings = { ...(userState.jobSettings || {}), enabled: jobRegistry.enabledOverridesSnapshot() }
  await fs.mkdir(path.dirname(statePath), { recursive: true })
  await fs.writeFile(statePath, JSON.stringify(userState, null, 2)).catch((error) => {
    logError('state.save.failed', error, { source: 'host', scope: 'state' })
  })
}

function pollClipboardChange() {
  const item = readClipboardItem()
  if (!item || item.id === clipboardWatcherLastId) return
  const suppressUntil = suppressedClipboardItemIds.get(item.id) || 0
  if (suppressUntil > Date.now()) {
    clipboardWatcherLastId = item.id
    return
  }
  if (suppressUntil) suppressedClipboardItemIds.delete(item.id)
  clipboardWatcherLastId = item.id
  rememberClipboardItem(item)
}

function startClipboardWatcher() {
  clipboardWatcherLastId = readClipboardItem()?.id || ''
  jobRegistry.register({
    id: 'clipboard.poll',
    title: 'Clipboard Poll',
    owner: 'host',
    scope: 'clipboard',
    triggers: [{ type: 'interval', everyMs: CLIPBOARD_POLL_INTERVAL_MS, delayMs: CLIPBOARD_POLL_INTERVAL_MS }],
    timeoutMs: 2_000,
    run: pollClipboardChange,
  })
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
    paletteWindow.win?.webContents.send('action:view-open', normalizeHostViewResult({
      view: progressView({ title: currentAction?.title || 'Opening…', label: 'Opening…' }),
      revealWhenReady: true,
      asSibling: false,
    }))
  }
  const result = normalizeHostViewResult(await executeAction(currentAction, { keepPaletteOpen: true }))
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
  return visibleExtensionActionEntries().map((entry) => {
    const accelerator = entry.item.globalShortcut || (entry.item.shortcutScope === 'global' ? entry.item.shortcut : null)
    if (!accelerator) return null
    const action = extensionActionFromContribution(entry)
    return action ? { actionId: action.id, accelerator: normalizeAccelerator(accelerator), action } : null
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
      accelerator: String(accelerator),
      scope: 'global',
      source: 'user' as const,
      action: currentActionForStoredShortcut(userState.shortcutActions[actionId]),
    }))
    .filter((item) => item.action)
  const declared = declaredGlobalShortcuts()
    .filter((item) => !userState.shortcuts[item.actionId] && !userState.removedShortcuts?.[item.actionId])
    .map((item) => ({ ...item, accelerator: String(item.accelerator), scope: 'global' as const, source: 'extension' as const }))
  return [...configured, ...declared]
    .sort((a, b) => a.action.title.localeCompare(b.action.title))
}

function extensionShortcutRecords() {
  return getShortcuts().map((item) => ({
    actionId: item.actionId,
    title: item.action.title,
    subtitle: item.action.subtitle,
    accelerator: item.accelerator,
    scope: 'global' as const,
    source: item.source,
  }))
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
  if (!canCustomizeAction(action)) return { ok: false, message: 'Aliases are only available for persistent actions' }
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
  if (!canCustomizeAction(action)) return { ok: false, message: 'Shortcuts are only available for persistent actions' }
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

function setShortcutSetting(id, accelerator) {
  const definition = settingDefinition(id)
  if (!definition || definition.type !== 'shortcut') return { ok: false, message: 'Setting not found' }
  if (id === 'paletteHotkey') return { ok: false, message: 'Use palette hotkey registration' }
  if (!accelerator?.trim()) return { ok: false, message: 'Missing shortcut' }
  const normalized = normalizeAccelerator(accelerator)
  setSetting(id, normalized)
  return { ok: true, message: `${definition.title} set: ${normalized}` }
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
  if (!['extension-root-item', 'extension-action'].includes(action?.kind) || !action.removable) return { ok: false, message: 'Only generated extensions can be duplicated' }
  const extension = extensionModuleForAction(action)
  const filePath = extension?.__filePath
  if (!filePath) return { ok: false, message: 'Generated extension not found' }

  const duplicateId = hashValue(`${filePath}:${Date.now()}`)
  const duplicateExtensionId = `${extension.id}-copy-${duplicateId.slice(0, 8)}`
  const duplicateTitle = `Copy of ${extension.title || action.title}`
  const duplicateFile = `${extensionSourceBasename(filePath)}-copy-${duplicateId.slice(0, 8)}.ts`
  const sourceFile = path.basename(filePath)
  const sourceCode = `import type { NevermindExtension } from './${EXTENSION_TYPES_FILENAME.replace(/\.d\.ts$/, '')}'\nimport source from './${sourceFile.replace(/'/g, "\\'")}'\n\nconst duplicateExtensionId = ${JSON.stringify(duplicateExtensionId)}\nconst namespacedActionId = (actionId: unknown) => typeof actionId === 'string' && actionId ? duplicateExtensionId + ':' + actionId : undefined\nconst duplicateContributions = (items: any[]) => items.map((item) => ({ ...item, ...(item.actionId ? { actionId: namespacedActionId(item.actionId) } : {}) }))\n\nexport default {\n  ...source,\n  id: duplicateExtensionId,\n  title: ${JSON.stringify(duplicateTitle)},\n  commands: (source.commands || []).map((command) => ({ ...command, ...(command.actionId ? { actionId: namespacedActionId(command.actionId) } : {}) })),\n  actions: source.actions ? (ctx) => {\n    const result = source.actions(ctx)\n    const items = Array.isArray(result) ? result : Array.isArray(result?.actions) ? result.actions : []\n    return duplicateContributions(items)\n  } : undefined,\n} satisfies NevermindExtension\n`

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
  const targetRegisteredActionId = action.registeredActionId || action.commandId
  const duplicateEntry = targetRegisteredActionId
    ? extensionActionRegistry.get(`${duplicateExtensionId}:${targetRegisteredActionId}`)
    : Array.from(extensionActionRegistry.values()).find((candidate) => candidate.extension?.id === duplicateExtensionId)
  return { ok: true, message: 'Action duplicated', action: duplicateEntry ? extensionActionFromContribution(duplicateEntry) : { id: `ai-tweak-extension:${duplicateFile}`, kind: 'ai-tweak-extension', extensionFile: duplicateFile, title: duplicateTitle, subtitle: 'Tweak extension with AI', icon: 'sparkles', score: 0 } }
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

async function removeAiChatReferencesToExtensionFile(extensionFile, preserveChatId?: string) {
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
    if (remainingFiles.length === 0 && chat.id !== preserveChatId) {
      await removeAiChat(chat.id)
      continue
    }
    chat.touchedExtensionFiles = remainingFiles
    if (remainingFiles.length === 0) {
      delete chat.contextExtensionFile
      delete chat.generatedExtensionFile
    }
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

  if (['extension-root-item', 'extension-action'].includes(action?.kind) && action.removable) {
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

function ipcHandleMeasured(channel: string, handler: (event: Electron.IpcMainInvokeEvent, ...args: any[]) => unknown) {
  ipcMain.handle(channel, (event, ...args) => measureDebugPerformance(`ipc.${channel}.handler`, { args: args.map(summarizeDebugValue), alwaysLog: true }, () => handler(event, ...args)))
}

async function pickFormFieldPaths(event, input: any = {}) {
  const senderWindow = BrowserWindow.fromWebContents(event.sender) || paletteWindow.win || undefined
  const type = input.type === 'folder' ? 'folder' : input.type === 'files' ? 'files' : 'file'
  const properties: Array<'openFile' | 'openDirectory' | 'multiSelections' | 'createDirectory'> = type === 'folder' ? ['openDirectory'] : ['openFile']
  if (type === 'files') properties.push('multiSelections')
  if (type === 'folder' && input.canCreateDirectories !== false) properties.push('createDirectory')
  const filters = Array.isArray(input.extensions) && input.extensions.length
    ? [{ name: input.filterName || 'Allowed files', extensions: input.extensions.map((value) => String(value).replace(/^\./, '')).filter(Boolean) }]
    : undefined
  const result = await dialog.showOpenDialog(senderWindow, {
    title: input.title || (type === 'folder' ? 'Choose Folder' : type === 'files' ? 'Choose Files' : 'Choose File'),
    buttonLabel: input.buttonLabel || 'Choose',
    properties,
    filters,
    defaultPath: typeof input.defaultPath === 'string' ? expandUserPath(input.defaultPath) : undefined,
  })
  return result.canceled ? { canceled: true, paths: [] } : { canceled: false, paths: result.filePaths }
}

app.whenReady().then(async () => {
  nativeTheme.themeSource = 'dark'
  prepareAppWindowPolicy()
  registerLocalFileProtocol()
  installPermissionHandlers(isDev)
  updateManager.configure()
  updateManager.onStateChange(() => { patchUpdatesView(); invalidateExtensionRootItems() })
  onNevermindCompatibilityChanged(() => invalidateExtensionRootItems())

  await loadUserState()
  registerHostJobs()
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
  await startAppWatcher()
  powerMonitor.on('resume', () => jobRegistry.emit('wake'))
  jobRegistry.emit('login')

  ipcHandleMeasured('actions:search', (_event, query, options) => searchActions(query, options))
  ipcHandleMeasured('actions:execute', (_event, action) => executeActionForIpc(action))
  ipcHandleMeasured('view-action:execute', (_event, action) => executeViewActionForIpc(action))
  ipcMain.handle('view:refresh', (event, input) => measureDebugPerformance('ipc.view:refresh.handler', { args: [summarizeDebugValue(input)], alwaysLog: true }, () => refreshViewForIpc(input)))
  ipcHandleMeasured('dialog:pick-form-field-paths', pickFormFieldPaths)
  ipcMain.on('drag:file', startFileDrag)
  ipcHandleMeasured('ai:chat:send', (_event, message, chatId) => sendAiChatMessage(message, chatId))
  ipcHandleMeasured('ai:chat:exited', (_event, chatId) => noteAiChatExited(chatId))
  ipcHandleMeasured('ai:chat:abort', (_event, chatId) => abortAiChat(chatId))
  ipcHandleMeasured('ai:chat:reset', (_event, chatId) => resetAiChat(chatId))
  ipcHandleMeasured('actions:set-alias', (_event, action, alias) => setAlias(action, alias))
  ipcHandleMeasured('actions:remove-alias', (_event, action, alias) => removeAlias(action, alias))
  ipcHandleMeasured('actions:set-shortcut', (_event, action, shortcut) => setShortcut(action, shortcut))
  ipcHandleMeasured('palette:set-hotkey', (_event, accelerator) => setPaletteHotkey(accelerator))
  ipcHandleMeasured('settings:get', (_event, id) => getSetting(id))
  ipcHandleMeasured('system:open-keyboard-settings', () => openSystemKeyboardSettings())
  ipcHandleMeasured('actions:get-shortcuts', () => getShortcuts())
  ipcHandleMeasured('actions:remove-shortcut', (_event, actionId) => removeShortcut(actionId))
  ipcHandleMeasured('actions:suspend-shortcuts', () => unregisterActionShortcuts())
  ipcHandleMeasured('actions:resume-shortcuts', () => registerActionShortcuts())
  ipcHandleMeasured('actions:set-override', (_event, action, instruction) => setOverride(action, instruction))
  ipcHandleMeasured('actions:clear-override', (_event, action) => clearOverride(action))
  ipcHandleMeasured('actions:duplicate-created', (_event, action) => duplicateCreatedAction(action))
  ipcHandleMeasured('actions:remove-created', (_event, action) => removeCreatedAction(action))
  ipcHandleMeasured('ai-builder:tweak-extension', (_event, input: any = {}) => {
    const file = input?.extensionFile || input?.extensionId
    if (!file) return { toast: { message: 'No extension specified', tone: 'error' } }
    const item = getOrCreateExtensionChat(file, input.title || file)
    return normalizeHostViewResult({ view: aiChatView(item, { initialPrompt: input.prompt }) })
  })
  ipcHandleMeasured('ai-builder:start-chat', (_event, input: any = {}) => {
    const item = createDraftAiChat(String(input?.prompt || input?.query || ''))
    return normalizeHostViewResult({ view: aiChatView(item, { start: item.messages.length <= 1 }) })
  })
  ipcHandleMeasured('nevermind:auth-status', async () => {
    const auth = await getNevermindAuth()
    activeNevermindBaseUrl = auth?.baseUrl || null
    if (auth?.baseUrl) warmNevermindCompatibilityCache(auth.baseUrl)
    logInfo('nevermind.auth-status.check', { authed: Boolean(auth), email: auth?.email, userData: app.getPath('userData') }, { source: 'host', scope: 'nevermind' })
    return auth ? { authed: true, email: auth.email } : { authed: false }
  })
  ipcHandleMeasured('nevermind:sign-in', async () => {
    const result = await signInToNevermind()
    if (result.ok) {
      activeNevermindBaseUrl = result.auth.baseUrl
      warmNevermindCompatibilityCache(result.auth.baseUrl)
      invalidateExtensionRootItems()
      broadcastAuthChanged({ authed: true, email: result.auth.email })
      return { ok: true, email: result.auth.email }
    }
    return { ok: false, error: 'error' in result ? result.error : 'unknown' }
  })
  ipcHandleMeasured('apps:icon', (_event, appPath) => getAppIconDataUrl(appPath))
  ipcHandleMeasured('palette:set-mode', (_event, mode) => {
    paletteWindow.setPaletteSizeForMode(mode)
    paletteWindow.centerWindow()
  })
  ipcHandleMeasured('palette:hide', () => paletteWindow.hidePalette())
  ipcHandleMeasured('app:quit', () => {
    requestQuitApp('ipc')
    return { ok: true }
  })
  ipcHandleMeasured('palette:shortcut-ready', () => paletteWindow.revealPalette())
  ipcHandleMeasured('camera:request-access', async () => {
    if (!hasCapability('camera')) return { ok: false, status: 'unsupported' }
    if (process.platform !== 'darwin') return { ok: true, status: 'unknown' }
    const status = systemPreferences.getMediaAccessStatus('camera')
    if (status === 'granted') return { ok: true, status }
    if (status === 'denied' || status === 'restricted') return { ok: false, status }
    return { ok: true, status }
  })
  ipcHandleMeasured('extension-window:get-state', (_event, id) => {
    const record = extensionWindows.get(String(id || ''))
    return record ? { id: record.id, view: record.view, options: record.options } : null
  })
  ipcHandleMeasured('extension-window:close', (event) => {
    const win = BrowserWindow.fromWebContents(event.sender)
    win?.close()
  })
  ipcHandleMeasured('logs:write', (_event, level, message, data) => {
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
