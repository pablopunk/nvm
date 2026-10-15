// biome-ignore-all lint: Host-side extension loading handles dynamic extension shapes by design.
import fs from 'node:fs/promises';
import path from 'node:path';
import {
  markDebugPerformance,
  measureDebugPerformance,
} from '../diagnostics/debug-performance';
import { isNvmTestMode } from '../platform/test-mode';
import { error as logError, warn as logWarn } from '../platform/logger';
import type { JobRegistry } from '../platform/jobs';
import { createProgressiveSearchTestExtension } from '../search/search-test-extension';
import {
  applyExtensionMetadata,
  normalizedExtensionMetadata,
} from './extension-metadata';
import { loadExtensionModule } from './extension-module-loader';
import type {
  ExtensionRuntimeRegistries,
  ExtensionRuntimeState,
} from './extension-runtime-state';
import {
  compareExtensionSourceEntries,
  isExtensionSourceFile,
} from './extension-source-files';

export type ExtensionLoaderDeps = ExtensionRuntimeRegistries & {
  jobRegistry: JobRegistry;
  runtimeState: ExtensionRuntimeState;
  isDev: boolean;
  EXTENSION_TYPES_FILENAME: string;
  getExtensionsDir: () => string;
  bundledResourcePath: (...relativePath: string[]) => Promise<string>;
  initExtensionRuntimeContext: () => void;
  stopFrontmostAppPolling: () => void;
  syncFrontmostAppPolling: () => void;
  registerExtension: (extension: any) => void;
  registerInternalExtensions: () => void;
  loadDevExtensions: () => Promise<void>;
  initializeExtensionManager: () => Promise<void>;
  extensionFileIsEnabled: (filename: string) => boolean;
  readExtensionStorage: (extension: any) => Promise<any>;
  mutateExtensionStorage: (
    extension: any,
    mutate: (current: any) => any,
  ) => Promise<any>;
  registerActionShortcuts: () => void;
};

export function createExtensionLoader(deps: ExtensionLoaderDeps) {
  const {
    extensionModules,
    extensionActionRegistry,
    extensionActionHandlers,
    viewActionExecutionRecords,
    rootActionExecutionRecords,
    viewRefreshRecords,
    extensionRootItemsCache,
    extensionRootItemsRefreshes,
    jobRegistry,
    runtimeState,
    isDev,
    EXTENSION_TYPES_FILENAME,
    getExtensionsDir,
    bundledResourcePath,
    initExtensionRuntimeContext,
    stopFrontmostAppPolling,
    syncFrontmostAppPolling,
    registerExtension,
    registerInternalExtensions,
    loadDevExtensions,
    initializeExtensionManager,
    extensionFileIsEnabled,
    readExtensionStorage,
    mutateExtensionStorage,
    registerActionShortcuts,
  } = deps;

  async function ensureExtensionTypeDefinitions() {
    if (!getExtensionsDir()) return;
    const sourcePath = await bundledResourcePath(EXTENSION_TYPES_FILENAME);
    const targetPath = path.join(getExtensionsDir(), EXTENSION_TYPES_FILENAME);
    await fs.copyFile(sourcePath, targetPath).catch((error) => {
      logWarn(
        'extension.types.copy.failed',
        { error: error?.message || String(error) },
        { source: 'host', scope: 'extension' },
      );
    });
    await fs
      .writeFile(
        path.join(getExtensionsDir(), 'package.json'),
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

  async function loadExtensions(preparedExtensions = new Map<string, any>()) {
    await measureDebugPerformance(
      'extensions.load-all',
      { alwaysLog: true },
      async () => {
        extensionActionRegistry.clear();
        extensionModules.clear();
        runtimeState.fixtureExtensions = [];
        extensionRootItemsCache.clear();
        extensionRootItemsRefreshes.clear();
        extensionActionHandlers.clear();
        viewActionExecutionRecords.clear();
        rootActionExecutionRecords.clear();
        viewRefreshRecords.clear();
        jobRegistry.unregisterWhere((job) => job.owner === 'extension');
        stopFrontmostAppPolling();
        for (const watcher of runtimeState.extensionFileWatchers)
          watcher.close();
        runtimeState.extensionFileWatchers = [];
        initExtensionRuntimeContext();
        registerInternalExtensions();
        if (isNvmTestMode)
          registerExtension(createProgressiveSearchTestExtension());
        if (isDev)
          await measureDebugPerformance('extensions.load-dev', undefined, () =>
            loadDevExtensions(),
          );

        await fs.mkdir(getExtensionsDir(), { recursive: true });
        await ensureExtensionTypeDefinitions();
        await initializeExtensionManager();
        const entries = (
          await fs
            .readdir(getExtensionsDir(), { withFileTypes: true })
            .catch(() => [])
        ).sort(compareExtensionSourceEntries);
        for (const entry of entries) {
          if (!isExtensionSourceFile(entry.name)) continue;
          if (!extensionFileIsEnabled(entry.name)) continue;
          const fullPath = path.join(getExtensionsDir(), entry.name);
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

  async function applyExtensionMetadataOverrides(extension) {
    const metadata = (await readExtensionStorage(extension)).__metadata;
    if (!metadata || typeof metadata !== 'object') return;
    applyExtensionMetadata(extension, metadata);
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

  return {
    loadExtensions,
    applyExtensionMetadataOverrides,
    renameExtension,
  };
}
