// biome-ignore-all lint: This Electron entry point follows established imperative startup conventions.
import { spawn } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { PaneInvocations } from './app/pane-invocations';

const paneInvocations = new PaneInvocations();
const paneInvocationWindows = new WeakSet<object>();

// Global safety net: unhandled rejections and uncaught exceptions must never
// crash the app. An extension action or shell call that throws unexpectedly
// should be logged and contained, not silently kill the entire process.
process.on('unhandledRejection', (reason) => {
  console.error('FATAL: unhandled rejection (would crash Electron):', reason);
  if (!app.isPackaged) recordOperationFailure(reason);
});
process.on('uncaughtException', (error) => {
  console.error('FATAL: uncaught exception (would crash Electron):', error);
  if (!app.isPackaged) recordOperationFailure(error);
});

import {
  app,
  BrowserWindow,
  ClipboardItem,
  clipboard,
  dialog,
  globalShortcut,
  ipcMain,
  nativeImage,
  nativeTheme,
  powerMonitor,
  protocol,
  safeStorage,
  screen,
  shell,
  systemPreferences,
} from 'electron';
import { createAiChatEventBroadcaster } from './ai/ai-chat-events';
import { createAiChatImageStorage } from './ai/ai-chat-image-storage';
import {
  type ActiveConversationRequest,
  createAiChatLifecycle,
  type PendingAiChatSend,
} from './ai/ai-chat-lifecycle';
import { createAiChatMigration } from './ai/ai-chat-migration';
import { createAiChatModelChange } from './ai/ai-chat-model-change';
import { createAiChatPrompts } from './ai/ai-chat-prompts';
import { chatTouchedExtensionFiles } from './ai/ai-chat-records';
import { createAiChatSend } from './ai/ai-chat-send';
import { createAiChatSessionTeardown } from './ai/ai-chat-session-teardown';
import { createAiChatStore } from './ai/ai-chat-store';
import { createAiChatViews } from './ai/ai-chat-views';
import { createAiCreatedActions } from './ai/ai-created-actions';
import { createAiExtensionOwnership } from './ai/ai-extension-ownership';
import { createAiLearningReview } from './ai/ai-learning-review';
import { trustedAiLimitAction } from './ai/ai-limit-action-policy';
import { createConversationAiWarmup } from './ai/conversation-ai-warmup';
import { createExtensionAiFactory } from './ai/extension-ai';
import {
  AI_BUILDER_EXTENSION_ID,
  createAiBuilderApiFactory,
} from './ai/extension-ai-builder';
import { createExtensionOwnershipApiFactory } from './ai/extension-ownership-api';
import { createNevermindAiEventHandler } from './ai/nevermind-ai-event-handler';
import { createNevermindAiInitializer } from './ai/nevermind-ai-init';
import {
  DEEP_LINK_SCHEME,
  parseAuthDeepLink,
  setDeepLinkLogger,
} from './app/deep-link';
import { getByoKey } from './backend/byo-key';
import {
  cancelNevermindDeviceSignIn,
  getNevermindAuth,
  getNevermindDashboardUrl,
  getNevermindDeviceSignInState,
  onNevermindDeviceSignInChanged,
  retryNevermindDeviceSignInBrowser,
  setActiveNevermindAuthBaseUrl,
} from './backend/nevermind-auth';
import { createNevermindAuthBroadcast } from './backend/nevermind-auth-broadcast';
import {
  onNevermindCompatibilityChanged,
  warmNevermindCompatibilityCache,
} from './backend/nevermind-compatibility';
import { createNevermindDeepLinks } from './backend/nevermind-deep-links';
import { createNevermindEnvironmentSelection } from './backend/nevermind-environment-selection';
import { createNevermindSignIn } from './backend/nevermind-sign-in';
import { createClipboardHistory } from './clipboard/clipboard-history';
import { createClipboardSearchActions } from './clipboard/clipboard-search-actions';
import { isClipboardHistoryAction } from './clipboard/clipboard-utils';
import { createClipboardWatcher } from './clipboard/clipboard-watcher';
import { createDesktopSelection } from './clipboard/desktop-selection';
import {
  recordOperationFailure,
  runOperation,
} from './diagnostics/observability';
import {
  flushDiagnosticRecords,
  initSentry,
  recentDiagnosticRecords,
  recordUpdateInstallAttempt,
  rendererFailureHandle,
  reportDiagnosticProblem,
  setDiagnosticReporting,
} from './diagnostics/sentry';
import { resolveExtensionPaths } from './extension-host/extension-paths';
import {
  configureLocalFileUrlSecret,
  expandUserPath,
  extensionForPath,
  fileUrlForPath,
  isImagePath,
  isVideoPath,
  LOCAL_FILE_PROTOCOL,
  LOCAL_THUMB_PROTOCOL,
  thumbnailUrlForPath,
} from './files/file-utils';
import { createElectronClipboardApi } from './os/electron-clipboard';
import { createLinuxX11Clipboard } from './os/linux-x11-clipboard';
import {
  configureNvmTestMode,
  installTestNetworkPolicy,
  isNvmDiagnosticTestMode,
  isNvmTestMode,
  recordTestWindowEvent,
} from './platform/test-mode';

configureNvmTestMode();
if (isNvmTestMode && process.env.NVM_TEST_USER_DATA_DIR)
  app.setPath('userData', path.resolve(process.env.NVM_TEST_USER_DATA_DIR));

import {
  INTERNAL_EXTENSION_FACTORIES,
  INTERNAL_EXTENSION_SOURCE_FILES,
} from '../extensions';
import { initExtensionContext } from '../extensions/_context';
import { createAiBuilderExtension } from '../extensions/ai-builder';
import { createActionExecutionCapabilities } from './actions/action-execution-policy';
import { createActionExecutionRecords } from './actions/action-execution-records';
import { createActionFeedback } from './actions/action-feedback';
import { createActionIpc } from './actions/action-ipc';
import { createActionIpcResolver } from './actions/action-ipc-resolution';
import { createActionRegistration } from './actions/action-registration';
import { createExtensionActionLookup } from './actions/extension-action-lookup';
import { createExtensionActionResults } from './actions/extension-action-results';
import { createExtensionRootActionNormalizer } from './actions/extension-root-action';
import { normalizeItemAppearance } from './actions/item-appearance';
import { createRootActionExecutor } from './actions/root-action-executor';
import { runInBackground } from './actions/run-in-background';
import { createShellCommands } from './actions/shell-commands';
import { createViewActionExecutor } from './actions/view-action-executor';
import { createClipboardActionHandlers } from './actions/view-action-handlers/clipboard-actions';
import { createCustomizationActionHandlers } from './actions/view-action-handlers/customization-actions';
import { createExtensionActionHandlers } from './actions/view-action-handlers/extension-actions';
import { createFileActionHandlers } from './actions/view-action-handlers/file-actions';
import { createNativeActionHandlers } from './actions/view-action-handlers/native-action';
import { createSettingActionHandlers } from './actions/view-action-handlers/settings-actions';
import { createShellActionHandlers } from './actions/view-action-handlers/shell-actions';
import { createSystemActionHandlers } from './actions/view-action-handlers/system-actions';
import { createWindowActionHandlers } from './actions/view-action-handlers/window-actions';
import { createViewNormalizer } from './actions/view-normalizer';
import { createViewResultNormalizer } from './actions/view-results';
import { readAppBundleIconPng } from './app/app-bundle-icons';
import { createAppIconCache } from './app/app-icon-cache';
import {
  createAppIndexService,
  trackFirstSeenApps,
} from './app/app-index-service';
import { createHostAppUninstallService } from './app/app-uninstall-host';
import {
  CHARACTER_RECORDS_BY_ID,
  characterCodePoints,
  characterWithSkinTone,
} from './app/emoji-symbol-catalog';
import { createExtensionRootItems } from './app/extension-root-items';
import { createFormFieldPathPicker } from './app/form-field-paths';
import { registerInstanceEvents } from './app/instance-events';
import { createPaletteSearch } from './app/palette-search';
import {
  createPaletteWindowController,
  installPermissionHandlers,
} from './app/palette-window';
import { createPortableConfigLive } from './app/portable-config-live';
import { createPortableConfigService } from './app/portable-config-service';
import { createQuitLifecycle } from './app/quit-lifecycle';
import { createPendingViewLoaderSpawner } from './app/pending-view-loader';
import { createRunningAppStatusService } from './app/running-app-status';
import {
  SETTING_DEFINITIONS,
  settingDefinition,
  settingValue,
  toggledSettingValue,
} from './app/settings';
import { createSettingsService } from './app/settings-service';
import { createTestModeIpc } from './app/test-mode-ipc';
import { createAppUpdateManager } from './app/update-manager-setup';
import { createUpdatesService } from './app/updates-service';
import {
  installExternalNavigationPolicy,
  isTrustedAppPage,
  isTrustedExtensionWindowPage,
} from './app/window-navigation-policy';
import {
  markDebugPerformance,
  measureDebugPerformance,
  measureDebugPerformanceSync,
  performanceTraces,
  recordDebugPerformance,
  summarizeDebugValue,
} from './diagnostics/debug-performance';
import { createDictationRecordings } from './dictation/dictation-recordings';
import {
  createDictationService,
  type DictationRendererReply,
} from './dictation/dictation-service';
import {
  apiDictationIsAvailable,
  cancelDictationPreparation,
  prepareDictationTranscription,
  transcribeDictationAudio,
} from './dictation/dictation-transcription';
import {
  buildPreviewItemAction,
  commandFromItem,
} from './extension-host/extension-action-builders';
import { createExtensionContextFactory } from './extension-host/extension-context';
import {
  checkAiRateLimit,
  checkRefreshBurst,
} from './extension-host/extension-rate-limits';
import { createExtensionDevFixtures } from './extension-host/extension-dev-fixtures';
import { createDiagnosticExtensionTargets } from './extension-host/extension-diagnostic-target';
import { createExtensionDraftPersistence } from './extension-host/extension-draft-persistence';
import { createExtensionDraftStore } from './extension-host/extension-draft-store';
import { createExtensionJsonStore } from './extension-host/extension-json-store';
import { createExtensionLoader } from './extension-host/extension-loader';
import { createExtensionManager } from './extension-host/extension-manager';
import { createExtensionMemoryCache } from './extension-host/extension-memory-cache';
import { createExtensionPrSubmitter } from './extension-host/extension-pr-submitter';
import { createExtensionProposals } from './extension-host/extension-proposals';
import { createExtensionRegistration } from './extension-host/extension-registration';
import { createExtensionRuntimeState } from './extension-host/extension-runtime-state';
import { createExtensionRuntimeTransaction } from './extension-host/extension-runtime-transaction';
import { createExtensionStorageService } from './extension-host/extension-storage-service';
import { createExtensionTriggers } from './extension-host/extension-triggers';
import { createExtensionWindowManager } from './extension-host/extension-window-manager';
import { createPersistentExtensionWindowRestorer } from './extension-host/extension-window-restore';
import { createExtensionWindowsState } from './extension-host/extension-windows-state';
import { createHostJobs } from './extension-host/host-jobs';
import { createOpenViewPatcher } from './extension-host/open-view-patch';
import { execFileText } from './files/exec-file-text';
import { fileToExtensionFile } from './files/extension-file';
import { createFileIndexService } from './files/file-index';
import { findFiles } from './files/find-files';
import { registerLocalFileProtocol } from './files/local-file-protocol';
import { createOpenWithApps } from './files/quick-look-and-open-with';
import { startFileDrag } from './files/start-file-drag';
import { createThumbnailService } from './files/thumbnails';
import { registerAppIpcHandlers } from './ipc/app-ipc-handlers';
import { registerDictationIpc } from './ipc/dictation-ipc';
import { createMeasuredIpcRegistrar } from './ipc/ipc-registration';
import {
  type AppFocusTarget,
  appIconSources,
  appIdentityKey,
  autoUpdatesUnavailableMessage,
  copySelectionIntoClipboard,
  runningAppPaths as detectRunningAppPaths,
  frontmostApp,
  frontmostAppFocusTarget,
  getLaunchAtLoginEnabled,
  hasCapability,
  launchApp as launchOsApp,
  osLabel,
  pasteIntoFrontmostApp,
  prepareAppWindowPolicy,
  selectedText as readAccessibilitySelectedText,
  readAppIconResourcePng,
  restoreAppFocus,
  scanApps,
  selectedFilePaths,
  setLaunchAtLoginEnabled,
  watchApps,
} from './os/os';
import { createSelectedTextReader } from './os/selected-text';
import { acquireDefaultSharedStateInstance } from './os/shared-state-instance';
import { createSystemAudioMuteCapability } from './os/system-audio';
import { createActivationStatus } from './platform/activation-status';
import { bundledResourcePath } from './platform/bundled-resource-path';
import { JobRegistry, type JobSnapshot } from './platform/jobs';
import { createMaintenanceActivity } from './platform/maintenance-activity';
import { resolveRendererUrl } from './platform/renderer-url';
import {
  configureLogger,
  error as logError,
  debug as loggerDebug,
  info as logInfo,
  warn as logWarn,
} from './platform/logger';
import { createActionRanker } from './search/action-ranker';
import {
  calculateDetailed,
  calculateRateResult,
  getUrlFromQuery,
  hashValue,
  normalize,
  parseRateExpression,
  score,
} from './search/search-utils';
import { createActionCustomization } from './shortcuts/action-customization';
import { createGlobalShortcutRegistration } from './shortcuts/global-shortcut-registration';
import { createShortcutCatalog } from './shortcuts/shortcut-catalog';
import { createShortcutHints } from './shortcuts/shortcut-hints';
import { withInheritedShortcutLifecycle } from './shortcuts/shortcut-lifecycle';
import { createShortcutRunner } from './shortcuts/shortcut-runner';
import { createWindowManagementComposition } from './app/window-management-composition';
import { createViewLoaderRegistry } from './state/data-loader';
import { LocalLearningStore } from './state/learning-store';
import { resolvePortableConfigPath } from './state/portable-config-file';
import { createStateSafeQuit } from './state/state-safe-quit';
import { createUserStateLoader } from './state/user-state-loader';
import { createUserStatePersistence } from './state/user-state-persistence';
import { createDefaultUserState } from './state/user-state-shape';
import { createUserStateStore } from './state/user-state-store';

const rendererUrl = resolveRendererUrl(process.env, app.isPackaged);
const isDev = Boolean(rendererUrl);
configureLogger(isDev);
if (!isNvmTestMode || isNvmDiagnosticTestMode) initSentry();
setDeepLinkLogger({ warn: logWarn });
const LINUX_DESKTOP_NAME = 'com.pablopunk.nvm.desktop';
if (process.platform === 'linux') {
  app.setDesktopName(LINUX_DESKTOP_NAME);
  const isWaylandSession =
    String(process.env.XDG_SESSION_TYPE || '').toLowerCase() === 'wayland' ||
    Boolean(process.env.WAYLAND_DISPLAY);
  if (isWaylandSession) {
    app.commandLine.appendSwitch('enable-features', 'GlobalShortcutsPortal');
    app.commandLine.appendSwitch(
      'disable-features',
      'WaylandFractionalScaleV1',
    );
  }
}

const updateManager: any = createAppUpdateManager({
  isTestMode: isNvmTestMode,
  recordInstallAttempt: recordUpdateInstallAttempt,
  getUpdateChannel: () => getUpdateChannel(),
});
const preloadPath = path.join(__dirname, '..', 'preload', 'preload.cjs');
const rendererIndexPath = path.join(__dirname, '..', 'renderer', 'index.html');
ipcMain.handle('diagnostics:renderer-failure', (event) => {
  if (
    event.senderFrame !== event.sender.mainFrame ||
    !isTrustedAppPage(
      event.sender.getURL(),
      isDev,
      rendererUrl,
      rendererIndexPath,
    )
  )
    return null;
  return rendererFailureHandle(event.sender.id);
});
nativeTheme.themeSource = 'system';
const paletteWindow = createPaletteWindowController({
  isDev: Boolean(rendererUrl),
  preloadPath,
  rendererUrl,
  rendererIndexPath,
  getPaletteHotkey: () => String(getPaletteHotkey()),
  captureFocusReturnTarget: frontmostAppFocusTarget,
  restoreFocusReturnTarget: (target) =>
    restoreAppFocus(target as AppFocusTarget),
  onOpen: () => warmConversationAiOnPaletteOpen(),
});
const windowManagement = createWindowManagementComposition({
  paletteWindow,
  frontmostTarget: frontmostAppFocusTarget,
});
const systemAudioMute = createSystemAudioMuteCapability();
const dictationService = createDictationService(
  (command) => {
    paletteWindow.win?.webContents.send('dictation:command', command);
  },
  {
    muteSystemAudio: systemAudioMute.temporarilyMute,
    onSystemAudioError: (error) =>
      logWarn('dictation.system-audio.restore.failed', error, {
        source: 'host',
        scope: 'dictation',
      }),
    apiAvailable: apiDictationIsAvailable,
    prepareTranscription: prepareDictationTranscription,
    cancelPreparedTranscription: cancelDictationPreparation,
    transcribeAudio: transcribeDictationAudio,
    recordings: createDictationRecordings(
      path.join(app.getPath('userData'), 'dictation-recordings'),
    ),
    recordTiming: recordDebugPerformance,
  },
);
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
  hasCapability,
  installNavigationPolicy: installExternalNavigationPolicy,
  isTrustedPage: (url, id) =>
    isTrustedExtensionWindowPage(
      url,
      id,
      isDev,
      rendererUrl,
      rendererIndexPath,
    ),
  persistence: {
    write: (state) => writeExtensionWindowsState(state as never),
  },
  debug: (message, data) =>
    loggerDebug(message, data, { source: 'host', scope: 'extensions' }),
  performanceTrace: performanceTraces,
});
nativeTheme.on('updated', () => extensionWindowManager.updateTheme());
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

    for (const source of await appIconSources(appPath)) {
      try {
        const iconPath = typeof source === 'string' ? source : source.path;
        if (typeof source !== 'string') {
          const resourcePng = await readAppIconResourcePng(
            source.path,
            source.resourceIndex,
          );
          if (resourcePng) return resourcePng;
        }
        if (
          ['.ico', '.jpeg', '.jpg', '.png'].includes(
            path.extname(iconPath).toLowerCase(),
          )
        ) {
          const image = nativeImage.createFromPath(iconPath);
          if (!image.isEmpty()) return image.toPNG();
        }
        const icon = await app.getFileIcon(iconPath, { size: 'normal' });
        if (!icon.isEmpty()) return icon.toPNG();
      } catch {}
    }
    throw new Error(`No icon for ${appPath}`);
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
const CLIPBOARD_LAST_5_MINUTES_MS = 5 * 60_000;
const CLIPBOARD_LAST_HOUR_MS = 60 * 60_000;
const CLIPBOARD_LAST_DAY_MS = 24 * CLIPBOARD_LAST_HOUR_MS;
const CLIPBOARD_LAST_WEEK_MS = 7 * CLIPBOARD_LAST_DAY_MS;
const EXTENSION_ROOT_ITEMS_TTL_MS = 60_000;
const EXTENSION_ROOT_ITEMS_TIMEOUT_MS = 10_000;
const EXTENSION_ROOT_ITEMS_INITIAL_TIMEOUT_MS = 100;
const EXTENSION_TYPES_FILENAME = 'nevermind-extension-api.d.ts';

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

function markAppQuitting() {
  nevermindApp.isQuiting = true;
}

let clipboardHistory: any[] = [];
const suppressedClipboardItemIds = new Map<string, number>();
let statePath = '';
let iconCacheDir = '';
let clipboardImagesDir = '';
let aiChatImagesDir = '';
let extensionsDir = '';
let extensionPrSubmitter: ReturnType<typeof createExtensionPrSubmitter> | null =
  null;
let extensionStorageDir = '';
let extensionSecretsDir = '';
let extensionCacheDir = '';
const jobRegistry = new JobRegistry({
  performanceTrace: (operation, attributes, task) =>
    runOperation(
      'job.run',
      () => performanceTraces.run(operation, attributes, task),
      { freshJourney: true },
    ),
});
const maintenanceActivity = createMaintenanceActivity();
let nevermindAi: any;
let learningStore: LocalLearningStore | null = null;
let activeAiChatId: string | undefined;
const draftAiChats = new Map<string, AnyRecord>();
const activeConversationRequests = new Map<string, ActiveConversationRequest>();
const activeConversationSessionIds = new Set<string>();
const activeBuilderSessionIds = new Set<string>();
const pendingAiChatModelChanges = new Set<string>();
const pendingAiChatSends = new Map<string, PendingAiChatSend>();
const viewLoaderRegistry = createViewLoaderRegistry({
  observe: (task) => runOperation('view.load', task),
  failed: (error) => {
    recordOperationFailure(error, 'load');
  },
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

const spawnPendingViewLoaders = createPendingViewLoaderSpawner({
  registry: viewLoaderRegistry,
  trace: performanceTraces,
  activity: maintenanceActivity,
});
const userStateStore = createUserStateStore(createDefaultUserState());
const userStatePersistence = createUserStatePersistence({
  getUserState: () => userStateStore.get(),
  getStatePath: () => statePath,
  beforeWrite: captureRuntimeStateIntoUserState,
  onSaveError: (error) =>
    logError('state.save.failed', error, { source: 'host', scope: 'state' }),
});
const { scheduleSaveState, saveUserState, persistUserState } =
  userStatePersistence;
const activationStatus = createActivationStatus();
const portableConfigService = createPortableConfigService({
  afterConfigAdopted: (keys) =>
    keys.includes('setting:updateChannel') &&
    updateManager.handleUpdateChannelChanged(),
  filePath: () =>
    resolvePortableConfigPath({
      isTestMode: isNvmTestMode,
      userDataDirectory: app.getPath('userData'),
    }),
  settingKinds: Object.fromEntries(
    SETTING_DEFINITIONS.map((definition) => [definition.id, definition.type]),
  ),
  userStateStore,
  scheduleSaveState,
  live: createPortableConfigLive({
    registerPaletteHotkey: (accelerator) =>
      paletteWindow.registerHotkey(accelerator),
    setLaunchAtLogin: (enabled) => setLaunchAtLoginEnabled(enabled),
    setDiagnosticReporting: (enabled) => setDiagnosticReporting(enabled),
    patchSettingsView: (settingId) => patchSettingsView(settingId),
    invalidateExtensionRootItems: () => invalidateExtensionRootItems(),
    invalidateShortcutCaches: () => invalidateShortcutCaches(),
    patchKeyboardShortcutsView: () => patchKeyboardShortcutsView(),
    applyJobEnablement: (overrides) =>
      jobRegistry.applyEnabledOverrides(overrides),
    applyShortcutView: (view) => applyShortcutView(view),
    activation: activationStatus,
  }),
});
function registerActionShortcuts() {
  void portableConfigService.refreshShortcutBindings();
}

const stateSafeQuit = createStateSafeQuit({
  flushPendingSave: async () => {
    await userStatePersistence.flushPendingSave();
    await learningStore?.flushPendingSave();
    await flushDiagnosticRecords();
  },
  quit: () => app.quit(),
  cleanup: () => runQuitCleanup(),
  exit: () => app.exit(0),
  onFlushError: (error) =>
    logError('state.flush.failed', error, {
      source: 'host',
      scope: 'state',
    }),
  onFlushTimeout: (reason) =>
    logWarn(
      'state.flush.timedOut',
      { reason },
      { source: 'host', scope: 'state' },
    ),
  onFallbackExit: (reason) =>
    logWarn(
      'app.quit.fallbackExit',
      { reason },
      { source: 'host', scope: 'app' },
    ),
  flushTimeoutMs: 2000,
  exitFallbackMs: 5000,
});

const { actionAliases, usageBoost, recentBoost, rankAction, recordRecent } =
  createActionRanker({ userStateStore, scheduleSaveState });
const {
  getPaletteHotkey,
  getUpdateChannel,
  availableSettingDefinitions,
  getSetting,
  setSetting,
  settingItemPatch,
  settingsItems,
  patchSettingsView,
} = createSettingsService({
  userStateStore,
  hasCapability,
  getLaunchAtLoginEnabled,
  setLaunchAtLoginEnabled,
  osLabel,
  commitPortable: (input) => portableConfigService.commit(input),
  activationFailure: (key) => activationStatus.failureFor(key),
  patchOpenView: (viewId, patch) => patchOpenView(viewId, patch),
  buildRecordShortcutAction: (input, options) =>
    buildRecordShortcutAction(input, options),
  afterSettingCommitted: (settingId) => {
    if (settingId === 'updateChannel')
      updateManager.handleUpdateChannelChanged();
  },
  canChangeUpdateChannel: () => !updateManager.state.installInFlight,
});
const linuxX11Clipboard = createLinuxX11Clipboard();
const clipboardApi = createElectronClipboardApi({
  clipboard,
  nativeImage,
  ClipboardItem,
  writeText: linuxX11Clipboard.enabled()
    ? linuxX11Clipboard.writeText
    : undefined,
});

const { selectedFiles, selectedExtensionFiles, readDesktopSelection } =
  createDesktopSelection({
    selectedFilePaths,
    fileToExtensionFile,
    selectedText: () => selectedText(),
    frontmostApp,
  });

const clipboardService = createClipboardHistory({
  getHistory: () => clipboardHistory,
  setHistory: (h) => {
    clipboardHistory = h;
  },
  getSuppressedItemIds: () => suppressedClipboardItemIds,
  getImagesDir: () => clipboardImagesDir,
  clipboard: clipboardApi,
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
  invalidateExtensionRootItems: () => invalidateExtensionRootItems(),
  emitChanged: () => jobRegistry.emit('clipboard.changed'),
  sendToRenderer: (channel, ...args) =>
    paletteWindow.win?.webContents.send(channel, ...args),
  patchOpenView: (viewId, patch) => patchOpenView(viewId, patch),
  pasteIntoFrontmostApp,
  getSetting,
  buildPreviewItemAction,
  rankAction,
  fileToExtensionFile,
  findFiles,
  selectedFilePaths,
  selectedExtensionFiles,
  selectedText: () => selectedText(),
  selectedFiles,
  frontmostApp,
  readDesktopSelection,
  CLIPBOARD_LIMIT,
  CLIPBOARD_POLL_INTERVAL_MS,
  CLIPBOARD_LAST_5_MINUTES_MS,
  CLIPBOARD_LAST_HOUR_MS,
  CLIPBOARD_LAST_DAY_MS,
  CLIPBOARD_LAST_WEEK_MS,
});
const {
  clipboardRootItem,
  clipboardHistorySnapshot,
  clipboardHistoryGet,
  removeClipboardHistoryByAction,
  removeClipboardHistoryEntries,
  clipboardHistoryView,
  clipboardSnapshot,
  restoreClipboardSnapshot,
  clipboardHistoryIdForText,
  suppressClipboardHistoryId,
  clipboardFiles,
  clipboardImageDataUrl,
  readDesktopClipboard,
  writeDesktopClipboard,
  pasteClipboardAction,
} = clipboardService;
const { clipboardSearchActions } = createClipboardSearchActions({
  getHistory: () => clipboardHistory,
  clipboardRootItem,
  rankAction: (item, query) => rankAction(item, query),
  prepareRootActionForRenderer: (item) => prepareRootActionForRenderer(item),
  measureSync: measureDebugPerformanceSync,
  clipboardLimit: CLIPBOARD_LIMIT,
});
const { startClipboardWatcher } = createClipboardWatcher({
  createWatcherJob: clipboardService.startClipboardWatcher,
  registerJob: (job) => jobRegistry.register(job),
});

const selectedText = createSelectedTextReader({
  selectionTarget: async () =>
    (paletteWindow.takeRestoredFocusReturnTarget() as AppFocusTarget | null) ??
    frontmostAppFocusTarget(),
  readAccessibilityText: readAccessibilitySelectedText,
  paletteIsFocused: () =>
    Boolean(paletteWindow.win?.isVisible() && paletteWindow.win.isFocused()),
  clipboardSnapshot,
  validateClipboardSnapshot: linuxX11Clipboard.validateSnapshot,
  readClipboardText: () => clipboardApi.readText(),
  writeClipboardText: (text) => clipboardApi.writeText(text),
  restoreClipboardSnapshot: async (snapshot: any) => {
    suppressClipboardHistoryId(clipboardHistoryIdForText(snapshot?.text));
    await restoreClipboardSnapshot(snapshot);
  },
  copySelectionIntoClipboard,
  concealClipboardText: (text) =>
    suppressClipboardHistoryId(clipboardHistoryIdForText(text)),
  selectionRead: (result) =>
    loggerDebug('selected-text.read.result', result, {
      source: 'host',
      scope: 'selected-text',
    }),
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

const appIndexService = createAppIndexService({
  scanApps,
  trackFirstSeen: (apps, initializeBaseline) => {
    const tracked = trackFirstSeenApps(apps, {
      firstSeenAtById: userStateStore.get().appFirstSeenAtById || {},
      initialized: userStateStore.get().appFirstSeenInitialized === true,
      initializeBaseline,
      now: Date.now(),
      identityKey: appIdentityKey,
    });
    if (tracked.changed) {
      userStateStore.get().appFirstSeenAtById = tracked.firstSeenAtById;
      userStateStore.get().appFirstSeenInitialized = tracked.initialized;
      scheduleSaveState();
    }
    return tracked.apps;
  },
  needsFirstSeenBaseline: () =>
    userStateStore.get().appFirstSeenInitialized !== true,
  watchApps,
  normalize,
  emitChanged: () => jobRegistry.emit('apps.changed'),
  invalidateRunningStatus: () => runningAppStatus.invalidate(),
  scheduleRunningStatusRefresh: (reason) =>
    runningAppStatus.scheduleRefresh(reason),
  notifyIndexed: (count) => {
    extensionRootItemsCache.delete('nevermind.apps');
    paletteWindow.win?.webContents.send('apps:indexed', count);
  },
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
const appUninstallService = createHostAppUninstallService({
  trashItem: (itemPath) => shell.trashItem(itemPath),
});
const { processPendingThumbnails, thumbnailResponseForPath } =
  createThumbnailService({
    jobRegistry,
    getIconCacheDir: () => iconCacheDir,
  });
const { fileIndexSnapshot, reindexFiles, indexFiles, currentFileIndex } =
  createFileIndexService({
    notifyRootItemsChanged: () =>
      paletteWindow.win?.webContents.send('root-items:changed'),
  });
const openWithApps = createOpenWithApps({ appIndexService });
const extensionActionRegistry = new Map<string, any>();
const extensionModules = new Map<string, any>();
const extensionStorageRefreshes = new Map<string, Promise<any>>();
const extensionJsonStore = createExtensionJsonStore();
const {
  readExtensionStorage,
  mutateExtensionStorage,
  readExtensionCache,
  mutateExtensionCache,
  createExtensionStorage,
  createExtensionSecrets,
} = createExtensionStorageService({
  getExtensionStorageDir: () => extensionStorageDir,
  getExtensionCacheDir: () => extensionCacheDir,
  getExtensionSecretsDir: () => extensionSecretsDir,
  extensionJsonStore,
  extensionStorageRefreshes,
  safeStorage,
});
const extensionDraftStore = createExtensionDraftStore();
const extensionDraftRefs = new Map<
  string,
  {
    ownerExtensionId: string;
    key: string;
    version: string | number;
    autosaveAction: any;
  }
>();
const {
  hydrateExtensionDrafts,
  schedulePersistExtensionDrafts,
  saveExtensionDraftForIpc,
  commitExtensionDraft,
  discardExtensionDraft,
} = createExtensionDraftPersistence({
  extensionDraftStore,
  extensionDraftRefs,
  extensionJsonStore,
  userDataPath: () => app.getPath('userData'),
  executeViewAction: (action) => executeViewAction(action),
});
const { hydrateExtensionWindowsState, writeExtensionWindowsState } =
  createExtensionWindowsState({
    extensionJsonStore,
    userDataPath: () => app.getPath('userData'),
    hydratePersistedState: (state) =>
      extensionWindowManager.hydratePersistedState(state),
  });

const extensionActionHandlers = new Map<string, any>();
const {
  viewActionExecutionRecords,
  rootActionExecutionRecords,
  viewRefreshRecords,
  pruneViewRefreshRecords,
} = createActionExecutionRecords();
const actionExecutionCapabilities = createActionExecutionCapabilities();
const {
  registerViewActionForRenderer,
  registerRootActionForRenderer,
  actionCapabilityOwnerIsCurrent,
} = createActionRegistration({
  viewActionExecutionRecords,
  rootActionExecutionRecords,
  actionExecutionCapabilities,
  extensionModules,
});
const { shortcutForAction, withShortcutHint, invalidateShortcutCaches } =
  createShortcutHints({
    userStateStore,
    chatTouchedExtensionFiles: (chat) => chatTouchedExtensionFiles(chat),
  });
const {
  extensionActionFromContribution,
  normalizeView,
  persistentActionForRef,
  normalizeViewItems,
  normalizeActionPanel,
  normalizeViewAction,
} = createViewNormalizer({
  extensionActionRegistry,
  extensionActionHandlers,
  extensionDraftRefs,
  extensionDraftStore,
  viewLoaderRegistry,
  viewRefreshRecords,
  pruneViewRefreshRecords,
  registerViewActionForRenderer,
  userStateStore,
  shortcutForAction,
  aiChatIdForExtensionFile: (filename) => aiChatIdForExtensionFile(filename),
});
const { normalizeExtensionView, normalizeViewPatch, normalizeHostViewResult } =
  createViewResultNormalizer({ normalizeView, normalizeViewItems });
const { patchOpenView } = createOpenViewPatcher({
  normalizeViewPatch,
  sendToPalette: (channel, ...args) =>
    paletteWindow.win?.webContents.send(channel, ...args),
});
const {
  declaredGlobalShortcuts,
  getShortcuts,
  extensionShortcutRecords,
  buildRecordShortcutAction,
  buildRemoveShortcutAction,
  patchKeyboardShortcutsView,
} = createShortcutCatalog({
  userStateStore,
  visibleExtensionActionEntries: () => visibleExtensionActionEntries(),
  extensionActionFromContribution,
  resolveStoredShortcutAction: (actionId) =>
    resolveStoredShortcutAction(actionId),
  activationFailure: (key) => activationStatus.failureFor(key),
  patchOpenView: (viewId, patch) => patchOpenView(viewId, patch),
});
const { executeShortcutAction } = createShortcutRunner({
  activity: maintenanceActivity,
  paletteWindow,
  captureFrontmostTarget: frontmostAppFocusTarget,
  paneInvocations,
  paneInvocationWindows,
  currentActionForStoredShortcut: (action) =>
    currentActionForStoredShortcut(action),
  isClipboardHistoryAction: (action) => isClipboardHistoryAction(action),
  clipboardHistoryView: () => clipboardHistoryView(),
  recordRecent: (action) => recordRecent(action),
  runInBackground: (task, traceId) => runInBackground(task, traceId),
  executeAction: (action, options) => executeAction(action, options),
  presentActionErrorFeedback: (error, action) =>
    presentActionErrorFeedback(error, action),
  normalizeHostViewResult,
  actionFailedFeedbackView: () => actionFailedFeedbackView(),
});
const { applyShortcutView, releaseAllShortcuts, isActionShortcutOwned } =
  createGlobalShortcutRegistration({
    globalShortcut,
    isPaletteHotkeyReserved: (accelerator) =>
      paletteWindow.isHotkeyReserved(accelerator),
    resolveStoredShortcutAction: (actionId) =>
      resolveStoredShortcutAction(actionId),
    declaredGlobalShortcuts,
    executeShortcutAction,
  });
const {
  removeShortcut,
  setAlias,
  removeAlias,
  setShortcut,
  setShortcutSetting,
  setPaletteHotkey,
  suspendPaletteHotkey,
  resumePaletteHotkey,
  openSystemKeyboardSettings,
  setOverride,
  clearOverride,
} = createActionCustomization({
  userStateStore,
  scheduleSaveState,
  paletteWindow,
  commitPortable: (input) => portableConfigService.commit(input),
  chatTouchedExtensionFiles: (chat) => chatTouchedExtensionFiles(chat),
  getPaletteHotkey: () => getPaletteHotkey(),
  setSetting: (id, value) => setSetting(id, value),
  settingDefinition,
  declaredGlobalShortcuts,
});
const { extensionRootActionFromItem } = createExtensionRootActionNormalizer({
  extensionActionRegistry,
  persistentActionForRef,
  extensionActionFromContribution,
  normalizeViewAction,
  normalizeActionPanel,
  withInheritedShortcutLifecycle,
});

const { runShellCommand, runShellScript, shellSpawnEnv } =
  createShellCommands();
const {
  extensionActionEntryForAction,
  extensionEntryForAction,
  extensionModuleForAction,
  currentActionForStoredShortcut,
  resolveStoredShortcutAction,
  resolveRegisteredActionRef,
} = createExtensionActionLookup({
  extensionActionRegistry,
  extensionModules,
  userStateStore,
  chatTouchedExtensionFiles: (chat) => chatTouchedExtensionFiles(chat),
  extensionActionFromContribution,
  extensionRootActionFromItem: (entry, item) =>
    extensionRootActionFromItem(entry, item),
});
const {
  extensionRootItemsCache,
  extensionRootItemsRefreshes,
  validCachedRootActions,
  invalidateExtensionRootItems,
  invalidateExtensionRootItemsForExtension,
  extensionSearchActionsForExtension,
  refreshExtensionRootActions,
} = createExtensionRootItems({
  paletteWindow,
  createExtensionContext: (extension, command, launchContext, options) =>
    createExtensionContext(extension, command, launchContext, options),
  extensionRootActionFromItem,
  rankAction,
  EXTENSION_ROOT_ITEMS_TTL_MS,
  EXTENSION_ROOT_ITEMS_TIMEOUT_MS,
});
const {
  visibleExtensionActionEntries,
  progressiveSearchCoordinator,
  startProgressiveSearch,
  cancelProgressiveSearch,
  searchActions,
  prepareRootActionForRenderer,
} = createPaletteSearch({
  extensionModules,
  extensionActionRegistry,
  extensionActionFromContribution,
  withShortcutHint,
  rankAction,
  clipboardSearchActions,
  normalizeViewAction,
  extensionEntryForAction,
  registerRootActionForRenderer,
  validCachedRootActions,
  refreshExtensionRootActions,
  extensionSearchActionsForExtension,
  EXTENSION_ROOT_ITEMS_INITIAL_TIMEOUT_MS,
});
const {
  extensionErrorView,
  actionFailedFeedbackView,
  presentActionResultFeedback,
  presentActionErrorFeedback,
  showRendererIndicator,
} = createActionFeedback({
  normalizeView,
  setDiagnosticExtensionTarget: (extension) =>
    setDiagnosticExtensionTarget(extension),
  aiChatView: (chat, options) => aiChatView(chat, options),
  getOrCreateExtensionChat: (extensionFile, title) =>
    getOrCreateExtensionChat(extensionFile, title),
  extensionActionHandlers,
  extensionWindowManager,
  paletteWindow,
});
const { resolveRootActionForIpc, resolveViewActionForIpc } =
  createActionIpcResolver({
    rootActionExecutionRecords,
    viewActionExecutionRecords,
    actionExecutionCapabilities,
    actionCapabilityOwnerIsCurrent,
    currentActionForStoredShortcut,
    characterRecordsById: CHARACTER_RECORDS_BY_ID,
    characterCodePoints,
    characterWithSkinTone,
  });
const {
  executeExtensionRootItem,
  executeViewActionResult,
  executeHostRefreshAction,
} = createExtensionActionResults({
  executeViewAction: (action, launchContext) =>
    executeViewAction(action, launchContext),
  createExtensionContext: (extension, command, launchContext, options) =>
    createExtensionContext(extension, command, launchContext, options),
  setDiagnosticExtensionTarget: (extension) =>
    setDiagnosticExtensionTarget(extension),
  extensionActionHandlers,
  extensionErrorView,
  extensionDraftStore,
  schedulePersistExtensionDrafts: () => schedulePersistExtensionDrafts(),
  normalizeViewAction,
  normalizeExtensionView,
  normalizeViewPatch,
  normalizeViewItems,
});
const { executeAction } = createRootActionExecutor({
  activity: maintenanceActivity,
  paletteWindow,
  recordRecent: (action) => recordRecent(action),
  currentActionForStoredShortcut,
  openSystemKeyboardSettings,
  executeExtensionRootItem,
  presentActionResultFeedback,
});
const { executeViewAction } = createViewActionExecutor({
  ...createNativeActionHandlers({ viewLoaderRegistry, executeAction }),
  ...createFileActionHandlers({ shell }),
  ...createClipboardActionHandlers({
    clipboardApi,
    nativeImage,
    paletteWindow,
    clipboardPasteText: (action) => clipboardService.pasteTextAction(action),
    pasteClipboardAction: (action) => pasteClipboardAction(action),
    removeClipboardHistoryEntries: (action) =>
      removeClipboardHistoryEntries(action),
  }),
  ...createWindowActionHandlers({
    extensionWindowManager,
    executeViewAction: (action, launchContext) =>
      executeViewAction(action, launchContext),
  }),
  ...createShellActionHandlers({ runShellCommand, runShellScript }),
  ...createSystemActionHandlers({
    shell,
    openSystemKeyboardSettings,
    requestQuitApp: (reason) => requestQuitApp(reason),
    checkForUpdatesView: () => checkForUpdatesView(),
    downloadUpdateView: () => downloadUpdateView(),
    installDownloadedUpdate: () => installDownloadedUpdate(),
  }),
  ...createSettingActionHandlers({
    settingDefinition,
    toggledSettingValue,
    getSetting: (id) => getSetting(id),
    setSetting: (id, value) => setSetting(id, value),
    settingItemPatch: (definition) => settingItemPatch(definition),
    setShortcutSetting,
  }),
  ...createCustomizationActionHandlers({
    setShortcut,
    removeShortcut,
    setAlias,
    removeAlias,
    clearOverride,
    duplicateCreatedAction: (action) => duplicateCreatedAction(action),
    removeCreatedAction: (action) => removeCreatedAction(action),
    getExtensionPrSubmitter: () => extensionPrSubmitter,
  }),
  ...createExtensionActionHandlers({
    resolveRegisteredActionRef,
    executeExtensionRootItem,
    executeViewActionResult,
    extensionModuleForAction,
    extensionModules,
    extensionActionRegistry,
    extensionActionHandlers,
    renameExtension: (extension, command, metadata) =>
      renameExtension(extension, command, metadata),
    invalidateExtensionRootItems: () => invalidateExtensionRootItems(),
    setDiagnosticExtensionTarget: (extension) =>
      setDiagnosticExtensionTarget(extension),
    createExtensionContext: (extension, command, launchContext, options) =>
      createExtensionContext(extension, command, launchContext, options),
    extensionErrorView,
  }),
});
const { executeActionForIpc, refreshViewForIpc, executeViewActionForIpc } =
  createActionIpc({
    resolveRootActionForIpc,
    resolveViewActionForIpc,
    executeAction,
    executeViewAction,
    executeHostRefreshAction,
    presentActionResultFeedback,
    presentActionErrorFeedback,
    extensionEntryForAction,
    extensionActionEntryForAction,
    extensionErrorView,
    actionFailedFeedbackView,
    normalizeHostViewResult,
    spawnPendingViewLoaders: (result, traceId) =>
      spawnPendingViewLoaders(result, traceId),
    pruneViewRefreshRecords,
    viewRefreshRecords,
    extensionActionHandlers,
    paneInvocations,
  });
const { extensionCacheFor, createExtensionCache } = createExtensionMemoryCache({
  invalidateExtensionRootItemsForExtension: (extension) =>
    invalidateExtensionRootItemsForExtension(extension),
});

function registerAiEventActionForRenderer(event) {
  const data = event?.data;
  if (!(data && typeof data === 'object' && data.action)) return event;
  const action = trustedAiLimitAction(event, getNevermindDashboardUrl());
  if (!action) return { ...event, data: { ...data, action: undefined } };
  return {
    ...event,
    data: {
      ...data,
      action: registerViewActionForRenderer(action),
    },
  };
}

const REQUIRED_INTERNAL_EXTENSIONS = [
  'nevermind.system',
  'nevermind.places',
  'nevermind.calculator',
  'nevermind.web',
  'nevermind.clipboard',
  'nevermind.dictation',
  'nevermind.ai-commands',
  'nevermind.emoji-symbols',
  'nevermind.apps',
  'nevermind.files',
  'nevermind.floating-notes',
  AI_BUILDER_EXTENSION_ID,
  'nevermind.updates',
  'nevermind.shortcuts',
  'nevermind.settings',
  'nevermind.background-tasks',
  'nevermind.account',
];
const REQUIRED_INTERNAL_COMMANDS = [
  { extensionId: AI_BUILDER_EXTENSION_ID, commandId: 'ai-chats' },
  {
    extensionId: 'nevermind.ai-commands',
    commandId: 'fix-selected-text-with-ai',
    capability: 'selected-text',
  },
];

const {
  persistAiChatImages,
  removeAiChatImageDirectory,
  aiChatMessagesForRenderer,
  aiChatMessageForRenderer,
  storedAiChatImagePath,
  removeStoredAiChatMessageImages,
} = createAiChatImageStorage({
  getAiChatImagesDir: () => aiChatImagesDir,
});
const { aiChatView, activatedExtensionPreview, extensionRuntimeStateForFile } =
  createAiChatViews({
    extensionModules,
    extensionActionRegistry,
    extensionActionFromContribution,
    prepareRootActionForRenderer,
    validCachedRootActions,
    refreshExtensionRootActions,
    aiChatMessagesForRenderer,
  });
const { broadcastAiChatEvent } = createAiChatEventBroadcaster({
  sendToPalette: sendToPaletteWindow,
  broadcastToExtensionWindows: (channel, event) =>
    extensionWindowManager.broadcast(channel, event),
});
const {
  aiChatsView,
  patchAiChatsItem,
  patchAiChatsPrepend,
  abortAiChat,
  resetAiChat,
  removeAiChat,
  cleanupExpiredAiConversations,
} = createAiChatLifecycle({
  userStateStore,
  draftAiChats,
  pendingAiChatSends,
  activeConversationRequests,
  activeConversationSessionIds,
  activeBuilderSessionIds,
  extensionActionRegistry,
  getActiveAiChatId: () => activeAiChatId,
  getNevermindAi: () => nevermindAi,
  isPaletteVisible: () => paletteWindow.win?.isVisible(),
  broadcastAiChatEvent,
  scheduleSaveState,
  invalidateExtensionRootItems,
  patchOpenView,
  normalizeViewItems,
  createAiBuilderExtension,
  aiChatView,
  removeAiChatImageDirectory,
});
const {
  aiLearningMetadata,
  createDraftAiChat,
  createDraftConversationChat,
  promoteDraftAiChat,
  appendAiChatMessage,
  appendAiChatDelta,
  getOrCreateExtensionChat,
} = createAiChatStore({
  userStateStore,
  draftAiChats,
  getLearningStore: () => learningStore,
  scheduleSaveState,
  invalidateExtensionRootItems,
  patchAiChatsItem,
  patchAiChatsPrepend,
  removeStoredAiChatMessageImages,
});
const {
  aiChatIdForExtensionFile,
  claimExtensionFileForChat,
  markGeneratedExtensionForActiveChat,
  chatCanWriteExtension,
  withLiveExtensionWriteLock,
  removeGeneratedExtensionForChat,
  removeAiChatReferencesToExtensionFile,
} = createAiExtensionOwnership({
  userStateStore,
  draftAiChats,
  extensionModules,
  getActiveAiChatId: () => activeAiChatId,
  getExtensionsDir: () => extensionsDir,
  scheduleSaveState,
  persistUserState,
  aiLearningMetadata,
  getLearningStore: () => learningStore,
  patchAiChatsItem,
  extensionManagerState: () => extensionManagerState(),
  captureManagedExtensionRuntime: (sourceFile) =>
    captureManagedExtensionRuntime(sourceFile),
  detachManagedExtensionRuntime: (sourceFile) =>
    detachManagedExtensionRuntime(sourceFile),
  restoreManagedExtensionRuntime: (snapshot, candidate) =>
    restoreManagedExtensionRuntime(snapshot, candidate),
  syncFrontmostAppPolling: () => syncFrontmostAppPolling(),
  registerActionShortcuts: () => registerActionShortcuts(),
  invalidateExtensionRootItems,
});
const {
  addAliasForGeneratedAction,
  duplicateCreatedAction,
  removeCreatedAction,
} = createAiCreatedActions({
  userStateStore,
  extensionActionRegistry,
  scheduleSaveState,
  commitPortable: (input) => portableConfigService.commit(input),
  logWarn: (message, data) =>
    logWarn(message, data, { source: 'host', scope: 'ai' }),
  invalidateExtensionRootItems,
  actionAliases,
  extensionActionFromContribution,
  extensionModuleForAction,
  stageExtensionProposal: (filename, source) =>
    stageExtensionProposal(filename, source),
  activateManagedExtension: (filename) => activateManagedExtension(filename),
  loadExtensions: () => loadExtensions(),
  registerActionShortcuts: () => registerActionShortcuts(),
  removeAiChat,
  removeAiChatReferencesToExtensionFile,
});

let activeNevermindBaseUrl: string | null = null;
const bufferedDeepLinks: string[] = [];
let appReady = false;

function sendToPaletteWindow(channel: string, payload: unknown) {
  paletteWindow.win?.webContents.send(channel, payload);
}

const {
  updatesStateSnapshot,
  checkForUpdatesView,
  downloadUpdateView,
  installDownloadedUpdate,
  compatibilityPromptAction,
  updatePromptAction,
  patchUpdatesView,
} = createUpdatesService({
  updateManager,
  getAppVersion: () => app.getVersion(),
  autoUpdatesUnavailableMessage,
  getActiveNevermindBaseUrl: () => activeNevermindBaseUrl,
  getUpdateChannel,
  markAppQuitting,
  requestQuit: stateSafeQuit.requestQuit,
  quitApp: () => app.quit(),
  patchOpenView: (viewId, patch) => patchOpenView(viewId, patch),
});

const createExtensionContext = createExtensionContextFactory({
  renameExtension: (extension, command, metadata) =>
    renameExtension(extension, command, metadata),
  ui: { extensionWindowManager },
  actions: {
    buildRecordShortcutAction,
    buildRemoveShortcutAction,
    refreshContributions: (extension) =>
      registration.refreshExtensionContributions(extension),
  },
  text: {
    readClipboardText: () => clipboardApi.readText(),
    selectedText,
  },
  clipboardHistory: {
    clipboardHistorySnapshot: (options) => clipboardHistorySnapshot(options),
    clipboardHistoryGet: (id) => clipboardHistoryGet(id),
    removeClipboardHistoryByAction,
  },
  desktop: {
    windowManagement,
    clipboardApi,
    writeDesktopClipboard,
    clipboardImageDataUrl: () => clipboardImageDataUrl(),
    clipboardFiles: () => clipboardFiles(),
    readDesktopClipboard: (options) => readDesktopClipboard(options),
    selectedText,
    readDesktopSelection: () => readDesktopSelection(),
    selectedFiles: () => selectedFiles(),
    appIndexService,
    appIconCache,
    openWithApps,
    fileIndexSnapshot,
    reindexFiles,
    runInBackground: (task, traceId) => runInBackground(task, traceId),
    runShellCommand,
    runShellScript: (script, options) => runShellScript(script, options),
    shellSpawnEnv: (extraEnv) => shellSpawnEnv(extraEnv),
  },
  storage: {
    createExtensionStorage,
    createExtensionSecrets,
    createExtensionCache,
  },
  drafts: {
    commitExtensionDraft,
    discardExtensionDraft,
  },
  settings: {
    availableSettingDefinitions: () => availableSettingDefinitions(),
    getSetting: (id) => getSetting(id),
    setSetting: (id, value) => setSetting(id, value),
    settingDefinition,
    toggledSettingValue,
  },
  dictation: { dictationService },
  shortcuts: {
    extensionShortcutRecords: () => extensionShortcutRecords(),
    getPaletteHotkey: () => getPaletteHotkey(),
  },
  views: {
    checkRefreshBurst: (extension) => checkRefreshBurst(extension, logWarn),
    invalidateExtensionRootItemsForExtension: (extension) =>
      invalidateExtensionRootItemsForExtension(extension),
    extensionCacheFor,
  },
  updates: { updatesStateSnapshot: () => updatesStateSnapshot() },
  ai: {
    createExtensionAi: createExtensionAiFactory({
      checkAiRateLimit: (extension) => checkAiRateLimit(extension),
      getNevermindAi: () => nevermindAi,
      selectedText,
      selectedExtensionFiles: () => selectedExtensionFiles(),
      readDesktopClipboard: (options) => readDesktopClipboard(options),
    }),
    createAiBuilderApi: createAiBuilderApiFactory({
      aiChatView: (item, options) => aiChatView(item, options),
      aiChatsView: () => aiChatsView(),
      chatTouchedExtensionFiles: (chat) => chatTouchedExtensionFiles(chat),
      cleanupExpiredAiConversations: () => cleanupExpiredAiConversations(),
      createDraftAiChat: (query) => createDraftAiChat(query),
      draftAiChats,
      getAiChats: () => userStateStore.get().aiChats,
      getOrCreateExtensionChat: (extensionFile, title) =>
        getOrCreateExtensionChat(extensionFile, title),
      removeAiChat: (chatId) => removeAiChat(chatId),
    }),
    createExtensionOwnershipApi: createExtensionOwnershipApiFactory({
      aiChatIdForExtensionFile: (extensionFile) =>
        aiChatIdForExtensionFile(extensionFile),
      chatCanWriteExtension: (extensionFile, chatId) =>
        chatCanWriteExtension(extensionFile, chatId),
      chatTouchedExtensionFiles: (chat) => chatTouchedExtensionFiles(chat),
      claimExtensionFileForChat: (chat, extensionFile) =>
        claimExtensionFileForChat(chat, extensionFile),
      draftAiChats,
      getAiChats: () => userStateStore.get().aiChats,
      loadExtensions: () => loadExtensions(),
      registerActionShortcuts: () => registerActionShortcuts(),
      removeGeneratedExtensionForChat: (extensionFile, chatId) =>
        removeGeneratedExtensionForChat(extensionFile, chatId),
      scheduleSaveState,
    }),
  },
});

const extensionRuntimeState = createExtensionRuntimeState();
const { stopFrontmostAppPolling, syncFrontmostAppPolling, registerHostJobs } =
  createHostJobs({
    jobRegistry,
    saveUserState,
    appIndexService,
    appIconCache,
    indexFiles,
    processPendingThumbnails,
    clipboardImages: {
      getClipboardHistory: () => clipboardHistory,
      getClipboardImagesDir: () => clipboardImagesDir,
      loadedStatePath: userStateStore.loadedStatePath,
    },
  });
const {
  attachExtensionFileTrigger,
  registerExtensionBackgroundJob,
  unregisterExtensionBackgroundJob,
} = createExtensionTriggers({
  jobRegistry,
  runtimeState: extensionRuntimeState,
  EXTENSION_ROOT_ITEMS_TIMEOUT_MS,
  executeViewAction: (action, launchContext) =>
    executeViewAction(action, launchContext),
});
const registration = createExtensionRegistration({
  extensionModules,
  extensionActionRegistry,
  INTERNAL_EXTENSION_FACTORIES,
  REQUIRED_INTERNAL_EXTENSIONS,
  REQUIRED_INTERNAL_COMMANDS,
  normalizeItemAppearance,
  normalizeViewItems,
  createExtensionContext,
  registerExtensionBackgroundJob,
  unregisterExtensionBackgroundJob,
  registerActionShortcuts: () => registerActionShortcuts(),
  invalidateExtensionRootItems: () => invalidateExtensionRootItems(),
});
const { loadDevExtensions } = createExtensionDevFixtures({
  extensionActionRegistry,
  runtimeState: extensionRuntimeState,
  getAppPath: () => app.getAppPath(),
  extensionActionFromContribution,
  registerExtension: registration.registerExtension,
});
const {
  initializeExtensionManager,
  extensionFileIsEnabled,
  extensionDraftsDir,
  extensionManagerState,
  stageExtensionProposal,
  discardExtensionProposal,
  discardManagedExtensionProposal,
  managedExtensionEntries,
} = createExtensionProposals({
  userStateStore,
  saveUserState,
  persistUserState,
  getExtensionsDir: () => extensionsDir,
  userDataPath: () => app.getPath('userData'),
});
const { loadExtensions, applyExtensionMetadataOverrides, renameExtension } =
  createExtensionLoader({
    extensionModules,
    extensionActionRegistry,
    extensionActionHandlers,
    viewActionExecutionRecords,
    rootActionExecutionRecords,
    viewRefreshRecords,
    extensionRootItemsCache,
    extensionRootItemsRefreshes,
    jobRegistry,
    runtimeState: extensionRuntimeState,
    isDev,
    EXTENSION_TYPES_FILENAME,
    getExtensionsDir: () => extensionsDir,
    bundledResourcePath: (...relativePath) =>
      bundledResourcePath(...relativePath),
    initExtensionRuntimeContext: () => initExtensionRuntimeContext(),
    stopFrontmostAppPolling,
    syncFrontmostAppPolling,
    registration,
    loadDevExtensions,
    initializeExtensionManager,
    extensionFileIsEnabled,
    readExtensionStorage,
    mutateExtensionStorage,
    registerActionShortcuts: () => registerActionShortcuts(),
  });
const {
  prepareManagedExtensionRuntime,
  detachManagedExtensionRuntime,
  captureManagedExtensionRuntime,
  restoreManagedExtensionRuntime,
  commitPreparedExtensionRuntime,
  failTestExtensionActivationAt,
} = createExtensionRuntimeTransaction({
  extensionModules,
  extensionActionRegistry,
  extensionActionHandlers,
  viewActionExecutionRecords,
  rootActionExecutionRecords,
  viewRefreshRecords,
  extensionRootItemsCache,
  extensionRootItemsRefreshes,
  jobRegistry,
  runtimeState: extensionRuntimeState,
  registration,
  applyExtensionMetadataOverrides,
  attachExtensionFileTrigger,
  syncFrontmostAppPolling,
  registerActionShortcuts: () => registerActionShortcuts(),
  invalidateExtensionRootItems: () => invalidateExtensionRootItems(),
});
const { activateManagedExtension, disableManagedExtension } =
  createExtensionManager({
    getExtensionsDir: () => extensionsDir,
    persistUserState,
    extensionManagerState,
    prepareManagedExtensionRuntime,
    detachManagedExtensionRuntime,
    captureManagedExtensionRuntime,
    restoreManagedExtensionRuntime,
    commitPreparedExtensionRuntime,
    failTestExtensionActivationAt,
    syncFrontmostAppPolling,
    registerActionShortcuts: () => registerActionShortcuts(),
    invalidateExtensionRootItems: () => invalidateExtensionRootItems(),
  });
const { restorePersistentExtensionWindows } =
  createPersistentExtensionWindowRestorer({
    extensionModules,
    extensionWindowManager,
    createExtensionContext,
    normalizeView,
  });
const { setDiagnosticExtensionTarget } = createDiagnosticExtensionTargets();
const { runQuitCleanup, requestQuitApp } = createQuitLifecycle({
  app,
  globalShortcut,
  progressiveSearchCoordinator,
  updateManager,
  jobRegistry,
  extensionWindowManager,
  appIndexService,
  extensionRuntimeState,
  markAppQuitting,
  stateSafeQuit,
});
const { registerTestModeIpcHandlers } = createTestModeIpc({
  ipcMain,
  createMeasuredIpcHandle: () =>
    createMeasuredIpcRegistrar({
      ipcMain,
      measure: measureDebugPerformance,
      summarize: summarizeDebugValue,
      trace: performanceTraces,
      activity: maintenanceActivity,
    }),
  globalShortcut,
  paletteWindow,
  extensionWindowManager,
  jobRegistry,
  extensionRuntimeState,
  isActionShortcutOwned,
  startProgressiveSearch,
  cancelProgressiveSearch,
  searchActions,
  executeActionForIpc,
  executeViewActionForIpc,
  stageExtensionProposal,
  setShortcut,
  getSetting,
  getShortcuts,
  showRendererIndicator,
  initializeAi: () => initNevermindAi(),
  getAi: () => nevermindAi,
});

function setActiveAiChatId(chatId: string | undefined) {
  activeAiChatId = chatId;
}

const { warmConversationAiOnPaletteOpen } = createConversationAiWarmup({
  getNevermindAi: () => nevermindAi,
});
const { relevantLearningContext, recordLearningReview } =
  createAiLearningReview({
    getLearningStore: () => learningStore,
    aiLearningMetadata,
    jobRegistry,
    getNevermindAi: () => nevermindAi,
  });
const {
  aiChatPromptWithContext,
  conversationPromptWithContext,
  restoredConversationImages,
} = createAiChatPrompts({
  userStateStore,
  relevantLearningContext,
  storedAiChatImagePath,
});
const { handleNevermindAiEvent } = createNevermindAiEventHandler({
  userStateStore,
  draftAiChats,
  getActiveAiChatId: () => activeAiChatId,
  getLearningStore: () => learningStore,
  aiLearningMetadata,
  appendAiChatDelta,
  appendAiChatMessage,
  patchAiChatsItem,
  scheduleSaveState,
  registerAiEventActionForRenderer,
  broadcastAiChatEvent,
});
const { initNevermindAi } = createNevermindAiInitializer({
  userStateStore,
  draftAiChats,
  EXTENSION_TYPES_FILENAME,
  internalExtensionFiles: INTERNAL_EXTENSION_SOURCE_FILES,
  bundledResourcePath,
  getUserDataPath: () => app.getPath('userData'),
  getAppPath: () => app.getAppPath(),
  getExtensionsDir: () => extensionsDir,
  extensionDraftsDir,
  loadExtensions,
  extensionShortcutRecords,
  getPaletteHotkey,
  extensionRuntimeStateForFile,
  getActiveAiChatId: () => activeAiChatId,
  getNevermindAi: () => nevermindAi,
  setNevermindAi: (createdAi) => {
    nevermindAi = createdAi;
  },
  markGeneratedExtensionForActiveChat,
  chatCanWriteExtension,
  claimExtensionFileForChat,
  stageExtensionProposal,
  discardExtensionProposal,
  activateManagedExtension,
  activatedExtensionPreview,
  scheduleSaveState,
  withLiveExtensionWriteLock,
  removeGeneratedExtensionForChat,
  addAliasForGeneratedAction,
  handleNevermindAiEvent,
});
const { setAiChatModel } = createAiChatModelChange({
  userStateStore,
  commitPortable: (input) => portableConfigService.commit(input),
  draftAiChats,
  pendingAiChatSends,
  activeConversationRequests,
  activeConversationSessionIds,
  pendingAiChatModelChanges,
  getNevermindAi: () => nevermindAi,
  scheduleSaveState,
  patchAiChatsItem,
  broadcastAiChatEvent,
});
const {
  sendAiChatMessage,
  beginAiChatSendSuspension,
  endAiChatSendSuspension,
} = createAiChatSend({
  userStateStore,
  pendingAiChatSends,
  activeConversationRequests,
  activeConversationSessionIds,
  activeBuilderSessionIds,
  pendingAiChatModelChanges,
  getNevermindAi: () => nevermindAi,
  setActiveAiChatId,
  initNevermindAi,
  promoteDraftAiChat,
  appendAiChatMessage,
  appendAiChatDelta,
  patchAiChatsItem,
  persistAiChatImages,
  aiChatMessageForRenderer,
  aiChatPromptWithContext,
  conversationPromptWithContext,
  restoredConversationImages,
  registerAiEventActionForRenderer,
  broadcastAiChatEvent,
  scheduleSaveState,
});
const { noteAiChatExited, disposeAiSessions } = createAiChatSessionTeardown({
  userStateStore,
  pendingAiChatSends,
  activeConversationRequests,
  activeConversationSessionIds,
  activeBuilderSessionIds,
  getNevermindAi: () => nevermindAi,
  getActiveAiChatId: () => activeAiChatId,
  setActiveAiChatId,
  abortAiChat,
  recordLearningReview,
});

function setActiveNevermindBaseUrlState(baseUrl: string | null) {
  activeNevermindBaseUrl = baseUrl;
}

const { selectedNevermindEnvironment, getNevermindDebugStatus } =
  createNevermindEnvironmentSelection({
    getStoredEnvironment: () => userStateStore.get().nevermindEnvironment,
    getActiveBaseUrl: () => activeNevermindBaseUrl,
  });
const { broadcastAuthChanged, sendNevermindDeviceSignInStatus } =
  createNevermindAuthBroadcast({
    sendToPalette: sendToPaletteWindow,
    broadcastToExtensionWindows: (channel, payload) =>
      extensionWindowManager.broadcast(channel, payload),
    isPaletteSender: (sender) => paletteWindow.win?.webContents === sender,
    getExtensionWindowStateForSender: (sender) =>
      extensionWindowManager.getStateForSender(sender),
  });
const {
  signInToSelectedNevermindEnvironment,
  switchNevermindBackendEnvironment,
} = createNevermindSignIn({
  selectedNevermindEnvironment,
  setSelectedEnvironment: (selection) => {
    userStateStore.get().nevermindEnvironment = selection;
  },
  setActiveBaseUrl: setActiveNevermindBaseUrlState,
  isPackaged: () => app.isPackaged,
  scheduleSaveState,
  invalidateExtensionRootItems,
  sendToPalette: sendToPaletteWindow,
  beginAiChatSendSuspension,
  endAiChatSendSuspension,
  disposeAiSessions,
  broadcastAuthChanged,
});
const { processBufferedDeepLink, flushBufferedDeepLinks } =
  createNevermindDeepLinks({
    parseAuthDeepLink,
    bufferedDeepLinks,
    getActiveBaseUrl: () => activeNevermindBaseUrl,
    setActiveBaseUrl: setActiveNevermindBaseUrlState,
    invalidateExtensionRootItems,
    broadcastAuthChanged,
  });

function initExtensionRuntimeContext() {
  initExtensionContext({
    diagnostics: {
      recent: recentDiagnosticRecords,
      report: reportDiagnosticProblem,
    },
    userState: userStateStore.get(),
    fileIndex: currentFileIndex(),
    clipboardService,
    dictationService,
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
    setJobEnabled: (id: string, enabled: boolean) =>
      portableConfigService.commit({
        label: 'job',
        mutate: (draft) => {
          draft.jobs.enabled[id] = enabled;
        },
      }),
    invalidateExtensionRootItems,
    broadcastAuthChanged,
    isPackaged: app.isPackaged,
    activeNevermindBaseUrl,
    setActiveNevermindBaseUrl: (value) => {
      activeNevermindBaseUrl = value;
      setActiveNevermindAuthBaseUrl(value);
    },
    switchNevermindBackendEnvironment,
    getNevermindDebugStatus,
    signInToNevermind: signInToSelectedNevermindEnvironment,
    getNevermindDeviceSignInState,
    onNevermindDeviceSignInChanged,
    retryNevermindDeviceSignInBrowser,
    cancelNevermindDeviceSignIn,
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

const { migrateAiChats } = createAiChatMigration({
  userStateStore,
  getExtensionsDir: () => extensionsDir,
});

const { loadUserState } = createUserStateLoader({
  osCacheRoot,
  ensureLocalFileUrlSecret,
  userDataPath: () => app.getPath('userData'),
  resolveExtensionPaths,
  assignRuntimePaths: (paths) => {
    statePath = paths.statePath;
    iconCacheDir = paths.iconCacheDir;
    clipboardImagesDir = paths.clipboardImagesDir;
    aiChatImagesDir = paths.aiChatImagesDir;
    extensionsDir = paths.extensionsDir;
    extensionStorageDir = paths.extensionStorageDir;
    extensionSecretsDir = paths.extensionSecretsDir;
    extensionCacheDir = paths.extensionCacheDir;
  },
  setLearningStore: (store) => {
    learningStore = store;
  },
  userStateStore,
  settingValue,
  setDiagnosticReporting,
  migrateAiChats,
  setClipboardHistory: (history) => {
    clipboardHistory = history;
  },
  clipboardLimit: CLIPBOARD_LIMIT,
  hydrateJobEnabled: (enabled) => jobRegistry.hydrateEnabled(enabled),
  loadPortableConfig: () => portableConfigService.loadPortableConfig(),
  scheduleSaveState,
});

function captureRuntimeStateIntoUserState() {
  userStateStore.get().clipboardHistory = clipboardHistory;
}
const { pickFormFieldPaths } = createFormFieldPathPicker({
  BrowserWindow,
  dialog,
  paletteWindow,
});
const startup = app.whenReady().then(async () => {
  if (!(await acquireDefaultSharedStateInstance(app, isNvmTestMode))) {
    app.exit(0);
    return;
  }
  if (!isDev && !isNvmTestMode)
    app.setAsDefaultProtocolClient(DEEP_LINK_SCHEME);
  if (isNvmTestMode) {
    installTestNetworkPolicy();
    await loadUserState();
    appReady = true;
    portableConfigService.startPollingPortableConfig();
    await loadExtensions();
    registerActionShortcuts();
    registerTestModeIpcHandlers();
    paletteWindow.createWindow();
    paletteWindow.showPaletteWhenReady();
    return;
  }
  prepareAppWindowPolicy();
  registerLocalFileProtocol({ protocol, thumbnailResponseForPath });
  installPermissionHandlers(isDev, rendererUrl, rendererIndexPath);
  updateManager.configure();
  updateManager.onStateChange(() => {
    patchUpdatesView();
    invalidateExtensionRootItems();
  });
  onNevermindCompatibilityChanged(() => invalidateExtensionRootItems());
  await loadUserState();
  portableConfigService.startPollingPortableConfig();
  await appIndexService.startWatcher();
  await Promise.all([hydrateExtensionDrafts(), hydrateExtensionWindowsState()]);
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
  await initNevermindAi();
  initExtensionContext({ nevermindAi });
  void restorePersistentExtensionWindows();
  appReady = true;
  flushBufferedDeepLinks();
  registerActionShortcuts();
  try {
    await startClipboardWatcher();
  } catch (error) {
    logWarn('clipboard.watcher.start.failed', error, {
      source: 'host',
      scope: 'clipboard',
    });
  }
  powerMonitor.on('resume', () => jobRegistry.emit('wake'));
  jobRegistry.emit('login');
  registerAppIpcHandlers({
    activity: maintenanceActivity,
    ipcMain,
    measureDebugPerformance,
    summarizeDebugValue,
    performanceTrace: performanceTraces,
    startSearch: startProgressiveSearch,
    cancelSearch: cancelProgressiveSearch,
    executeActionForIpc,
    executeViewActionForIpc,
    refreshViewForIpc,
    pickFormFieldPaths,
    startFileDrag,
    sendAiChatMessage,
    setAiChatModel,
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
    unregisterActionShortcuts: releaseAllShortcuts,
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
    createDraftConversationChat,
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
    getNevermindDeviceSignInState,
    sendNevermindDeviceSignInStatus,
    retryNevermindDeviceSignInBrowser,
    cancelNevermindDeviceSignIn,
    invalidateExtensionRootItems,
    broadcastAuthChanged,
    appIconCache,
    runningAppStatus,
    paletteWindow,
    hasCapability,
    processPlatform: process.platform,
    getCameraMediaAccessStatus: () =>
      systemPreferences.getMediaAccessStatus('camera'),
    askForCameraMediaAccess: () =>
      systemPreferences.askForMediaAccess('camera'),
    showRendererIndicator,
    extensionWindowManager,
    saveExtensionDraft: saveExtensionDraftForIpc,
    logError,
    logWarn,
    loggerDebug,
    probeGh: () =>
      extensionPrSubmitter?.probe() ??
      Promise.resolve({ installed: false, authed: false }),
  });
  paletteWindow.createWindow();
  const paletteHotkeyStatus = paletteWindow.registerHotkey();
  activationStatus.set(
    'setting:paletteHotkey',
    paletteHotkeyStatus.registered || !paletteHotkeyStatus.configured
      ? null
      : `Could not register ${paletteHotkeyStatus.accelerator}`,
  );
  if (paletteHotkeyStatus.registered || !paletteHotkeyStatus.configured)
    paletteWindow.showPaletteWhenReady();
  registerDictationIpc({
    ipcMain,
    systemPreferences,
    paletteWindow,
    extensionWindowManager,
    dictationService,
  });
  ipcMain.handle('view:hydrate:retry', async (_event, viewId: string) =>
    maintenanceActivity.run(() =>
      performanceTraces.run('view.loader.retry', { viewId }, async () => {
        if (!viewLoaderRegistry.has(viewId)) return;
        await viewLoaderRegistry.retry(viewId);
      }),
    ),
  );
});
startup.catch((error) => {
  logError('app.start.failed', error, { source: 'host', scope: 'app' });
  app.exit(1);
});
app.on('activate', () => {
  if (appReady) paletteWindow.showPaletteWhenReady();
});
app.on('before-quit', (event) => {
  nevermindApp.isQuiting = true;
  void dictationService.dispose();
  stateSafeQuit.handleBeforeQuit(event);
});
app.on('will-quit', () => {
  nevermindApp.isQuiting = true;
  stateSafeQuit.handleWillQuit();
});
registerInstanceEvents({
  app,
  scheme: DEEP_LINK_SCHEME,
  testMode: isNvmTestMode,
  isReady: () => appReady,
  showPalette: () => paletteWindow.showPaletteWhenReady(),
  processLink: processBufferedDeepLink,
  bufferedLinks: bufferedDeepLinks,
});
