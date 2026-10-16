import path from 'node:path';
import { appResultMarker } from '../search/action-ranking';
import { normalizeItemAppearance } from './item-appearance';

function actionsAreSame(left, right) {
  if (left === right) return true;
  if (
    left?.type === 'runExtensionRegisteredAction' &&
    right?.type === 'runExtensionRegisteredAction'
  )
    return (
      left.extensionId === right.extensionId &&
      (left.registeredActionId || left.actionId) ===
        (right.registeredActionId || right.actionId)
    );
  return Boolean(left?.actionId && left.actionId === right?.actionId);
}

function withoutPrimaryAction(actions, primaryAction) {
  return Array.isArray(actions)
    ? actions.filter((action) => !actionsAreSame(action, primaryAction))
    : actions;
}

function withoutPrimaryActionFromPanel(panel, primaryAction) {
  if (!panel?.sections) return panel;
  return {
    ...panel,
    sections: panel.sections.map((section) => ({
      ...section,
      actions: withoutPrimaryAction(section.actions, primaryAction),
      ...(section.lazyActions
        ? {
            lazyActions: withoutPrimaryAction(
              section.lazyActions,
              primaryAction,
            ),
          }
        : {}),
    })),
  };
}

export interface ExtensionRootActionNormalizerDeps {
  extensionActionRegistry: Map<string, any>;
  persistentActionForRef: (action: any, entry: any) => any;
  extensionActionFromContribution: (entry: any) => any;
  normalizeViewAction: (action: any, entry: any) => any;
  normalizeActionPanel: (panel: any, fallbackActions: any, entry: any) => any;
  withInheritedShortcutLifecycle: (action: any, inheritedFrom: unknown) => any;
}

export function createExtensionRootActionNormalizer(
  deps: ExtensionRootActionNormalizerDeps,
) {
  const {
    extensionActionRegistry,
    persistentActionForRef,
    extensionActionFromContribution,
    normalizeViewAction,
    normalizeActionPanel,
    withInheritedShortcutLifecycle,
  } = deps;

  function persistentActionForRootItem(entry, item, primaryAction) {
    const referencedAction = persistentActionForRef(primaryAction, entry);
    if (referencedAction) return referencedAction;
    if (!item.actionId) return null;
    const registered = extensionActionRegistry.get(
      `${entry.extension.id}:${item.actionId}`,
    );
    return registered ? extensionActionFromContribution(registered) : null;
  }

  function extensionRootActionFromItem(entry, item) {
    if (!(item?.id && item.title)) return null;
    const primarySource = item.primaryAction || item.action;
    const persistentAction = persistentActionForRootItem(
      entry,
      item,
      primarySource,
    );
    const primaryAction = normalizeViewAction(primarySource, entry);
    const actionPanel = normalizeActionPanel(
      withoutPrimaryActionFromPanel(item.actionPanel, primarySource),
      withoutPrimaryAction(item.actions || [], primarySource),
      entry,
    );
    const globalShortcut =
      item.globalShortcut ||
      (item.shortcutScope === 'global' ? item.shortcut : undefined);
    const globalAction = persistentAction || primaryAction;
    return withInheritedShortcutLifecycle(
      {
        id: `extension-root:${entry.extension.id}:${item.id}`,
        kind: 'extension-root-item',
        extensionId: entry.extension.id,
        ...appResultMarker(item),
        commandId: item.id,
        extensionFile: entry.extension.__filePath
          ? path.basename(entry.extension.__filePath)
          : undefined,
        rootAction: primaryAction,
        ...(globalShortcut || globalAction?.shortcutScope === 'global'
          ? {
              shortcut: globalShortcut || globalAction.shortcut,
              shortcutScope: 'global',
            }
          : {}),
        persistentAction,
        removable: Boolean(entry.extension.__generated),
        title: item.title,
        subtitle: item.subtitle || entry.extension.title || 'Extension item',
        aliases: item.aliases || item.keywords || [],
        icon: item.icon || 'sparkles',
        iconUrl: item.image || item.iconUrl || null,
        thumbnailUrl: item.thumbnailUrl || null,
        videoUrl: item.videoUrl || null,
        imageDataUrl: item.imageDataUrl || null,
        filePath: item.filePath || item.path || null,
        text: item.text || '',
        score: Math.min(Number(item.score || 35), 90),
        lastUsed: Number(item.lastUsed || 0),
        background: item.background,
        mode: item.mode,
        dismissAfterRun: item.dismissAfterRun || primaryAction?.dismissAfterRun,
        customizable: item.customizable ?? Boolean(persistentAction),
        actionPanel,
        appearance: normalizeItemAppearance(item.appearance),
      },
      persistentAction,
    );
  }

  return { extensionRootActionFromItem };
}
