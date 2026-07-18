// biome-ignore-all lint: This Electron entry point follows established imperative startup conventions.
import { execFile, spawn } from 'node:child_process';
import crypto from 'node:crypto';
import { createReadStream, watch } from 'node:fs';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { Readable } from 'node:stream';
import { fileURLToPath, pathToFileURL } from 'node:url';

// Global safety net: unhandled rejections and uncaught exceptions must never
// crash the app. An extension action or shell call that throws unexpectedly
// should be logged and contained, not silently kill the entire process.
process.on('unhandledRejection', (reason) => {
  console.error('FATAL: unhandled rejection (would crash Electron):', reason);
  captureException(reason, { source: 'unhandledRejection' });
});
process.on('uncaughtException', (error) => {
  console.error('FATAL: uncaught exception (would crash Electron):', error);
  captureException(error, { source: 'uncaughtException' });
});

import {
  app,
  BrowserWindow,
  clipboard,
  dialog,
  globalShortcut,
  ipcMain,
  nativeImage,
  nativeTheme,
  net,
  powerMonitor,
  protocol,
  screen,
  shell,
  systemPreferences,
} from 'electron';
import electronUpdater from 'electron-updater';
import { createNevermindAi } from './ai';
import { getByoKey } from './byo-key';
import { createClipboardHistory } from './clipboard-history';
import { normalizeClipboardHistory } from './clipboard-utils';
import {
  DEEP_LINK_SCHEME,
  type ParsedAuthDeepLink,
  parseAuthDeepLink,
  setDeepLinkLogger,
} from './deep-link';
import {
  configureLocalFileUrlSecret,
  expandUserPath,
  extensionForPath,
  fileUrlForPath,
  IMAGE_EXTENSIONS,
  isImagePath,
  isVideoPath,
  LOCAL_FILE_PROTOCOL,
  LOCAL_THUMB_PROTOCOL,
  partitionRootsByExistence,
  thumbnailUrlForPath,
  VIDEO_EXTENSIONS,
  verifyLocalFileToken,
} from './file-utils';
import {
  consumeDeviceCode,
  getDefaultNevermindBaseUrl,
  getNevermindAuth,
  isSigningIn,
  nevermindEnvironmentForBaseUrl,
  setActiveNevermindAuthBaseUrl,
  signInToNevermind,
} from './nevermind-auth';
import { switchNevermindBackendEnvironment as switchBackendEnvironment } from './nevermind-backend-environment';
import {
  checkNevermindCompatibility,
  currentNevermindCompatibilityManifest,
  invalidateNevermindCompatibilityCache,
  onNevermindCompatibilityChanged,
  warmNevermindCompatibilityCache,
} from './nevermind-compatibility';
import { resolvesToUnsafeNevermindAddress } from './nevermind-url';
import { captureException, initSentry } from './sentry';
import {
  configureNvmTestMode,
  installTestNetworkPolicy,
  isNvmTestMode,
  recordTestWindowEvent,
} from './test-mode';

configureNvmTestMode();
if (isNvmTestMode && process.env.NVM_TEST_USER_DATA_DIR)
  app.setPath('userData', path.resolve(process.env.NVM_TEST_USER_DATA_DIR));
if (!isNvmTestMode) initSentry();

import { feedbackView } from '../feedback';
import { type CommandAction, canCustomizeCommandAction } from '../model';
import {
  appResultMarker,
  compareRankedActions,
  priorityBoost,
} from './action-ranking';
import { readAppBundleIconPng } from './app-bundle-icons';
import { createAppIconCache } from './app-icon-cache';
import { createAppIndexService } from './app-index-service';
import { registerAppIpcHandlers } from './app-ipc-handlers';
import {
  createProductionAppUninstallService,
  NEVERMIND_BUNDLE_ID,
} from './app-uninstall-service';
import {
  createDataLoaderHandle,
  createStaleWhileRevalidateHandle,
  createViewLoaderRegistry,
  isLoaderHandle,
  normalizeLoaderItems,
  resolveLoaderEmptyView,
} from './data-loader';
import {
  markDebugPerformance,
  measureDebugPerformance,
  measureDebugPerformanceSync,
  summarizeDebugValue,
} from './debug-performance';
import { filterWebviewPermissionsForExtension } from './extension-capabilities';
import { createExtensionJsonStore } from './extension-json-store';
import { createStandaloneExtensionFork } from './extension-manifest';
import { createExtensionPrSubmitter } from './extension-pr-submitter';
import { createExtensionStorage as createPersistentExtensionStorage } from './extension-storage';
import { createExtensionUiApi } from './extension-ui-api';
import { createExtensionWindowManager } from './extension-window-manager';
import { INTERNAL_EXTENSION_FACTORIES } from './extensions';
import { initExtensionContext } from './extensions/_context';
import { createAiBuilderExtension } from './extensions/ai-builder';
import {
  applyDateAdded,
  findFilesNeedsStats,
  includeDimensionsForFindOptions,
  selectFindFiles,
  sortFoundFiles,
} from './file-index-sorting';
import { hasEnabledExtensionEventSubscriber } from './frontmost-app-polling';
import { markInternalExtension } from './internal-extension';
import { type JobDefinition, JobRegistry, type JobSnapshot } from './jobs';
import { type LearningKind, LocalLearningStore } from './learning-store';
import {
  configureLogger,
  extensionLogger,
  error as logError,
  debug as loggerDebug,
  info as logInfo,
  warn as logWarn,
} from './logger';
import {
  autoUpdatesUnavailableMessage,
  captureScreenImage,
  runningAppPaths as detectRunningAppPaths,
  executeSystemBuiltin,
  fileDateAddedMs,
  forceQuitApp as forceQuitOsApp,
  frontmostApp,
  getLaunchAtLoginEnabled,
  hasCapability,
  keyboardSettingsSubtitle,
  launchApp as launchOsApp,
  osLabel,
  pasteIntoFrontmostApp,
  prepareAppWindowPolicy,
  quickLookTitle,
  recognizeTextInImage,
  reservedPaletteShortcutName,
  revealPathTitle,
  scanApps,
  selectedFilePaths,
  selectedText,
  setLaunchAtLoginEnabled,
  settingsTitle,
  typeTextIntoFrontmostApp,
  watchApps,
} from './os';
import {
  createPaletteWindowController,
  installPermissionHandlers,
} from './palette-window';
import { createRunningAppStatusService } from './running-app-status';
import {
  calculate,
  calculateDetailed,
  calculateRateResult,
  getUrlFromQuery,
  hashValue,
  normalize,
  parseRateExpression,
  score,
  scoreNormalized,
} from './search-utils';
import {
  SETTING_DEFINITIONS,
  settingDefinition,
  settingValue,
  toggledSettingValue,
} from './settings';
import { buildShortcutByAiChatIdMap } from './shortcut-ownership';
import { isSpotlightAccelerator, normalizeAccelerator } from './shortcut-utils';
import { systemSettingsPaneUrl } from './system-settings';
import { createUpdateManager } from './update-manager';
import { openExternalUrl } from './url-utils';
import { isNewerVersion as isVersionNewerThan } from './version-utils';
import {
  installExternalNavigationPolicy,
  isTrustedExtensionWindowPage,
} from './window-navigation-policy';

const { autoUpdater } = electronUpdater;
const isDev = Boolean(process.env.ELECTRON_RENDERER_URL);
configureLogger(isDev);
setDeepLinkLogger({ warn: logWarn });

const updateManager: any = isNvmTestMode
  ? {
      state: { status: 'unsupported' as const },
      canUseAutoUpdates: () => false,
      configure: () => {},
      onStateChange: () => {},
      checkForUpdates: async () => {},
      downloadAvailableUpdate: async () => {},
      quitAndInstall: () => false,
      clearTimers: () => {},
    }
  : createUpdateManager(autoUpdater as any);
const rendererUrl = process.env.ELECTRON_RENDERER_URL;
const preloadPath = path.join(__dirname, '..', 'preload', 'preload.cjs');
const rendererIndexPath = path.join(__dirname, '..', 'renderer', 'index.html');
const paletteWindow = createPaletteWindowController({
  isDev: Boolean(rendererUrl),
  preloadPath,
  rendererUrl,
  rendererIndexPath,
  getPaletteHotkey: () => String(getPaletteHotkey()),
});
const extensionWindowManager = createExtensionWindowManager({
  BrowserWindow,
  preloadPath,
  rendererIndexPath,
  rendererUrl,
  isDev,
  shouldUseDarkColors: () => nativeTheme.shouldUseDarkColors,
  getCursorScreenPoint: () => screen.getCursorScreenPoint(),
  getDisplayNearestPoint: (point) => screen.getDisplayNearestPoint(point),
  normalizeView: (view) => normalizeView(view, null),
  hashValue,
  installNavigationPolicy: installExternalNavigationPolicy,
  isTrustedPage: (url, id) =>
    isTrustedExtensionWindowPage(
      url,
      id,
      isDev,
      rendererUrl,
      rendererIndexPath,
    ),
  debug: (message, data) =>
    loggerDebug(message, data, { source: 'host', scope: 'extensions' }),
});
const appIconCache = createAppIconCache({
  hasAppIcons: () => hasCapability('app-icons'),
  hashValue,
  readCachedIcon: (cacheKey) =>
    fs.readFile(path.join(iconCacheDir, `${cacheKey}.png`)).catch(() => null),
  writeCachedIcon: async (cacheKey, png) => {
    await fs.mkdir(iconCacheDir, { recursive: true });
    await fs.writeFile(path.join(iconCacheDir, `${cacheKey}.png`), png);
  },
  loadIcon: async (appPath) => {
    const bundleIconPng = await readAppBundleIconPng(appPath);
    if (bundleIconPng) return bundleIconPng;

    const icon = await app.getFileIcon(appPath, { size: 'normal' });
    if (icon.isEmpty()) throw new Error(`No icon for ${appPath}`);
    return icon.toPNG();
  },
  schedule: (reason, delayMs = 0) =>
    jobRegistry.schedule('cache.app-icons', reason, delayMs),
  mark: markDebugPerformance,
  measure: measureDebugPerformance,
  warn: (message, data) =>
    logWarn(message, data, { source: 'host', scope: 'apps' }),
});

const CLIPBOARD_LIMIT = 300;
const FILE_RESULT_LIMIT = 6;
const CLIPBOARD_POLL_INTERVAL_MS = 1000;
const CLIPBOARD_LAST_HOUR_MS = 60 * 60_000;
const CLIPBOARD_LAST_DAY_MS = 24 * CLIPBOARD_LAST_HOUR_MS;
const APP_REINDEX_DEBOUNCE_MS = 1000;
const THUMBNAIL_SIZE = 512;
const EXTENSION_ROOT_ITEMS_TTL_MS = 60_000;
const EXTENSION_ROOT_ITEMS_TIMEOUT_MS = 10_000;
const EXTENSION_ITEMS_PER_PROVIDER_LIMIT = 20;
const ITEM_FOREGROUND_COLORS = new Set([
  'yellow',
  'blue',
  'purple',
  'green',
  'red',
  'orange',
  'pink',
]);
const EXTENSION_CACHE_MAX_TTL_MS = 24 * 60 * 60_000;
const EXTENSION_CACHE_MAX_ENTRIES = 1000;
const EXTENSION_REFRESH_MAX_BURST = 5;
const EXTENSION_REFRESH_BURST_WINDOW_MS = 2000;
const EXTENSION_AI_CALLS_PER_MINUTE = 30;
const EXTENSION_AI_RATE_WINDOW_MS = 60_000;
const EXTENSION_TYPES_FILENAME = 'nevermind-extension-api.d.ts';
const FRONTMOST_APP_CHANGED_EVENT = 'app.frontmost.changed';
const FRONTMOST_APP_POLL_JOB_ID = 'frontmost-app.poll';

function trustedExtensionApiUnavailable(capability: string) {
  return new Error(`Trusted extension API is unavailable: ${capability}`);
}
const LEARNING_RULES_FILENAME = 'ai-learnings.md';
const LEGACY_LEARNING_RULES_FILENAME = 'ai-learnings.json';
const LEARNING_TRACES_FILENAME = 'ai-learning-traces.json';

protocol.registerSchemesAsPrivileged([
  {
    scheme: LOCAL_FILE_PROTOCOL,
    privileges: { standard: true, secure: true, supportFetchAPI: true },
  },
  {
    scheme: LOCAL_THUMB_PROTOCOL,
    privileges: { standard: true, secure: true, supportFetchAPI: true },
  },
]);

type AnyRecord = Record<string, any>;

type NevermindApp = typeof app & { isQuiting?: boolean };

const nevermindApp = app as NevermindApp;

async function bundledResourcePath(...relativePath) {
  const candidates = [
    path.join(app.getAppPath(), 'src', 'resources', ...relativePath),
    path.join(
      process.resourcesPath,
      'app.asar',
      'src',
      'resources',
      ...relativePath,
    ),
    path.join(process.resourcesPath, 'src', 'resources', ...relativePath),
  ];
  for (const candidate of candidates) {
    try {
      await fs.access(candidate);
      return candidate;
    } catch {}
  }
  return candidates[0];
}

let fileIndex: any[] = [];
let clipboardHistory: any[] = [];
const suppressedClipboardItemIds = new Map<string, number>();
let clipboardService: ReturnType<typeof createClipboardHistory> | null = null;
let statePath = '';
let iconCacheDir = '';
let clipboardImagesDir = '';
let extensionsDir = '';
let extensionPrSubmitter: ReturnType<typeof createExtensionPrSubmitter> | null =
  null;
let extensionStorageDir = '';
let extensionCacheDir = '';
let learningRulesPath = '';
let legacyLearningRulesPath = '';
let learningTracesPath = '';
let saveTimer: NodeJS.Timeout | undefined;
let stateWriteQueue: Promise<void> = Promise.resolve();
type ExtensionFileWatcher = PreparedFileWatcher & {
  close: () => unknown;
};
let extensionFileWatchers: ExtensionFileWatcher[] = [];
type PreparedFileWatcher = {
  extensionId: string;
  event: string;
  trigger: any;
};
type PreparedExtensionRuntime = {
  filename: string;
  extensionIds: Set<string>;
  modules: Map<string, any>;
  actions: Map<string, any>;
  handlers: Map<string, any>;
  viewActions: Map<string, any>;
  rootActions: Map<string, any>;
  viewRefreshes: Map<string, any>;
  jobs: JobDefinition[];
  fileWatchers: PreparedFileWatcher[];
};
type LiveExtensionRuntimeSnapshot = {
  extensionIds: Set<string>;
  modules: Map<string, any>;
  actions: Map<string, any>;
  handlers: Map<string, any>;
  viewActions: Map<string, ExtensionExecutionRecord>;
  rootActions: Map<string, ExtensionExecutionRecord>;
  viewRefreshes: Map<string, any>;
  jobs: JobDefinition[];
  fileWatchers: PreparedFileWatcher[];
};
let extensionRuntimePreparation:
  | Pick<PreparedExtensionRuntime, 'jobs' | 'fileWatchers'>
  | undefined;
let testExtensionActivationFailurePhase: string | undefined;
let frontmostWatcherLastId = '';
const jobRegistry = new JobRegistry();
let nevermindAi: any;
let learningStore: LocalLearningStore | null = null;
const learningReviewJobs = new Map<string, Promise<void>>();
let activeAiChatId: string | undefined;
const draftAiChats = new Map<string, AnyRecord>();
const viewLoaderRegistry = createViewLoaderRegistry({
  sendHydrate: (viewId, payload) =>
    paletteWindow.win?.webContents.send('view:hydrate', { viewId, ...payload }),
  normalizeItems: (items, entry) => normalizeViewItems(items, entry),
  warn: (viewId, message) =>
    logWarn(
      'view.loader.failed',
      { viewId, error: message },
      { source: 'host', scope: 'extension' },
    ),
  readCache: (extension) => readExtensionCache(extension),
  mutateCache: async (extension, update) => {
    await mutateExtensionCache(extension, update);
  },
});

function spawnPendingViewLoaders(result: any) {
  const viewId = result?.view?.id;
  if (viewId && viewLoaderRegistry.has(viewId))
    viewLoaderRegistry.spawn(viewId);
}
let didRunQuitCleanup = false;
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
  rateCache: {},
  extensionManager: { schemaVersion: 1, files: {}, proposals: {} },
  nevermindEnvironment: {
    environment: nevermindEnvironmentForBaseUrl(getDefaultNevermindBaseUrl()),
    baseUrl: getDefaultNevermindBaseUrl(),
  },
};

clipboardService = createClipboardHistory({
  getHistory: () => clipboardHistory,
  setHistory: (h) => {
    clipboardHistory = h;
  },
  getSuppressedItemIds: () => suppressedClipboardItemIds,
  getImagesDir: () => clipboardImagesDir,
  clipboard,
  nativeImage,
  ensureDir: (dir) => fs.mkdir(dir, { recursive: true }).then(() => {}),
  writeFile: (filePath, data) => fs.writeFile(filePath, data),
  hashValue,
  fileUrlForPath,
  thumbnailUrlForPath,
  isVideoPath,
  expandUserPath,
  isImagePath,
  extensionForPath,
  pathJoin: path.join.bind(path),
  pathBasename: path.basename.bind(path),
  pathToFileURL: (filePath) => pathToFileURL(filePath),
  logWarn: (message, data, opts) => logWarn(message, data, opts),
  measureSync: measureDebugPerformanceSync,
  scheduleSaveState,
  invalidateExtensionRootItems,
  emitChanged: () => jobRegistry.emit('clipboard.changed'),
  sendToRenderer: (channel, ...args) =>
    paletteWindow.win?.webContents.send(channel, ...args),
  patchOpenView,
  pasteIntoFrontmostApp,
  getSetting,
  buildPreviewItemAction,
  rankAction,
  fileToExtensionFile,
  findFiles,
  selectedFilePaths,
  selectedExtensionFiles,
  selectedText,
  selectedFiles,
  frontmostApp,
  readDesktopSelection,
  CLIPBOARD_LIMIT,
  CLIPBOARD_POLL_INTERVAL_MS,
  CLIPBOARD_LAST_HOUR_MS,
  CLIPBOARD_LAST_DAY_MS,
});

function osCacheRoot() {
  const appName = app.getName();
  if (process.platform === 'darwin')
    return path.join(os.homedir(), 'Library', 'Caches', appName);
  if (process.platform === 'win32')
    return path.join(
      process.env.LOCALAPPDATA || app.getPath('appData'),
      appName,
      'Cache',
    );
  return path.join(
    process.env.XDG_CACHE_HOME || path.join(os.homedir(), '.cache'),
    appName,
  );
}

function getPaletteHotkey() {
  return getSetting('paletteHotkey') || 'Alt+Space';
}

function settingIsAvailable(definition: any) {
  return !definition?.capability || hasCapability(definition.capability);
}

function availableSettingDefinitions() {
  return SETTING_DEFINITIONS.filter(settingIsAvailable);
}

function getSetting(id: any) {
  const definition = settingDefinition(String(id));
  if (!(definition && settingIsAvailable(definition))) return;
  if (definition.id === 'startAtLogin') return getLaunchAtLoginEnabled();
  return settingValue(userState.settings, definition.id);
}

function setSetting(id: any, value: any) {
  const definition = settingDefinition(String(id));
  if (!definition) return { ok: false, message: 'Setting not found' };
  if (!settingIsAvailable(definition))
    return {
      ok: false,
      message: `${definition.title} is not available on ${osLabel()}`,
    };
  if (definition.id === 'startAtLogin') {
    const result = setLaunchAtLoginEnabled(Boolean(value));
    if (!result.ok) return result;
  }
  if (!userState.settings) userState.settings = {};
  userState.settings[definition.id] = value;
  scheduleSaveState();
  invalidateExtensionRootItems();
  patchSettingsView(definition.id);
  return { ok: true, message: `${definition.title} updated` };
}

function aiLearningMetadata(chatId: string) {
  const chat = userState.aiChats[chatId] || draftAiChats.get(chatId);
  return {
    query: chat?.query,
    title: chat?.title,
    contextExtensionFile: chat?.contextExtensionFile,
    extensionFiles: chatTouchedExtensionFiles(chat),
  };
}

function relevantLearningContext(message: string, chatId: string) {
  if (!learningStore) return '';
  const metadata = aiLearningMetadata(chatId);
  const learnings = learningStore.relevantLearnings({
    message,
    query: metadata.query,
    contextExtensionFile: metadata.contextExtensionFile,
    limit: 4,
  });
  if (!learnings.length) return '';
  const lines = learnings.map(
    (learning) =>
      `- ${learning.summary}${learning.appliesWhen ? ` When relevant: ${learning.appliesWhen}.` : ''}`,
  );
  return `\n\nLocal machine learnings for future extension builds:\n${lines.join('\n')}`;
}

function normalizedLearningReview(response: string) {
  const raw = String(response || '').trim();
  if (!raw) return [];
  const fencedMatch = raw.match(/```(?:json)?\s*([\s\S]*?)```/i);
  const candidate = (fencedMatch?.[1] || raw).trim();
  const parsed = JSON.parse(candidate) as {
    learnings?: Array<{
      kind?: string;
      summary?: string;
      appliesWhen?: string;
      keywords?: string[];
      confidence?: string;
      evidence?: string;
    }>;
  };
  return (parsed.learnings || [])
    .map((learning) => ({
      kind:
        learning.kind === 'workflow' || learning.kind === 'preference'
          ? learning.kind
          : 'environment',
      summary: String(learning.summary || '').trim(),
      appliesWhen: learning.appliesWhen
        ? String(learning.appliesWhen).trim()
        : undefined,
      keywords: Array.isArray(learning.keywords)
        ? learning.keywords.map(String)
        : [],
      confidence:
        learning.confidence === 'low' || learning.confidence === 'high'
          ? learning.confidence
          : 'medium',
      evidence: learning.evidence
        ? String(learning.evidence).trim()
        : undefined,
    }))
    .filter((learning) => learning.summary);
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
${JSON.stringify(snapshot, null, 2)}`;
}

function recordLearningReview(chatId: string) {
  const jobId = `ai.learning.review.${chatId}`;
  if (
    !learningStore?.shouldReview(chatId) ||
    learningReviewJobs.has(chatId) ||
    jobRegistry.snapshot().some((job) => job.id === jobId && job.running) ||
    !nevermindAi?.ask
  )
    return;
  const snapshot = learningStore.reviewSnapshot(chatId);
  if (!snapshot) return;
  jobRegistry.register({
    id: jobId,
    title: `AI Learning Review: ${chatId}`,
    owner: 'host',
    scope: 'ai',
    timeoutMs: 60_000,
    run: async () => {
      try {
        const response = await nevermindAi.ask(learningReviewPrompt(snapshot), {
          system:
            'You maintain a tiny canonical set of generic user learnings for future Nevermind extension-building chats. Merge and rewrite rules instead of appending; omit one-off implementation details. Return strict JSON and keep the final set minimal.',
        });
        const learnings = normalizedLearningReview(response);
        learningStore?.replaceLearningsFromReview(
          chatId,
          learnings as Array<{
            kind: LearningKind;
            summary: string;
            appliesWhen?: string;
            keywords?: string[];
            confidence?: 'low' | 'medium' | 'high';
            evidence?: string;
          }>,
        );
      } catch (error) {
        logWarn(
          'ai.learning.review.failed',
          {
            chatId,
            error: error instanceof Error ? error.message : String(error),
          },
          { source: 'host', scope: 'ai' },
        );
        throw error;
      } finally {
        learningReviewJobs.delete(chatId);
      }
    },
  });
  const job = jobRegistry
    .run(jobId, 'ai-chat-exited')
    .then(() => undefined)
    .catch(() => undefined);
  learningReviewJobs.set(chatId, job);
}

const appIndexService = createAppIndexService({
  scanApps,
  watchApps,
  normalize,
  emitChanged: () => jobRegistry.emit('apps.changed'),
  invalidateRunningStatus: () => runningAppStatus.invalidate(),
  scheduleRunningStatusRefresh: (reason) =>
    runningAppStatus.scheduleRefresh(reason),
  notifyIndexed: (count) =>
    paletteWindow.win?.webContents.send('apps:indexed', count),
  measure: measureDebugPerformance,
  mark: markDebugPerformance,
  error: (message, error) =>
    logError(message, error, { source: 'host', scope: 'apps' }),
});

const RUNNING_APPS_SNAPSHOT_TTL_MS = 1500;
const runningAppStatus = createRunningAppStatusService({
  ttlMs: RUNNING_APPS_SNAPSHOT_TTL_MS,
  getCandidates: () => appIndexService.get(),
  detectRunningAppPaths,
  notifyChanged: () =>
    paletteWindow.win?.webContents.send('apps:running-paths-changed'),
  measure: measureDebugPerformance,
  mark: markDebugPerformance,
  onRefreshFailed: (error) =>
    logWarn('apps.running.snapshot.failed', error, {
      source: 'host',
      scope: 'apps',
    }),
});
const nevermindBundlePath =
  process.execPath.match(/^(.*\.app)(?:\/|$)/)?.[1] || null;
const appUninstallService = createProductionAppUninstallService({
  trashItem: (itemPath) => shell.trashItem(itemPath),
  nevermindAppPath: nevermindBundlePath,
  nevermindBundleId: NEVERMIND_BUNDLE_ID,
  runningAppPaths: (appPath) =>
    detectRunningAppPaths([
      ...appIndexService.get(),
      { id: `uninstall:${appPath}`, path: appPath },
    ]),
});
const pendingThumbnailPaths = new Map<string, string>();
const extensionActionRegistry = new Map<string, any>();
const extensionModules = new Map<string, any>();
let fixtureExtensions: any[] = [];
const extensionRootItemsCache = new Map<
  string,
  { updatedAt: number; items: any[] }
>();
const extensionRootItemsRefreshes = new Map<string, Promise<any[]>>();
const extensionStorageRefreshes = new Map<string, Promise<any>>();
const extensionJsonStore = createExtensionJsonStore();
const extensionActionHandlers = new Map<string, any>();
type ExtensionExecutionRecord = {
  action: any;
  createdAt: number;
  extensionId?: string;
  extensionFile?: string;
};
const viewActionExecutionRecords = new Map<string, ExtensionExecutionRecord>();
const rootActionExecutionRecords = new Map<string, ExtensionExecutionRecord>();
const viewRefreshRecords = new Map<
  string,
  {
    entry: any;
    action: any | null;
    viewId?: string;
    mode?: any;
    createdAt: number;
    running?: Promise<any> | null;
    failureCount: number;
    backoffUntil?: number;
  }
>();
const extensionCaches = new Map<
  string,
  Map<string, { value: any; expiresAt: number }>
>();

function extensionCacheFor(extensionId) {
  let store = extensionCaches.get(extensionId);
  if (!store) {
    store = new Map();
    extensionCaches.set(extensionId, store);
  }
  return store;
}

function enforceExtensionCacheBudget(
  store: Map<string, { value: any; expiresAt: number }>,
) {
  if (store.size <= EXTENSION_CACHE_MAX_ENTRIES) return;
  const overflow = store.size - EXTENSION_CACHE_MAX_ENTRIES;
  const iterator = store.keys();
  for (let i = 0; i < overflow; i++) {
    const next = iterator.next();
    if (next.done) break;
    store.delete(next.value);
  }
}

const extensionRefreshBurstWindow = new Map<string, number[]>();
const extensionAiCallWindow = new Map<string, number[]>();

const ACTION_EXECUTION_TTL_MS = 30 * 60_000;
const ACTION_EXECUTION_MAX_RECORDS = 2000;
const RENDERER_ONLY_VIEW_ACTION_TYPES = new Set([
  'recordShortcut',
  'previewClipboardItem',
]);
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
  'forceQuitApp',
  'checkForUpdates',
  'downloadUpdate',
  'installUpdate',
  'toggleSetting',
]);

function clonePlain(value) {
  if (!value) return value;
  return structuredClone(value);
}

function withoutExecutionId(value) {
  if (!value || typeof value !== 'object') return value;
  const { executionId, ...rest } = value;
  return rest;
}

function pruneExecutionRecords(store: Map<string, { createdAt: number }>) {
  const now = Date.now();
  for (const [id, record] of store) {
    if (now - record.createdAt > ACTION_EXECUTION_TTL_MS) store.delete(id);
  }
  while (store.size > ACTION_EXECUTION_MAX_RECORDS) {
    const oldest = store.keys().next().value;
    if (!oldest) break;
    store.delete(oldest);
  }
}

function extensionExecutionOwner(action, entry?: any) {
  const inheritedRecord = action?.executionId
    ? viewActionExecutionRecords.get(String(action.executionId)) ||
      rootActionExecutionRecords.get(String(action.executionId))
    : undefined;
  const extensionId =
    entry?.extension?.id || action?.extensionId || inheritedRecord?.extensionId;
  const extensionPath =
    entry?.extension?.__filePath ||
    action?.extensionFile ||
    inheritedRecord?.extensionFile ||
    '';
  const extensionFile = extensionPath ? path.basename(extensionPath) : '';
  return {
    ...(extensionId ? { extensionId: String(extensionId) } : {}),
    ...(extensionFile ? { extensionFile } : {}),
  };
}

function registerViewActionForRenderer(action, entry?: any) {
  if (!action || typeof action !== 'object') return action;
  if (RENDERER_ONLY_VIEW_ACTION_TYPES.has(String(action.type || '')))
    return action;
  pruneExecutionRecords(viewActionExecutionRecords);
  const executionId = crypto.randomUUID();
  const stored = clonePlain(withoutExecutionId(action));
  viewActionExecutionRecords.set(executionId, {
    action: stored,
    createdAt: Date.now(),
    ...extensionExecutionOwner(action, entry),
  });
  return { ...action, executionId };
}

function registerRootActionForRenderer(action) {
  if (!action || typeof action !== 'object') return action;
  pruneExecutionRecords(rootActionExecutionRecords);
  const executionId = crypto.randomUUID();
  const stored = clonePlain(withoutExecutionId(action));
  rootActionExecutionRecords.set(executionId, {
    action: stored,
    createdAt: Date.now(),
    ...extensionExecutionOwner(action),
  });
  return { ...action, executionId };
}

function pruneViewRefreshRecords() {
  const now = Date.now();
  for (const [id, record] of viewRefreshRecords) {
    if (now - record.createdAt > ACTION_EXECUTION_TTL_MS)
      viewRefreshRecords.delete(id);
  }
  while (viewRefreshRecords.size > ACTION_EXECUTION_MAX_RECORDS) {
    const oldest = viewRefreshRecords.keys().next().value;
    if (!oldest) break;
    viewRefreshRecords.delete(oldest);
  }
}

function registerViewRefreshForRenderer(refresh, entry, view) {
  if (!refresh || typeof refresh !== 'object') return refresh;
  if (refresh.id && !refresh.action) return refresh;
  pruneViewRefreshRecords();
  const { action, ...safeRefresh } = refresh;
  const refreshId = crypto.randomUUID();
  const normalizedAction = action ? normalizeViewAction(action, entry) : null;
  viewRefreshRecords.set(refreshId, {
    entry,
    action: normalizedAction ? withoutExecutionId(normalizedAction) : null,
    viewId: view?.id,
    mode: refresh.mode,
    createdAt: Date.now(),
    running: null,
    failureCount: 0,
  });
  return { ...safeRefresh, id: refreshId };
}

function mergeRendererActionInput(storedAction, rendererAction) {
  const merged = { ...storedAction };
  if (rendererAction && typeof rendererAction === 'object') {
    if ('formValues' in rendererAction)
      merged.formValues = rendererAction.formValues;
    if ('selectedItemId' in rendererAction)
      merged.selectedItemId = rendererAction.selectedItemId;
    if ('value' in rendererAction) merged.value = rendererAction.value;
    if ('text' in rendererAction && !('text' in storedAction))
      merged.text = rendererAction.text;
  }
  return merged;
}

function resolveRootActionForIpc(action) {
  if (!action || typeof action !== 'object') return action;
  const record = action.executionId
    ? rootActionExecutionRecords.get(String(action.executionId))
    : null;
  if (record) return clonePlain(record.action);
  const fallback = withoutExecutionId(action);
  if (fallback.kind === 'extension-root-item' && fallback.rootAction)
    throw new Error('Untrusted extension root action');
  return fallback;
}

function resolveViewActionForIpc(action) {
  if (!action || typeof action !== 'object') return action;
  const record = action.executionId
    ? viewActionExecutionRecords.get(String(action.executionId))
    : null;
  if (record)
    return mergeRendererActionInput(clonePlain(record.action), action);
  const fallback = withoutExecutionId(action);
  if (fallback.type === 'nativeAction')
    return {
      ...fallback,
      nativeAction: resolveRootActionForIpc(fallback.nativeAction),
    };
  if (TOKEN_REQUIRED_VIEW_ACTION_TYPES.has(String(fallback.type || '')))
    throw new Error(`Untrusted ${fallback.type} action`);
  return fallback;
}

function checkRefreshBurst(extension: any) {
  const id = extension?.id || 'unknown';
  const now = Date.now();
  const recent = (extensionRefreshBurstWindow.get(id) || []).filter(
    (time) => now - time < EXTENSION_REFRESH_BURST_WINDOW_MS,
  );
  if (recent.length >= EXTENSION_REFRESH_MAX_BURST) {
    logWarn(
      'extension.refresh.throttled',
      { count: recent.length },
      { source: 'host', scope: 'extension', extensionId: id },
    );
    return false;
  }
  recent.push(now);
  extensionRefreshBurstWindow.set(id, recent);
  return true;
}

function checkAiRateLimit(extension: any) {
  const id = extension?.id || 'unknown';
  const now = Date.now();
  const recent = (extensionAiCallWindow.get(id) || []).filter(
    (time) => now - time < EXTENSION_AI_RATE_WINDOW_MS,
  );
  if (recent.length >= EXTENSION_AI_CALLS_PER_MINUTE) return false;
  recent.push(now);
  extensionAiCallWindow.set(id, recent);
  return true;
}

function createExtensionCache(extension) {
  const store = extensionCacheFor(extension.id);
  return {
    get(key) {
      const entry = store.get(key);
      if (!entry) return;
      if (entry.expiresAt && Date.now() > entry.expiresAt) return;
      return entry.value;
    },
    getStale(key) {
      return store.get(key)?.value;
    },
    has(key) {
      const entry = store.get(key);
      if (!entry) return false;
      return !entry.expiresAt || Date.now() <= entry.expiresAt;
    },
    set(key, value, options: any = {}) {
      const rawTtl = Number(options.ttlMs || 0);
      const clampedTtl =
        rawTtl > 0 ? Math.min(rawTtl, EXTENSION_CACHE_MAX_TTL_MS) : 0;
      store.set(key, {
        value,
        expiresAt: clampedTtl > 0 ? Date.now() + clampedTtl : 0,
      });
      enforceExtensionCacheBudget(store);
      return value;
    },
    invalidate(key) {
      if (key === undefined) store.clear();
      else store.delete(key);
      invalidateExtensionRootItemsForExtension(extension);
    },
    keys() {
      return Array.from(store.keys());
    },
  };
}
const registeredActionAccelerators = new Set<string>();
const AI_BUILDER_EXTENSION_ID = 'nevermind.ai-builder';

const REQUIRED_INTERNAL_EXTENSIONS = [
  'nevermind.system',
  'nevermind.places',
  'nevermind.calculator',
  'nevermind.web',
  'nevermind.clipboard',
  'nevermind.apps',
  'nevermind.files',
  AI_BUILDER_EXTENSION_ID,
  'nevermind.updates',
  'nevermind.shortcuts',
  'nevermind.settings',
  'nevermind.background-tasks',
  'nevermind.account',
];
const REQUIRED_INTERNAL_COMMANDS = [
  { extensionId: AI_BUILDER_EXTENSION_ID, commandId: 'ai-chats' },
];

function actionAliases(actionId: any) {
  const value = userState.aliases[actionId];
  if (!value) return [];
  return Array.isArray(value) ? value : [value];
}

function actionSearchScore(action: any, query: any) {
  const q = normalize(query);
  if (!q) return action.score || 0;
  let best = scoreNormalized(action.title, q);
  if (best === 100) return best;
  const subtitleScore = scoreNormalized(action.subtitle, q);
  if (subtitleScore > best) best = subtitleScore;
  if (best === 100) return best;
  const aliases = action.aliases;
  if (aliases) {
    for (const alias of aliases) {
      const s = scoreNormalized(alias, q);
      if (s > best) best = s;
      if (best === 100) return best;
    }
  }
  for (const alias of actionAliases(action.id)) {
    const s = scoreNormalized(alias, q);
    if (s > best) best = s;
    if (best === 100) return best;
  }
  return best;
}

function usageBoost(actionId: any) {
  const count = userState.recents[actionId]?.count || 0;
  return Math.min(90, count * 6);
}

function recentBoost(actionId: any) {
  const recent = userState.recents[actionId];
  if (!recent) return 0;
  const ageHours = Math.max(0, (Date.now() - recent.lastUsed) / 36e5);
  return Math.max(0, 20 - ageHours);
}

function defaultActionIdFor(action: any) {
  if (action.defaultActionId) return action.defaultActionId;
  if (action.kind === 'builtin') return action.id;
  if (action.kind === 'calculate') return 'default:calculator';
  return null;
}

let shortcutByAiChatIdCache: Map<string, string> | null = null;
function shortcutByAiChatIdMap() {
  if (shortcutByAiChatIdCache) return shortcutByAiChatIdCache;
  shortcutByAiChatIdCache = buildShortcutByAiChatIdMap(
    userState.shortcutActions,
    userState.shortcuts,
    userState.aiChats,
    chatTouchedExtensionFiles,
  );
  return shortcutByAiChatIdCache;
}

function invalidateShortcutCaches() {
  shortcutByAiChatIdCache = null;
}

function shortcutForAction(action: any) {
  if (userState.shortcuts[action.id]) return userState.shortcuts[action.id];
  if (userState.removedShortcuts?.[action.id]) return null;
  if (!action.aiChatId) return null;
  return shortcutByAiChatIdMap().get(action.aiChatId) || null;
}

function withShortcutHint(action: any) {
  const shortcut = shortcutForAction(action);
  return shortcut ? { ...action, shortcut } : action;
}

function withDefaultOverride(action: any) {
  const defaultActionId = defaultActionIdFor(action);
  if (!defaultActionId) return withShortcutHint(action);
  const override = userState.overrides[defaultActionId];
  return {
    ...action,
    defaultActionId,
    isOverridden: Boolean(override),
    overrideSummary: override?.instruction,
    shortcut: shortcutForAction(action),
  };
}

function rankAction(action: any, query: any) {
  const base = actionSearchScore(action, query);
  if (query.trim() && base <= 0) return null;
  return {
    ...action,
    aliases: [...(action.aliases || []), ...actionAliases(action.id)],
    userAliases: actionAliases(action.id),
    score:
      base +
      priorityBoost(action) +
      usageBoost(action.id) +
      recentBoost(action.id),
    lastUsed: userState.recents[action.id]?.lastUsed || 0,
  };
}

function recordRecent(action: any) {
  if (!action?.id) return;
  const current = userState.recents[action.id] || {
    count: 0,
    lastUsed: 0,
    title: action.title,
  };
  userState.recents[action.id] = {
    count: current.count + 1,
    lastUsed: Date.now(),
    title: action.title,
    kind: action.kind,
  };
  scheduleSaveState();
}

function isFixtureExtension(extension) {
  return Boolean(extension?.__fixture);
}

function visibleExtensions() {
  return Array.from(extensionModules.values()).filter(
    (extension) => !isFixtureExtension(extension),
  );
}

function visibleExtensionActionEntries() {
  return Array.from(extensionActionRegistry.values()).filter(
    (entry) => !isFixtureExtension(entry.extension),
  );
}

function searchableExtensions() {
  const extensions = visibleExtensions();
  return isNvmTestMode
    ? extensions.filter((extension) => testModeExtensionIsSafe(extension.id))
    : extensions;
}

function searchableExtensionActionEntries() {
  const entries = visibleExtensionActionEntries();
  return isNvmTestMode
    ? entries.filter((entry) => testModeExtensionIsSafe(entry.extension.id))
    : entries;
}

function extensionCommandActionId(extension, command) {
  return `extension:${extension.id}:${command.id}`;
}

function getOrCreateAiChat(query, options: any = {}) {
  const trimmed = query.trim();
  const baseId = hashValue(trimmed);
  const current = userState.aiChats[baseId];
  if (current && !options.fresh) return current;
  const id =
    current && options.fresh
      ? hashValue(`${trimmed}:${Date.now()}:${crypto.randomUUID()}`)
      : baseId;
  const item = aiChatItem(id, trimmed);
  userState.aiChats[id] = item;
  learningStore?.upsertTraceMetadata(id, aiLearningMetadata(id));
  scheduleSaveState();
  invalidateExtensionRootItems();
  return item;
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
      {
        role: 'assistant',
        content: `What should "${query}" do? Tell me the exact behavior, inputs, and what UI you want, then I'll build it.`,
      },
    ],
  };
}

function createDraftAiChat(query) {
  const trimmed = query.trim();
  const id = `draft:${hashValue(`${trimmed}:${Date.now()}:${crypto.randomUUID()}`)}`;
  const item = aiChatItem(id, trimmed);
  draftAiChats.set(id, item);
  return item;
}

function promoteDraftAiChat(chatId) {
  if (userState.aiChats[chatId]) return userState.aiChats[chatId];
  const draft = draftAiChats.get(chatId);
  if (!draft) return null;
  draftAiChats.delete(chatId);
  draft.createdAt = Date.now();
  draft.updatedAt = Date.now();
  userState.aiChats[chatId] = draft;
  learningStore?.upsertTraceMetadata(chatId, aiLearningMetadata(chatId));
  scheduleSaveState();
  invalidateExtensionRootItems();
  return draft;
}

function appendAiChatMessage(chatId, role, content) {
  const chat = userState.aiChats[chatId];
  if (!(chat && content)) return;
  chat.messages = [...(chat.messages || []), { role, content }].slice(-100);
  chat.updatedAt = Date.now();
  learningStore?.appendMessage(
    chatId,
    role,
    content,
    aiLearningMetadata(chatId),
  );
  scheduleSaveState();
  patchAiChatsItem(chatId);
}

function appendAiChatDelta(chatId, text) {
  const chat = userState.aiChats[chatId];
  if (!(chat && text)) return;
  const messages = chat.messages || [];
  const last = messages[messages.length - 1];
  if (last?.role === 'assistant') last.content = `${last.content}${text}`;
  else messages.push({ role: 'assistant', content: text });
  chat.messages = messages.slice(-100);
  chat.updatedAt = Date.now();
  learningStore?.appendAssistantDelta(chatId, text, aiLearningMetadata(chatId));
  scheduleSaveState();
}

function aiChatView(item, options: any = {}) {
  return {
    type: 'chat',
    title: `Automate "${item.query}"`,
    aiChat: true,
    chatId: item.id,
    initialPrompt: options.initialPrompt,
    messages: item.messages || [],
  };
}

function aiChatOpenAction(chatId) {
  return buildAiBuilderAction('Open Chat', async () => {
    const item = userState.aiChats[chatId] || draftAiChats.get(chatId);
    if (!item)
      return { toast: { message: 'AI chat not found', tone: 'error' } };
    return { view: aiChatView(item) };
  });
}

function aiChatRemoveAction(chat) {
  return wrapWithConfirmation(
    buildAiBuilderAction('Remove Chat', () => removeAiChat(chat.id), {
      style: 'destructive',
    }),
    {
      message: `Remove "${chat.title || chat.query || 'AI chat'}" and its history? Generated extension files stay.`,
      confirmLabel: 'Remove Chat',
      destructive: true,
    },
  );
}

function aiChatListItem(chat: any) {
  return {
    id: `ai-chat:${chat.id}`,
    title: chat.title || chat.query || 'AI Chat',
    subtitle:
      chat.contextExtensionFile ||
      (chat.touchedExtensionFiles || [])[0] ||
      chat.status ||
      'Builder chat',
    icon: 'sparkles',
    primaryAction: aiChatOpenAction(chat.id),
    actions: [aiChatRemoveAction(chat)],
  };
}

function aiChatListItems() {
  const chats = Object.values(userState.aiChats || {}) as any[];
  return chats
    .sort(
      (a, b) =>
        (b.updatedAt || b.createdAt || 0) - (a.updatedAt || a.createdAt || 0),
    )
    .map(aiChatListItem);
}

function aiChatsView() {
  return {
    type: 'list',
    id: 'ai-chats',
    title: 'AI Chats',
    searchBarPlaceholder: 'Search AI Chats',
    items: aiChatListItems(),
  };
}

function patchOpenView(viewId: string, patch: any) {
  const normalizedPatch = normalizeViewPatch(patch, null);
  structuredClone(normalizedPatch);
  paletteWindow.win?.webContents.send('view:patch', {
    viewId,
    patch: normalizedPatch,
  });
}

function aiBuilderRegistryEntry() {
  return (
    extensionActionRegistry.get(`${AI_BUILDER_EXTENSION_ID}:ai-chats`) || {
      extension: createAiBuilderExtension(),
      command: { id: 'ai-chats', title: 'AI Chats' },
    }
  );
}

function normalizedAiChatListItem(chat: any) {
  return normalizeViewItems(
    [aiChatListItem(chat)],
    aiBuilderRegistryEntry(),
  )[0];
}

function patchAiChatsItem(chatId: string) {
  const chat = userState.aiChats[chatId];
  if (!chat) return;
  patchOpenView('ai-chats', {
    mode: 'patch',
    items: [normalizedAiChatListItem(chat)],
  });
}

function patchAiChatsPrepend(chatId: string) {
  const chat = userState.aiChats[chatId];
  if (!chat) return;
  patchOpenView('ai-chats', {
    mode: 'prepend',
    items: [normalizedAiChatListItem(chat)],
  });
}

function patchAiChatsRemove(chatId: string) {
  patchOpenView('ai-chats', { removeItemIds: [`ai-chat:${chatId}`] });
}

function chatTouchedExtensionFiles(chat) {
  return Array.from(
    new Set(
      [
        ...(chat?.touchedExtensionFiles || []),
        chat?.generatedExtensionFile,
        chat?.contextExtensionFile,
      ]
        .filter(Boolean)
        .map((item) => path.basename(item)),
    ),
  );
}

function aiChatIdForExtensionFile(filename) {
  const base = path.basename(filename || '');
  if (!base) return null;
  const matches = Object.values(userState.aiChats).filter((chat: any) =>
    chatTouchedExtensionFiles(chat).includes(base),
  );
  return matches.length === 1 ? (matches[0] as any).id : null;
}

function touchExtensionFileForChat(chat, filename) {
  if (!(chat && filename)) return;
  chat.touchedExtensionFiles = Array.from(
    new Set([...chatTouchedExtensionFiles(chat), path.basename(filename)]),
  );
  if (!chat.contextExtensionFile)
    chat.contextExtensionFile = path.basename(filename);
  if (!chat.generatedExtensionFile)
    chat.generatedExtensionFile = path.basename(filename);
  chat.status = 'ready';
  chat.updatedAt = Date.now();
  learningStore?.upsertTraceMetadata(chat.id, aiLearningMetadata(chat.id));
  patchAiChatsItem(chat.id);
}

function getOrCreateExtensionChat(extensionFile, title = extensionFile) {
  const filename = path.basename(extensionFile || '');
  const existing = (Object.values(userState.aiChats || {}) as any[])
    .filter(
      (chat) =>
        chat.contextExtensionFile === filename ||
        chatTouchedExtensionFiles(chat).includes(filename),
    )
    .sort(
      (a, b) =>
        (b.updatedAt || b.createdAt || 0) - (a.updatedAt || a.createdAt || 0),
    )[0];
  if (existing) {
    existing.contextExtensionFile = filename;
    existing.updatedAt = Date.now();
    learningStore?.upsertTraceMetadata(
      existing.id,
      aiLearningMetadata(existing.id),
    );
    scheduleSaveState();
    patchAiChatsItem(existing.id);
    return existing;
  }
  const id = hashValue(`extension-chat:${filename}:${Date.now()}`);
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
      {
        role: 'assistant',
        content: `I can tweak "${title || filename}". I can read this extension as context and inspect any other extension if needed.`,
      },
    ],
  };
  userState.aiChats[id] = item;
  learningStore?.upsertTraceMetadata(id, aiLearningMetadata(id));
  scheduleSaveState();
  invalidateExtensionRootItems();
  patchAiChatsPrepend(id);
  return item;
}

function clipboardPreviewAction(item) {
  return clipboardService!.clipboardPreviewAction(item);
}

function clipboardCopyAction(item) {
  return clipboardService!.clipboardCopyAction(item);
}

function clipboardHistoryRemovalAction(range, title, message, itemId = '') {
  return clipboardService!.clipboardHistoryRemovalAction(
    range,
    title,
    message,
    itemId,
  );
}

function clipboardHistoryRemovalActions(item: any = null) {
  return clipboardService!.clipboardHistoryRemovalActions(item);
}

function clipboardRootItem(item) {
  return clipboardService!.clipboardRootItem(item);
}

function clipboardHistoryItem(item: any) {
  return clipboardService!.clipboardHistoryItem(item);
}

function clipboardHistoryItems() {
  return clipboardService!.clipboardHistoryItems();
}

function clipboardHistorySnapshot(options: any = {}) {
  return clipboardService!.clipboardHistorySnapshot(options);
}

function clipboardHistoryRemovalEntries(action) {
  return clipboardService!.clipboardHistoryRemovalEntries(action);
}

function clipboardHistoryGet(id) {
  return clipboardService!.clipboardHistoryGet(id);
}

function removeClipboardHistoryByAction(action) {
  return clipboardService!.removeClipboardHistoryByAction(action);
}

function clipboardHistoryRemovedMessage(count) {
  return clipboardService!.clipboardHistoryRemovedMessage(count);
}

function removeClipboardHistoryEntries(action) {
  return clipboardService!.removeClipboardHistoryEntries(action);
}

function viewRefreshAction(itemsBuilder) {
  return {
    type: 'runExtensionAction',
    title: 'Refresh',
    __handler: () => ({ patch: { mode: 'replace', items: itemsBuilder() } }),
  };
}

function clipboardHistoryView() {
  return clipboardService!.clipboardHistoryView();
}

function isNewerVersion(version) {
  return isVersionNewerThan(version, app.getVersion());
}

function updateStatusItems() {
  return updateStatusView().items;
}

function updateStatusView(_options: any = {}) {
  const downloadedInfo = isNewerVersion(
    updateManager.state.downloadedInfo?.version,
  )
    ? updateManager.state.downloadedInfo
    : null;
  const availableInfo = isNewerVersion(
    updateManager.state.availableInfo?.version,
  )
    ? updateManager.state.availableInfo
    : null;
  const version = downloadedInfo?.version || availableInfo?.version;
  const unsupported =
    updateManager.state.status === 'unsupported' ||
    !updateManager.canUseAutoUpdates();
  const installing =
    updateManager.state.installInFlight ||
    updateManager.state.status === 'installing';
  const primaryAction = installing
    ? undefined
    : downloadedInfo
      ? { type: 'installUpdate', title: 'Install and Restart' }
      : availableInfo
        ? {
            type: 'downloadUpdate',
            title: updateManager.state.downloadInFlight
              ? 'Downloading...'
              : 'Download Update',
          }
        : {
            type: 'checkForUpdates',
            title: updateManager.state.checkInFlight
              ? 'Checking...'
              : 'Check Again',
          };
  const title = unsupported
    ? 'Updates unavailable'
    : installing
      ? `Installing Nevermind ${version || ''}`.trim()
      : downloadedInfo
        ? `Nevermind ${version} is ready`
        : availableInfo
          ? `Nevermind ${version} is available`
          : updateManager.state.checkInFlight
            ? 'Checking for updates...'
            : updateManager.state.status === 'error'
              ? 'Update check failed'
              : 'No versions available';
  const subtitle = unsupported
    ? autoUpdatesUnavailableMessage()
    : installing
      ? 'Restarting Nevermind to finish updating...'
      : downloadedInfo
        ? 'Install the downloaded update and restart Nevermind'
        : availableInfo
          ? 'Download the update before installing it'
          : updateManager.state.checkInFlight
            ? `Current version: ${app.getVersion()}`
            : updateManager.state.status === 'error'
              ? updateManager.state.errorMessage
              : `Current version: ${app.getVersion()}`;
  return {
    type: 'list',
    id: 'app-updates',
    title: 'Updates',
    presentation: 'root',
    searchBarPlaceholder: 'Search Updates',
    isLoading:
      updateManager.state.checkInFlight ||
      updateManager.state.downloadInFlight ||
      updateManager.state.installInFlight,
    items: [
      {
        id: 'update-status',
        title,
        subtitle,
        icon: 'restart',
        accessories: version ? [{ text: version }] : [],
        primaryAction: unsupported ? undefined : primaryAction,
        actionPanel:
          unsupported || !primaryAction
            ? undefined
            : { sections: [{ actions: [primaryAction] }] },
      },
    ],
  };
}

function updatesStateSnapshot() {
  const state = updateManager.state;
  const downloadedInfo = isNewerVersion(state.downloadedInfo?.version)
    ? state.downloadedInfo
    : null;
  const availableInfo = isNewerVersion(state.availableInfo?.version)
    ? state.availableInfo
    : null;
  return {
    currentVersion: app.getVersion(),
    status: String(state.status || 'idle'),
    supported:
      updateManager.canUseAutoUpdates() && state.status !== 'unsupported',
    checking: Boolean(state.checkInFlight),
    downloading: Boolean(state.downloadInFlight),
    installing: Boolean(state.installInFlight || state.status === 'installing'),
    availableVersion: availableInfo?.version || null,
    downloadedVersion: downloadedInfo?.version || null,
    errorMessage: state.errorMessage || null,
  };
}

function checkForUpdatesView() {
  updateManager.checkForUpdates('manual', { download: true }).catch(() => {});
  return { view: updateStatusView(), navigation: 'replace' };
}

function downloadUpdateView() {
  updateManager.downloadAvailableUpdate().catch(() => {});
  return { view: updateStatusView(), navigation: 'replace' };
}

let updateInstallQuitFallbackTimer: NodeJS.Timeout | null = null;

function scheduleUpdateInstallQuitFallback() {
  if (updateInstallQuitFallbackTimer) return;
  updateInstallQuitFallbackTimer = setTimeout(() => {
    updateInstallQuitFallbackTimer = null;
    logWarn('updater.install.quitFallback', undefined, {
      source: 'host',
      scope: 'updater',
    });
    nevermindApp.isQuiting = true;
    app.quit();
    setTimeout(() => {
      logWarn('updater.install.exitFallback', undefined, {
        source: 'host',
        scope: 'updater',
      });
      runQuitCleanup();
      app.exit(0);
    }, 2000).unref?.();
  }, 5000);
  updateInstallQuitFallbackTimer.unref?.();
}

function installDownloadedUpdate() {
  if (!updateManager.state.downloadedInfo)
    return { view: updateStatusView(), navigation: 'replace' };
  nevermindApp.isQuiting = true;
  const didStart = updateManager.quitAndInstall();
  if (didStart || updateManager.state.installInFlight)
    scheduleUpdateInstallQuitFallback();
  return { view: updateStatusView(), navigation: 'replace' };
}

function settingItemPatch(definition) {
  const value = getSetting(definition.id);
  const accessoryText =
    definition.type === 'boolean'
      ? value
        ? 'On'
        : 'Off'
      : definition.type === 'shortcut'
        ? ''
        : String(value);
  const shortcutInput =
    definition.id === 'paletteHotkey'
      ? {
          scope: 'palette',
          title: 'Change Shortcut',
          shortcut: String(value || ''),
        }
      : {
          action: { id: '__hyper-key__' },
          title: 'Change Hyper Key',
          shortcut: String(value || ''),
        };
  const primaryAction =
    definition.type === 'shortcut'
      ? buildRecordShortcutAction(shortcutInput, {})
      : {
          type: 'toggleSetting',
          title: value ? 'Turn Off' : 'Turn On',
          settingId: definition.id,
        };
  return {
    id: `setting:${definition.id}`,
    accessories: accessoryText ? [{ text: accessoryText }] : [],
    primaryAction,
    actionPanel: { sections: [{ actions: [primaryAction] }] },
  };
}

function settingsItems() {
  return availableSettingDefinitions().map((definition) => ({
    id: `setting:${definition.id}`,
    title: definition.title,
    subtitle: definition.description,
    icon: definition.icon || 'settings',
    ...settingItemPatch(definition),
  }));
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
  };
}

function patchSettingsView(settingId: string, options: any = {}) {
  const definition = SETTING_DEFINITIONS.find((item) => item.id === settingId);
  if (!(definition && settingIsAvailable(definition))) return;
  patchOpenView('app-settings', {
    mode: 'patch',
    items: [
      { id: `setting:${definition.id}`, ...settingItemPatch(definition) },
    ],
    ...options,
  });
}

function patchUpdatesView() {
  patchOpenView('app-updates', {
    mode: 'patch',
    items: updateStatusItems(),
    isLoading:
      updateManager.state.checkInFlight ||
      updateManager.state.downloadInFlight ||
      updateManager.state.installInFlight,
  });
}

let activeNevermindBaseUrl: string | null = null;
const bufferedDeepLinks: string[] = [];
let appReady = false;

async function handleAuthDeepLink(parsed: ParsedAuthDeepLink) {
  const baseUrl = parsed.baseUrl || getDefaultNevermindBaseUrl();
  logInfo('deep_link.handle', {
    source: 'deep_link',
    code: parsed.code.slice(0, 8),
    baseUrl,
    intent: parsed.intent,
  });
  try {
    if (isSigningIn()) {
      logWarn('deep_link.signin_in_progress', undefined, {
        source: 'host',
        scope: 'deep_link',
      });
      return;
    }
    const existing = await getNevermindAuth();
    if (existing && parsed.intent !== 'reconnect') {
      logInfo(
        'deep_link.already_authed',
        { email: existing.email },
        { source: 'host', scope: 'deep_link' },
      );
      return;
    }
    const result = await consumeDeviceCode({ code: parsed.code, baseUrl });
    if (result.ok) {
      activeNevermindBaseUrl = result.auth.baseUrl;
      setActiveNevermindAuthBaseUrl(result.auth.baseUrl);
      warmNevermindCompatibilityCache(result.auth.baseUrl);
      invalidateExtensionRootItems();
      broadcastAuthChanged({ authed: true, email: result.auth.email });
      logInfo(
        'deep_link.auth_success',
        { email: result.auth.email },
        { source: 'host', scope: 'deep_link' },
      );
    } else {
      logWarn(
        'deep_link.auth_failed',
        { error: (result as { error?: string }).error },
        { source: 'host', scope: 'deep_link' },
      );
    }
  } catch (err) {
    logError('deep_link.handle.failed', err, {
      source: 'host',
      scope: 'deep_link',
    });
  }
}

function processBufferedDeepLink(rawUrl: string) {
  const baseUrl = activeNevermindBaseUrl || getDefaultNevermindBaseUrl();
  const parsed = parseAuthDeepLink(rawUrl, baseUrl);
  if (!parsed) return;
  void handleAuthDeepLink(parsed);
}

function flushBufferedDeepLinks() {
  const links = bufferedDeepLinks.splice(0);
  for (const link of links) processBufferedDeepLink(link);
}

function selectedNevermindEnvironment() {
  const selected = userState.nevermindEnvironment;
  if (!selected?.baseUrl) {
    return {
      environment: nevermindEnvironmentForBaseUrl(getDefaultNevermindBaseUrl()),
      baseUrl: getDefaultNevermindBaseUrl(),
    };
  }
  return selected;
}

function getNevermindDebugStatus() {
  const client = selectedNevermindEnvironment();
  const baseUrl = activeNevermindBaseUrl || client.baseUrl;
  const manifest = currentNevermindCompatibilityManifest(baseUrl);
  return {
    client,
    active: {
      environment: nevermindEnvironmentForBaseUrl(baseUrl),
      baseUrl,
    },
    backend:
      manifest?.backend?.environment && manifest.backend.version
        ? {
            environment: manifest.backend.environment,
            version: manifest.backend.version,
          }
        : null,
  };
}

async function signInToSelectedNevermindEnvironment() {
  const selected = selectedNevermindEnvironment();
  const result = await signInToNevermind({
    baseUrl: selected.baseUrl,
    environment: selected.environment,
  });
  if (result.ok) {
    activeNevermindBaseUrl = result.auth.baseUrl;
    setActiveNevermindAuthBaseUrl(result.auth.baseUrl);
    warmNevermindCompatibilityCache(result.auth.baseUrl);
  }
  return result;
}

async function switchNevermindBackendEnvironment(input: {
  environment: 'production' | 'pr_preview' | 'custom';
  baseUrl?: string;
}) {
  return switchBackendEnvironment(input, {
    isPackaged: app.isPackaged,
    selectedEnvironment: selectedNevermindEnvironment,
    resolvesToUnsafeAddress: resolvesToUnsafeNevermindAddress,
    invalidateCompatibilityCache: invalidateNevermindCompatibilityCache,
    checkCompatibility: checkNevermindCompatibility,
    setSelectedEnvironment: (selection) => {
      userState.nevermindEnvironment = selection;
    },
    scheduleSaveState,
    setActiveAuthBaseUrl: setActiveNevermindAuthBaseUrl,
    getAuth: getNevermindAuth,
    signIn: signInToSelectedNevermindEnvironment,
    setActiveBaseUrl: (baseUrl) => {
      activeNevermindBaseUrl = baseUrl;
    },
    warmCompatibilityCache: warmNevermindCompatibilityCache,
    disposeAiSessions: () => nevermindAi?.disposeAllSessions?.(),
    invalidateExtensionRootItems,
    broadcastAuthChanged,
  });
}

function safeExternalUpdateUrl(raw?: string) {
  if (!raw) return null;
  try {
    const parsed = new URL(raw);
    return parsed.protocol === 'https:' || parsed.protocol === 'http:'
      ? parsed.toString()
      : null;
  } catch {
    return null;
  }
}

function updateActionForCompatibilityPrompt(updateUrl?: string) {
  const safeUrl = safeExternalUpdateUrl(updateUrl);
  const downloadedInfo = isNewerVersion(
    updateManager.state.downloadedInfo?.version,
  )
    ? updateManager.state.downloadedInfo
    : null;
  const availableInfo = isNewerVersion(
    updateManager.state.availableInfo?.version,
  )
    ? updateManager.state.availableInfo
    : null;
  if (downloadedInfo)
    return {
      type: 'installUpdate',
      title: `Install Nevermind ${downloadedInfo.version || ''}`.trim(),
    };
  if (availableInfo)
    return {
      type: 'downloadUpdate',
      title: `Download Nevermind ${availableInfo.version || ''}`.trim(),
    };
  if (updateManager.canUseAutoUpdates())
    return { type: 'checkForUpdates', title: 'Check for Update' };
  return safeUrl
    ? { type: 'openUrl', title: 'Download Update', url: safeUrl }
    : undefined;
}

function compatibilityPromptAction() {
  if (!activeNevermindBaseUrl) return null;
  const manifest = currentNevermindCompatibilityManifest(
    activeNevermindBaseUrl,
  );
  if (manifest?.client?.compatible !== false) return null;
  const version =
    manifest.desktop?.latestVersion ||
    manifest.desktop?.minimumSupportedVersion ||
    '';
  const primaryAction = updateActionForCompatibilityPrompt(
    manifest.desktop?.updateUrl,
  );
  return {
    id: 'updates:compatibility-required',
    title: 'Update Nevermind',
    subtitle: version
      ? `Nevermind ${version} or newer is required for backend compatibility`
      : 'This version is no longer supported by the backend',
    icon: 'restart',
    score: 1100,
    primaryAction,
    actionPanel: primaryAction
      ? { sections: [{ actions: [primaryAction] }] }
      : undefined,
  };
}

function updatePromptAction() {
  const downloadedInfo = isNewerVersion(
    updateManager.state.downloadedInfo?.version,
  )
    ? updateManager.state.downloadedInfo
    : null;
  const availableInfo = isNewerVersion(
    updateManager.state.availableInfo?.version,
  )
    ? updateManager.state.availableInfo
    : null;
  const version = downloadedInfo?.version || availableInfo?.version;
  if (
    updateManager.state.installInFlight ||
    updateManager.state.status === 'installing'
  ) {
    return {
      id: 'updates:installing',
      title: `Installing Nevermind ${version || ''}`.trim(),
      subtitle: 'Restarting Nevermind to finish updating...',
      icon: 'restart',
      score: 1000,
    };
  }
  if (downloadedInfo) {
    return {
      id: 'updates:install',
      title: `Install Nevermind ${version}`,
      subtitle: 'Restart Nevermind to finish updating',
      icon: 'restart',
      score: 1000,
      primaryAction: {
        type: 'installUpdate',
        title: `Install Nevermind ${version}`,
      },
    };
  }
  if (availableInfo) {
    return {
      id: 'updates:download',
      title: `Download Nevermind ${version}`,
      subtitle: updateManager.state.downloadInFlight
        ? 'Downloading update...'
        : 'Update available',
      icon: 'restart',
      score: 1000,
      primaryAction: {
        type: 'downloadUpdate',
        title: `Download Nevermind ${version}`,
      },
    };
  }
  return null;
}

async function searchActions(query, options: any = {}) {
  return measureDebugPerformance(
    'search.actions',
    {
      queryLength: String(query || '').length,
      clipboardOnly: Boolean(options.clipboardOnly),
      alwaysLog: true,
    },
    async () => {
      const q = query.trim();

      if (options.clipboardOnly) {
        return measureDebugPerformanceSync(
          'search.clipboard-only',
          { queryLength: q.length, clipboardCount: clipboardHistory.length },
          () =>
            clipboardHistory
              .map(clipboardRootItem)
              .filter((item) => (q ? rankAction(item, q) : true))
              .sort((a, b) =>
                q
                  ? b.score - a.score || b.lastUsed - a.lastUsed
                  : b.lastUsed - a.lastUsed,
              )
              .slice(0, CLIPBOARD_LIMIT)
              .map(prepareRootActionForRenderer),
        );
      }

      const testAction = isNvmTestMode
        ? rankAction(testModeSafeAction(), q)
        : null;
      const results = testAction ? [testAction] : [];
      const contributedItems = await measureDebugPerformance(
        q ? 'search.extensions.query' : 'search.extensions.root',
        {
          queryLength: q.length,
          extensionCount: searchableExtensions().length,
          alwaysLog: true,
        },
        () =>
          q
            ? extensionSearchActions(q, searchableExtensions())
            : extensionRootActions(searchableExtensions()),
      );
      for (const item of contributedItems) {
        const ranked = item.__ranked
          ? withShortcutHint(item)
          : rankAction(withShortcutHint(item), q);
        if (ranked) results.push(ranked);
      }

      const entries = searchableExtensionActionEntries();
      measureDebugPerformanceSync(
        'search.rank-registered-actions',
        { queryLength: q.length, actionCount: entries.length },
        () => {
          for (const entry of entries) {
            const action = extensionActionFromContribution(entry);
            const ranked = action
              ? rankAction(withShortcutHint(action), q)
              : null;
            if (ranked) results.push(ranked);
          }
        },
      );

      const sorted = measureDebugPerformanceSync(
        'search.sort-prepare',
        { queryLength: q.length, resultCount: results.length },
        () =>
          results
            .filter(testModePaletteActionIsSafe)
            .sort(compareRankedActions)
            .slice(0, 30)
            .map(prepareRootActionForRenderer),
      );
      markDebugPerformance('search.actions.result', {
        queryLength: q.length,
        contributedCount: contributedItems.length,
        rankedCount: results.length,
        resultCount: sorted.length,
      });
      return sorted;
    },
  );
}

function testModeSafeAction() {
  return {
    id: 'test:confirm-safe-action',
    kind: 'test-action',
    title: 'Test: Confirm safe action',
    subtitle: 'In-memory deterministic Electron smoke action',
    icon: 'check',
    score: 100,
  };
}

function testModeExtensionIsSafe(extensionId: string) {
  return [
    'nevermind.system',
    'nevermind.extensions',
    'pab53.lifecycle',
    'pab53.legacy',
    'pab53.discovered',
  ].includes(extensionId);
}

function testModePaletteActionIsSafe(action) {
  if (!isNvmTestMode) return true;
  return (
    action.kind === 'test-action' ||
    (action.kind === 'extension-action' &&
      testModeExtensionIsSafe(action.extensionId))
  );
}

function invalidateExtensionRootItems() {
  extensionRootItemsCache.clear();
  paletteWindow.win?.webContents.send('root-items:changed');
}

function broadcastAuthChanged(status: { authed: boolean; email?: string }) {
  paletteWindow.win?.webContents.send('nevermind:auth-changed', status);
}

function prepareActionPanelForRenderer(panel, entry?: any) {
  if (!panel?.sections) return panel;
  return {
    ...panel,
    sections: panel.sections.map((section) => {
      const { lazyActions, ...rest } = section;
      return {
        ...rest,
        actions: [...(section.actions || []), ...(section.lazyActions || [])]
          .map((action) => normalizeViewAction(action, entry))
          .filter(Boolean),
      };
    }),
  };
}

function prepareRootActionForRenderer(action) {
  if (!action || typeof action !== 'object') return action;
  const entry = action.kind?.startsWith('extension-')
    ? extensionEntryForAction(action)
    : null;
  return registerRootActionForRenderer({
    ...action,
    primaryAction: normalizeViewAction(action.primaryAction, entry),
    rootAction: normalizeViewAction(action.rootAction, entry),
    actionPanel: prepareActionPanelForRenderer(action.actionPanel, entry),
  });
}

function invalidateExtensionRootItemsForExtension(extension) {
  const cacheKey = extension.__filePath || extension.id;
  extensionRootItemsCache.delete(cacheKey);
  paletteWindow.win?.webContents.send('root-items:changed');
}

function runInBackground(task) {
  setImmediate(() => {
    Promise.resolve()
      .then(task)
      .catch((error) =>
        logError('backgroundAction.failed', error, { source: 'host' }),
      );
  });
}

async function executeAction(action, options: any = {}) {
  if (!action) return;
  recordRecent(action);
  let result;

  switch (action.kind) {
    case 'test-action':
      if (isNvmTestMode) {
        result = { toast: { message: 'Safe action invoked' } };
      }
      break;
    case 'open-keyboard-settings':
      runInBackground(openSystemKeyboardSettings);
      break;
    case 'extension-root-item':
    case 'extension-action': {
      const result = await executeExtensionRootItem(action);
      if (result) return result;
      break;
    }
    case 'extension-command': {
      const upgraded = currentActionForStoredShortcut(action);
      if (upgraded !== action) return executeAction(upgraded, options);
      break;
    }
  }

  if (!options.keepPaletteOpen) {
    if (isNvmTestMode && action.kind === 'test-action')
      setTimeout(() => paletteWindow.hidePalette(), 0).unref?.();
    else paletteWindow.hidePalette();
  }
  return result;
}

function extensionActionEntryForAction(action) {
  const registeredActionId = action?.registeredActionId || action?.commandId;
  const direct = extensionActionRegistry.get(
    `${action.extensionId}:${registeredActionId}`,
  );
  if (direct) return direct;

  if (action?.extensionFile) {
    const fileMatches = Array.from(extensionActionRegistry.values()).filter(
      (entry) =>
        path.basename(entry.extension.__filePath || '') ===
        action.extensionFile,
    );
    if (fileMatches.length === 1) return fileMatches[0];
  }

  if (action?.aiChatId) {
    const files = chatTouchedExtensionFiles(userState.aiChats[action.aiChatId]);
    const chatMatches = Array.from(extensionActionRegistry.values()).filter(
      (entry) =>
        files.includes(path.basename(entry.extension.__filePath || '')),
    );
    if (chatMatches.length === 1) return chatMatches[0];
  }

  const matches = Array.from(extensionActionRegistry.values()).filter(
    (entry) => entry.extension.id === action?.extensionId,
  );
  return matches.length === 1 ? matches[0] : null;
}

function extensionEntryForAction(action) {
  return extensionActionEntryForAction(action);
}

function extensionModuleForAction(action) {
  const entry = extensionActionEntryForAction(action);
  if (entry?.extension) return entry.extension;
  if (!action?.extensionFile) return null;
  return (
    Array.from(extensionModules.values()).find(
      (extension) =>
        path.basename(extension.__filePath || '') === action.extensionFile,
    ) || null
  );
}

function currentActionForStoredShortcut(action) {
  if (action?.kind === 'extension-command') {
    const entry = extensionActionEntryForAction(action);
    return entry ? extensionActionFromContribution(entry) : null;
  }
  if (action?.kind === 'extension-action') {
    const entry = extensionActionEntryForAction(action);
    return entry ? extensionActionFromContribution(entry) : null;
  }
  return action;
}

async function executeExtensionRootItem(action) {
  return measureDebugPerformance(
    'extension.root-item.execute',
    { action: summarizeDebugValue(action), alwaysLog: true },
    async () => {
      if (!action.rootAction)
        return {
          view: {
            type: 'preview',
            title: action.title || 'Extension item',
            content: action.subtitle || '',
          },
        };
      if (action.rootAction.type !== 'runExtensionAction')
        return executeViewAction(action.rootAction);
      const record = extensionActionHandlers.get(action.rootAction.handlerId);
      if (!record)
        return {
          view: {
            type: 'preview',
            title: 'Action unavailable',
            content: 'This extension item is no longer available.',
          },
        };
      if (!(record.entry && record.entry.extension)) {
        logWarn('extension.rootItem.missingEntry', {
          handlerId: action.rootAction.handlerId,
          actionTitle: action.title,
        });
        return {
          view: {
            type: 'preview',
            title: 'Action unavailable',
            content: 'This action is not available in the current context.',
          },
        };
      }
      try {
        const result = await measureDebugPerformance(
          'extension.root-item.handler',
          {
            extensionId: record.entry.extension.id,
            commandId: record.entry.command?.id,
            alwaysLog: true,
          },
          () =>
            record.handler(
              createExtensionContext(
                record.entry.extension,
                record.entry.command || null,
              ),
              action,
            ),
        );
        return executeViewActionResult(result, record.entry);
      } catch (error) {
        logError('extension.rootItem.failed', error, {
          source: 'host',
          scope: 'extension',
          extensionId: record.entry.extension?.id,
        });
        return { view: extensionErrorView(record.entry, error) };
      }
    },
  );
}

async function extensionRootActions(extensions = visibleExtensions()) {
  const actionGroups = await Promise.all(
    extensions.map((extension) => extensionRootActionsForExtension(extension)),
  );
  return actionGroups.flat();
}

async function extensionSearchActions(query, extensions = visibleExtensions()) {
  const actionGroups = await measureDebugPerformance(
    'extension.search.all',
    {
      extensionCount: extensions.length,
      queryLength: String(query || '').length,
    },
    () =>
      Promise.all(
        extensions.map((extension) =>
          extensionSearchActionsForExtension(extension, query),
        ),
      ),
  );
  return actionGroups.flat();
}

function rankContributionActions(actions, query) {
  return actions
    .map((action) => {
      const ranked = rankAction(action, query);
      return ranked ? { ...ranked, __ranked: true } : null;
    })
    .filter(Boolean)
    .sort(
      (a, b) =>
        b.score - a.score ||
        b.lastUsed - a.lastUsed ||
        a.title.localeCompare(b.title),
    )
    .slice(0, EXTENSION_ITEMS_PER_PROVIDER_LIMIT);
}

async function extensionSearchActionsForExtension(extension, query) {
  if (typeof extension.searchItems !== 'function') return [];
  return measureDebugPerformance(
    'extension.search.provider',
    { extensionId: extension.id, queryLength: String(query || '').length },
    async () => {
      try {
        const entry = {
          extension,
          command: { id: 'search', title: extension.title || extension.id },
        };
        const items = await withTimeout(
          extension.searchItems(createExtensionContext(extension, null), query),
          EXTENSION_ROOT_ITEMS_TIMEOUT_MS,
        );
        const list = Array.isArray(items)
          ? items
          : Array.isArray(items?.items)
            ? items.items
            : [];
        return measureDebugPerformanceSync(
          'extension.search.provider.rank',
          {
            extensionId: extension.id,
            itemCount: list.length,
            queryLength: String(query || '').length,
          },
          () =>
            rankContributionActions(
              list
                .map((item) => extensionRootActionFromItem(entry, item))
                .filter(Boolean),
              query,
            ),
        );
      } catch (error) {
        if (!String(error?.message || error).includes('Timed out'))
          logError('extension.searchItems.failed', error, {
            source: 'host',
            scope: 'extension',
            extensionId: extension.id,
          });
        return [];
      }
    },
  );
}

async function extensionRootActionsForExtension(extension) {
  if (typeof extension.rootItems !== 'function') return [];
  const cacheKey = extension.__filePath || extension.id;
  const cached = extensionRootItemsCache.get(cacheKey);
  if (cached && Date.now() - cached.updatedAt < EXTENSION_ROOT_ITEMS_TTL_MS) {
    markDebugPerformance('extension.root.cache-hit', {
      extensionId: extension.id,
      itemCount: cached.items.length,
    });
    return cached.items;
  }
  const refresh = refreshExtensionRootActions(extension, cacheKey);
  return cached?.items || (await refresh);
}

function refreshExtensionRootActions(extension, cacheKey) {
  const current = extensionRootItemsRefreshes.get(cacheKey);
  if (current) return current;
  const promise = measureDebugPerformance(
    'extension.root.provider',
    { extensionId: extension.id },
    async () => {
      const entry = {
        extension,
        command: { id: 'root', title: extension.title || extension.id },
      };
      const items = await withTimeout(
        extension.rootItems(createExtensionContext(extension, null)),
        EXTENSION_ROOT_ITEMS_TIMEOUT_MS,
      );
      const list = Array.isArray(items)
        ? items
        : Array.isArray(items?.items)
          ? items.items
          : [];
      const actions = measureDebugPerformanceSync(
        'extension.root.provider.rank',
        { extensionId: extension.id, itemCount: list.length },
        () =>
          rankContributionActions(
            list
              .map((item) => extensionRootActionFromItem(entry, item))
              .filter(Boolean),
            '',
          ),
      );
      extensionRootItemsCache.set(cacheKey, {
        updatedAt: Date.now(),
        items: actions,
      });
      return actions;
    },
  )
    .catch((error) => {
      if (!String(error?.message || error).includes('Timed out'))
        logError('extension.rootItems.failed', error, {
          source: 'host',
          scope: 'extension',
          extensionId: extension.id,
        });
      return extensionRootItemsCache.get(cacheKey)?.items || [];
    })
    .finally(() => {
      extensionRootItemsRefreshes.delete(cacheKey);
    });
  extensionRootItemsRefreshes.set(cacheKey, promise);
  return promise;
}

function withTimeout(promise, timeoutMs) {
  return Promise.race([
    Promise.resolve(promise),
    new Promise((_, reject) =>
      setTimeout(
        () => reject(new Error(`Timed out after ${timeoutMs}ms`)),
        timeoutMs,
      ),
    ),
  ]);
}

function normalizeItemAppearance(appearance) {
  const foreground = appearance?.foreground;
  if (!ITEM_FOREGROUND_COLORS.has(foreground)) return;
  return { foreground };
}

function extensionRootActionFromItem(entry, item) {
  if (!(item?.id && item.title)) return null;
  const primaryAction = normalizeViewAction(
    item.primaryAction || item.action,
    entry,
  );
  const actionPanel = normalizeActionPanel(
    item.actionPanel,
    item.actions || [],
    entry,
  );
  return {
    id: `extension-root:${entry.extension.id}:${item.id}`,
    kind: 'extension-root-item',
    extensionId: entry.extension.id,
    ...appResultMarker(item),
    commandId: item.id,
    extensionFile: entry.extension.__filePath
      ? path.basename(entry.extension.__filePath)
      : undefined,
    rootAction: primaryAction,
    removable: Boolean(entry.extension.__generated),
    title: item.title,
    subtitle: item.subtitle || entry.extension.title || 'Extension item',
    aliases: item.aliases || item.keywords || [],
    icon: item.icon || 'sparkles',
    iconUrl: item.image || item.iconUrl || null,
    thumbnailUrl: item.thumbnailUrl || null,
    videoUrl: item.videoUrl || null,
    imageDataUrl: item.imageDataUrl || null,
    filePath: item.filePath || item.path || null,
    text: item.text || '',
    score: Math.min(Number(item.score || 35), 90),
    lastUsed: Number(item.lastUsed || 0),
    dismissAfterRun: item.dismissAfterRun || primaryAction?.dismissAfterRun,
    customizable: Boolean(item.customizable),
    actionPanel,
    appearance: normalizeItemAppearance(item.appearance),
  };
}

function extensionActionFromContribution(entry) {
  const item = entry.item;
  if (!(item?.id && item.title)) return null;
  const extensionFile = entry.extension.__filePath
    ? path.basename(entry.extension.__filePath)
    : undefined;
  const primaryAction = normalizeViewAction(
    item.primaryAction || item.action,
    entry,
  );
  const actionId =
    item.actionId || `extension-action:${entry.extension.id}:${item.id}`;
  const action = {
    id: actionId,
    kind: 'extension-action',
    extensionId: entry.extension.id,
    commandId: item.id,
    registeredActionId: item.id,
    extensionFile,
    aiChatId: extensionFile
      ? aiChatIdForExtensionFile(extensionFile)
      : undefined,
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
    actionPanel: normalizeActionPanel(
      item.actionPanel,
      item.actions || [],
      entry,
    ),
    appearance: normalizeItemAppearance(item.appearance),
  };
  const declaredShortcut = userState.removedShortcuts?.[action.id]
    ? null
    : item.globalShortcut ||
      (item.shortcutScope === 'global' ? item.shortcut : null);
  const shortcut = shortcutForAction(action) || declaredShortcut;
  return shortcut ? { ...action, shortcut } : action;
}

async function executeActionForIpc(action) {
  return measureDebugPerformance(
    'ipc.actions.execute',
    { action: summarizeDebugValue(action), alwaysLog: true },
    async () => {
      let trustedAction: any = null;
      try {
        trustedAction = resolveRootActionForIpc(action);
        const result = normalizeHostViewResult(
          await measureDebugPerformance(
            'action.execute',
            { action: summarizeDebugValue(trustedAction), alwaysLog: true },
            () => executeAction(trustedAction),
          ),
        );
        structuredClone(result);
        spawnPendingViewLoaders(result);
        return result;
      } catch (error) {
        if (trustedAction?.kind === 'extension-command') {
          const entry = extensionEntryForAction(trustedAction);
          if (entry) return { view: extensionErrorView(entry, error) };
        }
        if (action?.kind === 'extension-action') {
          const entry = extensionActionEntryForAction(action);
          if (entry) return { view: extensionErrorView(entry, error) };
        }
        return { view: actionFailedFeedbackView() };
      }
    },
  );
}

function extensionErrorMessage(error) {
  if (!error) return 'Unknown error';
  if (error.stack) return error.stack;
  if (error.message) return error.message;
  return String(error);
}

function extensionErrorView(entry, error) {
  const message = extensionErrorMessage(error);
  const title =
    (entry?.command?.title || entry?.extension?.title || 'Extension') +
    ' failed';
  const fixWithAi = extensionErrorAiAction(entry, message);
  const actions: CommandAction[] = [{ type: 'popView', title: 'Back' }];
  if (fixWithAi) actions.unshift(fixWithAi as CommandAction);
  return normalizeView(
    feedbackView({
      id: 'extension-action-error',
      title,
      message: 'The action could not be completed. Try again.',
      tone: 'error',
      actions,
    }),
    entry,
  );
}

function actionFailedFeedbackView() {
  return feedbackView({
    id: 'action-failed',
    title: 'Action failed',
    message: 'The action could not be completed. Try again.',
    tone: 'error',
  });
}

function extensionErrorAiAction(entry, message) {
  if (!entry?.extension) return null;
  const extensionFile = entry.extension.__filePath
    ? path.basename(entry.extension.__filePath)
    : null;
  if (!extensionFile) return null;
  const prompt = `This generated action failed. Please fix the extension.\n\nAction: ${entry.command?.title || entry.command?.id || 'unknown'}\nExtension: ${entry.extension.title || entry.extension.id}\nFile: ${extensionFile}\n\nError:\n\`\`\`\n${message}\n\`\`\``;
  return {
    type: 'runExtensionAction',
    title: 'Fix with AI',
    __handler: async () =>
      aiChatView(
        getOrCreateExtensionChat(
          extensionFile,
          entry.extension.title || entry.command?.title,
        ),
        { initialPrompt: prompt },
      ),
  };
}

const VIEW_TYPES = new Set([
  'list',
  'grid',
  'preview',
  'chat',
  'form',
  'editor',
  'progress',
  'webview',
  'camera',
]);

function isView(value) {
  return Boolean(value?.type && VIEW_TYPES.has(value.type));
}

function normalizeExtensionView(result, entry) {
  if (!result) return null;
  const view = isView(result)
    ? result
    : isView(result.view)
      ? result.view
      : null;
  return view ? normalizeView(view, entry) : null;
}

function normalizeView(view, entry) {
  const actions = normalizeViewActions(view.actions, entry);
  const webviewPermissions =
    view.type === 'webview'
      ? filterWebviewPermissionsForExtension(
          entry?.extension,
          view.webviewPermissions,
        )
      : view.webviewPermissions;
  const viewId = view.id || `view:${crypto.randomUUID()}`;

  // Detect and register data loader before stripping it from items
  const loaderHandle = isLoaderHandle(view.items) ? view.items : undefined;
  if (loaderHandle) viewLoaderRegistry.register(viewId, loaderHandle, entry);

  const items = normalizeViewItems(normalizeLoaderItems(view.items), entry);
  const sections = Array.isArray(view.sections)
    ? view.sections.map((section) => ({
        ...section,
        items: normalizeViewItems(section.items, entry),
      }))
    : view.sections;

  const emptyView = resolveLoaderEmptyView(view.emptyView, loaderHandle);

  return {
    ...view,
    id: viewId,
    ...(webviewPermissions === undefined ? {} : { webviewPermissions }),
    actions,
    actionPanel: normalizeActionPanel(view.actionPanel, actions, entry),
    onSelectionChange: normalizeViewAction(view.onSelectionChange, entry),
    submitAction: normalizeViewAction(view.submitAction, entry),
    searchAccessory: view.searchAccessory
      ? {
          ...view.searchAccessory,
          onChange: normalizeViewAction(view.searchAccessory.onChange, entry),
        }
      : view.searchAccessory,
    refresh: registerViewRefreshForRenderer(view.refresh, entry, view),
    items,
    sections,
    ...(emptyView ? { emptyView } : {}),
    ...(loaderHandle ? { isLoading: true } : {}),
  };
}

function persistentActionForRef(action, entry) {
  if (action?.type !== 'runExtensionRegisteredAction') return null;
  const extensionId = action.extensionId || entry?.extension?.id;
  const registeredActionId = action.registeredActionId || action.actionId;
  const registered = extensionActionRegistry.get(
    `${extensionId}:${registeredActionId}`,
  );
  return registered ? extensionActionFromContribution(registered) : null;
}

function normalizeViewItems(items, entry) {
  if (isLoaderHandle(items)) return [];
  return Array.isArray(items)
    ? items.map((item) => {
        const itemActions = normalizeViewActions(item.actions, entry);
        const primaryAction = normalizeViewAction(
          item.primaryAction || item.action,
          entry,
        );
        const { run, __handler, action, ...safeItem } = item;
        const detailActions = normalizeViewActions(item.detail?.actions, entry);
        return {
          ...safeItem,
          ...(item.detail
            ? { detail: { ...item.detail, actions: detailActions } }
            : {}),
          actions: itemActions,
          actionPanel: normalizeActionPanel(
            item.actionPanel,
            itemActions,
            entry,
          ),
          primaryAction,
          persistentAction:
            item.persistentAction ||
            persistentActionForRef(primaryAction, entry),
          appearance: normalizeItemAppearance(item.appearance),
        };
      })
    : items;
}

function normalizeActionPanel(panel, fallbackActions, entry) {
  if (panel?.sections)
    return {
      ...panel,
      sections: panel.sections.map((section) => {
        const { lazyActions, ...safeSection } = section;
        return {
          ...safeSection,
          actions: normalizeViewActions(
            [...(section.actions || []), ...(lazyActions || [])],
            entry,
          ),
        };
      }),
    };
  if (Array.isArray(fallbackActions) && fallbackActions.length)
    return {
      sections: [{ actions: normalizeViewActions(fallbackActions, entry) }],
    };
  return panel;
}

function normalizeViewActions(actions, entry) {
  return Array.isArray(actions)
    ? actions
        .map((action) => normalizeViewAction(action, entry))
        .filter(Boolean)
    : [];
}

function normalizeViewAction(action, entry) {
  if (!action) return null;
  const handler =
    typeof action.__handler === 'function'
      ? action.__handler
      : typeof action.run === 'function'
        ? action.run
        : null;
  if (handler) {
    // Do not register handlers without extension context entries.
    // When entry is null the handler would receive a broken context.
    if (!entry) {
      logWarn('extension.normalizeViewAction.nullEntry', {
        actionType: action.type,
        actionTitle: action.title,
      });
      return { ...action, __handler: undefined, run: undefined };
    }
    const handlerId = crypto.randomUUID();
    extensionActionHandlers.set(handlerId, { entry, handler });
    const { __handler, run, ...rest } = action;
    return normalizeViewAction(
      { ...rest, type: 'runExtensionAction', handlerId },
      entry,
    );
  }
  const normalized = action.submenu
    ? { ...action, submenu: normalizeActionPanel(action.submenu, [], entry) }
    : action;
  if (
    (normalized.type === 'rootView' ||
      normalized.type === 'pushView' ||
      normalized.type === 'replaceView') &&
    normalized.view
  ) {
    return registerViewActionForRenderer(
      {
        ...normalized,
        view: normalizeView(normalized.view, entry),
      },
      entry,
    );
  }
  if (normalized.type === 'promptAction' && normalized.targetAction) {
    return registerViewActionForRenderer(
      {
        ...normalized,
        targetAction: normalizeViewAction(normalized.targetAction, entry),
      },
      entry,
    );
  }
  if (
    (normalized.type === 'createWindow' ||
      normalized.type === 'toggleWindow') &&
    normalized.view
  ) {
    return registerViewActionForRenderer(
      {
        ...normalized,
        view: normalizeView(normalized.view, entry),
      },
      entry,
    );
  }
  return registerViewActionForRenderer(normalized, entry);
}

let cachedUserShellPathPromise: Promise<string> | null = null;

function resolveUserShellPath(): Promise<string> {
  if (cachedUserShellPathPromise) return cachedUserShellPathPromise;
  cachedUserShellPathPromise = new Promise((resolve) => {
    const userShell = process.platform === 'win32' ? '' : process.env.SHELL;
    if (!userShell) return resolve('');
    const delimiter = '__NVM_PATH_DELIM__';
    execFile(
      userShell,
      ['-ilc', `printf '%s%s%s' '${delimiter}' "$PATH" '${delimiter}'`],
      { timeout: 5000 },
      (_error, stdout) => {
        resolve(
          typeof stdout === 'string' ? stdout.split(delimiter)[1] || '' : '',
        );
      },
    );
  });
  return cachedUserShellPathPromise;
}

async function shellSpawnEnv(extraEnv?: Record<string, string>) {
  const userShellPath = await resolveUserShellPath();
  const mergedPath = [userShellPath, process.env.PATH]
    .filter(Boolean)
    .join(path.delimiter);
  return {
    ...process.env,
    ...(mergedPath ? { PATH: mergedPath } : {}),
    ...(extraEnv || {}),
  };
}

async function runShellCommand(command, args = [], options: any = {}) {
  const env = await shellSpawnEnv(options.env);
  const expandedCommand = expandUserPath(String(command));
  const expandedArgs = Array.isArray(args)
    ? args.map((arg) => expandUserPath(String(arg)))
    : [];
  const safeTimeoutMs = Math.max(
    1000,
    Math.min(Number(options.timeout || 30_000), 120_000),
  );

  return new Promise((resolve) => {
    const child = spawn(expandedCommand, expandedArgs, {
      cwd: options.cwd ? expandUserPath(options.cwd) : undefined,
      env,
      shell: Boolean(options.shell),
      timeout: safeTimeoutMs,
    });
    let stdout = '';
    let stderr = '';
    let settled = false;

    const settle = (result: {
      stdout: string;
      stderr: string;
      exitCode: number;
    }) => {
      if (settled) return;
      settled = true;
      resolve(result);
    };

    child.stdout?.on('data', (chunk) => {
      stdout = limitedOutput(stdout + chunk.toString(), options.outputLimit);
    });
    child.stderr?.on('data', (chunk) => {
      stderr = limitedOutput(stderr + chunk.toString(), options.outputLimit);
    });
    child.on('error', (error) =>
      settle({ stdout, stderr: stderr || error.message, exitCode: 1 }),
    );
    child.on('close', (exitCode) => settle({ stdout, stderr, exitCode }));

    // Hard kill after timeout + 5s grace period. Node's spawn timeout sends
    // SIGTERM; if the process ignores it we escalate to SIGKILL.
    const killer = setTimeout(() => {
      if (!(settled || child.killed)) {
        child.kill('SIGKILL');
        settle({
          stdout,
          stderr: `${stderr}\nKilled after ${safeTimeoutMs + 5000}ms`,
          exitCode: -1,
        });
      }
    }, safeTimeoutMs + 5000);
    killer.unref();
    child.on('close', () => clearTimeout(killer));
  });
}

function runShellScript(script, options: any = {}) {
  const home = os.homedir();
  const resolvedScript = String(script).replace(/~(?=\/|$)/g, home);
  return runShellCommand(
    options.shell || '/bin/bash',
    ['-lc', resolvedScript],
    { ...options, shell: false },
  );
}

function isAction(value) {
  return Boolean(value?.type && !isView(value));
}

function normalizeViewPatch(patch, entry) {
  if (!patch) return patch;
  return {
    ...patch,
    items: normalizeViewItems(patch.items, entry),
  };
}

function normalizeHostViewResult(result) {
  if (!result) return result;
  return {
    ...result,
    ...(result.view ? { view: normalizeView(result.view, null) } : {}),
    ...(result.patch ? { patch: normalizeViewPatch(result.patch, null) } : {}),
  };
}

async function executeViewActionResult(result, entry, launchContext?: any) {
  if (!result) return result;
  if (isAction(result))
    return executeViewAction(normalizeViewAction(result, entry), launchContext);
  if (isAction(result.action))
    return executeViewAction(
      normalizeViewAction(result.action, entry),
      launchContext,
    );
  const view = normalizeExtensionView(result, entry);
  return view
    ? {
        view,
        navigation: result?.navigation || 'push',
        toast: result?.toast,
        patch: normalizeViewPatch(result?.patch, entry),
      }
    : { ...result, patch: normalizeViewPatch(result?.patch, entry) };
}

async function executeHostRefreshAction(record, launchContext?: any) {
  if (record.action?.type === 'runExtensionAction') {
    const handlerRecord = extensionActionHandlers.get(record.action.handlerId);
    if (!handlerRecord) return { skipped: true };
    const result = await handlerRecord.handler(
      createExtensionContext(
        handlerRecord.entry.extension,
        handlerRecord.entry.command,
        launchContext,
      ),
      record.action,
    );
    return executeViewActionResult(result, handlerRecord.entry, launchContext);
  }
  if (record.action) return executeViewAction(record.action, launchContext);
  if (!record.entry?.command || typeof record.entry.command.run !== 'function')
    return { skipped: true };
  const result = await record.entry.command.run(
    createExtensionContext(
      record.entry.extension,
      record.entry.command,
      launchContext,
    ),
  );
  const view = result?.type ? result : result?.view?.type ? result.view : null;
  if (
    view?.items &&
    !isLoaderHandle(view.items) &&
    Array.isArray(view.items) &&
    view.items.length > 0
  )
    return {
      patch: {
        mode: record.mode || 'replace',
        items: normalizeViewItems(view.items, record.entry),
      },
    };
  return executeViewActionResult(result, record.entry, launchContext);
}

function refreshBackoffDelay(failureCount: number) {
  return Math.min(
    30_000,
    1000 * Math.max(1, 2 ** Math.max(0, failureCount - 1)),
  );
}

async function refreshViewForIpc(input: any = {}) {
  return measureDebugPerformance(
    'ipc.view.refresh',
    { input: summarizeDebugValue(input), alwaysLog: true },
    async () => {
      pruneViewRefreshRecords();
      const refreshId =
        typeof input === 'string' ? input : String(input?.id || '');
      const record = refreshId ? viewRefreshRecords.get(refreshId) : null;
      if (!record) return { skipped: true };
      if (input?.viewId && record.viewId && input.viewId !== record.viewId)
        return { skipped: true };
      const now = Date.now();
      if (record.backoffUntil && record.backoffUntil > now)
        return { skipped: true };
      if (record.running) return { skipped: true };
      record.running = measureDebugPerformance(
        'view.refresh.host-action',
        {
          refreshId,
          viewId: record.viewId,
          extensionId: record.entry?.extension?.id,
          commandId: record.entry?.command?.id,
          alwaysLog: true,
        },
        async () => {
          try {
            const result = normalizeHostViewResult(
              await executeHostRefreshAction(record, {
                refresh: true,
                reason: 'refresh',
                startedAt: Date.now(),
              }),
            );
            structuredClone(result);
            spawnPendingViewLoaders(result);
            record.failureCount = 0;
            record.backoffUntil = undefined;
            return result;
          } catch (error) {
            record.failureCount += 1;
            record.backoffUntil =
              Date.now() + refreshBackoffDelay(record.failureCount);
            logWarn(
              'extension.viewRefresh.failed',
              {
                viewId: record.viewId,
                error: error instanceof Error ? error.message : String(error),
              },
              {
                source: 'host',
                scope: 'extension',
                extensionId: record.entry?.extension?.id,
                commandId: record.entry?.command?.id,
              },
            );
            return { skipped: true };
          } finally {
            record.running = null;
          }
        },
      );
      return record.running;
    },
  );
}

async function executeViewActionForIpc(action) {
  return measureDebugPerformance(
    'ipc.view-action.execute',
    { action: summarizeDebugValue(action), alwaysLog: true },
    async () => {
      let trustedAction: any = null;
      try {
        trustedAction = resolveViewActionForIpc(action);
        const result = normalizeHostViewResult(
          await measureDebugPerformance(
            'view-action.execute',
            { action: summarizeDebugValue(trustedAction), alwaysLog: true },
            () => executeViewAction(trustedAction),
          ),
        );
        structuredClone(result);
        spawnPendingViewLoaders(result);
        return result;
      } catch (error) {
        const record =
          trustedAction?.type === 'runExtensionAction'
            ? extensionActionHandlers.get(trustedAction.handlerId)
            : null;
        if (record)
          return {
            view: extensionErrorView(record.entry, error),
            navigation: 'push',
          };
        return {
          view: actionFailedFeedbackView(),
          navigation: 'push',
        };
      }
    },
  );
}

function clipboardSnapshot() {
  return clipboardService!.clipboardSnapshot();
}

function restoreClipboardSnapshot(
  snapshot: ReturnType<typeof clipboardSnapshot>,
) {
  clipboardService!.restoreClipboardSnapshot(snapshot);
}

function clipboardHistoryIdForText(text: string) {
  return clipboardService!.clipboardHistoryIdForText(text);
}

function suppressClipboardHistoryId(id: string, durationMs = 2000) {
  clipboardService!.suppressClipboardHistoryId(id, durationMs);
}

function pasteTextAction(action: any) {
  clipboardService!.pasteTextAction(action);
}

function executeWindowAction(action: any) {
  return extensionWindowManager.executeWindowAction(action);
}

function shellResultView(title, result) {
  return {
    type: 'preview',
    title,
    content: `Exit code: ${result.exitCode ?? 0}\n\nSTDOUT\n${result.stdout || '-'}\n\nSTDERR\n${result.stderr || '-'}`,
  };
}

function runQuitCleanup() {
  if (didRunQuitCleanup) return;
  didRunQuitCleanup = true;
  if (app.isReady()) globalShortcut.unregisterAll();
  updateManager.clearTimers();
  jobRegistry.clear();
  extensionWindowManager.closeAll();
  appIndexService.closeWatchers();
  for (const watcher of extensionFileWatchers) watcher.close();
}

function registerTestModeIpcHandlers() {
  const handle = (channel: string, handler: (...args: any[]) => unknown) =>
    ipcMain.handle(channel, handler);
  handle('actions:search', (_event, query, options) =>
    searchActions(query, options),
  );
  handle('actions:execute', (_event, action) => {
    if (!testModePaletteActionIsSafe(action))
      return {
        toast: { message: 'Production actions are disabled in test mode' },
      };
    return executeActionForIpc(action);
  });
  handle('test:invoke', async () => {
    const actions = await searchActions('Test: Confirm safe action');
    void executeActionForIpc(actions[0]).catch((error) =>
      console.error('test action failed', error),
    );
    return { found: actions.length > 0 };
  });
  handle('test:stage-extension-proposal', (_event, filename, source) =>
    stageExtensionProposal(filename, source),
  );
  handle('test:run-job', async (_event, id) => {
    if (!jobRegistry.has(id)) return { found: false };
    await jobRegistry.run(id, 'electron-smoke');
    return { found: true };
  });
  handle('test:fail-next-extension-activation', (_event, phase) => {
    testExtensionActivationFailurePhase = String(phase || 'after-persist');
    return { armed: true };
  });
  handle('test:is-action-shortcut-registered', (_event, accelerator) => {
    const normalized = normalizeAccelerator(String(accelerator || ''));
    return {
      registered:
        registeredActionAccelerators.has(normalized) &&
        globalShortcut.isRegistered(normalized),
    };
  });
  handle('view-action:execute', (_event, action) =>
    executeViewActionForIpc(action),
  );
  handle('actions:set-shortcut', (_event, action, shortcut) =>
    setShortcut(action, shortcut),
  );
  handle('nevermind:auth-status', () => ({ authed: false }));
  handle('gh:status', () => ({ installed: false, authed: false }));
  handle('settings:get', (_event, id) => getSetting(id));
  handle('palette:set-mode', () => undefined);
  handle('palette:hide', () => paletteWindow.hidePalette());
  handle('palette:shortcut-ready', () => paletteWindow.revealPalette());
  handle('actions:suspend-shortcuts', () => undefined);
  handle('actions:resume-shortcuts', () => undefined);
  handle('actions:get-shortcuts', () => getShortcuts());
  handle('logs:write', () => undefined);
  handle('app:quit', () => {
    requestQuitApp('test');
    return { ok: true };
  });
}

function requestQuitApp(reason = 'action') {
  nevermindApp.isQuiting = true;
  logInfo('app.quit.requested', { reason }, { source: 'host', scope: 'app' });
  app.quit();
  setTimeout(() => {
    logWarn(
      'app.quit.fallbackExit',
      { reason },
      { source: 'host', scope: 'app' },
    );
    runQuitCleanup();
    app.exit(0);
  }, 250).unref?.();
}

async function executeViewAction(action, launchContext?: any) {
  switch (action?.type) {
    case 'nativeAction': {
      const native = action.nativeAction as
        | { kind?: string; viewId?: string }
        | undefined;
      if (native?.kind === 'view-hydrate-retry' && native.viewId) {
        if (viewLoaderRegistry.has(native.viewId))
          viewLoaderRegistry.retry(native.viewId);
        // Return a loading skeleton so the renderer replaces the error view
        return {
          view: {
            type: 'list' as const,
            id: native.viewId,
            title: 'Loading...',
            isLoading: true,
            items: [],
          },
          navigation: 'replace' as const,
        };
      }
      return executeAction(action.nativeAction, { keepPaletteOpen: true });
    }
    case 'openPath':
      runInBackground(() => shell.openPath(action.path));
      break;
    case 'revealPath':
      runInBackground(() => shell.showItemInFolder(action.path));
      break;
    case 'quickLook':
      return quickLookPath(action.path);
    case 'openWith':
      runInBackground(() =>
        openPathWithApp(action.path, action.appPath || action.app?.path),
      );
      break;
    case 'openUrl':
      runInBackground(async () => {
        if (!(await openExternalUrl(action.url)))
          logWarn(
            'openUrl.rejected',
            { url: action.url },
            { source: 'host', scope: 'action' },
          );
      });
      break;
    case 'copyText':
      clipboard.writeText(action.text || '');
      break;
    case 'pasteText':
      pasteTextAction(action);
      return { toast: { message: 'Pasted' } };
    case 'pasteClipboard':
      pasteClipboardAction(action);
      return { toast: { message: 'Pasted' } };
    case 'typeText': {
      const result = await typeTextIntoFrontmostApp(action.text || '', {
        delayMs: action.delayMs,
      });
      return result?.ok
        ? { toast: { message: 'Typed' } }
        : {
            toast: {
              message: result?.error || 'Unable to type text',
              tone: 'error',
            },
          };
    }
    case 'createWindow':
    case 'showWindow':
    case 'hideWindow':
    case 'toggleWindow':
    case 'closeWindow':
      return executeWindowAction(action);
    case 'copyImage':
      if (action.path)
        clipboard.writeImage(
          nativeImage.createFromPath(expandUserPath(action.path)),
        );
      else if (action.imagePath)
        clipboard.writeImage(nativeImage.createFromPath(action.imagePath));
      else
        clipboard.writeImage(
          nativeImage.createFromDataURL(action.imageDataUrl),
        );
      break;
    case 'removeClipboardHistory':
      return removeClipboardHistoryEntries(action);
    case 'trash': {
      const results: Array<{ path: string; ok: boolean }> = [];
      for (const itemPath of action.paths || [action.path]) {
        if (!itemPath) continue;
        const fullPath = expandUserPath(itemPath);
        try {
          let timer: NodeJS.Timeout | undefined;
          const timedOut = new Promise<'timedOut'>((resolve) => {
            timer = setTimeout(() => resolve('timedOut'), 10_000);
          });
          const outcome = await Promise.race([
            shell.trashItem(fullPath).then(() => 'ok' as const),
            timedOut,
          ]);
          clearTimeout(timer);
          if (outcome === 'timedOut') {
            logWarn(
              'extension.trash.timedOut',
              { path: fullPath },
              { source: 'host', scope: 'extension' },
            );
            results.push({ path: fullPath, ok: false });
          } else {
            results.push({ path: fullPath, ok: true });
          }
        } catch (err: any) {
          logWarn(
            'extension.trash.failed',
            { path: fullPath, error: err?.message },
            { source: 'host', scope: 'extension' },
          );
          results.push({ path: fullPath, ok: false });
        }
      }
      const successCount = results.filter((r) => r.ok).length;
      const total = results.length;
      return {
        toast: {
          message:
            total === 0
              ? 'Nothing to trash'
              : successCount === total
                ? `Moved ${total} item${total > 1 ? 's' : ''} to Trash`
                : `Moved ${successCount}/${total} items to Trash`,
          tone: successCount === total ? 'default' : 'error',
        },
      };
    }
    case 'rootView':
      return { view: action.view, navigation: 'root' };
    case 'pushView':
      return { view: action.view, navigation: 'push' };
    case 'replaceView':
      return { view: action.view, navigation: 'replace' };
    case 'popView':
      return { navigation: 'pop' };
    case 'shellExec': {
      const result = await runShellCommand(
        action.command,
        action.args || [],
        action.options || {},
      );
      return {
        view: shellResultView(
          action.title || action.command || 'Command',
          result,
        ),
        navigation: 'push',
      };
    }
    case 'shellScript': {
      const result = await runShellScript(action.script, action.options || {});
      return {
        view: shellResultView(action.title || 'Script', result),
        navigation: 'push',
      };
    }
    case 'lockScreen':
      runInBackground(() =>
        executeSystemBuiltin({ builtin: 'lock-screen' }, () => {}),
      );
      break;
    case 'sleepSystem':
      runInBackground(() =>
        executeSystemBuiltin({ builtin: 'sleep' }, () => {}),
      );
      break;
    case 'restartSystem':
      runInBackground(() =>
        executeSystemBuiltin({ builtin: 'restart' }, () => {}),
      );
      break;
    case 'openSystemSettings':
      runInBackground(() => {
        const paneUrl = systemSettingsPaneUrl(action.paneId);
        return paneUrl
          ? shell.openExternal(paneUrl)
          : executeSystemBuiltin({ builtin: 'settings' }, () => {});
      });
      break;
    case 'openKeyboardSettings':
      runInBackground(openSystemKeyboardSettings);
      break;
    case 'quitApp':
      requestQuitApp('view-action');
      break;
    case 'forceQuitApp': {
      const appPath = action.path || action.appPath || '';
      const appName =
        action.app?.name || action.title?.replace(/^Force Quit /, '') || '';
      try {
        const result = await forceQuitOsApp(appPath, appName);
        if (result.ok) {
          return { toast: { message: `Force quit ${appName}` } };
        }
        return {
          toast: {
            message: result.error || `Could not force quit ${appName}`,
            tone: 'error',
          },
        };
      } catch (error) {
        logWarn('forceQuitApp.failed', error, {
          source: 'host',
          scope: 'apps',
        });
        return {
          toast: { message: `Could not force quit ${appName}`, tone: 'error' },
        };
      }
    }
    case 'checkForUpdates':
      return checkForUpdatesView();
    case 'downloadUpdate':
      return downloadUpdateView();
    case 'installUpdate':
      return installDownloadedUpdate();
    case 'toggleSetting': {
      const definition = settingDefinition(action.settingId);
      if (!definition || definition.type !== 'boolean')
        return { toast: { message: 'Setting not found', tone: 'error' } };
      const result = setSetting(
        definition.id,
        toggledSettingValue(definition, getSetting(definition.id)),
      );
      if (!result.ok)
        return { toast: { message: result.message, tone: 'error' } };
      return { patch: { items: [settingItemPatch(definition)] } };
    }
    case 'setActionShortcut': {
      const result = await setShortcut(
        action.targetAction || action.action,
        action.accelerator || action.shortcut,
      );
      return {
        toast: {
          message: result.message,
          tone: result.ok ? 'default' : 'error',
        },
        ok: result.ok,
      };
    }
    case 'setSettingShortcut': {
      const result = setShortcutSetting(
        action.settingId,
        action.accelerator || action.shortcut,
      );
      return {
        patch: {
          items: result.ok
            ? [settingItemPatch(settingDefinition(action.settingId))]
            : [],
        },
        toast: {
          message: result.message,
          tone: result.ok ? 'default' : 'error',
        },
        ok: result.ok,
      };
    }
    case 'removeShortcut': {
      const result = await removeShortcut(action.actionId);
      if (!result.ok)
        return { toast: { message: result.message, tone: 'error' } };
      return {
        patch: { removeItemIds: [`shortcut:${action.actionId}`] },
        toast: { message: result.message },
        ok: true,
      };
    }
    case 'setActionAlias': {
      const result = await setAlias(
        action.targetAction || action.action,
        action.alias,
      );
      return {
        toast: {
          message: result.message,
          tone: result.ok ? 'default' : 'error',
        },
        ok: result.ok,
      };
    }
    case 'removeActionAlias': {
      const result = await removeAlias(
        action.targetAction || action.action,
        action.alias,
      );
      return {
        toast: {
          message: result.message,
          tone: result.ok ? 'default' : 'error',
        },
        ok: result.ok,
      };
    }
    case 'duplicateCreatedAction': {
      const result = await duplicateCreatedAction(
        action.targetAction || action.action,
      );
      if (!result.ok)
        return { toast: { message: result.message, tone: 'error' } };
      return { toast: { message: result.message }, action: result.action };
    }
    case 'removeCreatedAction': {
      const result = await removeCreatedAction(
        action.targetAction || action.action,
      );
      return {
        toast: {
          message: result.message,
          tone: result.ok ? 'default' : 'error',
        },
        ok: result.ok,
      };
    }
    case 'submitExtensionPr': {
      if (!extensionPrSubmitter) {
        return {
          toast: {
            message: 'Cannot submit extensions right now',
            tone: 'error',
          },
        };
      }
      const result = await extensionPrSubmitter.submitExtensionPr(action);
      return {
        toast: {
          message: result.prUrl ? `PR opened: ${result.prUrl}` : result.message,
          tone: result.ok ? 'default' : 'error',
        },
      };
    }
    case 'clearActionOverride': {
      const result = await clearOverride(action.targetAction || action.action);
      return {
        toast: {
          message: result.message,
          tone: result.ok ? 'default' : 'error',
        },
        ok: result.ok,
      };
    }
    case 'recordShortcut':
      return {
        toast: { message: 'Shortcut recording is handled by the palette' },
      };
    case 'runExtensionRegisteredAction': {
      const resolved = resolveRegisteredActionRef(action);
      if (resolved.error)
        return { toast: { message: resolved.error, tone: 'error' } };
      return resolved.rootAction
        ? executeExtensionRootItem(resolved.rootAction)
        : { toast: { message: 'Action unavailable', tone: 'error' } };
    }
    case 'renameExtensionPrompt': {
      const extAction = action.targetAction || action.action;
      const extension = extensionModuleForAction(extAction);
      if (!extension)
        return { toast: { message: 'Extension not found', tone: 'error' } };
      const itemTitle = extAction.title || extension.title || '';
      return {
        view: {
          type: 'form',
          title: `Rename "${itemTitle}"`,
          fields: [
            {
              id: 'title',
              type: 'text',
              label: 'Name',
              value: itemTitle,
              placeholder: 'Enter a new name',
              required: true,
            },
          ],
          submitAction: {
            type: 'renameExtension',
            title: 'Rename',
            extensionId: extension.id,
            commandId: extAction.commandId || '',
          },
        },
        navigation: 'push',
      };
    }
    case 'renameExtension': {
      const extension = Array.from(extensionModules.values()).find(
        (ext) => ext.id === action.extensionId,
      );
      if (!extension)
        return { toast: { message: 'Extension not found', tone: 'error' } };
      const newTitle = String(action.formValues?.title || '').trim();
      if (!newTitle)
        return { toast: { message: 'Name is required', tone: 'error' } };
      const commandId = action.commandId;
      const command = commandId
        ? extensionActionRegistry.get(`${extension.id}:${commandId}`)
            ?.command || null
        : null;
      const result = await renameExtension(extension, command, {
        title: newTitle,
        ...(command ? { commandTitle: newTitle } : {}),
      });
      invalidateExtensionRootItems();
      return {
        toast: { message: `Renamed to ${newTitle}` },
        navigation: 'pop',
      };
    }
    case 'promptAction':
      return { view: promptActionView(action), navigation: 'push' };
    case 'runExtensionAction': {
      const record = extensionActionHandlers.get(action.handlerId);
      if (!record)
        return {
          toast: { message: 'Action is no longer available', tone: 'error' },
        };
      if (!(record.entry && record.entry.extension)) {
        logWarn('extension.action.missingEntry', {
          handlerId: action.handlerId,
          actionTitle: action.title,
        });
        return {
          toast: {
            message: 'This action is not available in the current context.',
            tone: 'error',
          },
        };
      }
      try {
        const result = await measureDebugPerformance(
          'extension.action.handler',
          {
            extensionId: record.entry.extension.id,
            commandId: record.entry.command?.id,
            actionTitle: action.title,
            alwaysLog: true,
          },
          () =>
            record.handler(
              createExtensionContext(
                record.entry.extension,
                record.entry.command || null,
                launchContext,
              ),
              action,
            ),
        );
        return executeViewActionResult(result, record.entry, launchContext);
      } catch (error) {
        logError('extension.action.failed', error, {
          source: 'host',
          scope: 'extension',
          extensionId: record.entry.extension?.id,
          commandId: record.entry.command?.id,
        });
        return {
          view: extensionErrorView(record.entry, error),
          navigation: 'push',
        };
      }
    }
    default:
      throw new Error(
        `Unsupported action type: ${String(action?.type || 'unknown')}`,
      );
  }
}

function resolveRegisteredActionRef(action) {
  let current = action;
  const visited = new Set<string>();
  while (current?.type === 'runExtensionRegisteredAction') {
    const extensionId = current.extensionId;
    const registeredActionId = current.registeredActionId || current.actionId;
    const key = `${extensionId}:${registeredActionId}`;
    if (visited.has(key)) return { error: 'Action reference cycle detected' };
    visited.add(key);
    const entry = extensionActionRegistry.get(key);
    const rootAction = entry ? extensionActionFromContribution(entry) : null;
    if (!rootAction) return { error: 'Action unavailable' };
    if (rootAction.rootAction?.type !== 'runExtensionRegisteredAction')
      return { rootAction };
    current = rootAction.rootAction;
  }
  return { error: 'Action unavailable' };
}

async function executeBuiltin(action) {
  return executeSystemBuiltin(action, () => requestQuitApp('builtin'));
}

async function thumbnailCachePath(filePath) {
  const stat = await fs.stat(filePath);
  const key = crypto
    .createHash('sha1')
    .update(`${filePath}:${stat.mtimeMs}:${stat.size}:${THUMBNAIL_SIZE}`)
    .digest('hex');
  return path.join(iconCacheDir, 'thumbs', `${key}.png`);
}

async function generateQuickLookThumbnail(filePath, cachedPath) {
  if (!hasCapability('quick-look')) return false;
  const outputDir = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nevermind-thumb-'),
  );
  try {
    await execFileText(
      'qlmanage',
      ['-t', '-s', String(THUMBNAIL_SIZE), '-o', outputDir, filePath],
      { timeout: 10_000 },
    );
    const generatedPath = path.join(
      outputDir,
      `${path.basename(filePath)}.png`,
    );
    await fs.mkdir(path.dirname(cachedPath), { recursive: true });
    await fs.copyFile(generatedPath, cachedPath);
    return true;
  } catch {
    return false;
  } finally {
    await fs.rm(outputDir, { recursive: true, force: true }).catch(() => {});
  }
}

async function generateThumbnail(filePath, cachedPath) {
  return measureDebugPerformance(
    'thumbnail.generate',
    { filePath, alwaysLog: true },
    async () => {
      let image = null;
      if (typeof nativeImage.createThumbnailFromPath === 'function') {
        image = await nativeImage.createThumbnailFromPath(filePath, {
          width: THUMBNAIL_SIZE,
          height: THUMBNAIL_SIZE,
        });
      }
      if (!image || image.isEmpty())
        image = nativeImage
          .createFromPath(filePath)
          .resize({ width: THUMBNAIL_SIZE, quality: 'good' });
      if (!image || image.isEmpty())
        return generateQuickLookThumbnail(filePath, cachedPath);
      const png = image.toPNG();
      await fs.mkdir(path.dirname(cachedPath), { recursive: true });
      await fs.writeFile(cachedPath, png).catch(() => {});
      return true;
    },
  );
}

async function processPendingThumbnails() {
  const entries = Array.from(pendingThumbnailPaths.entries()).slice(0, 4);
  for (const [filePath] of entries) pendingThumbnailPaths.delete(filePath);
  for (const [filePath, cachedPath] of entries) {
    await generateThumbnail(filePath, cachedPath).catch((error) => {
      logWarn(
        'thumbnail.generate.failed',
        { filePath, error },
        { source: 'host', scope: 'cache' },
      );
    });
  }
  if (pendingThumbnailPaths.size)
    jobRegistry.schedule('cache.thumbnails', 'thumbnail-backlog', 250);
}

async function thumbnailResponseForPath(filePath) {
  return measureDebugPerformance(
    'thumbnail.response',
    { filePath, alwaysLog: true },
    async () => {
      const cachedPath = await thumbnailCachePath(filePath);
      const cached = await fs.readFile(cachedPath).catch(() => null);
      if (cached) {
        markDebugPerformance('thumbnail.cache-hit', { filePath });
        return new Response(cached, {
          headers: {
            'content-type': 'image/png',
            'cache-control': 'public, max-age=31536000, immutable',
          },
        });
      }

      pendingThumbnailPaths.set(filePath, cachedPath);
      await jobRegistry
        .run('cache.thumbnails', 'thumbnail-request')
        .catch(() => {});
      const generated = await fs.readFile(cachedPath).catch(() => null);
      if (generated)
        return new Response(generated, {
          headers: {
            'content-type': 'image/png',
            'cache-control': 'public, max-age=31536000, immutable',
          },
        });
      return net.fetch(pathToFileURL(filePath).href);
    },
  );
}

async function localFileResponse(requestPath: string, request: Request) {
  const stat = await fs.stat(requestPath).catch(() => null);
  if (!stat?.isFile()) return new Response('File not found', { status: 404 });

  const headers = new Headers({
    'accept-ranges': 'bytes',
    'cache-control': 'private, max-age=3600',
    'content-type': mimeTypeForPath(requestPath),
  });
  const range = request.headers.get('range');
  let start = 0;
  let end = stat.size - 1;
  let status = 200;

  if (range) {
    const match = range.match(/^bytes=(\d*)-(\d*)$/);
    if (!(match && (match[1] || match[2])))
      return new Response('Invalid range', {
        status: 416,
        headers: { 'content-range': `bytes */${stat.size}` },
      });
    if (match[1]) {
      start = Number(match[1]);
      end = match[2] ? Number(match[2]) : stat.size - 1;
    } else {
      start = Math.max(0, stat.size - Number(match[2]));
      end = stat.size - 1;
    }
    if (
      Number.isNaN(start) ||
      Number.isNaN(end) ||
      start > end ||
      start >= stat.size
    )
      return new Response('Invalid range', {
        status: 416,
        headers: { 'content-range': `bytes */${stat.size}` },
      });
    end = Math.min(end, stat.size - 1);
    status = 206;
    headers.set('content-range', `bytes ${start}-${end}/${stat.size}`);
  }

  headers.set('content-length', String(Math.max(0, end - start + 1)));
  return new Response(
    Readable.toWeb(createReadStream(requestPath, { start, end })) as BodyInit,
    { status, headers },
  );
}

function registerLocalFileProtocol() {
  protocol.handle(LOCAL_FILE_PROTOCOL, (request) => {
    const url = new URL(request.url);
    const encodedPath = url.host ? `/${url.host}${url.pathname}` : url.pathname;
    const requestPath = path.resolve(decodeURIComponent(encodedPath));
    if (!path.isAbsolute(requestPath))
      return new Response('Invalid file path', { status: 400 });
    if (
      !verifyLocalFileToken('file', requestPath, url.searchParams.get('token'))
    )
      return new Response('Forbidden', { status: 403 });
    return localFileResponse(requestPath, request);
  });

  protocol.handle(LOCAL_THUMB_PROTOCOL, async (request) => {
    const url = new URL(request.url);
    const requestPath = path.resolve(
      decodeURIComponent(url.searchParams.get('path') || ''),
    );
    if (!path.isAbsolute(requestPath))
      return new Response('Invalid file path', { status: 400 });
    if (
      !verifyLocalFileToken('thumb', requestPath, url.searchParams.get('token'))
    )
      return new Response('Forbidden', { status: 403 });
    try {
      return await thumbnailResponseForPath(requestPath);
    } catch (error) {
      logError(
        'thumbnail.create.failed',
        { requestPath, error },
        { source: 'host', scope: 'thumbnail' },
      );
      return new Response('Thumbnail not found', { status: 404 });
    }
  });
}

function mimeTypeForPath(filePath) {
  const extension = extensionForPath(filePath);
  const types = {
    png: 'image/png',
    jpg: 'image/jpeg',
    jpeg: 'image/jpeg',
    gif: 'image/gif',
    webp: 'image/webp',
    bmp: 'image/bmp',
    tif: 'image/tiff',
    tiff: 'image/tiff',
    heic: 'image/heic',
    mp4: 'video/mp4',
    mov: 'video/quicktime',
    webm: 'video/webm',
    m4v: 'video/x-m4v',
    mkv: 'video/x-matroska',
    avi: 'video/x-msvideo',
    txt: 'text/plain',
    md: 'text/markdown',
    json: 'application/json',
    pdf: 'application/pdf',
    html: 'text/html',
    htm: 'text/html',
    csv: 'text/csv',
  };
  return types[extension] || 'application/octet-stream';
}

async function imageDimensionsForPath(filePath) {
  if (!isImagePath(filePath)) return {};
  const image = nativeImage.createFromPath(filePath);
  if (!image || image.isEmpty()) return {};
  const size = image.getSize();
  return size.width && size.height
    ? { width: size.width, height: size.height }
    : {};
}

function thumbnailUrlForPreviewablePath(filePath) {
  const expandedPath = expandUserPath(filePath);
  return isImagePath(expandedPath) || isVideoPath(expandedPath)
    ? thumbnailUrlForPath(expandedPath)
    : null;
}

function dataUrlExtension(dataUrl: string) {
  const mime = dataUrl.match(/^data:([^;,]+)/)?.[1] || 'image/png';
  if (mime.includes('jpeg')) return 'jpg';
  if (mime.includes('webp')) return 'webp';
  if (mime.includes('gif')) return 'gif';
  if (mime.includes('tiff')) return 'tiff';
  if (mime.includes('heic')) return 'heic';
  return 'png';
}

async function writeOcrDataUrl(dataUrl: string) {
  const match = dataUrl.match(/^data:([^;,]+)?(;base64)?,(.*)$/);
  if (!match) throw new Error('Invalid OCR data URL');
  const outputPath = path.join(
    os.tmpdir(),
    `nevermind-ocr-input-${crypto.randomUUID()}.${dataUrlExtension(dataUrl)}`,
  );
  const body = decodeURIComponent(match[3] || '');
  await fs.writeFile(
    outputPath,
    match[2] ? Buffer.from(match[3] || '', 'base64') : Buffer.from(body),
  );
  return outputPath;
}

function filePathFromLocalUrl(value: string) {
  if (value.startsWith(`${LOCAL_FILE_PROTOCOL}:`))
    return fileURLToPath(
      `file:${value.slice(`${LOCAL_FILE_PROTOCOL}:`.length)}`,
    );
  if (value.startsWith('file:')) return fileURLToPath(value);
  return null;
}

async function ocrInputPath(input: any) {
  const value =
    typeof input === 'string'
      ? input
      : input?.path || input?.filePath || input?.fileUrl || input?.url;
  if (!value)
    throw new Error(
      'OCR requires an image path, file URL, data URL, or ExtensionFile',
    );
  const text = String(value);
  if (text.startsWith('data:'))
    return { path: await writeOcrDataUrl(text), cleanup: true };
  const urlPath = filePathFromLocalUrl(text);
  return { path: expandUserPath(urlPath || text), cleanup: false };
}

async function ocrImage(input: any, options: any = {}) {
  const resolved = await ocrInputPath(input);
  try {
    if (!isImagePath(resolved.path))
      throw new Error('OCR currently supports image files only');
    return await recognizeTextInImage(resolved.path, options);
  } finally {
    if (resolved.cleanup)
      await fs.rm(resolved.path, { force: true }).catch(() => {});
  }
}

async function ocrScreen(options: any = {}) {
  const imagePath = await captureScreenImage(options);
  try {
    return await recognizeTextInImage(imagePath, options);
  } finally {
    await fs.rm(imagePath, { force: true }).catch(() => {});
  }
}

async function fileToExtensionFile(filePath, options: any = {}) {
  const expandedPath = expandUserPath(filePath);
  const stat =
    options.stat === undefined
      ? await fs.stat(expandedPath).catch(() => null)
      : options.stat;
  const dateAddedMs = Number(options.dateAddedMs || 0);
  const dimensions = options.includeDimensions
    ? await imageDimensionsForPath(expandedPath)
    : {};
  return {
    path: expandedPath,
    name: path.basename(expandedPath),
    displayPath: displayUserPath(expandedPath),
    url:
      thumbnailUrlForPreviewablePath(expandedPath) ||
      fileUrlForPath(expandedPath),
    fileUrl: fileUrlForPath(expandedPath),
    videoUrl: isVideoPath(expandedPath) ? fileUrlForPath(expandedPath) : null,
    thumbnailUrl: thumbnailUrlForPreviewablePath(expandedPath),
    kind: isImagePath(expandedPath)
      ? 'image'
      : isVideoPath(expandedPath)
        ? 'video'
        : 'file',
    extension: extensionForPath(expandedPath),
    mimeType: mimeTypeForPath(expandedPath),
    ...dimensions,
    mtime: stat ? new Date(stat.mtimeMs).toISOString() : null,
    mtimeMs: stat?.mtimeMs || 0,
    birthtime: stat ? new Date(stat.birthtimeMs).toISOString() : null,
    birthtimeMs: stat?.birthtimeMs || 0,
    dateAdded: dateAddedMs ? new Date(dateAddedMs).toISOString() : null,
    dateAddedMs,
    size: stat?.size || 0,
  };
}

function normalizeFindRoots(roots) {
  if (Array.isArray(roots)) return roots;
  if (typeof roots === 'string') return [roots];
  return [];
}

function extensionsForFindOptions(options: any = {}) {
  const kinds = Array.isArray(options.kind)
    ? options.kind
    : options.kind
      ? [options.kind]
      : [];
  const kindExtensions = kinds.flatMap((kind) => {
    if (kind === 'image') return Array.from(IMAGE_EXTENSIONS);
    if (kind === 'video') return Array.from(VIDEO_EXTENSIONS);
    if (kind === 'media') return [...IMAGE_EXTENSIONS, ...VIDEO_EXTENSIONS];
    return [];
  });
  const patterns = Array.isArray(options.patterns)
    ? options.patterns
    : options.pattern
      ? [options.pattern]
      : [];
  const patternExtensions = patterns
    .map((pattern) => String(pattern).match(/\.([a-z0-9]+)$/i)?.[1])
    .filter(Boolean);
  const requestedExtensions = [
    ...(options.extensions || []),
    ...kindExtensions,
    ...patternExtensions,
  ];
  return requestedExtensions.length
    ? new Set(
        requestedExtensions.map((ext) =>
          String(ext).toLowerCase().replace(/^\./, ''),
        ),
      )
    : null;
}

function fileCandidate(fullPath, name = path.basename(fullPath)) {
  return {
    path: fullPath,
    name,
    displayPath: displayUserPath(fullPath),
    extension: extensionForPath(name),
    kind: isImagePath(fullPath)
      ? 'image'
      : isVideoPath(fullPath)
        ? 'video'
        : 'file',
  };
}

async function eachChunk(items, size, fn) {
  for (let index = 0; index < items.length; index += size)
    await Promise.all(items.slice(index, index + size).map(fn));
}

async function attachFileStats(files) {
  await eachChunk(files, 100, async (file: any) => {
    const stat = await fs.stat(file.path).catch(() => null);
    file.stat = stat;
    file.mtimeMs = stat?.mtimeMs || 0;
    file.birthtimeMs = stat?.birthtimeMs || 0;
    file.size = stat?.size || 0;
  });
  return files.filter((file) => file.stat);
}

async function attachDateAdded(files) {
  return applyDateAdded(
    files,
    await fileDateAddedMs(files.map((file) => file.path)),
  );
}

async function findFiles(roots, options: any = {}) {
  const limit = Math.max(
    1,
    Math.min(Number(options.limit || 100), MAX_FILE_INDEX_LIMIT),
  );
  const scanLimit = Math.max(
    limit,
    Math.min(
      Number(options.scanLimit || MAX_FILE_INDEX_LIMIT),
      MAX_FILE_INDEX_LIMIT,
    ),
  );
  const maxDepth = options.depth ?? 2;
  const extensions = extensionsForFindOptions(options);
  const ignored = normalizedIgnorePatterns(options.ignore);
  const includeHidden = Boolean(options.includeHidden);
  const candidates: any[] = [];

  async function walk(dir, depth) {
    if (candidates.length >= scanLimit) return;
    let entries = [];
    try {
      entries = await fs.readdir(dir, { withFileTypes: true });
    } catch {
      return;
    }

    for (const entry of entries) {
      if (candidates.length >= scanLimit) return;
      if (!includeHidden && entry.name.startsWith('.')) continue;
      const fullPath = path.join(dir, entry.name);
      if (ignoredByPattern(fullPath, entry.name, ignored)) continue;
      if (entry.isFile()) {
        const ext = extensionForPath(entry.name);
        if (!extensions || extensions.has(ext))
          candidates.push(fileCandidate(fullPath, entry.name));
        continue;
      }
      if (entry.isDirectory() && depth > 0) await walk(fullPath, depth - 1);
    }
  }

  const findRoots = normalizeFindRoots(roots)
    .map(expandUserPath)
    .filter((root) => root && path.isAbsolute(root));

  const { existing, missing } = await partitionRootsByExistence(findRoots);
  for (const root of missing)
    logWarn(
      'files.find.missingRoot',
      { root: displayUserPath(root) },
      { source: 'host', scope: 'files' },
    );

  await Promise.all(existing.map((root) => walk(root, maxDepth)));
  const sortBy = options.sortBy || options.sort || null;
  let found = candidates;
  if (findFilesNeedsStats(sortBy)) found = await attachFileStats(found);
  if (sortBy === 'added') found = await attachDateAdded(found);
  const selected = selectFindFiles(found, options, limit);
  return Promise.all(
    selected.map((file) =>
      fileToExtensionFile(file.path, {
        includeDimensions: includeDimensionsForFindOptions(options),
        stat: file.stat,
        dateAddedMs: file.dateAddedMs,
      }),
    ),
  );
}

function dragIconForPath(filePath) {
  const image = nativeImage.createFromPath(filePath);
  if (!image.isEmpty())
    return image.resize({ width: 64, height: 64, quality: 'good' });
  return nativeImage.createFromDataURL(
    'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=',
  );
}

function startFileDrag(event, filePath) {
  const resolvedPath = expandUserPath(filePath);
  if (!(resolvedPath && path.isAbsolute(resolvedPath))) return;
  event.sender.startDrag({
    file: resolvedPath,
    icon: dragIconForPath(resolvedPath),
  });
}

function quickLookPath(filePath) {
  if (!hasCapability('quick-look'))
    return {
      toast: {
        message: `${quickLookTitle()} is not available on this OS`,
        tone: 'error',
      },
    };
  const resolvedPath = expandUserPath(filePath);
  if (!(resolvedPath && path.isAbsolute(resolvedPath)))
    return {
      toast: { message: `Cannot ${quickLookTitle()} this item`, tone: 'error' },
    };
  const child = spawn('qlmanage', ['-p', resolvedPath], {
    detached: true,
    stdio: 'ignore',
  });
  child.on('error', (err) =>
    logWarn(
      'quickLook.failed',
      { path: resolvedPath, error: err?.message },
      { source: 'host', scope: 'action' },
    ),
  );
  child.unref();
}

function execFileText(
  command: string,
  args: string[] = [],
  options = {},
): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(command, args, options, (error, stdout) =>
      error ? reject(error) : resolve(stdout),
    );
  });
}

async function contentTypesForPath(filePath) {
  if (!hasCapability('quick-look')) return [];
  try {
    const stdout = (await execFileText('mdls', [
      '-raw',
      '-name',
      'kMDItemContentTypeTree',
      filePath,
    ])) as string;
    return stdout.match(/"([^"]+)"/g)?.map((item) => item.slice(1, -1)) || [];
  } catch {
    return [];
  }
}

async function documentTypesForApp(appPath) {
  try {
    const stdout = (await execFileText('/usr/bin/plutil', [
      '-convert',
      'json',
      '-o',
      '-',
      path.join(appPath, 'Contents', 'Info.plist'),
    ])) as string;
    return JSON.parse(stdout).CFBundleDocumentTypes || [];
  } catch {
    return [];
  }
}

async function openWithApps(filePath) {
  const resolvedPath = expandUserPath(filePath);
  if (!(resolvedPath && path.isAbsolute(resolvedPath))) return [];
  if (!hasCapability('open-with')) return appIndexService.get();
  const extension = path.extname(resolvedPath).replace(/^\./, '').toLowerCase();
  const contentTypes = new Set(await contentTypesForPath(resolvedPath));
  const scored = [];
  await Promise.all(
    appIndexService.get().map(async (item) => {
      if (!item.path?.endsWith('.app')) return;
      const documentTypes = await documentTypesForApp(item.path);
      let score = 0;
      for (const type of documentTypes) {
        const extensions = (type.CFBundleTypeExtensions || []).map((value) =>
          String(value).toLowerCase(),
        );
        const itemTypes = type.LSItemContentTypes || [];
        if (extension && extensions.includes(extension))
          score = Math.max(score, 3);
        if (itemTypes.some((itemType) => contentTypes.has(itemType)))
          score = Math.max(score, 2);
        if (
          extensions.includes('*') ||
          itemTypes.includes('public.data') ||
          itemTypes.includes('public.item')
        )
          score = Math.max(score, 1);
      }
      if (score) scored.push({ ...item, score });
    }),
  );
  return scored
    .sort((a, b) => b.score - a.score || a.name.localeCompare(b.name))
    .map(({ score, ...item }) => item);
}

async function openPathWithApp(filePath, appPath) {
  const resolvedPath = expandUserPath(filePath);
  const resolvedAppPath = expandUserPath(appPath);
  if (
    !(
      resolvedPath &&
      resolvedAppPath &&
      path.isAbsolute(resolvedPath) &&
      path.isAbsolute(resolvedAppPath)
    )
  )
    return {
      toast: { message: 'Cannot open this file with that app', tone: 'error' },
    };
  if (hasCapability('open-with')) {
    const child = spawn('open', ['-a', resolvedAppPath, resolvedPath], {
      detached: true,
      stdio: 'ignore',
    });
    child.on('error', (err) =>
      logWarn(
        'openWith.failed',
        { app: resolvedAppPath, file: resolvedPath, error: err?.message },
        { source: 'host', scope: 'action' },
      ),
    );
    child.unref();
  } else await shell.openPath(resolvedPath);
}

async function selectedFiles() {
  return selectedFilePaths();
}

async function selectedExtensionFiles() {
  const paths = await selectedFilePaths();
  return Promise.all(paths.map(fileToExtensionFile));
}

async function clipboardFiles() {
  return clipboardService!.clipboardFiles();
}

function clipboardImageDataUrl() {
  return clipboardService!.clipboardImageDataUrl();
}

function clipboardFormats(options: any = {}) {
  return clipboardService!.clipboardFormats(options);
}

async function readDesktopClipboard(options: any = {}) {
  return clipboardService!.readDesktopClipboard(options);
}

function clipboardImageForContent(item: any) {
  return clipboardService!.clipboardImageForContent(item);
}

function suppressClipboardHistoryForContent(item: any) {
  clipboardService!.suppressClipboardHistoryForContent(item);
}

function writeDesktopClipboardFiles(paths) {
  clipboardService!.writeDesktopClipboardFiles(paths);
}

function writeDesktopClipboard(item, options: any = {}) {
  return clipboardService!.writeDesktopClipboard(item, options);
}

function pasteClipboardAction(action: any) {
  clipboardService!.pasteClipboardAction(action);
}

async function readDesktopSelection() {
  const [text, files, app] = await Promise.all([
    selectedText(),
    selectedExtensionFiles(),
    frontmostApp(),
  ]);
  return { text, files, sourceApp: app };
}

function extensionSourceBasename(filePath) {
  const base = path.basename(filePath || '');
  if (base.endsWith('.d.ts')) return base.slice(0, -5);
  return base.replace(/\.(cjs|ts)$/i, '');
}

function isExtensionSourceFile(filename) {
  return (
    typeof filename === 'string' &&
    filename.endsWith('.ts') &&
    !filename.endsWith('.d.ts')
  );
}

function safeExtensionStorageKey(extension) {
  const key = extension.__filePath
    ? extensionSourceBasename(extension.__filePath)
    : extension.id || 'extension';
  return String(key).replace(/[^a-zA-Z0-9._-]/g, '-');
}

function extensionStoragePath(extension) {
  return path.join(
    extensionStorageDir,
    `${safeExtensionStorageKey(extension)}.json`,
  );
}

function extensionCachePath(extension) {
  return path.join(
    extensionCacheDir,
    `${safeExtensionStorageKey(extension)}.json`,
  );
}

async function readExtensionStorage(extension) {
  return extensionJsonStore.read(extensionStoragePath(extension));
}

async function mutateExtensionStorage(extension, update) {
  return extensionJsonStore.mutate(extensionStoragePath(extension), update);
}

async function readExtensionCache(extension) {
  return extensionJsonStore.read(extensionCachePath(extension));
}

async function mutateExtensionCache(extension, update) {
  return extensionJsonStore.mutate(extensionCachePath(extension), update);
}

function limitedOutput(value, limit = 200_000) {
  const text = String(value || '');
  return text.length > limit
    ? `${text.slice(0, limit)}\n... output truncated ...`
    : text;
}

function createExtensionStorage(extension) {
  return createPersistentExtensionStorage({
    storagePath: extensionStoragePath(extension),
    cachePath: extensionCachePath(extension),
    store: extensionJsonStore,
    refreshes: extensionStorageRefreshes,
  });
}

const EXTENSION_AI_ATTACHMENT_LIMIT = 8;
const EXTENSION_AI_TEXT_ATTACHMENT_LIMIT = 80_000;
const EXTENSION_AI_IMAGE_ATTACHMENT_MAX_BYTES = 8 * 1024 * 1024;

function extensionAiDataUrlImage(dataUrl) {
  const match = String(dataUrl || '').match(/^data:([^;,]+);base64,(.*)$/s);
  if (!match) throw new Error('AI image attachment must be a base64 data URL');
  return { type: 'image' as const, mimeType: match[1], data: match[2] };
}

async function extensionAiPathImage(filePath) {
  const resolvedPath = expandUserPath(filePath);
  const stat = await fs.stat(resolvedPath);
  if (stat.size > EXTENSION_AI_IMAGE_ATTACHMENT_MAX_BYTES)
    throw new Error(
      `AI image attachment is too large: ${displayUserPath(resolvedPath)}`,
    );
  return {
    type: 'image' as const,
    mimeType: mimeTypeForPath(resolvedPath) || 'image/png',
    data: (await fs.readFile(resolvedPath)).toString('base64'),
  };
}

function isTextLikeAttachmentPath(filePath) {
  const mime = mimeTypeForPath(filePath);
  const ext = extensionForPath(filePath);
  return (
    mime.startsWith('text/') ||
    [
      'md',
      'markdown',
      'txt',
      'json',
      'csv',
      'tsv',
      'xml',
      'html',
      'css',
      'js',
      'ts',
      'tsx',
      'jsx',
      'yml',
      'yaml',
      'log',
    ].includes(ext)
  );
}

async function extensionAiFileContext(filePath, options: any = {}) {
  const resolvedPath = expandUserPath(filePath);
  const stat = await fs.stat(resolvedPath).catch(() => null);
  const title = options.title || path.basename(resolvedPath);
  if (!stat)
    return `### ${title}\nMissing file: ${displayUserPath(resolvedPath)}`;
  const metadata = await fileToExtensionFile(resolvedPath);
  const header = `### ${title}\nPath: ${metadata.displayPath}\nMIME: ${metadata.mimeType || 'unknown'}\nSize: ${metadata.size} bytes`;
  if (options.as === 'metadata') return header;
  if (!isTextLikeAttachmentPath(resolvedPath)) return header;
  const text = await fs.readFile(resolvedPath, 'utf8');
  return `${header}\n\n${limitedOutput(text, EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`;
}

async function resolveExtensionAiAttachmentList(input) {
  const output: any[] = [];
  async function visit(value) {
    const resolved = await value;
    if (resolved == null || resolved === false) return;
    if (Array.isArray(resolved)) {
      for (const item of resolved) await visit(item);
      return;
    }
    output.push(resolved);
  }
  await visit(input);
  return output.slice(0, EXTENSION_AI_ATTACHMENT_LIMIT);
}

function extensionAiOcrText(result) {
  if (typeof result === 'string') return result;
  if (Array.isArray(result?.blocks))
    return result.blocks
      .map((block) => block.text)
      .filter(Boolean)
      .join('\n');
  if (Array.isArray(result?.observations))
    return result.observations
      .map((item) => item.text || item.transcript)
      .filter(Boolean)
      .join('\n');
  return result?.text || result?.transcript || JSON.stringify(result);
}

async function normalizeExtensionAiAttachments(
  extension,
  attachments,
  capabilities,
) {
  const textSections: string[] = [];
  const images: Array<{ type: 'image'; data: string; mimeType: string }> = [];
  for (const attachment of await resolveExtensionAiAttachmentList(
    attachments || [],
  )) {
    if (typeof attachment === 'string') {
      textSections.push(
        limitedOutput(attachment, EXTENSION_AI_TEXT_ATTACHMENT_LIMIT),
      );
      continue;
    }
    const type =
      attachment?.type ||
      (attachment?.text == null
        ? attachment?.path || attachment?.file
          ? 'file'
          : attachment?.dataUrl || attachment?.imageDataUrl
            ? 'image'
            : ''
        : 'text');
    if (type === 'text') {
      textSections.push(
        `${attachment.title ? `### ${attachment.title}\n` : ''}${limitedOutput(attachment.text || '', EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`,
      );
      continue;
    }
    if (type === 'image') {
      const source =
        attachment.dataUrl ||
        attachment.imageDataUrl ||
        attachment.data ||
        attachment.path ||
        attachment.file?.path ||
        attachment.filePath;
      if (
        attachment.data &&
        attachment.mimeType &&
        !String(source || '').startsWith('data:')
      )
        images.push({
          type: 'image' as const,
          data: String(attachment.data),
          mimeType: String(attachment.mimeType),
        });
      else if (String(source || '').startsWith('data:'))
        images.push(extensionAiDataUrlImage(source));
      else {
        if (!capabilities.files)
          throw trustedExtensionApiUnavailable('desktop.files');
        images.push(await extensionAiPathImage(source));
      }
      if (attachment.ocr) {
        if (!capabilities.ocr) throw trustedExtensionApiUnavailable('ocr');
        textSections.push(
          `### ${attachment.title || 'OCR'}\n${limitedOutput(extensionAiOcrText(await ocrImage(source)), EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`,
        );
      }
      continue;
    }
    if (type === 'file') {
      if (!capabilities.files)
        throw trustedExtensionApiUnavailable('desktop.files');
      const filePath =
        attachment.path || attachment.file?.path || attachment.filePath;
      if (!filePath) continue;
      const resolvedPath = expandUserPath(filePath);
      const shouldAttachImage =
        (attachment.as === 'image' ||
          (!attachment.as && isImagePath(resolvedPath))) &&
        attachment.as !== 'text';
      if (shouldAttachImage)
        images.push(await extensionAiPathImage(resolvedPath));
      if (
        !shouldAttachImage ||
        attachment.as === 'text' ||
        attachment.as === 'metadata'
      )
        textSections.push(
          await extensionAiFileContext(resolvedPath, attachment),
        );
      if (attachment.ocr || attachment.as === 'ocr') {
        if (!capabilities.ocr) throw trustedExtensionApiUnavailable('ocr');
        textSections.push(
          `### ${attachment.title || `OCR ${path.basename(resolvedPath)}`}\n${limitedOutput(extensionAiOcrText(await ocrImage(resolvedPath)), EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`,
        );
      }
    }
  }
  return { context: textSections.filter(Boolean).join('\n\n'), images };
}

function normalizeExtensionAiModelRole(value) {
  if (value == null || value === '') return;
  if (value === 'smart' || value === 'fast') return value;
  throw new Error(
    `Unsupported AI model role: ${value}. Use 'smart' or 'fast'.`,
  );
}

function normalizeExtensionAiCallOptions(input: any = {}) {
  if (typeof input === 'string')
    return { model: normalizeExtensionAiModelRole(input) };
  const output = { ...(input || {}) };
  const model = normalizeExtensionAiModelRole(input?.model);
  if (model) output.model = model;
  else delete output.model;
  return output;
}

function createExtensionAi(extension) {
  const extensionKey = path
    .basename(extension.__filePath || extension.id || 'extension')
    .replace(/[^a-zA-Z0-9._-]/g, '-');
  // Trusted local extensions are not sandboxed by their declarations.
  const capabilities = { files: true, clipboard: true, ocr: true };
  const enforceAiQuota = () => {
    if (!checkAiRateLimit(extension))
      throw Object.assign(new Error('AI rate limit exceeded'), {
        code: 'ai-rate-limit-exceeded',
        extensionId: extension?.id,
      });
  };
  const normalizeOptions = async (options: any = {}) => {
    const callOptions = normalizeExtensionAiCallOptions(options);
    const normalized = await normalizeExtensionAiAttachments(
      extension,
      callOptions.attachments || [],
      capabilities,
    );
    return {
      model: callOptions.model,
      system: callOptions.system,
      signal: callOptions.signal,
      context: normalized.context,
      images: normalized.images,
      onEvent: (event) => {
        if (event.type === 'delta' && event.text)
          callOptions.onDelta?.(event.text);
        callOptions.onEvent?.(event);
      },
    };
  };
  const stream = (prompt, options: any = {}, session: any = nevermindAi) => {
    enforceAiQuota();
    const callOptions = normalizeExtensionAiCallOptions(options);
    const controller = new AbortController();
    const removeExternalAbortListener = callOptions.signal?.addEventListener
      ? (() => {
          const listener = () => controller.abort();
          callOptions.signal.addEventListener('abort', listener, {
            once: true,
          });
          return () =>
            callOptions.signal.removeEventListener?.('abort', listener);
        })()
      : () => {};
    let inner: any = null;
    const result = (async () => {
      const normalized = await normalizeOptions({
        ...callOptions,
        signal: controller.signal,
      });
      inner = session.stream(String(prompt || ''), normalized);
      return inner.result;
    })().finally(removeExternalAbortListener);
    return {
      result,
      abort: () => {
        controller.abort();
        inner?.abort?.();
      },
    };
  };
  const ai: any = async (prompt, options: any = {}) => ai.ask(prompt, options);
  ai.ask = async (prompt, options: any = {}) => {
    enforceAiQuota();
    return nevermindAi.ask(
      String(prompt || ''),
      await normalizeOptions(options),
    );
  };
  ai.stream = stream;
  ai.session = (id = 'default', options: any = {}) => {
    const sessionOptions = normalizeExtensionAiCallOptions(options);
    const session = nevermindAi.session(
      `${extensionKey}:${String(id || 'default')}`,
      { system: sessionOptions.system, model: sessionOptions.model },
    );
    return {
      ...session,
      ask: async (prompt: any, askOptions: any = {}) => {
        enforceAiQuota();
        return session.ask(
          prompt,
          await normalizeOptions({
            ...sessionOptions,
            ...normalizeExtensionAiCallOptions(askOptions),
          }),
        );
      },
      stream: (prompt: any, streamOptions: any = {}) =>
        stream(
          prompt,
          {
            ...sessionOptions,
            ...normalizeExtensionAiCallOptions(streamOptions),
          },
          session,
        ),
    };
  };
  ai.attachments = {
    text: (text, title) => ({ type: 'text', text: String(text || ''), title }),
    image: (input, options: any = {}) => ({
      ...options,
      type: 'image',
      ...(String(input || '').startsWith('data:')
        ? { dataUrl: input }
        : { path: input }),
    }),
    file: (input, options: any = {}) => ({
      ...options,
      type: 'file',
      path: typeof input === 'string' ? input : input?.path || input?.filePath,
    }),
    selectedText: async (title = 'Selected Text') => ({
      type: 'text',
      title,
      text: await selectedText(),
    }),
    selectedFiles: async (options: any = {}) => {
      if (!capabilities.files)
        throw trustedExtensionApiUnavailable('desktop.files');
      return (await selectedExtensionFiles()).map((file) => ({
        ...options,
        type: 'file',
        file,
      }));
    },
    clipboard: async (options: any = {}) => {
      if (!capabilities.clipboard)
        throw trustedExtensionApiUnavailable('clipboard.history');
      const item: any = await readDesktopClipboard();
      if (item.type === 'text')
        return {
          type: 'text',
          title: options.title || 'Clipboard',
          text: item.text,
        };
      if (item.type === 'image')
        return {
          ...options,
          type: 'image',
          title: options.title || 'Clipboard Image',
          dataUrl: item.image,
        };
      if (item.type === 'files')
        return item.files.map((file) => ({ ...options, type: 'file', file }));
      return null;
    },
    ocrImage: async (input, options: any = {}) => {
      if (!capabilities.ocr) throw trustedExtensionApiUnavailable('ocr');
      return {
        type: 'text',
        title: options.title || 'OCR Text',
        text: extensionAiOcrText(await ocrImage(input, options)),
      };
    },
  };
  return ai;
}

function commandFromItem(item) {
  return { ...item, run: (ctx) => ctx.navigation.run(item.primaryAction) };
}

function fileIndexSnapshot(options: any = {}) {
  return measureDebugPerformanceSync(
    'files.index-snapshot',
    {
      queryLength: String(options.query || '').length,
      indexedCount: fileIndex.length,
      limit: options.limit,
    },
    () => {
      const { limit, query } = options;
      const roots = normalizeFindRoots(options.roots)
        .map(expandUserPath)
        .filter(Boolean);
      const extensions = extensionsForFindOptions(options);
      const ignored = normalizedIgnorePatterns(options.ignore);
      let entries = fileIndex;
      if (roots.length)
        entries = entries.filter((entry) =>
          roots.some(
            (root) =>
              entry.path === root ||
              entry.path.startsWith(`${root}${path.sep}`),
          ),
        );
      if (extensions)
        entries = entries.filter((entry) =>
          extensions.has(entry.extension || extensionForPath(entry.path)),
        );
      if (options.ignore)
        entries = entries.filter(
          (entry) => !ignoredByPattern(entry.path, entry.name, ignored),
        );
      if (query) {
        const needle = String(query).toLowerCase();
        entries = entries.filter((entry) =>
          `${entry.name || ''} ${entry.displayPath || ''}`
            .toLowerCase()
            .includes(needle),
        );
      }
      entries = sortFoundFiles(entries, {
        sortBy: options.sortBy || options.sort || 'added',
        order: options.order,
      });
      const max =
        typeof limit === 'number'
          ? Math.max(0, Math.min(limit, entries.length))
          : entries.length;
      return entries.slice(0, max).map((entry) => ({
        id: entry.id,
        name: entry.name,
        path: entry.path,
        displayPath: entry.displayPath,
        extension: entry.extension,
        kind: entry.kind,
        url: entry.url,
        fileUrl: entry.fileUrl,
        videoUrl: entry.videoUrl,
        thumbnailUrl: entry.thumbnailUrl,
        mtimeMs: entry.mtimeMs,
        birthtimeMs: entry.birthtimeMs,
        dateAddedMs: entry.dateAddedMs,
        size: entry.size,
      }));
    },
  );
}

async function reindexFiles(options: any = {}) {
  return measureDebugPerformance(
    'files.reindex',
    {
      roots: normalizedIndexRoots(options).map(displayUserPath),
      alwaysLog: true,
    },
    async () => {
      fileIndex = await scanFiles(options);
      paletteWindow.win?.webContents.send('root-items:changed');
      return {
        count: fileIndex.length,
        roots: normalizedIndexRoots(options).map(displayUserPath),
      };
    },
  );
}

function keyboardShortcutItem(record: any) {
  const changeAction = buildRecordShortcutAction(
    { actionId: record.actionId, title: 'Change shortcut' },
    {},
  );
  const removeAction =
    record.source === 'user'
      ? buildRemoveShortcutAction(
          { actionId: record.actionId, title: 'Remove shortcut' },
          {},
        )
      : null;
  return {
    id: `shortcut:${record.actionId}`,
    title: record.title,
    subtitle: record.subtitle,
    shortcut: record.accelerator,
    icon: 'keyboard',
    primaryAction: changeAction,
    actionPanel: {
      sections: [{ actions: [changeAction, removeAction].filter(Boolean) }],
    },
  };
}

function keyboardShortcutItems() {
  return extensionShortcutRecords().map(keyboardShortcutItem);
}

function patchKeyboardShortcutsView() {
  patchOpenView('keyboard-shortcuts', {
    mode: 'replace',
    items: keyboardShortcutItems(),
  });
}

const PALETTE_HOTKEY_ACTION_ID = '__palette-hotkey__';

function resolveShortcutTargetAction(input: any) {
  if (input?.action) return input.action;
  const actionId = input?.actionId;
  if (!actionId) return null;
  if (actionId === PALETTE_HOTKEY_ACTION_ID || input?.scope === 'palette')
    return { id: PALETTE_HOTKEY_ACTION_ID };
  const record = getShortcuts().find((item) => item.actionId === actionId);
  return record?.action || { id: actionId };
}

function buildRecordShortcutAction(input: any, options: any) {
  const scope =
    input?.scope === 'palette' || input?.actionId === PALETTE_HOTKEY_ACTION_ID
      ? 'palette'
      : 'action';
  const targetAction =
    scope === 'palette'
      ? { id: PALETTE_HOTKEY_ACTION_ID }
      : resolveShortcutTargetAction(input);
  const title =
    input?.title ||
    options?.title ||
    (scope === 'palette' ? 'Change Shortcut' : 'Record shortcut');
  const shortcut =
    input?.shortcut === undefined ? undefined : String(input.shortcut);
  return {
    ...options,
    type: 'recordShortcut',
    title,
    action: targetAction,
    ...(shortcut === undefined ? {} : { shortcut }),
  };
}

function buildRemoveShortcutAction(input: any, options: any) {
  const actionId = input?.actionId || input?.action?.id;
  const title = input?.title || options?.title || 'Remove shortcut';
  return {
    style: 'destructive',
    ...options,
    type: 'removeShortcut',
    title,
    actionId,
  };
}

function wrapWithConfirmation(action: any, input: any) {
  if (!action) return action;
  const destructive = input?.destructive ?? action.style === 'destructive';
  return {
    ...action,
    title: input?.title || action.title,
    requiresConfirmation: true,
    ...(destructive ? { style: 'destructive' } : {}),
    ...(input?.message === undefined
      ? {}
      : { confirmMessage: String(input.message) }),
    ...(input?.confirmLabel === undefined
      ? {}
      : { confirmLabel: String(input.confirmLabel) }),
    ...(input?.cancelLabel === undefined
      ? {}
      : { cancelLabel: String(input.cancelLabel) }),
  };
}

function buildPromptAction(input: any = {}, options: any = {}) {
  const targetAction = input?.action || input?.onSubmit;
  if (!targetAction) throw new Error('ctx.input.prompt requires action');
  const title = String(input?.title || targetAction.title || 'Prompt');
  return {
    ...options,
    type: 'promptAction',
    title,
    subtitle:
      input?.message === undefined ? options.subtitle : String(input.message),
    promptMessage:
      input?.message === undefined ? undefined : String(input.message),
    fields: Array.isArray(input?.fields) ? input.fields : [],
    targetAction,
    submitTitle:
      input?.submitTitle === undefined ? undefined : String(input.submitTitle),
  };
}

function promptActionView(action: any = {}) {
  const targetAction = action.targetAction || action.action;
  if (!targetAction)
    throw new Error('Prompt action is missing its target action');
  const fields = Array.isArray(action.fields) ? action.fields : [];
  const promptMessage = action.promptMessage || action.subtitle;
  return {
    type: 'form',
    title: action.title || 'Prompt',
    fields: promptMessage
      ? [
          {
            id: '__prompt_message',
            type: 'description',
            description: String(promptMessage),
          },
          ...fields,
        ]
      : fields,
    submitAction: {
      ...targetAction,
      title: action.submitTitle || targetAction.title || 'Submit',
    },
  };
}

function buildConfirmAction(input: any) {
  const inner = input?.onConfirm || input?.action;
  if (!inner) throw new Error('ctx.ui.confirm requires onConfirm action');
  return wrapWithConfirmation(inner, input);
}

function buildPreviewItemAction(input: any) {
  const kind =
    input?.kind ||
    (input?.text
      ? 'text'
      : input?.imageDataUrl || input?.imagePath
        ? 'image'
        : input?.videoUrl
          ? 'video'
          : input?.filePath
            ? 'file'
            : 'clipboard');
  return {
    type: 'previewClipboardItem',
    title: input?.title || 'Preview',
    description:
      input?.description ||
      (kind === 'file' || kind === 'image' || kind === 'video'
        ? 'Preview this file'
        : 'Preview clipboard item'),
    shortcut: input?.shortcut || 'Command+Y',
    clipboardType: input?.clipboardType || kind,
    text: input?.text,
    imageDataUrl: input?.imageDataUrl,
    imagePath: input?.imagePath,
    videoUrl: input?.videoUrl,
    filePath: input?.filePath,
    thumbnailUrl: input?.thumbnailUrl,
  };
}

function progressView(input: any = {}) {
  const hasSteps = Array.isArray(input.steps) && input.steps.length;
  const steps = hasSteps
    ? input.steps
    : [
        {
          title: input.label || input.title || 'Loading...',
          status: input.status || 'active',
        },
      ];
  return {
    ...input,
    type: 'progress',
    title: input.title || input.label || 'Loading...',
    steps,
    ...(input.id === undefined ? {} : { id: String(input.id) }),
    ...(typeof input.value === 'number' ? { value: input.value } : {}),
    ...(typeof input.total === 'number' ? { total: input.total } : {}),
  };
}

async function safeSelectedText() {
  try {
    return String(await selectedText());
  } catch {
    return '';
  }
}

function templateDateParts(now = new Date()) {
  return {
    date: now.toLocaleDateString(),
    time: now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    datetime: now.toLocaleString(),
  };
}

function normalizeTemplateOptions(input: any = {}) {
  const looksLikeOptions =
    input &&
    typeof input === 'object' &&
    !Array.isArray(input) &&
    [
      'variables',
      'cursorToken',
      'returnCursor',
      'returnResult',
      'includeClipboard',
      'includeSelectedText',
      'promptMissing',
    ].some((key) => key in input);
  return looksLikeOptions
    ? { ...input, variables: input.variables || {} }
    : { variables: input || {} };
}

async function expandTextTemplate(
  input: string,
  variablesOrOptions: Record<string, unknown> = {},
  hostOptions: { includeClipboard?: boolean } = {},
) {
  const options: any = normalizeTemplateOptions(variablesOrOptions);
  const cursorToken = String(
    options.cursorToken || '\uE000NEVERMIND_CURSOR\uE000',
  );
  const missingVariables = new Set<string>();
  const builtins = {
    ...templateDateParts(),
    uuid: crypto.randomUUID(),
    clipboard:
      hostOptions.includeClipboard && options.includeClipboard !== false
        ? clipboard.readText()
        : '',
    selectedText:
      options.includeSelectedText === false ? '' : await safeSelectedText(),
    cursor: cursorToken,
  };
  const values = { ...builtins, ...(options.variables || {}) };
  const textWithCursor = String(input || '').replace(
    /\{\{\s*([^{}]+?)\s*\}\}|\{\s*([^{}]+?)\s*\}/g,
    (_match, doubleName, singleName) => {
      const rawName = String(doubleName || singleName || '').trim();
      if (rawName.startsWith('calculator:')) {
        const result = calculate(rawName.slice('calculator:'.length).trim());
        return result == null ? '' : String(result);
      }
      if (rawName.startsWith('argument:')) {
        const argumentName = rawName.slice('argument:'.length).trim();
        const value = values[argumentName];
        if (value == null) missingVariables.add(argumentName);
        return value == null ? '' : String(value);
      }
      const value = values[rawName];
      if (value == null) missingVariables.add(rawName);
      return value == null ? '' : String(value);
    },
  );
  const cursor = textWithCursor.indexOf(cursorToken);
  const text =
    cursor >= 0 ? textWithCursor.replace(cursorToken, '') : textWithCursor;
  if (options.returnCursor || options.returnResult || options.promptMissing)
    return {
      text,
      cursor: cursor >= 0 ? cursor : undefined,
      missingVariables: Array.from(missingVariables),
    };
  return text;
}

function createExtensionContext(extension, command, launchContext?: any) {
  // The enable decision is the trust boundary. Manifest capabilities are review
  // declarations, not runtime privileges.
  const canUseDesktopApps = true;
  const canUseDesktopFiles = true;
  const canUseClipboard = true;
  const canUseSystem = true;
  const canUseOcr = true;
  const canUseUpdates = true;
  const canUseShortcuts = true;
  const canUseAi = true;
  const canWriteSettings = true;
  const canManageExtensionOwnership = true;
  const denyShortcut = (name: string) => () => {
    throw trustedExtensionApiUnavailable(`shortcuts (${name})`);
  };
  const context = {
    extension: createExtensionRuntimeMetadata(extension, command),
    command,
    launch: launchContext ? structuredClone(launchContext) : undefined,
    ui: createExtensionUiApi({
      buildPreviewItemAction,
      progressView,
      buildConfirmAction,
    }),
    actions: {
      openPath: (filePath, title = 'Open', options: any = {}) => ({
        dismissAfterRun: 'auto',
        ...options,
        type: 'openPath',
        title,
        path: filePath,
      }),
      revealPath: (filePath, title = revealPathTitle(), options: any = {}) => ({
        dismissAfterRun: 'auto',
        ...options,
        type: 'revealPath',
        title,
        path: filePath,
      }),
      quickLook: (filePath, title = quickLookTitle(), options: any = {}) => ({
        ...options,
        type: 'quickLook',
        title,
        path: filePath,
      }),
      openWith: (filePath, app, title, options: any = {}) => ({
        dismissAfterRun: 'auto',
        ...options,
        type: 'openWith',
        title: title || `Open with ${app?.name || 'App'}`,
        path: filePath,
        app,
        appPath: app?.path || app,
      }),
      openUrl: (url, title = 'Open URL', options: any = {}) => ({
        dismissAfterRun: 'auto',
        ...options,
        type: 'openUrl',
        title,
        url,
      }),
      copyText: (text, title = 'Copy', options: any = {}) => ({
        ...options,
        type: 'copyText',
        title,
        text,
      }),
      pasteText: (text, title = 'Paste', options: any = {}) => ({
        ...options,
        type: 'pasteText',
        title,
        text,
      }),
      paste: (content, title = 'Paste', options: any = {}) => ({
        ...options,
        type: 'pasteClipboard',
        title,
        content,
      }),
      ref: (registeredActionId, title = 'Run Action', options: any = {}) => ({
        ...options,
        type: 'runExtensionRegisteredAction',
        title,
        extensionId: extension.id,
        registeredActionId,
      }),
      typeText: (text, title = 'Type Text', options: any = {}) => ({
        ...options,
        type: 'typeText',
        title,
        text,
      }),
      copyImage: (image, title = 'Copy image', options: any = {}) =>
        String(image || '').startsWith('data:')
          ? { ...options, type: 'copyImage', title, imageDataUrl: image }
          : { ...options, type: 'copyImage', title, path: image },
      trash: (paths, title = 'Move to Trash', options: any = {}) => ({
        ...options,
        type: 'trash',
        title,
        paths: Array.isArray(paths) ? paths : [paths],
        style: options.style || 'destructive',
        requiresConfirmation: options.requiresConfirmation ?? true,
      }),
      root: (title, view, options: any = {}) => ({
        ...options,
        type: 'rootView',
        title,
        view,
      }),
      push: (title, view, options: any = {}) => ({
        ...options,
        type: 'pushView',
        title,
        view,
      }),
      replace: (title, view, options: any = {}) => ({
        ...options,
        type: 'replaceView',
        title,
        view,
      }),
      pop: (title = 'Back', options: any = {}) => ({
        ...options,
        type: 'popView',
        title,
      }),
      run: (title, handler, options: any = {}) => ({
        ...options,
        type: 'runExtensionAction',
        title,
        __handler: handler,
      }),
      background: (title, handler, options: any = {}) => ({
        ...options,
        type: 'runExtensionAction',
        title,
        __handler: handler,
        dismissAfterRun: options.dismissAfterRun || 'auto',
      }),
      shellExec: canUseSystem
        ? (title, command, args = [], options: any = {}) => ({
            ...options,
            type: 'shellExec',
            title,
            command,
            args,
            options,
            requiresConfirmation: options.requiresConfirmation ?? true,
          })
        : () => {
            throw trustedExtensionApiUnavailable('system');
          },
      shellScript: canUseSystem
        ? (title, script, options: any = {}) => ({
            ...options,
            type: 'shellScript',
            title,
            script,
            options,
            requiresConfirmation: options.requiresConfirmation ?? true,
          })
        : () => {
            throw trustedExtensionApiUnavailable('system');
          },
      toggleSetting: canWriteSettings
        ? (settingId, title = 'Toggle', options: any = {}) => ({
            ...options,
            type: 'toggleSetting',
            title,
            settingId,
          })
        : denyShortcut('settings.write'),
      recordShortcut: canUseShortcuts
        ? (input: any = {}, options: any = {}) =>
            buildRecordShortcutAction(input, options)
        : denyShortcut('recordShortcut'),
      removeShortcut: canUseShortcuts
        ? (input: any = {}, options: any = {}) =>
            buildRemoveShortcutAction(input, options)
        : denyShortcut('removeShortcut'),
      setPaletteShortcut: canUseShortcuts
        ? (title = 'Change Shortcut', options: any = {}) =>
            buildRecordShortcutAction({ scope: 'palette', title }, options)
        : denyShortcut('setPaletteShortcut'),
      native: (title, nativeAction, options: any = {}) => ({
        ...options,
        type: 'nativeAction',
        title,
        nativeAction,
      }),
      system: canUseSystem
        ? {
            lockScreen: (title = 'Lock Screen', options: any = {}) => ({
              dismissAfterRun: 'auto',
              ...options,
              type: 'lockScreen',
              title,
            }),
            sleep: (title = 'Sleep', options: any = {}) => ({
              dismissAfterRun: 'auto',
              ...options,
              type: 'sleepSystem',
              title,
            }),
            restart: (title = 'Restart Computer', options: any = {}) => ({
              dismissAfterRun: 'auto',
              ...options,
              type: 'restartSystem',
              title,
            }),
            openSystemSettings: (
              title = settingsTitle(),
              options: any = {},
            ) => ({
              dismissAfterRun: 'auto',
              ...options,
              type: 'openSystemSettings',
              title,
            }),
            openKeyboardSettings: (
              title = 'Keyboard Settings',
              options: any = {},
            ) => ({
              dismissAfterRun: 'auto',
              ...options,
              type: 'openKeyboardSettings',
              title,
            }),
            quit: (title = 'Quit Nevermind', options: any = {}) => ({
              dismissAfterRun: 'auto',
              ...options,
              type: 'quitApp',
              title,
            }),
          }
        : denyShortcut('system'),
      updates: canUseUpdates
        ? {
            check: (title = 'Check for Updates', options: any = {}) => ({
              ...options,
              type: 'checkForUpdates',
              title,
            }),
            download: (title = 'Download Update', options: any = {}) => ({
              ...options,
              type: 'downloadUpdate',
              title,
            }),
            install: (title = 'Install and Restart', options: any = {}) => ({
              ...options,
              type: 'installUpdate',
              title,
            }),
          }
        : denyShortcut('updates'),
      camera: {
        switchDevice: (title = 'Switch Camera', options: any = {}) => ({
          ...options,
          type: 'nativeAction',
          title,
          nativeAction: { kind: 'camera.switchDevice' },
        }),
        nextDevice: (title = 'Next Camera', options: any = {}) => ({
          ...options,
          type: 'nativeAction',
          title,
          nativeAction: { kind: 'camera.nextDevice' },
        }),
        previousDevice: (title = 'Previous Camera', options: any = {}) => ({
          ...options,
          type: 'nativeAction',
          title,
          nativeAction: { kind: 'camera.previousDevice' },
        }),
        toggleMuted: (title = 'Toggle Camera Audio', options: any = {}) => ({
          ...options,
          type: 'nativeAction',
          title,
          nativeAction: { kind: 'camera.toggleMuted' },
        }),
        toggleControls: (
          title = 'Toggle Camera Controls',
          options: any = {},
        ) => ({
          ...options,
          type: 'nativeAction',
          title,
          nativeAction: { kind: 'camera.toggleControls' },
        }),
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
      template: (input, variables) =>
        expandTextTemplate(input, variables, {
          includeClipboard: canUseClipboard,
        }),
    },
    input: {
      prompt: buildPromptAction,
    },
    windows: {
      create: (view, options: any = {}) => ({
        dismissAfterRun: 'auto',
        type: 'createWindow',
        title: options.title || view?.title || 'Open Window',
        view,
        windowOptions: options,
        windowId: options.id || view?.id,
      }),
      show: (id, title = 'Show Window', options: any = {}) => ({
        dismissAfterRun: 'auto',
        ...options,
        type: 'showWindow',
        title,
        windowId: id,
      }),
      hide: (id, title = 'Hide Window', options: any = {}) => ({
        dismissAfterRun: 'auto',
        ...options,
        type: 'hideWindow',
        title,
        windowId: id,
      }),
      toggle: (
        idOrView,
        titleOrOptions: any = 'Toggle Window',
        options: any = {},
      ) => {
        if (typeof idOrView === 'string')
          return {
            dismissAfterRun: 'auto',
            ...options,
            type: 'toggleWindow',
            title:
              typeof titleOrOptions === 'string'
                ? titleOrOptions
                : titleOrOptions.title || 'Toggle Window',
            windowId: idOrView,
            windowOptions:
              typeof titleOrOptions === 'string' ? options : titleOrOptions,
          };
        const windowOptions =
          typeof titleOrOptions === 'string' ? options : titleOrOptions || {};
        return {
          dismissAfterRun: 'auto',
          type: 'toggleWindow',
          title:
            typeof titleOrOptions === 'string'
              ? titleOrOptions
              : windowOptions.title || idOrView?.title || 'Toggle Window',
          view: idOrView,
          windowOptions,
          windowId: windowOptions.id || idOrView?.id,
        };
      },
      close: (id, title = 'Close Window', options: any = {}) => ({
        dismissAfterRun: 'auto',
        ...options,
        type: 'closeWindow',
        title,
        windowId: id,
      }),
    },
    clipboard: {
      history: canUseClipboard
        ? {
            list: (options: any = {}) => clipboardHistorySnapshot(options),
            search: (query, options: any = {}) =>
              clipboardHistorySnapshot({ ...options, query }),
            get: (id) => clipboardHistoryGet(String(id || '')),
            remove: async (idOrIds) => ({
              removed: removeClipboardHistoryByAction({
                clipboardHistoryRange: 'ids',
                clipboardHistoryItemIds: Array.isArray(idOrIds)
                  ? idOrIds
                  : [idOrIds],
              }),
            }),
            clear: async (options: any = {}) => ({
              removed: removeClipboardHistoryByAction({
                clipboardHistoryRange: options.olderThanMs
                  ? 'older-than'
                  : 'all',
                olderThanMs: options.olderThanMs,
                types: options.types,
              }),
            }),
          }
        : undefined,
    },
    desktop: {
      keyboard: {
        typeText: (text, options: any = {}) =>
          typeTextIntoFrontmostApp(String(text || ''), options),
      },
      clipboard: canUseClipboard
        ? {
            readText: () => clipboard.readText(),
            writeText: (text, options: any = {}) =>
              writeDesktopClipboard({ type: 'text', text }, options),
            readHtml: () => clipboard.readHTML(),
            writeHtml: (html, text = '', options: any = {}) =>
              writeDesktopClipboard({ type: 'html', html, text }, options),
            readImage: clipboardImageDataUrl,
            writeImage: (image, options: any = {}) =>
              writeDesktopClipboard({ type: 'image', image }, options),
            readFiles: clipboardFiles,
            writeFiles: (paths, options: any = {}) =>
              writeDesktopClipboard({ type: 'files', paths }, options),
            read: readDesktopClipboard,
            write: writeDesktopClipboard,
          }
        : undefined,
      selection: {
        text: selectedText,
        files: selectedFiles,
        read: readDesktopSelection,
      },
      apps: canUseDesktopApps
        ? {
            frontmost: frontmostApp,
            launch: (appPath) =>
              runInBackground(() => shell.openPath(expandUserPath(appPath))),
            list: () =>
              appIndexService.get().map((entry) => ({
                id: entry.id,
                name: entry.name,
                path: entry.path,
              })),
            search: (query) => {
              const needle = String(query || '').toLowerCase();
              return appIndexService
                .get()
                .filter(
                  (entry) =>
                    !needle ||
                    String(entry.name || '')
                      .toLowerCase()
                      .includes(needle),
                )
                .map((entry) => ({
                  id: entry.id,
                  name: entry.name,
                  path: entry.path,
                }));
            },
            icon: (appPath) => appIconCache.get(appPath),
          }
        : undefined,
      files: canUseDesktopFiles
        ? {
            find: findFiles,
            findImages: (roots, options) =>
              findFiles(roots, { ...options, kind: 'image' }),
            findVideos: (roots, options) =>
              findFiles(roots, { ...options, kind: 'video' }),
            findMedia: (roots, options) =>
              findFiles(roots, { ...options, kind: 'media' }),
            openWithApps,
            open: (filePath) =>
              runInBackground(() => shell.openPath(expandUserPath(filePath))),
            reveal: (filePath) =>
              runInBackground(() =>
                shell.showItemInFolder(expandUserPath(filePath)),
              ),
            preview: quickLookPath,
            readText: (filePath) =>
              fs.readFile(expandUserPath(filePath), 'utf8'),
            toFileUrl: (filePath) => fileUrlForPath(expandUserPath(filePath)),
            thumbnail: (filePath) => thumbnailUrlForPreviewablePath(filePath),
            metadata: (filePath) =>
              fileToExtensionFile(filePath, { includeDimensions: true }),
            indexedRoots: () => defaultFileIndexRoots().map(displayUserPath),
            indexSnapshot: (options: any = {}) => fileIndexSnapshot(options),
            reindex: (options: any = {}) => reindexFiles(options),
            recent: (options: any = {}) => fileIndexSnapshot(options),
            searchIndex: (query, options: any = {}) =>
              fileIndexSnapshot({ ...options, query }),
          }
        : undefined,
      shell: canUseSystem
        ? {
            openExternal: async (url) => {
              if (!(await openExternalUrl(url)))
                throw new Error('Unsafe external URL');
            },
            exec: runShellCommand,
            script: runShellScript,
            appleScript: (script, options: any = {}) =>
              new Promise((resolve) => {
                if (!hasCapability('applescript'))
                  return resolve({
                    stdout: '',
                    stderr: 'AppleScript is not available on this OS',
                    exitCode: 1,
                  });
                execFile(
                  'osascript',
                  ['-e', String(script)],
                  { timeout: Number(options.timeout || 30_000) },
                  (error, stdout, stderr) =>
                    resolve({
                      stdout: limitedOutput(stdout, options.outputLimit),
                      stderr: limitedOutput(
                        stderr || error?.message || '',
                        options.outputLimit,
                      ),
                      exitCode: error ? 1 : 0,
                    }),
                );
              }),
            which: async (command) => {
              const env = await shellSpawnEnv();
              return new Promise((resolve) => {
                execFile(
                  '/usr/bin/which',
                  [String(command)],
                  { env },
                  (error, stdout, stderr) =>
                    resolve({
                      stdout: stdout.trim(),
                      stderr: stderr || error?.message || '',
                      exitCode: error ? 1 : 0,
                    }),
                );
              });
            },
          }
        : undefined,
    },
    ocr: canUseOcr
      ? {
          image: ocrImage,
          screen: (options: any = {}) => ocrScreen(options),
          region: (rect, options: any = {}) =>
            ocrScreen({ ...options, region: rect }),
        }
      : undefined,
    storage: createExtensionStorage(extension),
    settings: {
      definitions: () =>
        availableSettingDefinitions().map((definition) => ({
          ...definition,
          value: getSetting(definition.id),
        })),
      get: (id) => getSetting(id),
      set: canWriteSettings
        ? (id, value) => {
            const result = setSetting(id, value);
            if (!result.ok) throw new Error(result.message);
            return value;
          }
        : () => {
            throw trustedExtensionApiUnavailable('settings.write');
          },
      toggle: canWriteSettings
        ? (id) => {
            const definition = settingDefinition(id);
            if (!definition) throw new Error(`Unknown setting: ${id}`);
            const next = toggledSettingValue(definition, getSetting(id));
            const result = setSetting(id, next);
            if (!result.ok) throw new Error(result.message);
            return next;
          }
        : () => {
            throw trustedExtensionApiUnavailable('settings.write');
          },
    },
    shortcuts: canUseShortcuts
      ? {
          list: () => extensionShortcutRecords(),
          palette: () => ({
            title: 'Open Nevermind',
            accelerator: String(getPaletteHotkey()),
            scope: 'palette' as const,
          }),
        }
      : {
          list: denyShortcut('list'),
          palette: denyShortcut('palette'),
        },
    logs: extensionLogger(extension.id, command?.id),
    cache: createExtensionCache(extension),
    views: createExtensionViewsApi(extension, command),
    updates: canUseUpdates
      ? { getState: () => updatesStateSnapshot() }
      : undefined,
    state: {},
    data: {
      loader(fn, options: any = {}) {
        return createDataLoaderHandle(fn, options);
      },
      staleWhileRevalidate({ cacheKey, ttlMs, staleTtlMs, loader, retry }) {
        return createStaleWhileRevalidateHandle({
          cacheKey,
          ttlMs,
          staleTtlMs,
          loader,
          retry,
        });
      },
    },
    ai: canUseAi ? createExtensionAi(extension) : undefined,
    aiBuilder: createAiBuilderApi(extension),
    extensions: {
      ownership: canManageExtensionOwnership
        ? createExtensionOwnershipApi(extension)
        : undefined,
    },
  };
  // Belt-and-suspenders: ensure `data` is never stripped from the context.
  // Every handler path expects `ctx.data` to be available.
  if (!context.data) {
    logError(
      'extension.context.missingData',
      new Error('ctx.data is missing'),
      {
        source: 'host',
        scope: 'extension',
      },
    );
    // Recover: create a minimal data object so extension code doesn't crash.
    context.data = {
      loader(fn, options: any = {}) {
        return createDataLoaderHandle(fn, options);
      },
      staleWhileRevalidate(params) {
        return createStaleWhileRevalidateHandle(params);
      },
    };
  }
  return context;
}

function assertAiBuilderPrivilege(extension) {
  if (extension?.id !== AI_BUILDER_EXTENSION_ID) {
    throw new Error(
      'ctx.aiBuilder is only available to the built-in AI Builder extension',
    );
  }
}

function buildAiBuilderAction(title, handler, options: any = {}) {
  return { ...options, type: 'runExtensionAction', title, __handler: handler };
}

function createAiBuilderApi(extension) {
  const privileged = extension?.id === AI_BUILDER_EXTENSION_ID;
  if (!privileged) return;
  return {
    startChat: (input: any = {}) => {
      const prompt = String(input.prompt || input.query || '');
      return buildAiBuilderAction(
        input.title || `Automate "${prompt}"`,
        async () => {
          const item = createDraftAiChat(prompt);
          return {
            view: aiChatView(item, { start: item.messages.length <= 1 }),
          };
        },
        input.options,
      );
    },
    openChat: (chatId, input: any = {}) =>
      buildAiBuilderAction(
        input.title || 'Open Chat',
        async () => {
          const item = userState.aiChats[chatId] || draftAiChats.get(chatId);
          if (!item)
            return { toast: { message: 'AI chat not found', tone: 'error' } };
          return { view: aiChatView(item) };
        },
        input.options,
      ),
    removeChat: (chatId, input: any = {}) =>
      buildAiBuilderAction(
        input.title || 'Remove Chat',
        async () => removeAiChat(chatId),
        { style: 'destructive', ...(input.options || {}) },
      ),
    tweakExtension: (input: any = {}) =>
      buildAiBuilderAction(
        input.title || 'Tweak with AI',
        async () => {
          const file = input.extensionFile || input.extensionId;
          if (!file)
            return {
              toast: { message: 'No extension specified', tone: 'error' },
            };
          const item = getOrCreateExtensionChat(file, input.title || file);
          return { view: aiChatView(item, { initialPrompt: input.prompt }) };
        },
        input.options,
      ),
    openChatsList: (input: any = {}) =>
      buildAiBuilderAction(
        input.title || 'AI Chats',
        async () => ({ view: aiChatsView() }),
        input.options,
      ),
    listChats: () =>
      Object.values(userState.aiChats || {}).map((chat: any) => ({
        id: chat.id,
        title: chat.title || chat.query,
        query: chat.query,
        status: chat.status,
        createdAt: chat.createdAt,
        updatedAt: chat.updatedAt,
        extensionFiles: chatTouchedExtensionFiles(chat),
      })),
    getChat: (chatId) => {
      const chat = userState.aiChats[chatId] || draftAiChats.get(chatId);
      if (!chat) return null;
      return {
        id: chat.id,
        title: chat.title || chat.query,
        query: chat.query,
        status: chat.status,
        messages: chat.messages,
        extensionFiles: chatTouchedExtensionFiles(chat),
      };
    },
  };
}

function createExtensionOwnershipApi(extension) {
  const privileged = extension?.id === AI_BUILDER_EXTENSION_ID;
  const readOnly = {
    ownerOf: (extensionFile) => aiChatIdForExtensionFile(extensionFile),
    filesForChat: (chatId) => {
      const chat = userState.aiChats[chatId] || draftAiChats.get(chatId);
      return chat ? chatTouchedExtensionFiles(chat) : [];
    },
    canWrite: (extensionFile, chatId) =>
      chatCanWriteExtension(extensionFile, chatId),
  };
  if (!privileged) return readOnly;
  return {
    ...readOnly,
    claim: (extensionFile, chatId) => {
      const chat = chatId
        ? userState.aiChats[chatId] || draftAiChats.get(chatId)
        : null;
      if (!chat) return false;
      touchExtensionFileForChat(chat, extensionFile);
      scheduleSaveState();
      return true;
    },
    reload: async () => {
      await loadExtensions();
      registerActionShortcuts();
    },
    remove: async (extensionFile, chatId) => {
      if (!chatCanWriteExtension(extensionFile, chatId)) return false;
      const { removed } = await removeGeneratedExtensionForChat(
        extensionFile,
        chatId,
      );
      return removed;
    },
  };
}

function createExtensionViewsApi(extension, command) {
  return {
    refresh: () => ({
      type: 'runExtensionAction',
      title: 'Refresh',
      __handler: async (innerCtx) => {
        if (!command || typeof command.run !== 'function')
          return { toast: { message: 'View cannot refresh', tone: 'error' } };
        if (!checkRefreshBurst(extension)) return { skipped: true };
        const result = await command.run(innerCtx);
        const view = result?.type
          ? result
          : result?.view?.type
            ? result.view
            : null;
        if (
          view?.items &&
          !isLoaderHandle(view.items) &&
          Array.isArray(view.items) &&
          view.items.length > 0
        )
          return { patch: { mode: 'replace', items: view.items } };
        if (view) return { view, navigation: 'replace' };
        return result;
      },
    }),
    invalidate: () => {
      invalidateExtensionRootItemsForExtension(extension);
      extensionCacheFor(extension.id).clear();
    },
  };
}

async function initNevermindAi() {
  const [extensionTypesPath, skillPath] = await Promise.all([
    bundledResourcePath(EXTENSION_TYPES_FILENAME),
    bundledResourcePath('skills', 'nevermind-extension-builder', 'SKILL.md'),
  ]);
  nevermindAi = createNevermindAi({
    agentDir: path.join(app.getPath('userData'), 'pi-agent'),
    workspaceDir: path.join(app.getPath('userData'), 'ai-workspace'),
    extensionsDir,
    extensionApiPath: extensionTypesPath,
    extensionTypesPath,
    skillPath,
    reloadExtensions: loadExtensions,
    getShortcuts: () => extensionShortcutRecords(),
    getPaletteShortcut: () => ({
      title: 'Open Nevermind',
      accelerator: String(getPaletteHotkey()),
      scope: 'palette' as const,
    }),
    getExtensionRuntimeState: (filename) =>
      extensionRuntimeStateForFile(filename),
    getActiveChat: () =>
      activeAiChatId
        ? userState.aiChats[activeAiChatId] ||
          draftAiChats.get(activeAiChatId) ||
          null
        : null,
    getChat: (chatId) =>
      userState.aiChats[chatId] || draftAiChats.get(chatId) || null,
    markGeneratedExtension: (filePath, chatId) =>
      markGeneratedExtensionForActiveChat(filePath, chatId),
    stageExtensionProposal,
    canWriteExtension: (filename, chatId) =>
      chatCanWriteExtension(filename, chatId),
    removeExtension: (filename, chatId) =>
      removeGeneratedExtensionForChat(filename, chatId),
    addAliasForChat: (chatId) => addAliasForGeneratedAction(chatId),
    onEvent: (event) => {
      const chatId = event.chatId || activeAiChatId;
      const metadata = chatId ? aiLearningMetadata(chatId) : undefined;
      if (chatId && event.type === 'start')
        learningStore?.recordStatus(chatId, 'start', metadata);
      if (chatId && event.type === 'delta' && event.text)
        appendAiChatDelta(chatId, event.text);
      if (chatId && event.type === 'tool_start' && event.name)
        appendAiChatMessage(chatId, 'system', event.name);
      if (chatId && event.type === 'tool_trace_start' && event.name)
        learningStore?.recordToolStart(
          chatId,
          event.name,
          (event.data as any)?.input,
          metadata,
          (event.data as any)?.toolCallId,
        );
      if (chatId && event.type === 'tool_trace_end' && event.name) {
        const toolData = event.data as any;
        learningStore?.recordToolEnd(
          chatId,
          event.name,
          {
            ok: !event.isError,
            outputSummary: toolData?.output,
            error: toolData?.error,
            toolCallId: toolData?.toolCallId,
          },
          metadata,
        );
        if (
          [
            'write_extension',
            'validate_extension',
            'remove_extension',
            'install_extension',
          ].includes(event.name)
        ) {
          learningStore?.recordExtensionEvent(
            chatId,
            {
              kind: event.name as
                | 'write_extension'
                | 'validate_extension'
                | 'remove_extension'
                | 'install_extension',
              filename: toolData?.output?.filePath || toolData?.input?.filename,
              extensionId: toolData?.output?.extensionId,
              commandIds: Array.isArray(toolData?.output?.commandIds)
                ? toolData.output.commandIds
                : undefined,
              ok: !event.isError,
              error: toolData?.error,
              details: toolData?.output,
            },
            metadata,
          );
        }
      }
      if (chatId && event.type === 'error' && event.message)
        appendAiChatMessage(chatId, 'system', event.message);
      if (chatId && event.type === 'done' && userState.aiChats[chatId]) {
        if (userState.aiChats[chatId].status !== 'ready')
          userState.aiChats[chatId].status = 'done';
        userState.aiChats[chatId].updatedAt = Date.now();
        learningStore?.recordStatus(chatId, 'done', metadata);
        patchAiChatsItem(chatId);
        scheduleSaveState();
      }
      if (chatId && event.type === 'error' && event.message) {
        learningStore?.recordStatus(chatId, 'error', metadata, event.message);
      }
      if (chatId && event.type === 'aborted') {
        learningStore?.recordStatus(chatId, 'aborted', metadata);
      }
      paletteWindow.win?.webContents.send('ai:chat:event', {
        ...event,
        chatId,
      });
    },
  });
}

function extensionRuntimeStateForFile(filename) {
  const base = path.basename(filename || '');
  const matches = Array.from(extensionActionRegistry.values()).filter(
    (entry) => path.basename(entry.extension?.__filePath || '') === base,
  );
  return {
    loaded: matches.length > 0,
    extensionId: matches[0]?.extension?.id,
    commandIds: matches.map((entry) => entry.command?.id).filter(Boolean),
  };
}

function aiChatPromptWithContext(message, chatId) {
  const chat = userState.aiChats[chatId];
  const focused = chat?.contextExtensionFile
    ? `\n\nFocused extension file: ${chat.contextExtensionFile}. Use read_current_extension before editing it. You may list/read other extensions if needed.`
    : '';
  const learnings = relevantLearningContext(message, chatId);
  const messages = (chat?.messages || [])
    .filter((item) => item.role === 'user' || item.role === 'assistant')
    .slice(-12);

  if (!messages.length)
    return `Use this Nevermind AI chat transcript as context. If the user has provided enough details, proceed by calling read_extension_api immediately; do not merely say you will.${focused}${learnings}\n\nNew user message:\n${message}`;

  const transcript = messages
    .map(
      (item) =>
        `${item.role === 'user' ? 'User' : 'Assistant'}: ${item.content}`,
    )
    .join('\n\n');

  return `Use this Nevermind AI chat transcript as context. Do not ask questions that the user already answered. If the user has now provided enough details, proceed by calling read_extension_api immediately; do not merely say you will.${focused}${learnings}\n\n${transcript}\n\nNew user message:\n${message}`;
}

function markGeneratedExtensionForActiveChat(
  filePath,
  chatId = activeAiChatId,
) {
  const chat = chatId ? userState.aiChats[chatId] : null;
  if (!chat) return;
  touchExtensionFileForChat(chat, path.basename(filePath));
  scheduleSaveState();
}

function chatCanWriteExtension(filename, chatId = activeAiChatId) {
  const base = path.basename(filename || '');
  if (!base) return false;
  const chat = chatId
    ? userState.aiChats[chatId] || draftAiChats.get(chatId)
    : null;
  if (!chat) return false;
  const ownedFiles = chatTouchedExtensionFiles(chat);
  if (ownedFiles.includes(base)) return true;
  const owner = (Object.values(userState.aiChats || {}) as any[]).find((item) =>
    chatTouchedExtensionFiles(item).includes(base),
  );
  if (owner) return false;
  return !Array.from(extensionModules.values()).some(
    (extension) => path.basename(extension.__filePath || '') === base,
  );
}

async function removeGeneratedExtensionForChat(
  filename,
  chatId = activeAiChatId,
) {
  const base = path.basename(filename || '');
  if (!(base && isExtensionSourceFile(base))) return { removed: false };
  if (!chatCanWriteExtension(base, chatId))
    throw new Error(
      `Refusing to remove ${base}: this AI chat does not own that extension.`,
    );
  const filePath = path.join(extensionsDir, base);
  const existed = await fs
    .stat(filePath)
    .then(() => true)
    .catch(() => false);
  if (!existed) return { removed: false, filePath };
  await fs.unlink(filePath);
  await removeAiChatReferencesToExtensionFile(base, chatId);
  scheduleSaveState();
  await loadExtensions();
  registerActionShortcuts();
  return { removed: true, filePath };
}

function addAliasForGeneratedAction(chatId) {
  const chat = userState.aiChats[chatId];
  if (!chat?.query) return;
  const files = chatTouchedExtensionFiles(chat);
  const entry = Array.from(extensionActionRegistry.values()).find((e) =>
    files.includes(path.basename(e.extension?.__filePath || '')),
  );
  if (!entry) return;
  const action = extensionActionFromContribution(entry);
  if (!action?.id) return;
  const aliases = new Set(actionAliases(action.id));
  aliases.add(chat.query.trim());
  userState.aliases[action.id] = Array.from(aliases);
  scheduleSaveState();
}

async function sendAiChatMessage(message, chatId) {
  if (!nevermindAi) await initNevermindAi();
  activeAiChatId = chatId || activeAiChatId;
  if (activeAiChatId?.startsWith('draft:')) promoteDraftAiChat(activeAiChatId);
  const prompt = activeAiChatId
    ? aiChatPromptWithContext(message, activeAiChatId)
    : message;
  if (activeAiChatId) appendAiChatMessage(activeAiChatId, 'user', message);
  return nevermindAi.send(prompt, activeAiChatId);
}

async function abortAiChat(chatId) {
  return nevermindAi?.abort(chatId || activeAiChatId);
}

async function resetAiChat(chatId) {
  activeAiChatId = chatId || activeAiChatId;
  return nevermindAi?.reset(activeAiChatId);
}

async function noteAiChatExited(chatId) {
  if (!chatId) return;
  recordLearningReview(chatId);
}

async function ensureExtensionTypeDefinitions() {
  if (!extensionsDir) return;
  const sourcePath = await bundledResourcePath(EXTENSION_TYPES_FILENAME);
  const targetPath = path.join(extensionsDir, EXTENSION_TYPES_FILENAME);
  await fs.copyFile(sourcePath, targetPath).catch((error) => {
    logWarn(
      'extension.types.copy.failed',
      { error: error?.message || String(error) },
      { source: 'host', scope: 'extension' },
    );
  });
  await fs
    .writeFile(
      path.join(extensionsDir, 'package.json'),
      `${JSON.stringify({ type: 'module' }, null, 2)}\n`,
    )
    .catch((error) => {
      logWarn(
        'extension.packageJson.write.failed',
        { error: error?.message || String(error) },
        { source: 'host', scope: 'extension' },
      );
    });
}

async function loadExtensionModule(fullPath) {
  const url = pathToFileURL(fullPath);
  url.searchParams.set('reload', String(Date.now()));
  const imported = await import(url.href);
  return imported.default || imported;
}

async function initializeExtensionManager() {
  const manager = userState.extensionManager;
  const entries = await fs
    .readdir(extensionsDir, { withFileTypes: true })
    .catch(() => []);
  const sourceFiles = entries
    .filter((entry) => isExtensionSourceFile(entry.name))
    .map((entry) => entry.name);
  if (manager?.schemaVersion !== 1) {
    userState.extensionManager = {
      schemaVersion: 1,
      files: Object.fromEntries(
        sourceFiles.map((filename) => [filename, { enabled: true }]),
      ),
      proposals: manager?.proposals || {},
    };
    // Existing extensions stay enabled on first upgrade. Persist the complete
    // inventory before the first import so later discoveries cannot run by
    // merely appearing on disk.
    await saveUserState();
    return;
  }
  let discovered = false;
  for (const filename of sourceFiles) {
    if (manager.files?.[filename]) continue;
    manager.files ||= {};
    manager.files[filename] = { enabled: false };
    discovered = true;
  }
  if (discovered) await saveUserState();
}

function extensionFileIsEnabled(filename: string) {
  return userState.extensionManager?.files?.[filename]?.enabled === true;
}

function extensionDraftsDir() {
  return path.join(path.dirname(extensionsDir), 'extension-drafts');
}

function extensionManagerState() {
  return (userState.extensionManager ||= {
    schemaVersion: 1,
    files: {},
    proposals: {},
  });
}

async function stageExtensionProposal(filename: string, source: string) {
  const safeName = path.basename(filename);
  const draftFile = path.join(extensionDraftsDir(), safeName);
  await fs.mkdir(extensionDraftsDir(), { recursive: true });
  await fs.writeFile(draftFile, source);
  extensionManagerState().proposals[safeName] = {
    draftFile,
    provenance: 'ai',
    updatedAt: Date.now(),
  };
  await persistUserState();
  return { draftFile };
}

function replaceMapContents(target: Map<any, any>, source: Map<any, any>) {
  target.clear();
  for (const [key, value] of source) target.set(key, value);
}

function runtimeEntryFilename(entry: any) {
  return path.basename(entry?.extension?.__filePath || '');
}

function executionRecordBelongsToExtension(
  record: ExtensionExecutionRecord,
  filename: string,
  extensionIds: Set<string>,
) {
  return (
    record.extensionFile === filename ||
    Boolean(record.extensionId && extensionIds.has(record.extensionId))
  );
}

async function prepareManagedExtensionRuntime(
  filename: string,
  activeFile: string,
  extension: any,
): Promise<PreparedExtensionRuntime> {
  extension.__filePath = activeFile;
  extension.__generated = true;
  await applyExtensionMetadataOverrides(extension);

  const live = {
    modules: new Map(extensionModules),
    actions: new Map(extensionActionRegistry),
    handlers: new Map(extensionActionHandlers),
    viewActions: new Map(viewActionExecutionRecords),
    rootActions: new Map(rootActionExecutionRecords),
    viewRefreshes: new Map(viewRefreshRecords),
  };
  const jobs: JobDefinition[] = [];
  const fileWatchers: PreparedFileWatcher[] = [];
  try {
    extensionModules.clear();
    extensionActionRegistry.clear();
    extensionActionHandlers.clear();
    viewActionExecutionRecords.clear();
    rootActionExecutionRecords.clear();
    viewRefreshRecords.clear();
    extensionRuntimePreparation = { jobs, fileWatchers };
    registerExtension(extension);
    if (!extensionModules.size)
      throw new Error(`Extension ${filename} does not export a valid id`);
    const extensionIds = new Set(extensionModules.keys());
    for (const extensionId of extensionIds) {
      const existing = live.modules.get(extensionId);
      if (existing && path.basename(existing.__filePath || '') !== filename)
        throw new Error(
          `Extension id ${extensionId} is already owned by another file`,
        );
    }
    return {
      filename,
      extensionIds,
      modules: new Map(extensionModules),
      actions: new Map(extensionActionRegistry),
      handlers: new Map(extensionActionHandlers),
      viewActions: new Map(viewActionExecutionRecords),
      rootActions: new Map(rootActionExecutionRecords),
      viewRefreshes: new Map(viewRefreshRecords),
      jobs,
      fileWatchers,
    };
  } finally {
    extensionRuntimePreparation = undefined;
    replaceMapContents(extensionModules, live.modules);
    replaceMapContents(extensionActionRegistry, live.actions);
    replaceMapContents(extensionActionHandlers, live.handlers);
    replaceMapContents(viewActionExecutionRecords, live.viewActions);
    replaceMapContents(rootActionExecutionRecords, live.rootActions);
    replaceMapContents(viewRefreshRecords, live.viewRefreshes);
  }
}

function detachManagedExtensionRuntime(filename: string) {
  const extensionIds = new Set(
    Array.from(extensionModules.entries())
      .filter(([, extension]) =>
        filename
          ? path.basename(extension?.__filePath || '') === filename
          : false,
      )
      .map(([extensionId]) => extensionId),
  );
  for (const extensionId of extensionIds) extensionModules.delete(extensionId);
  for (const [key, entry] of extensionActionRegistry)
    if (
      runtimeEntryFilename(entry) === filename ||
      extensionIds.has(entry?.extension?.id)
    )
      extensionActionRegistry.delete(key);
  for (const [key, record] of extensionActionHandlers)
    if (
      runtimeEntryFilename(record?.entry) === filename ||
      extensionIds.has(record?.entry?.extension?.id)
    )
      extensionActionHandlers.delete(key);
  for (const [key, record] of viewRefreshRecords)
    if (
      runtimeEntryFilename(record?.entry) === filename ||
      extensionIds.has(record?.entry?.extension?.id)
    )
      viewRefreshRecords.delete(key);
  for (const [key, record] of viewActionExecutionRecords)
    if (executionRecordBelongsToExtension(record, filename, extensionIds))
      viewActionExecutionRecords.delete(key);
  for (const [key, record] of rootActionExecutionRecords)
    if (executionRecordBelongsToExtension(record, filename, extensionIds))
      rootActionExecutionRecords.delete(key);
  jobRegistry.unregisterWhere(
    (job) => job.owner === 'extension' && extensionIds.has(job.scope || ''),
  );
  const retainedWatchers = [] as typeof extensionFileWatchers;
  for (const watcher of extensionFileWatchers) {
    if (watcher.extensionId && extensionIds.has(watcher.extensionId))
      watcher.close();
    else retainedWatchers.push(watcher);
  }
  extensionFileWatchers = retainedWatchers;
  for (const cacheKey of extensionRootItemsCache.keys())
    if (path.basename(cacheKey) === filename)
      extensionRootItemsCache.delete(cacheKey);
  for (const cacheKey of extensionRootItemsRefreshes.keys())
    if (path.basename(cacheKey) === filename)
      extensionRootItemsRefreshes.delete(cacheKey);
  return extensionIds;
}

function captureManagedExtensionRuntime(
  filename: string,
): LiveExtensionRuntimeSnapshot {
  const extensionIds = new Set(
    Array.from(extensionModules.entries())
      .filter(
        ([, extension]) =>
          path.basename(extension?.__filePath || '') === filename,
      )
      .map(([extensionId]) => extensionId),
  );
  return {
    extensionIds,
    modules: new Map(extensionModules),
    actions: new Map(extensionActionRegistry),
    handlers: new Map(extensionActionHandlers),
    viewActions: new Map(viewActionExecutionRecords),
    rootActions: new Map(rootActionExecutionRecords),
    viewRefreshes: new Map(viewRefreshRecords),
    jobs: jobRegistry.definitionsWhere(
      (job) => job.owner === 'extension' && extensionIds.has(job.scope || ''),
    ),
    fileWatchers: Array.from(
      new Map(
        extensionFileWatchers
          .filter((watcher) => extensionIds.has(watcher.extensionId))
          .map(({ trigger, event, extensionId }) => [
            `${extensionId}:${event}`,
            { trigger, event, extensionId },
          ]),
      ).values(),
    ),
  };
}

function removeManagedExtensionSideEffects(extensionIds: Set<string>) {
  jobRegistry.unregisterWhere(
    (job) => job.owner === 'extension' && extensionIds.has(job.scope || ''),
  );
  const retainedWatchers: ExtensionFileWatcher[] = [];
  for (const watcher of extensionFileWatchers) {
    if (extensionIds.has(watcher.extensionId)) watcher.close();
    else retainedWatchers.push(watcher);
  }
  extensionFileWatchers = retainedWatchers;
}

function restoreManagedExtensionRuntime(
  snapshot: LiveExtensionRuntimeSnapshot,
  candidate: PreparedExtensionRuntime,
) {
  removeManagedExtensionSideEffects(
    new Set([...snapshot.extensionIds, ...candidate.extensionIds]),
  );
  replaceMapContents(extensionModules, snapshot.modules);
  replaceMapContents(extensionActionRegistry, snapshot.actions);
  replaceMapContents(extensionActionHandlers, snapshot.handlers);
  replaceMapContents(viewActionExecutionRecords, snapshot.viewActions);
  replaceMapContents(rootActionExecutionRecords, snapshot.rootActions);
  replaceMapContents(viewRefreshRecords, snapshot.viewRefreshes);
  for (const watcher of snapshot.fileWatchers)
    attachExtensionFileTrigger(
      watcher.trigger,
      watcher.event,
      watcher.extensionId,
    );
  for (const job of snapshot.jobs)
    jobRegistry.register(job, { skipStartup: true });
  syncFrontmostAppPolling();
  registerActionShortcuts();
  invalidateExtensionRootItems();
}

function commitPreparedExtensionRuntime(runtime: PreparedExtensionRuntime) {
  detachManagedExtensionRuntime(runtime.filename);
  for (const [key, value] of runtime.modules) extensionModules.set(key, value);
  for (const [key, value] of runtime.actions)
    extensionActionRegistry.set(key, value);
  for (const [key, value] of runtime.handlers)
    extensionActionHandlers.set(key, value);
  for (const [key, value] of runtime.viewActions)
    viewActionExecutionRecords.set(key, value);
  for (const [key, value] of runtime.rootActions)
    rootActionExecutionRecords.set(key, value);
  for (const [key, value] of runtime.viewRefreshes)
    viewRefreshRecords.set(key, value);
  for (const watcher of runtime.fileWatchers)
    attachExtensionFileTrigger(
      watcher.trigger,
      watcher.event,
      watcher.extensionId,
    );
  for (const job of runtime.jobs) jobRegistry.register(job);
  failTestExtensionActivationAt('runtime-commit');
  syncFrontmostAppPolling();
  registerActionShortcuts();
  invalidateExtensionRootItems();
}

function failTestExtensionActivationAt(phase: string) {
  if (testExtensionActivationFailurePhase !== phase) return;
  testExtensionActivationFailurePhase = undefined;
  throw new Error(`Injected extension activation failure at ${phase}`);
}

async function activateManagedExtension(filename: string) {
  const safeName = path.basename(filename);
  const manager = extensionManagerState();
  const proposal = manager.proposals?.[safeName];
  const activeFile = path.join(extensionsDir, safeName);
  const candidateFile = proposal?.draftFile || activeFile;
  const previousSource = await fs
    .readFile(activeFile, 'utf8')
    .catch(() => null);
  const previousFileState = manager.files[safeName]
    ? { ...manager.files[safeName] }
    : undefined;
  const previousProposal = proposal ? { ...proposal } : undefined;
  // Candidate evaluation and host runtime preparation happen once against
  // isolated registries. Jobs, watchers, and shortcuts are attached only by
  // the synchronous commit below.
  const preparedCandidate = await loadExtensionModule(candidateFile);
  const preparedRuntime = await prepareManagedExtensionRuntime(
    safeName,
    activeFile,
    preparedCandidate,
  );
  let previousRuntime: LiveExtensionRuntimeSnapshot | undefined;
  let runtimeCommitStarted = false;
  try {
    if (proposal) {
      const candidateSource = await fs.readFile(candidateFile);
      await atomicWriteFile(activeFile, candidateSource);
    }
    manager.files[safeName] = { enabled: true };
    if (proposal) delete manager.proposals[safeName];
    failTestExtensionActivationAt('state-persist');
    await persistUserState();
    failTestExtensionActivationAt('after-persist');
    previousRuntime = captureManagedExtensionRuntime(safeName);
    runtimeCommitStarted = true;
    commitPreparedExtensionRuntime(preparedRuntime);
  } catch (error) {
    if (runtimeCommitStarted && previousRuntime)
      restoreManagedExtensionRuntime(previousRuntime, preparedRuntime);
    if (previousFileState) manager.files[safeName] = previousFileState;
    else delete manager.files[safeName];
    if (previousProposal) manager.proposals[safeName] = previousProposal;
    else delete manager.proposals[safeName];
    if (proposal) {
      if (previousSource === null) await fs.unlink(activeFile).catch(() => {});
      else await atomicWriteFile(activeFile, previousSource);
    }
    await persistUserState();
    throw error;
  }
  if (proposal) await fs.unlink(candidateFile).catch(() => {});
}

async function disableManagedExtension(filename: string) {
  const safeName = path.basename(filename);
  const manager = extensionManagerState();
  const previousFileState = manager.files[safeName]
    ? { ...manager.files[safeName] }
    : undefined;
  manager.files[safeName] = { enabled: false };
  try {
    await persistUserState();
  } catch (error) {
    if (previousFileState) manager.files[safeName] = previousFileState;
    else delete manager.files[safeName];
    throw error;
  }
  detachManagedExtensionRuntime(safeName);
  syncFrontmostAppPolling();
  registerActionShortcuts();
  invalidateExtensionRootItems();
}

async function discardManagedExtensionProposal(filename: string) {
  const safeName = path.basename(filename);
  const proposal = extensionManagerState().proposals?.[safeName];
  if (!proposal) return;
  await fs.unlink(proposal.draftFile).catch(() => {});
  delete extensionManagerState().proposals[safeName];
  await persistUserState();
}

async function managedExtensionEntries() {
  const manager = extensionManagerState();
  const names = new Set(Object.keys(manager.files || {}));
  for (const name of Object.keys(manager.proposals || {})) names.add(name);
  const entries = [] as any[];
  for (const filename of Array.from(names).sort()) {
    const proposal = manager.proposals?.[filename];
    const activeFile = path.join(extensionsDir, filename);
    const source = await fs.readFile(activeFile, 'utf8').catch(() => '');
    entries.push({
      filename,
      enabled: extensionFileIsEnabled(filename),
      proposal: Boolean(proposal),
      source,
      proposalSource: proposal
        ? await fs.readFile(proposal.draftFile, 'utf8').catch(() => '')
        : undefined,
    });
  }
  return entries;
}

async function loadExtensions(preparedExtensions = new Map<string, any>()) {
  await measureDebugPerformance(
    'extensions.load-all',
    { alwaysLog: true },
    async () => {
      extensionActionRegistry.clear();
      extensionModules.clear();
      fixtureExtensions = [];
      extensionRootItemsCache.clear();
      extensionRootItemsRefreshes.clear();
      extensionActionHandlers.clear();
      viewActionExecutionRecords.clear();
      rootActionExecutionRecords.clear();
      viewRefreshRecords.clear();
      jobRegistry.unregisterWhere((job) => job.owner === 'extension');
      stopFrontmostAppPolling();
      for (const watcher of extensionFileWatchers) watcher.close();
      extensionFileWatchers = [];
      initExtensionContext({
        userState,
        fileIndex,
        clipboardService,
        nevermindAi,
        activeAiChatId,
        draftAiChats,
        jobRegistry,
        appIndexService,
        runningAppStatus,
        hasCapability,
        appUninstallService,
        FILE_RESULT_LIMIT,
        usageBoost,
        recentBoost,
        rankAction,
        actionAliases,
        commandFromItem,
        createExtensionContext,
        scheduleSaveState,
        saveUserState,
        invalidateExtensionRootItems,
        broadcastAuthChanged,
        activeNevermindBaseUrl,
        setActiveNevermindBaseUrl: (value) => {
          activeNevermindBaseUrl = value;
          setActiveNevermindAuthBaseUrl(value);
        },
        switchNevermindBackendEnvironment,
        getNevermindDebugStatus,
        signInToNevermind: signInToSelectedNevermindEnvironment,
        getPaletteHotkey,
        extensionShortcutRecords,
        patchKeyboardShortcutsView,
        patchOpenView,
        aiChatsView,
        aiChatView,
        updatesStateSnapshot,
        checkForUpdatesView,
        compatibilityPromptAction,
        updatePromptAction,
        settingsItems,
        buildRecordShortcutAction,
        buildRemoveShortcutAction,
        extensionManager: {
          list: managedExtensionEntries,
          enable: activateManagedExtension,
          disable: disableManagedExtension,
          discard: discardManagedExtensionProposal,
        },
        paletteWindow,
      });
      registerInternalExtensions();
      if (isDev)
        await measureDebugPerformance('extensions.load-dev', undefined, () =>
          loadDevExtensions(),
        );

      await fs.mkdir(extensionsDir, { recursive: true });
      await ensureExtensionTypeDefinitions();
      await initializeExtensionManager();
      const entries = await fs
        .readdir(extensionsDir, { withFileTypes: true })
        .catch(() => []);
      for (const entry of entries) {
        if (!isExtensionSourceFile(entry.name)) continue;
        if (!extensionFileIsEnabled(entry.name)) continue;
        const fullPath = path.join(extensionsDir, entry.name);
        try {
          await measureDebugPerformance(
            'extension.load-file',
            { file: entry.name },
            async () => {
              const extension =
                preparedExtensions.get(fullPath) ||
                (await loadExtensionModule(fullPath));
              extension.__filePath = fullPath;
              extension.__generated = true;
              await applyExtensionMetadataOverrides(extension);
              registerExtension(extension);
            },
          );
        } catch (error) {
          logError('extension.load.failed', error, {
            source: 'host',
            scope: 'extension',
            extensionId: path.basename(fullPath),
          });
        }
      }
      markDebugPerformance('extensions.load-all.result', {
        extensionCount: extensionModules.size,
        actionCount: extensionActionRegistry.size,
      });
      syncFrontmostAppPolling();
    },
  );
}

function createExtensionRuntimeMetadata(extension, command) {
  return {
    ...extension,
    rename: (metadata) => renameExtension(extension, command, metadata),
  };
}

function normalizedExtensionMetadata(metadata) {
  const value =
    typeof metadata === 'string'
      ? { title: metadata, commandTitle: metadata }
      : metadata || {};
  return {
    title: value.title == null ? undefined : String(value.title).trim(),
    subtitle:
      value.subtitle == null ? undefined : String(value.subtitle).trim(),
    commandTitle:
      value.commandTitle == null
        ? undefined
        : String(value.commandTitle).trim(),
    commandSubtitle:
      value.commandSubtitle == null
        ? undefined
        : String(value.commandSubtitle).trim(),
  };
}

async function applyExtensionMetadataOverrides(extension) {
  const metadata = (await readExtensionStorage(extension)).__metadata;
  if (!metadata || typeof metadata !== 'object') return;
  applyExtensionMetadata(extension, metadata);
}

function applyExtensionMetadata(extension, metadata) {
  const normalized = normalizedExtensionMetadata(metadata);
  if (normalized.title) extension.title = normalized.title;
  if (normalized.subtitle) extension.subtitle = normalized.subtitle;
  if (!Array.isArray(extension.commands)) return;
  for (const command of extension.commands) {
    if (normalized.commandTitle) command.title = normalized.commandTitle;
    if (normalized.commandSubtitle)
      command.subtitle = normalized.commandSubtitle;
  }
}

async function renameExtension(extension, command, metadata) {
  const normalized = normalizedExtensionMetadata(metadata);
  if (
    !(
      normalized.title ||
      normalized.subtitle ||
      normalized.commandTitle ||
      normalized.commandSubtitle
    )
  )
    throw new Error(
      'rename requires a title, subtitle, commandTitle, or commandSubtitle',
    );
  await mutateExtensionStorage(extension, (current) => ({
    ...current,
    __metadata: { ...(current.__metadata || {}), ...normalized },
  }));
  applyExtensionMetadata(extension, normalized);
  if (command && normalized.commandTitle)
    command.title = normalized.commandTitle;
  if (command && normalized.commandSubtitle)
    command.subtitle = normalized.commandSubtitle;
  await loadExtensions();
  registerActionShortcuts();
  return { ok: true, title: extension.title, commandTitle: command?.title };
}

function fixturePersistentActionItems(fixture) {
  const registeredEntries = Array.from(extensionActionRegistry.values()).filter(
    (entry) => entry.extension.id === fixture.id,
  );
  return registeredEntries.map((entry) => {
    const item = entry.item;
    const persistentAction = extensionActionFromContribution(entry);
    return {
      ...item,
      id: `fixture-action:${fixture.id}:${item.id}`,
      persistentAction,
      primaryAction: persistentAction?.rootAction || item.primaryAction,
      subtitle:
        item.subtitle || `Persistent action · ${fixture.title || fixture.id}`,
      accessories: [
        ...(item.accessories || []),
        { text: entry.source === 'command' ? 'command' : 'action' },
      ],
    };
  });
}

function fixturesIndexView(ctx) {
  return ctx.ui.list({
    id: 'extension-api-fixtures',
    title: 'Extension API Fixtures',
    subtitle: 'Dev-only runnable fixtures for host-rendered extension UI',
    searchBarPlaceholder: 'Search fixture commands and persistent actions',
    emptyView: {
      title: 'No fixtures found',
      subtitle: 'Add fixture extensions under src/fixtures.',
    },
    sections: fixtureExtensions.map((fixture) => ({
      title: fixture.title || fixture.id,
      subtitle: fixture.subtitle || 'Extension API fixture',
      items: fixturePersistentActionItems(fixture),
    })),
  });
}

function fixturesRootItem(ctx) {
  const persistentItems = fixtureExtensions.flatMap((fixture) =>
    fixturePersistentActionItems(fixture),
  );
  const runnableCount = persistentItems.length;
  return {
    id: 'fixtures',
    title: 'Fixtures',
    subtitle: `${fixtureExtensions.length} dev-only extension API ${fixtureExtensions.length === 1 ? 'fixture' : 'fixtures'} · ${runnableCount} ${runnableCount === 1 ? 'item' : 'items'}`,
    icon: 'wrench',
    aliases: [
      'fixture',
      'fixtures',
      'dev fixtures',
      'extension fixtures',
      ...fixtureExtensions.map((fixture) => fixture.title || fixture.id),
      ...persistentItems.map((item) => item.title),
    ].filter(Boolean),
    score: 100,
    primaryAction: ctx.actions.push('Open Fixtures', fixturesIndexView(ctx)),
  };
}

function createFixturesExtension() {
  return {
    id: 'dev.fixtures',
    title: 'Fixtures',
    subtitle: 'Dev-only extension API fixtures',
    rootItems(ctx) {
      return [fixturesRootItem(ctx)];
    },
    searchItems(ctx) {
      return [fixturesRootItem(ctx)];
    },
  };
}

async function loadDevExtensions() {
  const fixturesDir = path.join(app.getAppPath(), 'src', 'fixtures');
  const entries = await fs
    .readdir(fixturesDir, { withFileTypes: true })
    .catch(() => []);
  for (const entry of entries) {
    if (!isExtensionSourceFile(entry.name)) continue;
    const fullPath = path.join(fixturesDir, entry.name);
    try {
      const extension = await loadExtensionModule(fullPath);
      extension.__filePath = fullPath;
      extension.__dev = true;
      extension.__fixture = true;
      fixtureExtensions.push(extension);
      registerExtension(extension);
    } catch (error) {
      logError('extension.dev.load.failed', error, {
        source: 'host',
        scope: 'extension',
        extensionId: path.basename(fullPath),
      });
    }
  }
  if (fixtureExtensions.length) registerExtension(createFixturesExtension());
}

function registerInternalExtensions() {
  for (const createExtension of INTERNAL_EXTENSION_FACTORIES)
    registerExtension(markInternalExtension(createExtension()));
  assertInternalExtensionsRegistered();
}

function durationMs(value: any) {
  if (typeof value === 'number') return Math.max(1000, value);
  const match = String(value || '')
    .trim()
    .match(/^(\d+(?:\.\d+)?)\s*(ms|s|m|h|d)?$/i);
  if (!match) return 0;
  const amount = Number(match[1]);
  const unit = (match[2] || 'ms').toLowerCase();
  const multiplier =
    unit === 'd'
      ? 86_400_000
      : unit === 'h'
        ? 3_600_000
        : unit === 'm'
          ? 60_000
          : unit === 's'
            ? 1000
            : 1;
  return Math.max(1000, Math.round(amount * multiplier));
}

function triggerPermission(_extension, _trigger: any) {
  // Host trigger registration is not a capability-enforcement boundary.
  return true;
}

function normalizedFileTrigger(trigger: any) {
  const roots = normalizeFindRoots(trigger.roots)
    .map(expandUserPath)
    .filter((root) => root && path.isAbsolute(root));
  return {
    ...trigger,
    roots,
    includeHidden: Boolean(trigger.includeHidden),
    extensions: extensionsForFindOptions(trigger) || null,
    ignored: normalizedIgnorePatterns(trigger.ignore),
  };
}

function fileWatchChangedPath(root: string, filename: string | Buffer | null) {
  if (!filename) return root;
  const value = String(filename);
  return path.isAbsolute(value) ? value : path.join(root, value);
}

function fileWatchPathMatches(filePath: string, trigger: any) {
  const name = path.basename(filePath);
  if (!trigger.includeHidden && name.startsWith('.')) return false;
  if (ignoredByPattern(filePath, name, trigger.ignored || [])) return false;
  const ext = extensionForPath(filePath);
  if (trigger.extensions && !trigger.extensions.has(ext)) return false;
  if (trigger.kind === 'image' && !isImagePath(filePath)) return false;
  if (trigger.kind === 'video' && !isVideoPath(filePath)) return false;
  if (
    trigger.kind === 'media' &&
    !isImagePath(filePath) &&
    !isVideoPath(filePath)
  )
    return false;
  return true;
}

function attachExtensionFileTrigger(
  trigger: any,
  event: string,
  extensionId: string,
) {
  const normalized = normalizedFileTrigger(trigger);
  for (const root of normalized.roots) {
    try {
      const watcher = watch(
        root,
        {
          recursive:
            process.platform === 'darwin' || process.platform === 'win32',
        },
        (_eventType, filename) => {
          const changedPath = fileWatchChangedPath(root, filename);
          if (!fileWatchPathMatches(changedPath, normalized)) return;
          jobRegistry.emit(event, {
            trigger: normalizedFileTriggerForLaunch(trigger),
            root: displayUserPath(root),
            changedPaths: [changedPath],
          });
        },
      );
      watcher.on('error', () => {});
      extensionFileWatchers.push({
        close: () => watcher.close(),
        extensionId,
        trigger,
        event,
      });
    } catch {}
  }
}

function watchExtensionFileTrigger(
  trigger: any,
  event: string,
  extensionId: string,
) {
  if (extensionRuntimePreparation) {
    extensionRuntimePreparation.fileWatchers.push({
      trigger,
      event,
      extensionId,
    });
    return;
  }
  attachExtensionFileTrigger(trigger, event, extensionId);
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
  };
}

function extensionTriggerForLaunch(trigger: any) {
  if (!trigger) return;
  if (trigger.type === 'files.changed')
    return normalizedFileTriggerForLaunch(trigger);
  return structuredClone(trigger);
}

function jobTriggersFromExtensionTriggers(
  extension,
  triggers: any[] = [],
  jobId = '',
) {
  return triggers
    .map((trigger, index) => {
      if (!triggerPermission(extension, trigger)) {
        logWarn(
          'extension.jobTrigger.permissionDenied',
          { trigger: trigger?.type },
          { source: 'host', scope: 'extension', extensionId: extension.id },
        );
        return null;
      }
      if (trigger?.type === 'startup')
        return {
          type: 'startup' as const,
          delayMs: trigger.delayMs || 0,
          payload: { trigger: extensionTriggerForLaunch(trigger) },
        };
      if (trigger?.type === 'login')
        return {
          type: 'event' as const,
          event: 'login',
          debounceMs: trigger.debounceMs || 0,
          payload: { trigger: extensionTriggerForLaunch(trigger) },
        };
      if (trigger?.type === 'wake')
        return {
          type: 'event' as const,
          event: 'wake',
          debounceMs: trigger.debounceMs || 0,
          payload: { trigger: extensionTriggerForLaunch(trigger) },
        };
      if (trigger?.type === 'interval') {
        const everyMs = durationMs(trigger.every);
        return everyMs
          ? {
              type: 'interval' as const,
              everyMs,
              delayMs: trigger.delayMs,
              payload: { trigger: extensionTriggerForLaunch(trigger) },
            }
          : null;
      }
      if (trigger?.type === 'clipboard.changed')
        return {
          type: 'event' as const,
          event: 'clipboard.changed',
          debounceMs: trigger.debounceMs || 0,
          payload: { trigger: extensionTriggerForLaunch(trigger) },
        };
      if (trigger?.type === 'app.frontmost.changed')
        return {
          type: 'event' as const,
          event: FRONTMOST_APP_CHANGED_EVENT,
          debounceMs: trigger.debounceMs || 0,
          payload: { trigger: extensionTriggerForLaunch(trigger) },
        };
      if (trigger?.type === 'files.changed') {
        const event = `files.changed:${jobId}:${index}`;
        watchExtensionFileTrigger(trigger, event, extension.id);
        return {
          type: 'event' as const,
          event,
          debounceMs: trigger.debounceMs || 0,
          payload: { trigger: extensionTriggerForLaunch(trigger) },
        };
      }
      return null;
    })
    .filter(Boolean);
}

async function extensionLaunchContextFromJob(context: any) {
  const payload =
    context?.payload && typeof context.payload === 'object'
      ? context.payload
      : {};
  const changedPaths = Array.isArray(payload.changedPaths)
    ? payload.changedPaths
        .map((value) => expandUserPath(String(value)))
        .filter(Boolean)
        .slice(-100)
    : [];
  const files = changedPaths.length
    ? await Promise.all(
        changedPaths.map((filePath) => fileToExtensionFile(filePath)),
      )
    : [];
  return structuredClone({
    trigger: payload.trigger,
    files,
    changedPaths,
    reason: context?.reason || 'manual',
    event: context?.event,
    startedAt: context?.startedAt || Date.now(),
  });
}

function registerExtensionBackgroundJob(entry, item) {
  const mode = item.mode || (item.background ? 'background' : 'view');
  const id = `extension.${entry.extension.id}.${item.id}`;
  const triggers = jobTriggersFromExtensionTriggers(
    entry.extension,
    item.triggers || [],
    id,
  );
  if (mode === 'view' && triggers.length === 0) return;
  const definition: JobDefinition = {
    id,
    title: item.title,
    owner: 'extension',
    scope: entry.extension.id,
    triggers,
    timeoutMs: Number(item.timeoutMs || EXTENSION_ROOT_ITEMS_TIMEOUT_MS),
    run: async (context) => {
      const action = item.primaryAction || item.action;
      if (!action) return;
      const launchContext = await extensionLaunchContextFromJob(context);
      const result = await executeViewAction(action, launchContext);
      if (result?.view)
        logInfo(
          'extension.background.viewIgnored',
          { jobId: id, title: item.title },
          {
            source: 'host',
            scope: 'extension',
            extensionId: entry.extension.id,
            commandId: item.id,
          },
        );
    },
  };
  if (extensionRuntimePreparation)
    extensionRuntimePreparation.jobs.push(definition);
  else jobRegistry.register(definition);
}

function assertInternalExtensionsRegistered() {
  const missingExtensions = REQUIRED_INTERNAL_EXTENSIONS.filter(
    (extensionId) => !extensionModules.has(extensionId),
  );
  const missingCommands = REQUIRED_INTERNAL_COMMANDS.filter(
    ({ extensionId, commandId }) =>
      !extensionActionRegistry.has(`${extensionId}:${commandId}`),
  );
  if (missingExtensions.length || missingCommands.length) {
    const details = [
      missingExtensions.length
        ? `extensions: ${missingExtensions.join(', ')}`
        : '',
      missingCommands.length
        ? `commands: ${missingCommands.map(({ extensionId, commandId }) => `${extensionId}:${commandId}`).join(', ')}`
        : '',
    ]
      .filter(Boolean)
      .join('; ');
    throw new Error(`Missing required internal extensions (${details})`);
  }
}

function registerExtension(extension) {
  measureDebugPerformanceSync(
    'extension.register',
    {
      extensionId: extension?.id,
      commandCount: extension?.commands?.length || 0,
    },
    () => {
      if (!extension?.id) return;
      extensionModules.set(extension.id, extension);
      for (const command of extension.commands || []) {
        if (
          !(command?.id && command.title) ||
          typeof command.run !== 'function'
        )
          continue;
        const entry = { extension, command, source: 'command' };
        const action = {
          type: 'runExtensionAction',
          title: command.title,
          __handler: async (ctx, actionArg) => command.run(ctx, actionArg),
        };
        const item = {
          id: command.id,
          actionId:
            command.actionId || extensionCommandActionId(extension, command),
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
          background:
            command.background ||
            command.mode === 'background' ||
            command.mode === 'noView',
          mode: command.mode,
          triggers: command.triggers,
          primaryAction: action,
        };
        const normalizedItem = normalizeViewItems([item], entry)[0];
        extensionActionRegistry.set(`${extension.id}:${command.id}`, {
          ...entry,
          item: normalizedItem,
        });
        registerExtensionBackgroundJob(entry, normalizedItem);
      }
      if (typeof extension.actions === 'function') {
        try {
          const result = extension.actions(
            createExtensionContext(extension, null),
          );
          const items = Array.isArray(result)
            ? result
            : Array.isArray(result?.actions)
              ? result.actions
              : [];
          const entry = {
            extension,
            command: { id: 'actions', title: extension.title || extension.id },
            source: 'action',
          };
          for (const item of items) {
            if (!(item?.id && item.title)) continue;
            const action = item.run
              ? {
                  type: 'runExtensionAction',
                  title: item.title,
                  __handler: item.run,
                }
              : item.action;
            const normalizedItem = normalizeViewItems(
              [
                {
                  ...item,
                  background:
                    item.background ||
                    item.mode === 'background' ||
                    item.mode === 'noView',
                  primaryAction: action,
                },
              ],
              entry,
            )[0];
            extensionActionRegistry.set(`${extension.id}:${item.id}`, {
              ...entry,
              item: normalizedItem,
            });
            registerExtensionBackgroundJob(entry, normalizedItem);
          }
        } catch (error) {
          logError('extension.actions.failed', error, {
            source: 'host',
            scope: 'extension',
            extensionId: extension.id,
          });
        }
      }
    },
  );
}

function displayUserPath(filePath) {
  const home = os.homedir();
  return filePath.startsWith(home)
    ? `~${filePath.slice(home.length)}`
    : filePath;
}

const DEFAULT_FILE_INDEX_IGNORES = [
  'node_modules',
  '.git',
  'Library',
  'Applications',
];
const DEFAULT_FILE_INDEX_LIMIT = 5000;
const MAX_FILE_INDEX_LIMIT = 20_000;

function defaultFileIndexRoots() {
  return ['Desktop', 'Documents', 'Downloads'].map((name) =>
    path.join(os.homedir(), name),
  );
}

function normalizedIndexRoots(options: any = {}) {
  const roots = normalizeFindRoots(options.roots);
  return (roots.length ? roots : defaultFileIndexRoots())
    .map(expandUserPath)
    .filter((root) => root && path.isAbsolute(root));
}

function normalizedIgnorePatterns(ignore) {
  const values = Array.isArray(ignore) ? ignore : ignore ? [ignore] : [];
  return [...DEFAULT_FILE_INDEX_IGNORES, ...values]
    .map((value) => String(value).trim())
    .filter(Boolean);
}

function wildcardPatternMatches(pattern, value) {
  if (!pattern.includes('*')) return false;
  const escaped = pattern
    .split('*')
    .map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
    .join('.*');
  return new RegExp(`^${escaped}$`, 'i').test(value);
}

function ignoredByPattern(fullPath, name, patterns) {
  return patterns.some(
    (pattern) =>
      pattern === name ||
      wildcardPatternMatches(pattern, name) ||
      fullPath.includes(pattern),
  );
}

async function scanFiles(options: any = {}) {
  return measureDebugPerformance(
    'files.scan',
    {
      roots: normalizedIndexRoots(options).map(displayUserPath),
      depth: options.depth ?? 2,
      limit: options.limit || DEFAULT_FILE_INDEX_LIMIT,
      alwaysLog: true,
    },
    async () => {
      const roots = normalizedIndexRoots(options);
      const ignored = normalizedIgnorePatterns(options.ignore);
      const includeHidden = Boolean(options.includeHidden);
      const maxDepth = options.depth ?? 2;
      const limit = Math.max(
        1,
        Math.min(
          Number(options.limit || DEFAULT_FILE_INDEX_LIMIT),
          MAX_FILE_INDEX_LIMIT,
        ),
      );
      const extensions = extensionsForFindOptions(options);
      const found = [];

      async function walk(dir, depth) {
        if (found.length >= limit) return;
        let entries = [];
        try {
          entries = await fs.readdir(dir, { withFileTypes: true });
        } catch {
          return;
        }

        for (const entry of entries) {
          if (found.length >= limit) return;
          if (!includeHidden && entry.name.startsWith('.')) continue;
          const fullPath = path.join(dir, entry.name);
          if (ignoredByPattern(fullPath, entry.name, ignored)) continue;
          if (entry.isFile()) {
            const ext = extensionForPath(entry.name);
            if (extensions && !extensions.has(ext)) continue;
            found.push(fileCandidate(fullPath, entry.name));
            continue;
          }
          if (entry.isDirectory() && depth > 0) await walk(fullPath, depth - 1);
        }
      }

      const { existing, missing } = await partitionRootsByExistence(roots);
      for (const root of missing)
        logWarn(
          'files.scan.missingRoot',
          { root: displayUserPath(root) },
          { source: 'host', scope: 'files' },
        );

      await Promise.all(existing.map((root) => walk(root, maxDepth)));
      await attachFileStats(found);
      await attachDateAdded(found);
      const sorted = sortFoundFiles(found, {
        sortBy: options.sortBy || options.sort || 'added',
        order: options.order,
      });
      markDebugPerformance('files.scan.result', {
        foundCount: sorted.length,
        limit,
      });
      return sorted.slice(0, limit).map((file) => ({
        ...file,
        url:
          thumbnailUrlForPreviewablePath(file.path) ||
          fileUrlForPath(file.path),
        fileUrl: fileUrlForPath(file.path),
        videoUrl: isVideoPath(file.path) ? fileUrlForPath(file.path) : null,
        thumbnailUrl: thumbnailUrlForPreviewablePath(file.path),
      }));
    },
  );
}

async function indexFiles() {
  await measureDebugPerformance(
    'files.index',
    { alwaysLog: true },
    async () => {
      try {
        fileIndex = await scanFiles();
        markDebugPerformance('files.index.result', {
          indexedCount: fileIndex.length,
        });
      } catch (error) {
        logError('files.index.failed', error, {
          source: 'host',
          scope: 'files',
        });
      }
    },
  );
}

async function pollFrontmostAppChange() {
  if (!hasCapability('frontmost-app')) return;
  const current: any = await frontmostApp();
  const currentId = current?.bundleId || current?.path || current?.name || '';
  if (!currentId || currentId === frontmostWatcherLastId) return;
  frontmostWatcherLastId = currentId;
  jobRegistry.emit(FRONTMOST_APP_CHANGED_EVENT);
}

function stopFrontmostAppPolling() {
  jobRegistry.unregister(FRONTMOST_APP_POLL_JOB_ID);
  frontmostWatcherLastId = '';
}

function syncFrontmostAppPolling() {
  const hasSubscriber = hasEnabledExtensionEventSubscriber(
    jobRegistry.snapshot(),
    FRONTMOST_APP_CHANGED_EVENT,
  );
  if (!hasSubscriber) return stopFrontmostAppPolling();
  if (jobRegistry.has(FRONTMOST_APP_POLL_JOB_ID)) return;
  jobRegistry.register({
    id: FRONTMOST_APP_POLL_JOB_ID,
    title: 'Frontmost App Poll',
    owner: 'host',
    scope: 'apps',
    triggers: [{ type: 'interval', everyMs: 5000, delayMs: 5000 }],
    timeoutMs: 3000,
    run: pollFrontmostAppChange,
  });
}

function registerHostJobs() {
  jobRegistry.register({
    id: 'state.save',
    title: 'Save User State',
    owner: 'host',
    scope: 'state',
    timeoutMs: 5000,
    run: saveUserState,
  });
  jobRegistry.register({
    id: 'apps.index',
    title: 'Application Index',
    owner: 'host',
    scope: 'apps',
    triggers: [
      { type: 'startup', delayMs: 100 },
      {
        type: 'event',
        event: 'apps.changed',
        debounceMs: APP_REINDEX_DEBOUNCE_MS,
      },
    ],
    timeoutMs: 30_000,
    run: appIndexService.indexApplications,
  });
  jobRegistry.register({
    id: 'files.index',
    title: 'File Index',
    owner: 'host',
    scope: 'files',
    triggers: [{ type: 'startup', delayMs: 200 }],
    timeoutMs: 30_000,
    run: indexFiles,
  });
  jobRegistry.register({
    id: 'cache.app-icons',
    title: 'App Icon Cache',
    owner: 'host',
    scope: 'cache',
    timeoutMs: 15_000,
    run: appIconCache.processPending,
  });
  jobRegistry.register({
    id: 'cache.thumbnails',
    title: 'Thumbnail Cache',
    owner: 'host',
    scope: 'cache',
    timeoutMs: 20_000,
    run: processPendingThumbnails,
  });
}

async function ensureLocalFileUrlSecret() {
  const secretPath = path.join(
    app.getPath('userData'),
    'local-file-url-secret',
  );
  const existing = await fs.readFile(secretPath, 'utf8').catch(() => '');
  const secret =
    existing.trim() || crypto.randomBytes(32).toString('base64url');
  if (!existing.trim()) {
    await fs.mkdir(path.dirname(secretPath), { recursive: true });
    await fs.writeFile(secretPath, secret, { mode: 0o600 });
  }
  configureLocalFileUrlSecret(secret);
}

async function loadUserState() {
  const cacheRoot = osCacheRoot();
  await ensureLocalFileUrlSecret();
  statePath = path.join(app.getPath('userData'), 'state.json');
  learningRulesPath = path.join(
    app.getPath('userData'),
    LEARNING_RULES_FILENAME,
  );
  legacyLearningRulesPath = path.join(
    app.getPath('userData'),
    LEGACY_LEARNING_RULES_FILENAME,
  );
  learningTracesPath = path.join(
    app.getPath('userData'),
    LEARNING_TRACES_FILENAME,
  );
  iconCacheDir = path.join(cacheRoot, 'icons');
  clipboardImagesDir = path.join(app.getPath('userData'), 'clipboard-images');
  extensionsDir = path.join(app.getPath('userData'), 'extensions');
  extensionStorageDir = path.join(app.getPath('userData'), 'extension-storage');
  extensionCacheDir = path.join(cacheRoot, 'extension-storage');
  learningStore = new LocalLearningStore({
    tracesPath: learningTracesPath,
    learningsPath: learningRulesPath,
    legacyLearningsPath: legacyLearningRulesPath,
  });
  await learningStore.load();
  await fs
    .rm(path.join(app.getPath('userData'), 'icon-cache'), {
      recursive: true,
      force: true,
    })
    .catch(() => {});

  try {
    const loaded = JSON.parse(await fs.readFile(statePath, 'utf8'));
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
      rateCache: loaded.rateCache || {},
      extensionManager: loaded.extensionManager || {
        schemaVersion: 0,
        files: {},
        proposals: {},
      },
      nevermindEnvironment: loaded.nevermindEnvironment || {
        environment: nevermindEnvironmentForBaseUrl(
          getDefaultNevermindBaseUrl(),
        ),
        baseUrl: getDefaultNevermindBaseUrl(),
      },
    };
  } catch {
    // First run.
  }

  migrateAiChats();
  clipboardHistory = await normalizeClipboardHistory(
    userState.clipboardHistory,
    CLIPBOARD_LIMIT,
    async (png, hash) => {
      const imagePath = path.join(clipboardImagesDir, `${hash}.png`);
      try {
        await fs.mkdir(clipboardImagesDir, { recursive: true });
        await fs.writeFile(imagePath, png);
      } catch {
        // Keep the legacy item even when its image cannot be migrated.
      }
      return imagePath;
    },
  );
  jobRegistry.hydrateEnabled(userState.jobSettings?.enabled || {});
}

function migrateAiChats() {
  for (const chat of Object.values(userState.aiChats || {}) as any[]) {
    if (chat.generatedExtensionFile && !chat.touchedExtensionFiles)
      chat.touchedExtensionFiles = [chat.generatedExtensionFile];
    if (chat.generatedExtensionFile && !chat.contextExtensionFile)
      chat.contextExtensionFile = chat.generatedExtensionFile;
  }
}

async function persistClipboardImage(png, hash) {
  return clipboardService!.persistClipboardImage(png, hash);
}

function readClipboardItem() {
  return clipboardService!.readClipboardItem();
}

function rememberClipboardItem(item) {
  clipboardService!.rememberClipboardItem(item);
}

function scheduleSaveState() {
  if (jobRegistry.has('state.save')) {
    jobRegistry.schedule('state.save', 'state.changed', 200);
    return;
  }
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = setTimeout(saveUserState, 200);
  saveTimer.unref?.();
}

async function saveUserState() {
  try {
    await persistUserState();
  } catch (error) {
    logError('state.save.failed', error, { source: 'host', scope: 'state' });
  }
}

async function atomicWriteFile(
  targetPath: string,
  contents: string | Uint8Array,
) {
  await fs.mkdir(path.dirname(targetPath), { recursive: true });
  const temporaryPath = `${targetPath}.${process.pid}.${crypto.randomUUID()}.tmp`;
  try {
    await fs.writeFile(temporaryPath, contents);
    await fs.rename(temporaryPath, targetPath);
  } finally {
    await fs.unlink(temporaryPath).catch(() => {});
  }
}

function persistUserState() {
  const write = stateWriteQueue.then(writeCurrentUserState);
  stateWriteQueue = write.catch(() => {});
  return write;
}

async function writeCurrentUserState() {
  userState.clipboardHistory = clipboardHistory;
  userState.jobSettings = {
    ...(userState.jobSettings || {}),
    enabled: jobRegistry.enabledOverridesSnapshot(),
  };
  await atomicWriteFile(statePath, JSON.stringify(userState, null, 2));
}

function pollClipboardChange() {
  return clipboardService!.pollClipboardChange();
}

async function startClipboardWatcher() {
  const watcherJob = await clipboardService!.startClipboardWatcher();
  jobRegistry.register(watcherJob);
}

function unregisterShortcutForAction(actionId) {
  const current = userState.shortcuts[actionId];
  if (current) globalShortcut.unregister(current);
}

async function executeShortcutAction(action) {
  const currentAction = currentActionForStoredShortcut(action);
  const wasVisible = Boolean(paletteWindow.win?.isVisible());
  if (!wasVisible) {
    paletteWindow.showPalette({ skipShownEvent: true, deferReveal: true });
    paletteWindow.win?.webContents.send(
      'action:view-open',
      normalizeHostViewResult({
        view: progressView({
          title: currentAction?.title || 'Opening...',
          label: 'Opening...',
        }),
        revealWhenReady: true,
        asSibling: false,
      }),
    );
  }
  const result = normalizeHostViewResult(
    await executeAction(currentAction, { keepPaletteOpen: true }),
  );
  if (result?.view) {
    if (wasVisible) paletteWindow.showPalette({ skipShownEvent: true });
    paletteWindow.win?.webContents.send('action:view-open', {
      ...result,
      revealWhenReady: false,
      asSibling: wasVisible,
    });
  } else if (!wasVisible) {
    paletteWindow.hidePalette();
  }
}

function bindGlobalActionShortcut(actionId, accelerator, action) {
  if (accelerator === getPaletteHotkey()) return false;
  globalShortcut.unregister(accelerator);
  registeredActionAccelerators.add(accelerator);
  const ok = globalShortcut.register(accelerator, () =>
    executeShortcutAction(action),
  );
  paletteWindow.debugLog('registerActionShortcut', {
    actionId,
    accelerator,
    title: action?.title,
    ok,
    isRegistered: globalShortcut.isRegistered(accelerator),
  });
  return ok;
}

function registerActionShortcut(actionId, accelerator, action) {
  const ok = bindGlobalActionShortcut(actionId, accelerator, action);
  if (!ok) return false;
  userState.shortcuts[actionId] = accelerator;
  userState.shortcutActions[actionId] = action;
  invalidateShortcutCaches();
  return true;
}

function declaredGlobalShortcuts() {
  return visibleExtensionActionEntries()
    .map((entry) => {
      const accelerator =
        entry.item.globalShortcut ||
        (entry.item.shortcutScope === 'global' ? entry.item.shortcut : null);
      if (!accelerator) return null;
      const action = extensionActionFromContribution(entry);
      return action
        ? {
            actionId: action.id,
            accelerator: normalizeAccelerator(accelerator),
            action,
          }
        : null;
    })
    .filter(Boolean);
}

function unregisterActionShortcuts() {
  for (const accelerator of registeredActionAccelerators)
    globalShortcut.unregister(accelerator);
  registeredActionAccelerators.clear();
}

function registerActionShortcuts() {
  unregisterActionShortcuts();
  const bound = new Set();
  for (const [actionId, accelerator] of Object.entries(userState.shortcuts)) {
    const action = currentActionForStoredShortcut(
      userState.shortcutActions[actionId],
    );
    if (!action) continue;
    const ok = bindGlobalActionShortcut(actionId, accelerator, action);
    if (ok) bound.add(accelerator);
    else
      logWarn(
        'actionShortcut.register.failed',
        { actionId, accelerator },
        { source: 'host', scope: 'shortcuts' },
      );
  }
  for (const { actionId, accelerator, action } of declaredGlobalShortcuts()) {
    if (
      userState.shortcuts[actionId] ||
      userState.removedShortcuts?.[actionId] ||
      bound.has(accelerator)
    )
      continue;
    const ok = bindGlobalActionShortcut(actionId, accelerator, action);
    if (ok) bound.add(accelerator);
    else
      logWarn(
        'declaredActionShortcut.register.failed',
        { actionId, accelerator },
        { source: 'host', scope: 'shortcuts' },
      );
  }
}

function suspendPaletteHotkey() {
  globalShortcut.unregister(String(getPaletteHotkey()));
}

function resumePaletteHotkey() {
  globalShortcut.unregister(String(getPaletteHotkey()));
  paletteWindow.registerHotkey();
}

function canCustomizeAction(action) {
  return canCustomizeCommandAction(action);
}

function getShortcuts() {
  const configured = Object.entries(userState.shortcuts)
    .map(([actionId, accelerator]) => ({
      actionId,
      accelerator: String(accelerator),
      scope: 'global',
      source: 'user' as const,
      action: currentActionForStoredShortcut(
        userState.shortcutActions[actionId],
      ),
    }))
    .filter((item) => item.action);
  const declared = declaredGlobalShortcuts()
    .filter(
      (item) =>
        !(
          userState.shortcuts[item.actionId] ||
          userState.removedShortcuts?.[item.actionId]
        ),
    )
    .map((item) => ({
      ...item,
      accelerator: String(item.accelerator),
      scope: 'global' as const,
      source: 'extension' as const,
    }));
  return [...configured, ...declared].sort((a, b) =>
    a.action.title.localeCompare(b.action.title),
  );
}

function extensionShortcutRecords() {
  return getShortcuts().map((item) => ({
    actionId: item.actionId,
    title: item.action.title,
    subtitle: item.action.subtitle,
    accelerator: item.accelerator,
    scope: 'global' as const,
    source: item.source,
  }));
}

async function removeShortcut(actionId) {
  if (!actionId) return { ok: false, message: 'Shortcut not found' };
  if (userState.shortcuts[actionId]) {
    globalShortcut.unregister(userState.shortcuts[actionId]);
    delete userState.shortcuts[actionId];
    delete userState.shortcutActions[actionId];
  } else {
    const declared = declaredGlobalShortcuts().find(
      (item) => item.actionId === actionId,
    );
    if (!declared) return { ok: false, message: 'Shortcut not found' };
    globalShortcut.unregister(declared.accelerator);
    if (!userState.removedShortcuts) userState.removedShortcuts = {};
    userState.removedShortcuts[actionId] = declared.accelerator;
  }
  invalidateShortcutCaches();
  scheduleSaveState();
  patchKeyboardShortcutsView();
  return { ok: true, message: 'Shortcut removed' };
}

async function setAlias(action, alias) {
  if (!canCustomizeAction(action))
    return {
      ok: false,
      message: 'Aliases are only available for persistent actions',
    };
  if (!(action?.id && alias.trim()))
    return { ok: false, message: 'Missing alias' };
  const aliases = new Set(actionAliases(action.id));
  aliases.add(alias.trim());
  userState.aliases[action.id] = Array.from(aliases);
  scheduleSaveState();
  return { ok: true, message: `Alias set: ${alias.trim()}` };
}

async function removeAlias(action, alias) {
  if (!(action?.id && alias)) return { ok: false, message: 'Missing alias' };
  const current = actionAliases(action.id).filter((value) => value !== alias);
  if (current.length) userState.aliases[action.id] = current;
  else delete userState.aliases[action.id];
  scheduleSaveState();
  return { ok: true, message: `Alias removed: ${alias}` };
}

async function setShortcut(action, shortcut) {
  if (!canCustomizeAction(action))
    return {
      ok: false,
      message: 'Shortcuts are only available for persistent actions',
    };
  if (!(action?.id && shortcut.trim()))
    return { ok: false, message: 'Missing shortcut' };
  const accelerator = normalizeAccelerator(shortcut);
  if (accelerator === getPaletteHotkey())
    return {
      ok: false,
      message: `${accelerator} is reserved for opening Nevermind`,
    };
  const conflictingActionId = Object.entries(userState.shortcuts).find(
    ([actionId, current]) => actionId !== action.id && current === accelerator,
  )?.[0];
  if (conflictingActionId) {
    const conflictingAction = userState.shortcutActions[conflictingActionId];
    if (
      action.aiChatId &&
      conflictingAction?.aiChatId === action.aiChatId &&
      chatTouchedExtensionFiles(userState.aiChats[action.aiChatId]).length === 1
    ) {
      await removeShortcut(conflictingActionId);
    } else {
      const title = conflictingAction?.title || 'another action';
      return {
        ok: false,
        message: `${accelerator} is already used by ${title}`,
      };
    }
  }
  unregisterShortcutForAction(action.id);
  delete userState.removedShortcuts?.[action.id];
  const ok = registerActionShortcut(action.id, accelerator, action);
  if (!ok) return { ok: false, message: `Could not register ${accelerator}` };
  scheduleSaveState();
  patchKeyboardShortcutsView();
  return { ok: true, message: `Shortcut set: ${accelerator}` };
}

function setShortcutSetting(id, accelerator) {
  const definition = settingDefinition(id);
  if (!definition || definition.type !== 'shortcut')
    return { ok: false, message: 'Setting not found' };
  if (id === 'paletteHotkey')
    return { ok: false, message: 'Use palette hotkey registration' };
  if (!accelerator?.trim()) return { ok: false, message: 'Missing shortcut' };
  const normalized = normalizeAccelerator(accelerator);
  setSetting(id, normalized);
  return { ok: true, message: `${definition.title} set: ${normalized}` };
}

async function setPaletteHotkey(accelerator) {
  if (!accelerator?.trim()) return { ok: false, message: 'Missing shortcut' };
  const normalized = normalizeAccelerator(accelerator);
  const current = String(getPaletteHotkey());
  if (normalized === current)
    return {
      ok: true,
      message: `Shortcut unchanged: ${normalized}`,
      spotlightConflict: isSpotlightAccelerator(normalized),
    };
  const conflictingActionId = Object.entries(userState.shortcuts).find(
    ([, value]) => value === normalized,
  )?.[0];
  if (conflictingActionId) {
    const title =
      userState.shortcutActions[conflictingActionId]?.title || 'another action';
    return { ok: false, message: `${normalized} is already used by ${title}` };
  }
  globalShortcut.unregister(current);
  const ok = globalShortcut.register(normalized, paletteWindow.togglePalette);
  if (!ok) {
    globalShortcut.register(current, paletteWindow.togglePalette);
    const spotlightConflict = isSpotlightAccelerator(normalized);
    return {
      ok: false,
      message: spotlightConflict
        ? `${normalized} is used by ${reservedPaletteShortcutName()}`
        : `Could not register ${normalized}`,
      spotlightConflict,
    };
  }
  setSetting('paletteHotkey', normalized);
  return {
    ok: true,
    message: `Shortcut set: ${normalized}`,
    spotlightConflict: isSpotlightAccelerator(normalized),
  };
}

async function openSystemKeyboardSettings() {
  await executeSystemBuiltin({ builtin: 'open-keyboard-settings' }, () => {});
  return { ok: true };
}

async function setOverride(action, instruction) {
  const defaultActionId = defaultActionIdFor(action);
  if (!defaultActionId)
    return { ok: false, message: 'This action cannot be overridden yet' };
  if (!instruction.trim())
    return { ok: false, message: 'Missing override instructions' };
  userState.overrides[defaultActionId] = {
    instruction: instruction.trim(),
    updatedAt: Date.now(),
    originalTitle: action.title,
  };
  scheduleSaveState();
  return { ok: true, message: 'Override saved' };
}

async function clearOverride(action) {
  const defaultActionId = defaultActionIdFor(action);
  if (!defaultActionId)
    return { ok: false, message: 'This action has no original to restore' };
  delete userState.overrides[defaultActionId];
  scheduleSaveState();
  return { ok: true, message: 'Original restored' };
}

async function duplicateCreatedAction(action) {
  if (
    !(
      ['extension-root-item', 'extension-action'].includes(action?.kind) &&
      action.removable
    )
  )
    return {
      ok: false,
      message: 'Only generated extensions can be duplicated',
    };
  const extension = extensionModuleForAction(action);
  const filePath = extension?.__filePath;
  if (!filePath) return { ok: false, message: 'Generated extension not found' };

  const duplicateId = hashValue(`${filePath}:${Date.now()}`);
  const duplicateExtensionId = `${extension.id}-copy-${duplicateId.slice(0, 8)}`;
  const duplicateTitle = `Copy of ${extension.title || action.title}`;
  const duplicateFile = `${extensionSourceBasename(filePath)}-copy-${duplicateId.slice(0, 8)}.ts`;
  const sourceCode = createStandaloneExtensionFork(
    await fs.readFile(filePath, 'utf8'),
    { id: duplicateExtensionId, title: duplicateTitle },
  );
  await stageExtensionProposal(duplicateFile, sourceCode);
  await activateManagedExtension(duplicateFile);
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
      {
        role: 'system',
        content: `Duplicated from "${extension.title || action.title}". Tweak this copy without changing the original.`,
      },
    ],
  };
  scheduleSaveState();
  invalidateExtensionRootItems();
  await loadExtensions();
  registerActionShortcuts();
  const targetRegisteredActionId =
    action.registeredActionId || action.commandId;
  const duplicateEntry = targetRegisteredActionId
    ? extensionActionRegistry.get(
        `${duplicateExtensionId}:${targetRegisteredActionId}`,
      )
    : Array.from(extensionActionRegistry.values()).find(
        (candidate) => candidate.extension?.id === duplicateExtensionId,
      );
  return {
    ok: true,
    message: 'Action duplicated',
    action: duplicateEntry
      ? extensionActionFromContribution(duplicateEntry)
      : {
          id: `ai-tweak-extension:${duplicateFile}`,
          kind: 'ai-tweak-extension',
          extensionFile: duplicateFile,
          title: duplicateTitle,
          subtitle: 'Tweak extension with AI',
          icon: 'sparkles',
          score: 0,
        },
  };
}

async function removeAiChat(chatId) {
  if (!(chatId && userState.aiChats[chatId]))
    return { toast: { message: 'AI chat not found', tone: 'error' } };
  await nevermindAi?.reset?.(chatId);
  const chat = userState.aiChats[chatId];
  // INVARIANT: removing a chat deletes only conversation history and AI session state.
  // It must NEVER unlink generated extension files. Generated extensions are durable
  // artifacts owned by chats via touchedExtensionFiles; chat removal preserves them so
  // the user can keep the extension after discarding the conversation that built it.
  delete userState.aiChats[chatId];
  for (const actionId of Object.keys(userState.recents || {})) {
    if (actionId === `ai-chat:${chatId}`) delete userState.recents[actionId];
  }
  scheduleSaveState();
  invalidateExtensionRootItems();
  patchAiChatsRemove(chatId);
  return {
    toast: { message: `Removed ${chat.title || chat.query || 'AI chat'}` },
  };
}

async function removeAiChatReferencesToExtensionFile(
  extensionFile,
  preserveChatId?: string,
) {
  const removedFile = path.basename(extensionFile || '');
  if (!removedFile) return;
  for (const chat of Object.values(userState.aiChats || {}) as any[]) {
    const touchedFiles = chatTouchedExtensionFiles(chat);
    if (!touchedFiles.includes(removedFile)) continue;
    const remainingFiles = [] as string[];
    for (const filename of touchedFiles.filter(
      (item) => item !== removedFile,
    )) {
      const exists = await fs
        .stat(path.join(extensionsDir, filename))
        .then(() => true)
        .catch(() => false);
      if (exists) remainingFiles.push(filename);
    }
    if (remainingFiles.length === 0 && chat.id !== preserveChatId) {
      await removeAiChat(chat.id);
      continue;
    }
    chat.touchedExtensionFiles = remainingFiles;
    if (remainingFiles.length === 0) {
      delete chat.contextExtensionFile;
      delete chat.generatedExtensionFile;
    }
    if (chat.contextExtensionFile === removedFile)
      chat.contextExtensionFile = remainingFiles[0];
    if (chat.generatedExtensionFile === removedFile)
      chat.generatedExtensionFile = remainingFiles[0];
    chat.updatedAt = Date.now();
    scheduleSaveState();
  }
}

async function removeCreatedAction(action) {
  if (action?.kind === 'ai-chat' && action.aiChatId) {
    await removeAiChat(action.aiChatId);
    delete userState.recents[action.id];
    return { ok: true, message: 'AI chat removed' };
  }

  if (
    ['extension-root-item', 'extension-action'].includes(action?.kind) &&
    action.removable
  ) {
    const extension = extensionModuleForAction(action);
    const filePath = extension?.__filePath;
    if (!filePath)
      return { ok: false, message: 'This extension cannot be removed' };
    await fs.unlink(filePath).catch((error) => {
      if (error?.code !== 'ENOENT') throw error;
    });
    await removeAiChatReferencesToExtensionFile(path.basename(filePath));
    delete userState.recents[action.id];
    scheduleSaveState();
    await loadExtensions();
    registerActionShortcuts();
    return { ok: true, message: 'Generated extension removed' };
  }

  return { ok: false, message: 'This action cannot be removed' };
}

async function runPaletteDebugCli() {
  await appIndexService.indexApplications();
  await indexFiles();
  const query = String(process.env.NVM_PALETTE_QUERY || '');
  const actions = await searchActions(query);
  const selected = process.env.NVM_PALETTE_EXECUTE
    ? actions.find(
        (action) =>
          action.id === process.env.NVM_PALETTE_EXECUTE ||
          action.title === process.env.NVM_PALETTE_EXECUTE,
      )
    : null;
  const result = selected ? await executeActionForIpc(selected) : undefined;
  console.log(
    JSON.stringify(
      { query, count: actions.length, actions, selected, result },
      null,
      2,
    ),
  );
}

async function pickFormFieldPaths(event, input: any = {}) {
  const senderWindow =
    BrowserWindow.fromWebContents(event.sender) ||
    paletteWindow.win ||
    undefined;
  const type =
    input.type === 'folder'
      ? 'folder'
      : input.type === 'files'
        ? 'files'
        : 'file';
  const properties: Array<
    'openFile' | 'openDirectory' | 'multiSelections' | 'createDirectory'
  > = type === 'folder' ? ['openDirectory'] : ['openFile'];
  if (type === 'files') properties.push('multiSelections');
  if (type === 'folder' && input.canCreateDirectories !== false)
    properties.push('createDirectory');
  const filters =
    Array.isArray(input.extensions) && input.extensions.length
      ? [
          {
            name: input.filterName || 'Allowed files',
            extensions: input.extensions
              .map((value) => String(value).replace(/^\./, ''))
              .filter(Boolean),
          },
        ]
      : undefined;
  const result = await dialog.showOpenDialog(senderWindow, {
    title:
      input.title ||
      (type === 'folder'
        ? 'Choose Folder'
        : type === 'files'
          ? 'Choose Files'
          : 'Choose File'),
    buttonLabel: input.buttonLabel || 'Choose',
    properties,
    filters,
    defaultPath:
      typeof input.defaultPath === 'string'
        ? expandUserPath(input.defaultPath)
        : undefined,
  });
  return result.canceled
    ? { canceled: true, paths: [] }
    : { canceled: false, paths: result.filePaths };
}

app.whenReady().then(async () => {
  appReady = true;
  if (isNvmTestMode) {
    installTestNetworkPolicy();
    await loadUserState();
    await loadExtensions();
    registerTestModeIpcHandlers();
    paletteWindow.createWindow();
    paletteWindow.showPaletteWhenReady();
    return;
  }
  nativeTheme.themeSource = 'dark';
  prepareAppWindowPolicy();
  registerLocalFileProtocol();
  installPermissionHandlers(isDev, rendererUrl, rendererIndexPath);
  updateManager.configure();
  updateManager.onStateChange(() => {
    patchUpdatesView();
    invalidateExtensionRootItems();
  });
  onNevermindCompatibilityChanged(() => invalidateExtensionRootItems());

  await loadUserState();
  extensionPrSubmitter = createExtensionPrSubmitter({
    execFileText,
    extensionsDir,
    repoOwner: 'pablopunk',
    repoName: 'nvm',
    logInfo,
    logWarn,
  });
  setActiveNevermindAuthBaseUrl(selectedNevermindEnvironment().baseUrl);
  const [storedNevermindAuth] = await Promise.all([
    getNevermindAuth(),
    getByoKey(),
  ]);
  activeNevermindBaseUrl = storedNevermindAuth?.baseUrl || null;
  registerHostJobs();
  jobRegistry.onChange(syncFrontmostAppPolling);
  await loadExtensions();
  if (process.env.NVM_PALETTE_DEBUG) {
    await runPaletteDebugCli();
    app.quit();
    return;
  }
  await initNevermindAi();
  initExtensionContext({ nevermindAi });
  paletteWindow.createWindow();
  paletteWindow.registerHotkey();
  flushBufferedDeepLinks();
  registerActionShortcuts();
  await startClipboardWatcher();
  await appIndexService.startWatcher();
  powerMonitor.on('resume', () => jobRegistry.emit('wake'));
  jobRegistry.emit('login');

  registerAppIpcHandlers({
    ipcMain,
    measureDebugPerformance,
    summarizeDebugValue,
    searchActions,
    executeActionForIpc,
    executeViewActionForIpc,
    refreshViewForIpc,
    pickFormFieldPaths,
    startFileDrag,
    sendAiChatMessage,
    noteAiChatExited,
    abortAiChat,
    resetAiChat,
    setAlias,
    removeAlias,
    setShortcut,
    setPaletteHotkey,
    getSetting,
    openSystemKeyboardSettings,
    getShortcuts,
    removeShortcut,
    unregisterActionShortcuts,
    registerActionShortcuts,
    suspendPaletteHotkey,
    resumePaletteHotkey,
    setOverride,
    clearOverride,
    duplicateCreatedAction,
    removeCreatedAction,
    getOrCreateExtensionChat,
    aiChatView,
    normalizeHostViewResult,
    createDraftAiChat,
    getNevermindAuth,
    getNevermindDebugStatus,
    setActiveNevermindBaseUrl: (baseUrl) => {
      activeNevermindBaseUrl = baseUrl;
      setActiveNevermindAuthBaseUrl(baseUrl);
    },
    warmNevermindCompatibilityCache,
    logInfo,
    userDataPath: () => app.getPath('userData'),
    signInToNevermind: signInToSelectedNevermindEnvironment,
    invalidateExtensionRootItems,
    broadcastAuthChanged,
    appIconCache,
    runningAppStatus,
    paletteWindow,
    requestQuitApp,
    hasCapability,
    processPlatform: process.platform,
    getCameraMediaAccessStatus: () =>
      systemPreferences.getMediaAccessStatus('camera'),
    extensionWindowManager,
    BrowserWindow,
    logError,
    logWarn,
    loggerDebug,
    probeGh: () =>
      extensionPrSubmitter?.probe() ??
      Promise.resolve({ installed: false, authed: false }),
  });

  ipcMain.handle('view:hydrate:retry', async (_event, viewId: string) => {
    if (!viewLoaderRegistry.has(viewId)) return;
    await viewLoaderRegistry.retry(viewId);
  });
});

app.on('activate', () => paletteWindow.showPalette());
app.on('before-quit', () => {
  nevermindApp.isQuiting = true;
});
app.on('will-quit', () => {
  nevermindApp.isQuiting = true;
  runQuitCleanup();
});

if (!isDev && !isNvmTestMode) {
  const gotLock = app.requestSingleInstanceLock();
  if (gotLock) {
    app.setAsDefaultProtocolClient(DEEP_LINK_SCHEME);
    app.on('second-instance', (_event, argv) => {
      paletteWindow.showPalette();
      const deepLinkArg = argv?.find((arg: string) =>
        arg.startsWith(`${DEEP_LINK_SCHEME}://`),
      );
      if (deepLinkArg && appReady) processBufferedDeepLink(deepLinkArg);
    });
  } else app.quit();
}
app.on('open-url', (_event, url) => {
  if (appReady) processBufferedDeepLink(url);
  else bufferedDeepLinks.push(url);
});
