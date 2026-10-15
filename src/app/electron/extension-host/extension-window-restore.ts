// biome-ignore-all lint: Host-side extension window restore handles dynamic extension shapes by design.
import { warn as logWarn } from '../platform/logger';
import { isNvmTestMode } from '../platform/test-mode';
import type { ExtensionRuntimeRegistries } from './extension-runtime-state';
import type { createExtensionWindowManager } from './extension-window-manager';

export type PersistentExtensionWindowRestorerDeps = Pick<
  ExtensionRuntimeRegistries,
  'extensionModules'
> & {
  extensionWindowManager: ReturnType<typeof createExtensionWindowManager>;
  createExtensionContext: (extension: any, command: any) => any;
  normalizeView: (view: any, context: { extension: any; command: any }) => any;
};

export function createPersistentExtensionWindowRestorer(
  deps: PersistentExtensionWindowRestorerDeps,
) {
  const {
    extensionModules,
    extensionWindowManager,
    createExtensionContext,
    normalizeView,
  } = deps;

  async function restorePersistentExtensionWindows() {
    if (isNvmTestMode) return;
    for (const record of extensionWindowManager.persistentWindowRecords()) {
      try {
        const extension = record.ownerExtensionId
          ? extensionModules.get(record.ownerExtensionId)
          : null;
        const descriptor =
          extension && typeof extension.restoreWindow === 'function'
            ? await extension.restoreWindow(
                createExtensionContext(extension, null),
                record.restoreKey,
              )
            : null;
        if (!descriptor?.view) {
          extensionWindowManager.forgetPersistentWindow(record.restoreKey);
          continue;
        }
        extensionWindowManager.createOrUpdate(
          normalizeView(descriptor.view, { extension, command: null }),
          {
            ...(descriptor.options || {}),
            restoreKey: record.restoreKey,
            persistent: true,
          },
          'preserve',
          extension.id,
        );
      } catch (error) {
        logWarn('extensionWindow.restore.failed', {
          restoreKey: record.restoreKey,
          extensionId: record.ownerExtensionId,
          error: error instanceof Error ? error.message : String(error),
        });
        extensionWindowManager.forgetPersistentWindow(record.restoreKey);
      }
    }
  }

  return { restorePersistentExtensionWindows };
}
