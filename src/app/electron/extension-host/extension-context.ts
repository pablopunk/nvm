// biome-ignore-all lint: Host-side extension payload boundary keeps dynamic context shapes by design.
import { extensionLogger, error as logError } from '../platform/logger';
import {
  createDataLoaderHandle,
  createStaleWhileRevalidateHandle,
} from '../state/data-loader';
import { buildPromptAction } from './extension-action-builders';
import {
  createContextActions,
  type ContextActionsDeps,
} from './extension-context-actions';
import { trustedContextCapabilities } from './extension-context-capabilities';
import {
  type ContextClipboardHistoryDeps,
  type ContextDesktopDeps,
  createContextClipboardHistory,
  createContextDesktop,
  createContextOcr,
  createContextSystem,
} from './extension-context-desktop';
import {
  type ContextDictationDeps,
  type ContextDraftsDeps,
  type ContextSettingsDeps,
  type ContextShortcutsDeps,
  type ContextUiDeps,
  type ContextUpdatesDeps,
  createContextData,
  createContextDictation,
  createContextDrafts,
  createContextSettings,
  createContextShortcuts,
  createContextText,
  createContextUi,
  createContextUpdates,
} from './extension-context-services';
import {
  type ContextViewsDeps,
  createExtensionViewsApi,
} from './extension-context-views';
import {
  createTextTemplateExpander,
  type TextTemplateDeps,
} from './extension-text-template';
import { createExtensionWindowActions } from './extension-window-actions';

type ExtensionFacetFactory = (extension: any) => any;

export type ExtensionContextDeps = {
  renameExtension: (extension: any, command: any, metadata: any) => unknown;
  ui: ContextUiDeps;
  actions: ContextActionsDeps;
  text: TextTemplateDeps;
  clipboardHistory: ContextClipboardHistoryDeps;
  desktop: ContextDesktopDeps;
  storage: {
    createExtensionStorage: ExtensionFacetFactory;
    createExtensionSecrets: ExtensionFacetFactory;
    createExtensionCache: ExtensionFacetFactory;
  };
  drafts: ContextDraftsDeps;
  settings: ContextSettingsDeps;
  dictation: ContextDictationDeps;
  shortcuts: ContextShortcutsDeps;
  views: ContextViewsDeps;
  updates: ContextUpdatesDeps;
  ai: {
    createExtensionAi: ExtensionFacetFactory;
    createAiBuilderApi: ExtensionFacetFactory;
    createExtensionOwnershipApi: ExtensionFacetFactory;
  };
};

function createExtensionRuntimeMetadata(extension, command, renameExtension) {
  return {
    ...extension,
    rename: (metadata) => renameExtension(extension, command, metadata),
  };
}

export function createExtensionContextFactory(deps: ExtensionContextDeps) {
  const expandTextTemplate = createTextTemplateExpander(deps.text);
  return function createExtensionContext(
    extension,
    command,
    launchContext?: any,
    runtimeOptions: { signal?: AbortSignal } = {},
  ) {
    const { canUseAi, canManageExtensionOwnership } =
      trustedContextCapabilities;
    const context = {
      extension: createExtensionRuntimeMetadata(
        extension,
        command,
        deps.renameExtension,
      ),
      command,
      launch: launchContext ? structuredClone(launchContext) : undefined,
      ...(runtimeOptions.signal ? { signal: runtimeOptions.signal } : {}),
      ui: createContextUi(extension, deps.ui),
      actions: createContextActions(extension, deps.actions),
      action: (input) => input,
      navigation: {
        root: (view) => ({ view, navigation: 'root' }),
        push: (view) => ({ view, navigation: 'push' }),
        replace: (view) => ({ view, navigation: 'replace' }),
        pop: () => ({ navigation: 'pop' }),
        run: (action) => ({ action }),
      },
      system: createContextSystem(),
      text: createContextText(expandTextTemplate),
      input: {
        prompt: buildPromptAction,
      },
      windows: createExtensionWindowActions(),
      clipboard: createContextClipboardHistory(deps.clipboardHistory),
      desktop: createContextDesktop(deps.desktop),
      ocr: createContextOcr(),
      storage: deps.storage.createExtensionStorage(extension),
      secrets: deps.storage.createExtensionSecrets(extension),
      drafts: createContextDrafts(extension, deps.drafts),
      settings: createContextSettings(deps.settings),
      dictation: createContextDictation(extension, deps.dictation),
      shortcuts: createContextShortcuts(deps.shortcuts),
      logs: extensionLogger(extension.id, command?.id),
      cache: deps.storage.createExtensionCache(extension),
      views: createExtensionViewsApi(extension, command, deps.views),
      updates: createContextUpdates(deps.updates),
      state: {},
      data: createContextData(),
      ai: canUseAi ? deps.ai.createExtensionAi(extension) : undefined,
      aiBuilder: deps.ai.createAiBuilderApi(extension),
      extensions: {
        ownership: canManageExtensionOwnership
          ? deps.ai.createExtensionOwnershipApi(extension)
          : undefined,
      },
    };
    // Belt-and-suspenders: ensure `data` is never stripped from the context.
    // Every handler path expects `ctx.data` to be available.
    if (!context.data) {
      logError(
        'extension.context.missingData',
        new Error('ctx.data is missing'),
        {
          source: 'host',
          scope: 'extension',
        },
      );
      // Recover: create a minimal data object so extension code doesn't crash.
      context.data = {
        loader(fn, options: any = {}) {
          return createDataLoaderHandle(fn, options);
        },
        staleWhileRevalidate(params) {
          return createStaleWhileRevalidateHandle(params);
        },
      };
    }
    return context;
  };
}
