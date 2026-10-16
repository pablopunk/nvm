// biome-ignore-all lint: Host-side extension runtime keeps dynamic extension shapes by design.
import path from 'node:path';
import type { JobDefinition, JobRegistry } from '../platform/jobs';
import { sameExtensionSourceFile } from './extension-identity';
import type {
  ExtensionFileWatcher,
  ExtensionOwnedRecord,
  ExtensionRuntimeRegistries,
  ExtensionRuntimeState,
  LiveExtensionRuntimeSnapshot,
  PreparedExtensionRuntime,
  PreparedFileWatcher,
} from './extension-runtime-state';

function replaceMapContents(target: Map<any, any>, source: Map<any, any>) {
  target.clear();
  for (const [key, value] of source) target.set(key, value);
}

function executionRecordBelongsToExtension(
  record: ExtensionOwnedRecord,
  extensionIds: Set<string>,
) {
  return Boolean(record.extensionId && extensionIds.has(record.extensionId));
}

export type ExtensionRuntimeTransactionDeps = ExtensionRuntimeRegistries & {
  jobRegistry: JobRegistry;
  runtimeState: ExtensionRuntimeState;
  registerExtension: (extension: any) => void;
  applyExtensionMetadataOverrides: (extension: any) => Promise<void>;
  attachExtensionFileTrigger: (
    trigger: any,
    event: string,
    extensionId: string,
  ) => void;
  syncFrontmostAppPolling: () => void;
  registerActionShortcuts: () => void;
  invalidateExtensionRootItems: () => void;
};

export function createExtensionRuntimeTransaction(
  deps: ExtensionRuntimeTransactionDeps,
) {
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
    registerExtension,
    applyExtensionMetadataOverrides,
    attachExtensionFileTrigger,
    syncFrontmostAppPolling,
    registerActionShortcuts,
    invalidateExtensionRootItems,
  } = deps;

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
    const existingOwner = extension.id ? live.modules.get(extension.id) : null;
    if (
      existingOwner &&
      !sameExtensionSourceFile(existingOwner.__filePath, activeFile)
    ) {
      throw new Error(
        `Extension id ${extension.id} is already owned by another file`,
      );
    }
    const jobs: JobDefinition[] = [];
    const fileWatchers: PreparedFileWatcher[] = [];
    try {
      extensionModules.clear();
      extensionActionRegistry.clear();
      extensionActionHandlers.clear();
      viewActionExecutionRecords.clear();
      rootActionExecutionRecords.clear();
      viewRefreshRecords.clear();
      runtimeState.extensionRuntimePreparation = { jobs, fileWatchers };
      registerExtension(extension);
      if (!extensionModules.size)
        throw new Error(`Extension ${filename} does not export a valid id`);
      const extensionIds = new Set(extensionModules.keys());
      for (const extensionId of extensionIds) {
        const existing = live.modules.get(extensionId);
        if (
          existing &&
          !sameExtensionSourceFile(existing.__filePath, activeFile)
        )
          throw new Error(
            `Extension id ${extensionId} is already owned by another file`,
          );
      }
      return {
        filePath: path.resolve(activeFile),
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
      runtimeState.extensionRuntimePreparation = undefined;
      replaceMapContents(extensionModules, live.modules);
      replaceMapContents(extensionActionRegistry, live.actions);
      replaceMapContents(extensionActionHandlers, live.handlers);
      replaceMapContents(viewActionExecutionRecords, live.viewActions);
      replaceMapContents(rootActionExecutionRecords, live.rootActions);
      replaceMapContents(viewRefreshRecords, live.viewRefreshes);
    }
  }

  function detachManagedExtensionRuntime(sourceFile: string) {
    const extensionIds = new Set(
      Array.from(extensionModules.entries())
        .filter(([, extension]) =>
          sameExtensionSourceFile(extension?.__filePath, sourceFile),
        )
        .map(([extensionId]) => extensionId),
    );
    for (const extensionId of extensionIds)
      extensionModules.delete(extensionId);
    for (const [key, entry] of extensionActionRegistry)
      if (extensionIds.has(entry?.extension?.id))
        extensionActionRegistry.delete(key);
    for (const [key, record] of extensionActionHandlers)
      if (extensionIds.has(record?.entry?.extension?.id))
        extensionActionHandlers.delete(key);
    for (const [key, record] of viewRefreshRecords)
      if (extensionIds.has(record?.entry?.extension?.id))
        viewRefreshRecords.delete(key);
    for (const [key, record] of viewActionExecutionRecords)
      if (executionRecordBelongsToExtension(record, extensionIds))
        viewActionExecutionRecords.delete(key);
    for (const [key, record] of rootActionExecutionRecords)
      if (executionRecordBelongsToExtension(record, extensionIds))
        rootActionExecutionRecords.delete(key);
    jobRegistry.unregisterWhere(
      (job) => job.owner === 'extension' && extensionIds.has(job.scope || ''),
    );
    const retainedWatchers = [] as typeof runtimeState.extensionFileWatchers;
    for (const watcher of runtimeState.extensionFileWatchers) {
      if (watcher.extensionId && extensionIds.has(watcher.extensionId))
        watcher.close();
      else retainedWatchers.push(watcher);
    }
    runtimeState.extensionFileWatchers = retainedWatchers;
    for (const cacheKey of extensionRootItemsCache.keys())
      if (
        sameExtensionSourceFile(cacheKey, sourceFile) ||
        extensionIds.has(cacheKey)
      )
        extensionRootItemsCache.delete(cacheKey);
    for (const cacheKey of extensionRootItemsRefreshes.keys())
      if (
        sameExtensionSourceFile(cacheKey, sourceFile) ||
        extensionIds.has(cacheKey)
      )
        extensionRootItemsRefreshes.delete(cacheKey);
    return extensionIds;
  }

  function captureManagedExtensionRuntime(
    sourceFile: string,
  ): LiveExtensionRuntimeSnapshot {
    const extensionIds = new Set(
      Array.from(extensionModules.entries())
        .filter(([, extension]) =>
          sameExtensionSourceFile(extension?.__filePath, sourceFile),
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
          runtimeState.extensionFileWatchers
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
    for (const watcher of runtimeState.extensionFileWatchers) {
      if (extensionIds.has(watcher.extensionId)) watcher.close();
      else retainedWatchers.push(watcher);
    }
    runtimeState.extensionFileWatchers = retainedWatchers;
  }

  function restoreManagedExtensionRuntime(
    snapshot: LiveExtensionRuntimeSnapshot,
    candidate: Pick<PreparedExtensionRuntime, 'extensionIds'>,
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
    detachManagedExtensionRuntime(runtime.filePath);
    for (const [key, value] of runtime.modules)
      extensionModules.set(key, value);
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
    if (runtimeState.testExtensionActivationFailurePhase !== phase) return;
    runtimeState.testExtensionActivationFailurePhase = undefined;
    throw new Error(`Injected extension activation failure at ${phase}`);
  }

  return {
    prepareManagedExtensionRuntime,
    detachManagedExtensionRuntime,
    captureManagedExtensionRuntime,
    restoreManagedExtensionRuntime,
    commitPreparedExtensionRuntime,
    failTestExtensionActivationAt,
  };
}
