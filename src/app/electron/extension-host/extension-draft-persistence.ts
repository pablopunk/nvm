// biome-ignore-all lint: Draft IPC input and autosave actions are dynamic renderer payloads.
import fs from 'node:fs/promises';
import path from 'node:path';
import { warn as logWarn } from '../platform/logger';
import { isNvmTestMode } from '../platform/test-mode';
import type { createExtensionDraftStore } from './extension-draft-store';
import type { createExtensionJsonStore } from './extension-json-store';

export type ExtensionDraftRef = {
  ownerExtensionId: string;
  key: string;
  version: string | number;
  autosaveAction: any;
};

export type ExtensionDraftPersistenceDeps = {
  extensionDraftStore: ReturnType<typeof createExtensionDraftStore>;
  extensionDraftRefs: Map<string, ExtensionDraftRef>;
  extensionJsonStore: Pick<
    ReturnType<typeof createExtensionJsonStore>,
    'replace'
  >;
  userDataPath: () => string;
  executeViewAction: (action: any) => Promise<unknown>;
};

export function createExtensionDraftPersistence(
  deps: ExtensionDraftPersistenceDeps,
) {
  const {
    extensionDraftStore,
    extensionDraftRefs,
    extensionJsonStore,
    userDataPath,
    executeViewAction,
  } = deps;

  let extensionDraftsHydrated = false;
  let extensionDraftPersistTimer: NodeJS.Timeout | null = null;

  function extensionEditorDraftsFile() {
    return path.join(userDataPath(), 'extension-editor-drafts.json');
  }

  async function hydrateExtensionDrafts() {
    if (extensionDraftsHydrated || isNvmTestMode) return;
    extensionDraftsHydrated = true;
    try {
      const parsed = JSON.parse(
        await fs.readFile(extensionEditorDraftsFile(), 'utf8'),
      );
      extensionDraftStore.hydrate(
        Array.isArray(parsed?.drafts) ? parsed.drafts : [],
      );
    } catch {
      // Missing or malformed draft state is best-effort; start empty.
    }
  }

  function persistExtensionDraftsNow() {
    if (isNvmTestMode) return;
    void extensionJsonStore
      .replace(extensionEditorDraftsFile(), {
        drafts: extensionDraftStore.list() as never,
      })
      .catch((error) =>
        logWarn('extension.draft.persist.failed', {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
  }

  function schedulePersistExtensionDrafts() {
    if (isNvmTestMode) return;
    if (extensionDraftPersistTimer) clearTimeout(extensionDraftPersistTimer);
    extensionDraftPersistTimer = setTimeout(() => {
      extensionDraftPersistTimer = null;
      persistExtensionDraftsNow();
    }, 250);
    extensionDraftPersistTimer.unref?.();
  }

  async function saveExtensionDraftForIpc(input: any = {}) {
    const ref = String(input?.ref || '');
    const record = extensionDraftRefs.get(ref);
    if (!record) return { ok: false, error: 'unknown-draft' };
    const content = String(input?.content ?? '');
    try {
      extensionDraftStore.save(
        record.ownerExtensionId,
        record.key,
        record.version,
        content,
      );
    } catch {
      return { ok: false, error: 'invalid-draft-content' };
    }
    schedulePersistExtensionDrafts();
    if (record.autosaveAction) {
      try {
        await executeViewAction({
          ...record.autosaveAction,
          editorContent: content,
        });
      } catch (error) {
        logWarn('extension.draft.autosave.failed', {
          extensionId: record.ownerExtensionId,
          error: error instanceof Error ? error.message : String(error),
        });
      }
    }
    return { ok: true };
  }

  function commitExtensionDraft(
    extensionId: string,
    key: unknown,
    version: unknown,
  ) {
    const draftKey = String(key || '');
    const updated = extensionDraftStore.commit(
      extensionId,
      draftKey,
      version as string | number,
    );
    if (!updated) return false;
    for (const record of extensionDraftRefs.values()) {
      if (record.ownerExtensionId === extensionId && record.key === draftKey)
        record.version = updated.draftVersion;
    }
    schedulePersistExtensionDrafts();
    return true;
  }

  function discardExtensionDraft(extensionId: string, key: unknown) {
    extensionDraftStore.remove(extensionId, String(key || ''));
    schedulePersistExtensionDrafts();
  }

  return {
    hydrateExtensionDrafts,
    schedulePersistExtensionDrafts,
    saveExtensionDraftForIpc,
    commitExtensionDraft,
    discardExtensionDraft,
  };
}
