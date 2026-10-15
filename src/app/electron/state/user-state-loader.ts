// biome-ignore-all lint: Startup state loading moved verbatim from main.ts keeps its linear boot sequence and best-effort cleanup.
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import {
  type ClipboardHistoryItem,
  normalizeClipboardHistory,
} from '../clipboard/clipboard-utils';
import { error as logError, warn as logWarn } from '../platform/logger';
import { isNvmTestMode } from '../platform/test-mode';
import { LocalLearningStore } from './learning-store';
import { readUserStateFile } from './user-state';
import { userStateFromLoadedFile } from './user-state-shape';
import type { UserStateStore } from './user-state-store';

const LEARNING_RULES_FILENAME = 'ai-learnings.md';
const LEGACY_LEARNING_RULES_FILENAME = 'ai-learnings.json';
const LEARNING_TRACES_FILENAME = 'ai-learning-traces.json';

export type RuntimePaths = {
  statePath: string;
  iconCacheDir: string;
  clipboardImagesDir: string;
  aiChatImagesDir: string;
  extensionsDir: string;
  extensionStorageDir: string;
  extensionSecretsDir: string;
  extensionCacheDir: string;
};

export type UserStateLoaderDeps = {
  osCacheRoot: () => string;
  ensureLocalFileUrlSecret: () => Promise<void>;
  userDataPath: () => string;
  resolveExtensionPaths: (options: {
    homeDirectory: string;
    userDataDirectory: string;
    useUserDataForExtensions?: boolean;
  }) => { extensionsDir: string };
  assignRuntimePaths: (paths: RuntimePaths) => void;
  setLearningStore: (learningStore: LocalLearningStore) => void;
  userStateStore: Pick<UserStateStore, 'get' | 'replace'>;
  settingValue: (
    settings: Record<string, unknown> | undefined,
    id: 'errorReporting',
  ) => unknown;
  setDiagnosticReporting: (enabled: boolean) => void;
  migrateAiChats: () => Promise<boolean>;
  setClipboardHistory: (history: ClipboardHistoryItem[]) => void;
  clipboardLimit: number;
  hydrateJobEnabled: (enabled: Record<string, boolean>) => void;
  scheduleSaveState: () => void;
};

export function createUserStateLoader(deps: UserStateLoaderDeps) {
  const {
    osCacheRoot,
    ensureLocalFileUrlSecret,
    userDataPath,
    assignRuntimePaths,
    setLearningStore,
    userStateStore,
    settingValue,
    setDiagnosticReporting,
    migrateAiChats,
    setClipboardHistory,
    clipboardLimit,
    hydrateJobEnabled,
    scheduleSaveState,
  } = deps;

  function resolveRuntimePaths(cacheRoot: string): RuntimePaths {
    return {
      statePath: path.join(userDataPath(), 'state.json'),
      iconCacheDir: path.join(cacheRoot, 'icons'),
      clipboardImagesDir: path.join(userDataPath(), 'clipboard-images'),
      aiChatImagesDir: path.join(userDataPath(), 'ai-chat-images'),
      extensionsDir: deps.resolveExtensionPaths({
        homeDirectory: os.homedir(),
        userDataDirectory: userDataPath(),
        useUserDataForExtensions: isNvmTestMode,
      }).extensionsDir,
      extensionStorageDir: path.join(userDataPath(), 'extension-storage'),
      extensionSecretsDir: path.join(userDataPath(), 'extension-secrets'),
      extensionCacheDir: path.join(cacheRoot, 'extension-storage'),
    };
  }

  function createLearningStore() {
    return new LocalLearningStore({
      tracesPath: path.join(userDataPath(), LEARNING_TRACES_FILENAME),
      learningsPath: path.join(userDataPath(), LEARNING_RULES_FILENAME),
      legacyLearningsPath: path.join(
        userDataPath(),
        LEGACY_LEARNING_RULES_FILENAME,
      ),
    });
  }

  async function loadUserState() {
    const cacheRoot = osCacheRoot();
    await ensureLocalFileUrlSecret();
    const paths = resolveRuntimePaths(cacheRoot);
    assignRuntimePaths(paths);
    const learningStore = createLearningStore();
    setLearningStore(learningStore);
    await learningStore.load();
    await fs
      .rm(path.join(userDataPath(), 'icon-cache'), {
        recursive: true,
        force: true,
      })
      .catch(() => {});

    const loaded = await readUserStateFile(paths.statePath, {
      onCorrupt: (error, backupPath) =>
        logWarn(
          'state.load.corrupt',
          {
            backupPath,
            error: error instanceof Error ? error.message : String(error),
          },
          { source: 'host', scope: 'state' },
        ),
      onCorruptBackupError: (error) =>
        logError('state.load.corruptBackupFailed', error, {
          source: 'host',
          scope: 'state',
        }),
      onReadError: (error) =>
        logError('state.load.failed', error, {
          source: 'host',
          scope: 'state',
        }),
    });
    if (loaded) {
      userStateStore.replace(userStateFromLoadedFile(loaded));
    }

    setDiagnosticReporting(
      settingValue(userStateStore.get().settings, 'errorReporting') === true,
    );
    const aiChatsChanged = await migrateAiChats();
    setClipboardHistory(
      await normalizeClipboardHistory(
        userStateStore.get().clipboardHistory,
        clipboardLimit,
        async (png, hash) => {
          const imagePath = path.join(paths.clipboardImagesDir, `${hash}.png`);
          try {
            await fs.mkdir(paths.clipboardImagesDir, { recursive: true });
            await fs.writeFile(imagePath, png);
          } catch {
            // Keep the legacy item even when its image cannot be migrated.
          }
          return imagePath;
        },
      ),
    );
    hydrateJobEnabled(userStateStore.get().jobSettings?.enabled || {});
    if (
      loaded &&
      (aiChatsChanged ||
        loaded.aiChatDefaultModel !== userStateStore.get().aiChatDefaultModel)
    )
      scheduleSaveState();
  }

  return { loadUserState };
}
