// biome-ignore-all lint: Host-side extension payload boundary keeps dynamic service API shapes by design.
import type { DictationService } from '../dictation/dictation-service';
import {
  createDataLoaderHandle,
  createStaleWhileRevalidateHandle,
} from '../state/data-loader';
import {
  buildConfirmAction,
  buildPreviewItemAction,
  progressView,
} from './extension-action-builders';
import {
  denyShortcut,
  trustedContextCapabilities,
  trustedExtensionApiUnavailable,
} from './extension-context-capabilities';
import { createExtensionUiApi } from './extension-ui-api';
import type { createExtensionWindowManager } from './extension-window-manager';

export type ContextUiDeps = {
  extensionWindowManager: Pick<
    ReturnType<typeof createExtensionWindowManager>,
    'showIndicator' | 'updateIndicator' | 'hideIndicator'
  >;
};

export function createContextUi(extension, deps: ContextUiDeps) {
  const { extensionWindowManager } = deps;
  return createExtensionUiApi({
    buildPreviewItemAction,
    progressView,
    buildConfirmAction,
    showIndicator: (input) =>
      extensionWindowManager.showIndicator(input, extension.id),
    updateIndicator: (input) =>
      extensionWindowManager.updateIndicator(input, extension.id),
    hideIndicator: (id) =>
      extensionWindowManager.hideIndicator(extension.id, id),
  });
}

export function createContextText(expandTextTemplate) {
  const { canUseClipboard } = trustedContextCapabilities;
  return {
    template: (input, variables) =>
      expandTextTemplate(input, variables, {
        includeClipboard: canUseClipboard,
      }),
  };
}

export type ContextDraftsDeps = {
  commitExtensionDraft: (
    extensionId: string,
    key: unknown,
    version: unknown,
  ) => boolean;
  discardExtensionDraft: (extensionId: string, key: unknown) => void;
};

export function createContextDrafts(extension, deps: ContextDraftsDeps) {
  const { commitExtensionDraft, discardExtensionDraft } = deps;
  return {
    commit: (key, version) => commitExtensionDraft(extension.id, key, version),
    discard: (key) => discardExtensionDraft(extension.id, key),
  };
}

export type ContextSettingsDeps = {
  availableSettingDefinitions: () => any[];
  getSetting: (id: any) => any;
  setSetting: (id: any, value: any) => { ok: boolean; message?: string };
  settingDefinition: (id: string) => any;
  toggledSettingValue: (definition: any, current: any) => any;
};

export function createContextSettings(deps: ContextSettingsDeps) {
  const { canWriteSettings } = trustedContextCapabilities;
  const {
    availableSettingDefinitions,
    getSetting,
    setSetting,
    settingDefinition,
    toggledSettingValue,
  } = deps;
  return {
    definitions: () =>
      availableSettingDefinitions().map((definition) => ({
        ...definition,
        value: getSetting(definition.id),
      })),
    get: (id) => getSetting(id),
    set: canWriteSettings
      ? async (id, value) => {
          const result = await setSetting(id, value);
          if (!result.ok) throw new Error(result.message);
          return value;
        }
      : async () => {
          throw trustedExtensionApiUnavailable('settings.write');
        },
    toggle: canWriteSettings
      ? async (id) => {
          const definition = settingDefinition(id);
          if (!definition) throw new Error(`Unknown setting: ${id}`);
          const next = toggledSettingValue(definition, getSetting(id));
          const result = await setSetting(id, next);
          if (!result.ok) throw new Error(result.message);
          return next;
        }
      : async () => {
          throw trustedExtensionApiUnavailable('settings.write');
        },
  };
}

export type ContextDictationDeps = { dictationService: DictationService };

export function createContextDictation(extension, deps: ContextDictationDeps) {
  const { canUseDictation } = trustedContextCapabilities;
  const { dictationService } = deps;
  return canUseDictation
    ? extension.id === 'nevermind.dictation'
      ? dictationService
      : {
          status: dictationService.status,
          apiAvailable: dictationService.apiAvailable,
          devices: dictationService.devices,
          start: dictationService.start,
          stop: dictationService.stop,
          cancel: dictationService.cancel,
        }
    : undefined;
}

export type ContextShortcutsDeps = {
  extensionShortcutRecords: () => unknown[];
  getPaletteHotkey: () => string;
};

export function createContextShortcuts(deps: ContextShortcutsDeps) {
  const { canUseShortcuts } = trustedContextCapabilities;
  const { extensionShortcutRecords, getPaletteHotkey } = deps;
  return canUseShortcuts
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
      };
}

export type ContextUpdatesDeps = { updatesStateSnapshot: () => unknown };

export function createContextUpdates(deps: ContextUpdatesDeps) {
  const { canUseUpdates } = trustedContextCapabilities;
  const { updatesStateSnapshot } = deps;
  return canUseUpdates ? { getState: () => updatesStateSnapshot() } : undefined;
}

export function createContextData() {
  return {
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
  };
}
