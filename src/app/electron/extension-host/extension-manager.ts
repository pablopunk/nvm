// biome-ignore-all lint: Extension manager state is a loosely shaped persisted document.
import fs from 'node:fs/promises';
import path from 'node:path';
import { operationStage, runOperation } from '../diagnostics/observability';
import { atomicWriteFile } from '../platform/atomic-file';
import { loadExtensionModule } from './extension-module-loader';
import type {
  LiveExtensionRuntimeSnapshot,
  PreparedExtensionRuntime,
} from './extension-runtime-state';

export type ExtensionManagerDeps = {
  getExtensionsDir: () => string;
  persistUserState: () => Promise<void>;
  extensionManagerState: () => any;
  prepareManagedExtensionRuntime: (
    filename: string,
    activeFile: string,
    extension: any,
  ) => Promise<PreparedExtensionRuntime>;
  detachManagedExtensionRuntime: (sourceFile: string) => Set<string>;
  captureManagedExtensionRuntime: (
    sourceFile: string,
  ) => LiveExtensionRuntimeSnapshot;
  restoreManagedExtensionRuntime: (
    snapshot: LiveExtensionRuntimeSnapshot,
    candidate: Pick<PreparedExtensionRuntime, 'extensionIds'>,
  ) => void;
  commitPreparedExtensionRuntime: (runtime: PreparedExtensionRuntime) => void;
  failTestExtensionActivationAt: (phase: string) => void;
  syncFrontmostAppPolling: () => void;
  registerActionShortcuts: () => void;
  invalidateExtensionRootItems: () => void;
};

export function createExtensionManager(deps: ExtensionManagerDeps) {
  const {
    getExtensionsDir,
    persistUserState,
    extensionManagerState,
    prepareManagedExtensionRuntime,
    detachManagedExtensionRuntime,
    captureManagedExtensionRuntime,
    restoreManagedExtensionRuntime,
    commitPreparedExtensionRuntime,
    failTestExtensionActivationAt,
    syncFrontmostAppPolling,
    registerActionShortcuts,
    invalidateExtensionRootItems,
  } = deps;

  function activateManagedExtension(filename: string) {
    return runOperation('extension.install', () =>
      installManagedExtension(filename),
    );
  }

  async function installManagedExtension(filename: string) {
    operationStage('install');
    const safeName = path.basename(filename);
    const manager = extensionManagerState();
    const proposal = manager.proposals?.[safeName];
    const activeFile = path.join(getExtensionsDir(), safeName);
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
      previousRuntime = captureManagedExtensionRuntime(activeFile);
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
        if (previousSource === null)
          await fs.unlink(activeFile).catch(() => {});
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
    detachManagedExtensionRuntime(path.join(getExtensionsDir(), safeName));
    syncFrontmostAppPolling();
    registerActionShortcuts();
    invalidateExtensionRootItems();
  }

  return { activateManagedExtension, disableManagedExtension };
}
