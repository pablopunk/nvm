// biome-ignore-all lint: Extension manager state is a loosely shaped persisted document.
import fs from 'node:fs/promises';
import path from 'node:path';
import { operationStage, runOperation } from '../diagnostics/observability';
import type { UserStateStore } from '../state/user-state-store';
import { isExtensionSourceFile } from './extension-source-files';

export type ExtensionProposalsDeps = {
  userStateStore: UserStateStore;
  saveUserState: () => Promise<void>;
  persistUserState: () => Promise<void>;
  getExtensionsDir: () => string;
  userDataPath: () => string;
};

export function createExtensionProposals(deps: ExtensionProposalsDeps) {
  const {
    userStateStore,
    saveUserState,
    persistUserState,
    getExtensionsDir,
    userDataPath,
  } = deps;

  async function initializeExtensionManager() {
    const manager = userStateStore.get().extensionManager;
    const entries = await fs
      .readdir(getExtensionsDir(), { withFileTypes: true })
      .catch(() => []);
    const sourceFiles = entries
      .filter((entry) => isExtensionSourceFile(entry.name))
      .map((entry) => entry.name);
    if (manager?.schemaVersion !== 1) {
      userStateStore.get().extensionManager = {
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
    return (
      userStateStore.get().extensionManager?.files?.[filename]?.enabled === true
    );
  }

  function extensionDraftsDir() {
    return path.join(userDataPath(), 'extension-drafts');
  }

  function extensionManagerState() {
    return (userStateStore.get().extensionManager ||= {
      schemaVersion: 1,
      files: {},
      proposals: {},
    });
  }

  function stageExtensionProposal(filename: string, source: string) {
    return runOperation('extension.generate', () =>
      validateExtensionProposal(filename, source),
    );
  }

  async function validateExtensionProposal(filename: string, source: string) {
    operationStage('validate');
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

  async function discardExtensionProposal(filename: string) {
    const safeName = path.basename(filename);
    const proposal = extensionManagerState().proposals[safeName];
    if (!proposal) return;
    delete extensionManagerState().proposals[safeName];
    await fs.unlink(proposal.draftFile).catch(() => {});
    await persistUserState();
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
      const activeFile = path.join(getExtensionsDir(), filename);
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

  return {
    initializeExtensionManager,
    extensionFileIsEnabled,
    extensionDraftsDir,
    extensionManagerState,
    stageExtensionProposal,
    discardExtensionProposal,
    discardManagedExtensionProposal,
    managedExtensionEntries,
  };
}
