import crypto from 'node:crypto';
import path from 'node:path';
import { filterWebviewPermissionsForExtension } from '../extension-host/extension-capabilities';
import type { createExtensionDraftStore } from '../extension-host/extension-draft-store';
import { warn as logWarn } from '../platform/logger';
import {
  type createViewLoaderRegistry,
  isLoaderHandle,
  normalizeLoaderItems,
  resolveLoaderEmptyView,
} from '../state/data-loader';
import type { UserStateStore } from '../state/user-state-store';
import {
  type ViewRefreshRecord,
  withoutExecutionId,
} from './action-execution-records';
import { normalizeItemAppearance } from './item-appearance';

export interface ViewNormalizerDeps {
  extensionActionRegistry: Map<string, any>;
  extensionActionHandlers: Map<string, any>;
  extensionDraftRefs: Map<
    string,
    {
      ownerExtensionId: string;
      key: string;
      version: string | number;
      autosaveAction: any;
    }
  >;
  extensionDraftStore: ReturnType<typeof createExtensionDraftStore>;
  viewLoaderRegistry: ReturnType<typeof createViewLoaderRegistry>;
  viewRefreshRecords: Map<string, ViewRefreshRecord>;
  pruneViewRefreshRecords: () => void;
  registerViewActionForRenderer: (action: any, entry?: any) => any;
  userStateStore: Pick<UserStateStore, 'get'>;
  shortcutForAction: (action: any) => any;
  aiChatIdForExtensionFile: (filename: any) => any;
}

export function createViewNormalizer(deps: ViewNormalizerDeps) {
  const {
    extensionActionRegistry,
    extensionActionHandlers,
    extensionDraftRefs,
    extensionDraftStore,
    viewLoaderRegistry,
    viewRefreshRecords,
    pruneViewRefreshRecords,
    registerViewActionForRenderer,
    userStateStore,
    shortcutForAction,
    aiChatIdForExtensionFile,
  } = deps;

  function registerViewRefreshForRenderer(refresh, entry, view) {
    if (!refresh || typeof refresh !== 'object') return refresh;
    if (refresh.id && !refresh.action) return refresh;
    pruneViewRefreshRecords();
    const { action, ...safeRefresh } = refresh;
    const refreshId = crypto.randomUUID();
    const normalizedAction = action ? normalizeViewAction(action, entry) : null;
    viewRefreshRecords.set(refreshId, {
      entry,
      action: normalizedAction ? withoutExecutionId(normalizedAction) : null,
      viewId: view?.id,
      mode: refresh.mode,
      createdAt: Date.now(),
      running: null,
      failureCount: 0,
    });
    return { ...safeRefresh, id: refreshId };
  }

  function extensionActionFromContribution(entry) {
    const item = entry.item;
    if (!(item?.id && item.title)) return null;
    const extensionFile = entry.extension.__filePath
      ? path.basename(entry.extension.__filePath)
      : undefined;
    const primaryAction = normalizeViewAction(
      item.primaryAction || item.action,
      entry,
    );
    const actionId =
      item.actionId || `extension-action:${entry.extension.id}:${item.id}`;
    const action = {
      id: actionId,
      kind: 'extension-action',
      extensionId: entry.extension.id,
      commandId: item.id,
      registeredActionId: item.id,
      extensionFile,
      aiChatId: extensionFile
        ? aiChatIdForExtensionFile(extensionFile)
        : undefined,
      rootAction: primaryAction,
      removable: Boolean(entry.extension.__generated),
      customizable: item.customizable !== false,
      title: item.title,
      subtitle: item.subtitle || entry.extension.title || 'Extension action',
      aliases: item.aliases || item.keywords || [],
      icon: item.icon || 'sparkles',
      iconUrl: item.image || item.iconUrl || null,
      thumbnailUrl: item.thumbnailUrl || null,
      score: item.score || 12,
      dismissAfterRun: item.dismissAfterRun || primaryAction?.dismissAfterRun,
      background: item.background,
      mode: item.mode,
      actionPanel: normalizeActionPanel(
        item.actionPanel,
        item.actions || [],
        entry,
      ),
      appearance: normalizeItemAppearance(item.appearance),
    };
    const declaredShortcut = userStateStore.get().removedShortcuts?.[action.id]
      ? null
      : item.globalShortcut ||
        (item.shortcutScope === 'global' ? item.shortcut : null);
    const shortcut = shortcutForAction(action) || declaredShortcut;
    return shortcut
      ? { ...action, shortcut, shortcutScope: 'global' as const }
      : action;
  }

  function prepareViewDraft(draft, view, entry) {
    if (!draft || typeof draft !== 'object') return { draft: undefined };
    if (typeof draft.ref === 'string' && extensionDraftRefs.has(draft.ref))
      return { draft };
    const ownerExtensionId = entry?.extension?.id;
    if (!ownerExtensionId) return { draft: undefined };
    try {
      const key = String(draft.key || '');
      const version = draft.version;
      const opened = extensionDraftStore.open(
        ownerExtensionId,
        key,
        version,
        String(view.content || ''),
      );
      const ref = crypto.randomUUID();
      const autosaveAction = draft.autosave?.action
        ? normalizeViewAction(draft.autosave.action, entry)
        : null;
      if (extensionDraftRefs.size >= 1000)
        extensionDraftRefs.delete(extensionDraftRefs.keys().next().value);
      extensionDraftRefs.set(ref, {
        ownerExtensionId,
        key,
        version,
        autosaveAction,
      });
      const prepared = {
        key,
        version,
        ref,
        ...(draft.autosave
          ? {
              autosave: {
                debounceMs: Number(draft.autosave.debounceMs) || 500,
              },
            }
          : {}),
        ...(draft.onConflict
          ? { onConflict: normalizeViewAction(draft.onConflict, entry) }
          : {}),
        ...(opened.kind === 'conflict'
          ? {
              conflict: {
                key,
                storedVersion: opened.conflict.storedVersion,
                storedContent: opened.conflict.storedContent,
                currentVersion: version,
                currentContent: opened.conflict.currentContent,
              },
            }
          : {}),
      };
      structuredClone(prepared);
      return { draft: prepared, content: opened.content };
    } catch (error) {
      logWarn('extension.draft.prepare.failed', {
        extensionId: ownerExtensionId,
        error: error instanceof Error ? error.message : String(error),
      });
      return { draft: undefined };
    }
  }

  function normalizeView(view, entry) {
    const actionCache = new WeakMap<object, any>();
    const actions = normalizeViewActions(view.actions, entry, actionCache);
    const webviewPermissions =
      view.type === 'webview'
        ? filterWebviewPermissionsForExtension(
            entry?.extension,
            view.webviewPermissions,
          )
        : view.webviewPermissions;
    const viewId = view.id || `view:${crypto.randomUUID()}`;

    // Detect and register data loader before stripping it from items
    const loaderHandle = isLoaderHandle(view.items) ? view.items : undefined;
    if (loaderHandle) viewLoaderRegistry.register(viewId, loaderHandle, entry);

    const items = normalizeViewItems(
      normalizeLoaderItems(view.items),
      entry,
      actionCache,
    );
    const sections = Array.isArray(view.sections)
      ? view.sections.map((section) => ({
          ...section,
          items: normalizeViewItems(section.items, entry, actionCache),
        }))
      : view.sections;

    const emptyView = resolveLoaderEmptyView(view.emptyView, loaderHandle);

    const preparedDraft =
      view.type === 'editor' && view.draft
        ? prepareViewDraft(view.draft, view, entry)
        : null;

    return {
      ...view,
      id: viewId,
      ...(webviewPermissions === undefined ? {} : { webviewPermissions }),
      ...(preparedDraft
        ? {
            draft: preparedDraft.draft,
            ...(preparedDraft.content !== undefined
              ? { content: preparedDraft.content }
              : {}),
          }
        : {}),
      actions,
      actionPanel: normalizeActionPanel(
        view.actionPanel,
        actions,
        entry,
        actionCache,
      ),
      onSelectionChange: normalizeViewAction(
        view.onSelectionChange,
        entry,
        actionCache,
      ),
      submitAction: normalizeViewAction(view.submitAction, entry, actionCache),
      searchAccessory: view.searchAccessory
        ? {
            ...view.searchAccessory,
            onChange: normalizeViewAction(
              view.searchAccessory.onChange,
              entry,
              actionCache,
            ),
          }
        : view.searchAccessory,
      refresh: registerViewRefreshForRenderer(view.refresh, entry, view),
      items,
      sections,
      ...(emptyView ? { emptyView } : {}),
      ...(loaderHandle ? { isLoading: true } : {}),
    };
  }

  function persistentActionForRef(action, entry) {
    if (action?.type !== 'runExtensionRegisteredAction') return null;
    const extensionId = action.extensionId || entry?.extension?.id;
    const registeredActionId = action.registeredActionId || action.actionId;
    const registered = extensionActionRegistry.get(
      `${extensionId}:${registeredActionId}`,
    );
    return registered ? extensionActionFromContribution(registered) : null;
  }

  function normalizeViewItems(
    items,
    entry,
    actionCache = new WeakMap<object, any>(),
  ) {
    if (isLoaderHandle(items)) return [];
    return Array.isArray(items)
      ? items.map((item) => {
          const itemActions = normalizeViewActions(
            item.actions,
            entry,
            actionCache,
          );
          const primaryAction = normalizeViewAction(
            item.primaryAction || item.action,
            entry,
            actionCache,
          );
          const { run, __handler, action, ...safeItem } = item;
          const detailActions = normalizeViewActions(
            item.detail?.actions,
            entry,
            actionCache,
          );
          return {
            ...safeItem,
            ...(item.detail
              ? { detail: { ...item.detail, actions: detailActions } }
              : {}),
            actions: itemActions,
            actionPanel: normalizeActionPanel(
              item.actionPanel,
              itemActions,
              entry,
              actionCache,
            ),
            primaryAction,
            persistentAction:
              item.persistentAction ||
              persistentActionForRef(primaryAction, entry),
            appearance: normalizeItemAppearance(item.appearance),
          };
        })
      : items;
  }

  function normalizeActionPanel(
    panel,
    fallbackActions,
    entry,
    actionCache = new WeakMap<object, any>(),
  ) {
    if (panel?.sections)
      return {
        ...panel,
        sections: panel.sections.map((section) => {
          const { lazyActions, ...safeSection } = section;
          return {
            ...safeSection,
            actions: normalizeViewActions(
              [...(section.actions || []), ...(lazyActions || [])],
              entry,
              actionCache,
            ),
          };
        }),
      };
    if (Array.isArray(fallbackActions) && fallbackActions.length)
      return {
        sections: [
          {
            actions: normalizeViewActions(fallbackActions, entry, actionCache),
          },
        ],
      };
    return panel;
  }

  function normalizeViewActions(
    actions,
    entry,
    actionCache = new WeakMap<object, any>(),
  ) {
    return Array.isArray(actions)
      ? actions
          .map((action) => normalizeViewAction(action, entry, actionCache))
          .filter(Boolean)
      : [];
  }

  function normalizeViewAction(
    action,
    entry,
    actionCache = new WeakMap<object, any>(),
  ) {
    if (!action) return null;
    const canCache = typeof action === 'object' && action.submenu;
    if (canCache && actionCache.has(action)) return actionCache.get(action);
    const normalized = normalizeViewActionUncached(action, entry, actionCache);
    if (canCache) actionCache.set(action, normalized);
    return normalized;
  }

  function normalizeViewActionUncached(action, entry, actionCache) {
    if (!action) return null;
    const handler =
      typeof action.__handler === 'function'
        ? action.__handler
        : typeof action.run === 'function'
          ? action.run
          : null;
    if (handler) {
      // Do not register handlers without extension context entries.
      // When entry is null the handler would receive a broken context.
      if (!entry) {
        logWarn('extension.normalizeViewAction.nullEntry', {
          actionType: action.type,
          actionTitle: action.title,
        });
        return { ...action, __handler: undefined, run: undefined };
      }
      const handlerId = crypto.randomUUID();
      extensionActionHandlers.set(handlerId, { entry, handler });
      const { __handler, run, ...rest } = action;
      return normalizeViewAction(
        { ...rest, type: 'runExtensionAction', handlerId },
        entry,
        actionCache,
      );
    }
    const normalized = action.submenu
      ? {
          ...action,
          submenu: normalizeActionPanel(action.submenu, [], entry, actionCache),
        }
      : action;
    if (
      (normalized.type === 'rootView' ||
        normalized.type === 'pushView' ||
        normalized.type === 'replaceView') &&
      normalized.view
    ) {
      return registerViewActionForRenderer(
        {
          ...normalized,
          view: normalizeView(normalized.view, entry),
        },
        entry,
      );
    }
    if (normalized.type === 'promptAction' && normalized.targetAction) {
      return registerViewActionForRenderer(
        {
          ...normalized,
          targetAction: normalizeViewAction(
            normalized.targetAction,
            entry,
            actionCache,
          ),
        },
        entry,
      );
    }
    if (
      (normalized.type === 'createWindow' ||
        normalized.type === 'toggleWindow') &&
      normalized.view
    ) {
      return registerViewActionForRenderer(
        {
          ...normalized,
          view: normalizeView(normalized.view, entry),
          ...(entry?.extension?.id
            ? { ownerExtensionId: entry.extension.id }
            : {}),
        },
        entry,
      );
    }
    return registerViewActionForRenderer(normalized, entry);
  }

  return {
    extensionActionFromContribution,
    normalizeView,
    persistentActionForRef,
    normalizeViewItems,
    normalizeActionPanel,
    normalizeViewAction,
  };
}
