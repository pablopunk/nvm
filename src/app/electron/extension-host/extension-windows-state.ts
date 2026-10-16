// biome-ignore-all lint: Window state persistence moved verbatim from main.ts keeps compact guards and fire-and-forget writes.
import fs from 'node:fs/promises';
import path from 'node:path';
import { warn as logWarn } from '../platform/logger';
import { isNvmTestMode } from '../platform/test-mode';
import type { createExtensionJsonStore } from './extension-json-store';

export type ExtensionWindowsStateDeps = {
  extensionJsonStore: Pick<
    ReturnType<typeof createExtensionJsonStore>,
    'replace'
  >;
  userDataPath: () => string;
  hydratePersistedState: (state: Record<string, unknown>) => void;
};

export function createExtensionWindowsState(deps: ExtensionWindowsStateDeps) {
  const { extensionJsonStore, userDataPath, hydratePersistedState } = deps;

  let extensionWindowsStateCache: Record<string, unknown> | null = null;

  function extensionWindowsStateFile() {
    return path.join(userDataPath(), 'extension-windows.json');
  }

  async function hydrateExtensionWindowsState() {
    if (extensionWindowsStateCache || isNvmTestMode) return;
    try {
      const parsed = JSON.parse(
        await fs.readFile(extensionWindowsStateFile(), 'utf8'),
      );
      extensionWindowsStateCache =
        parsed && typeof parsed === 'object' ? parsed : {};
    } catch {
      extensionWindowsStateCache = {};
    }
    hydratePersistedState(extensionWindowsStateCache);
  }

  function writeExtensionWindowsState(state: Record<string, unknown>) {
    extensionWindowsStateCache = state;
    if (isNvmTestMode) return;
    void extensionJsonStore
      .replace(extensionWindowsStateFile(), state as never)
      .catch((error) =>
        logWarn('extensionWindow.persist.failed', {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
  }

  return { hydrateExtensionWindowsState, writeExtensionWindowsState };
}
