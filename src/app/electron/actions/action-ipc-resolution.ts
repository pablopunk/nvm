import { hasCapability } from '../os/os';
import {
  actionFromExecutionRecord,
  type createActionExecutionCapabilities,
  currentActionExecutionRecord,
} from './action-execution-policy';
import {
  ACTION_EXECUTION_TTL_MS,
  type ActionExecutionRecords,
  clonePlain,
  withoutExecutionId,
} from './action-execution-records';
import { CHARACTER_INSERT_ACTION_TYPES } from './action-registration';

const TARGET_DERIVED_VIEW_ACTION_TYPES = new Set([
  'setActionShortcut',
  'setActionAlias',
  'removeActionAlias',
  'duplicateCreatedAction',
  'removeCreatedAction',
  'submitExtensionPr',
  'clearActionOverride',
  'renameExtensionPrompt',
]);

export type ActionIpcResolverDeps = {
  rootActionExecutionRecords: ActionExecutionRecords['rootActionExecutionRecords'];
  viewActionExecutionRecords: ActionExecutionRecords['viewActionExecutionRecords'];
  actionExecutionCapabilities: ReturnType<
    typeof createActionExecutionCapabilities
  >;
  actionCapabilityOwnerIsCurrent: (
    owner: string,
    ownerVersion: string,
  ) => boolean;
  currentActionForStoredShortcut: (action: any) => any;
  characterRecordsById: ReadonlyMap<
    string,
    { glyph: string; supportsSkinTone?: boolean }
  >;
  characterCodePoints: (glyph: string) => string;
  characterWithSkinTone: (glyph: string, skinTone: number) => string;
};

export function createActionIpcResolver(deps: ActionIpcResolverDeps) {
  const {
    rootActionExecutionRecords,
    viewActionExecutionRecords,
    actionExecutionCapabilities,
    actionCapabilityOwnerIsCurrent,
    currentActionForStoredShortcut,
    characterRecordsById,
    characterCodePoints,
    characterWithSkinTone,
  } = deps;

  function mergeRendererActionInput(storedAction, rendererAction) {
    const merged = { ...storedAction };
    if (rendererAction && typeof rendererAction === 'object') {
      if ('formValues' in rendererAction)
        merged.formValues = rendererAction.formValues;
      if ('editorContent' in rendererAction)
        merged.editorContent = rendererAction.editorContent;
      if ('selectedItemId' in rendererAction)
        merged.selectedItemId = rendererAction.selectedItemId;
      if ('value' in rendererAction) merged.value = rendererAction.value;
      if ('text' in rendererAction && !('text' in storedAction))
        merged.text = rendererAction.text;
    }
    return merged;
  }

  function resolveRootActionForIpc(action) {
    if (!action || typeof action !== 'object')
      throw new Error('Untrusted action');
    const record = currentActionExecutionRecord(
      action.executionId,
      rootActionExecutionRecords,
      Date.now(),
      ACTION_EXECUTION_TTL_MS,
    );
    if (record)
      return actionFromExecutionRecord(action, rootActionExecutionRecords, {
        actionName: (input) => String(input.kind || 'root'),
        ttlMs: ACTION_EXECUTION_TTL_MS,
      });
    const capabilityAction = actionExecutionCapabilities.resolve(action, {
      scope: 'root',
      ownerIsCurrent: actionCapabilityOwnerIsCurrent,
      maxAgeMs: ACTION_EXECUTION_TTL_MS,
    });
    if (capabilityAction) return capabilityAction;
    const fallback = withoutExecutionId(action);
    const current = currentActionForStoredShortcut(fallback);
    if (current && current !== fallback)
      return {
        ...clonePlain(withoutExecutionId(current)),
        ...(typeof action.traceId === 'string'
          ? { traceId: action.traceId }
          : {}),
      };
    throw new Error(`Untrusted ${fallback.kind || 'root'} action`);
  }

  function resolveActionReferenceForIpc(action) {
    if (!action || typeof action !== 'object')
      throw new Error('Untrusted action reference');
    if (
      currentActionExecutionRecord(
        action.executionId,
        rootActionExecutionRecords,
        Date.now(),
        ACTION_EXECUTION_TTL_MS,
      )
    )
      return resolveRootActionForIpc(action);
    const rootCapabilityAction = actionExecutionCapabilities.resolve(action, {
      scope: 'root',
      ownerIsCurrent: actionCapabilityOwnerIsCurrent,
      maxAgeMs: ACTION_EXECUTION_TTL_MS,
    });
    if (rootCapabilityAction) return rootCapabilityAction;
    return resolveViewActionForIpc(action);
  }

  function filePathsForAction(action, paths = new Set<string>()) {
    if (!action || typeof action !== 'object') return paths;
    for (const value of [action.path, action.filePath])
      if (typeof value === 'string' && value) paths.add(value);
    for (const nested of [
      action.rootAction,
      action.primaryAction,
      action.nativeAction,
    ])
      filePathsForAction(nested, paths);
    return paths;
  }

  function resolveRendererDerivedViewAction(action) {
    const type = String(action.type || '');
    if (TARGET_DERIVED_VIEW_ACTION_TYPES.has(type)) {
      const targetAction = resolveRootActionForIpc(action.targetAction);
      const mutable =
        type === 'setActionShortcut'
          ? { accelerator: action.accelerator }
          : type === 'setActionAlias' || type === 'removeActionAlias'
            ? { alias: action.alias }
            : {};
      return { type, title: action.title, targetAction, ...mutable };
    }
    if (type === 'setSettingShortcut' && action.settingId === 'hyperKey')
      return {
        type,
        title: action.title,
        settingId: 'hyperKey',
        shortcut: action.shortcut,
      };
    if (type === 'quickLook' || type === 'revealPath') {
      const sourceAction = resolveActionReferenceForIpc(action.sourceAction);
      const requestedPath = String(action.path || '');
      if (!filePathsForAction(sourceAction).has(requestedPath))
        throw new Error(`Untrusted ${type} path`);
      return { type, title: action.title, path: requestedPath };
    }
    return null;
  }

  function resolveViewActionForIpc(action) {
    if (!action || typeof action !== 'object')
      throw new Error('Untrusted action');
    const record = action.executionId
      ? viewActionExecutionRecords.get(String(action.executionId))
      : null;
    if (record)
      return mergeRendererActionInput(
        { ...clonePlain(record.action), traceId: action.traceId },
        action,
      );
    const capabilityAction = actionExecutionCapabilities.resolve(action, {
      scope: 'view',
      ownerIsCurrent: actionCapabilityOwnerIsCurrent,
    });
    if (capabilityAction) return capabilityAction;
    const fallback = withoutExecutionId(action);
    if (CHARACTER_INSERT_ACTION_TYPES.has(String(fallback.type || ''))) {
      const record = characterRecordsById.get(
        String(fallback.characterId || ''),
      );
      if (!record) throw new Error('Untrusted character insertion action');
      const skinTone = Number(fallback.skinTone || 0);
      if (!Number.isInteger(skinTone) || skinTone < 0 || skinTone > 5)
        throw new Error('Untrusted character skin tone');
      if (skinTone > 0 && !record.supportsSkinTone)
        throw new Error('Character does not support skin tones');
      const mode =
        fallback.mode === 'copyUnicode' ? 'copyUnicode' : fallback.mode;
      if (!['copy', 'paste', 'copyUnicode'].includes(mode))
        throw new Error('Untrusted character insertion mode');
      if (mode === 'paste' && !hasCapability('frontmost-paste'))
        throw new Error('Frontmost paste is unavailable');
      return {
        ...fallback,
        mode,
        text:
          mode === 'copyUnicode'
            ? characterCodePoints(record.glyph)
            : characterWithSkinTone(record.glyph, skinTone),
      };
    }
    if (fallback.type === 'nativeAction') {
      const traceId =
        typeof fallback.traceId === 'string' ? fallback.traceId : undefined;
      return {
        type: 'nativeAction',
        title: fallback.title,
        nativeAction: resolveRootActionForIpc({
          ...fallback.nativeAction,
          ...(traceId ? { traceId } : {}),
        }),
        ...(traceId ? { traceId } : {}),
      };
    }
    const derived = resolveRendererDerivedViewAction(fallback);
    if (derived) return derived;
    throw new Error(`Untrusted ${fallback.type || 'view'} action`);
  }

  return { resolveRootActionForIpc, resolveViewActionForIpc };
}
