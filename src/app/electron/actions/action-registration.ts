import crypto from 'node:crypto';
import path from 'node:path';
import {
  type createActionExecutionCapabilities,
  mutableViewActionFields,
} from './action-execution-policy';
import { actionMatchesExecutionRecord } from './action-execution-record';
import {
  type ActionExecutionRecords,
  clonePlain,
  pruneExecutionRecords,
  withoutExecutionId,
} from './action-execution-records';

const RENDERER_ONLY_VIEW_ACTION_TYPES = new Set([
  'recordShortcut',
  'previewClipboardItem',
]);
export const CHARACTER_INSERT_ACTION_TYPES = new Set(['insertCharacter']);
const HOST_ACTION_OWNER = 'host:main';
const EXTENSION_ACTION_OWNER_PREFIX = 'extension:';

export interface ActionRegistrationDeps {
  viewActionExecutionRecords: ActionExecutionRecords['viewActionExecutionRecords'];
  rootActionExecutionRecords: ActionExecutionRecords['rootActionExecutionRecords'];
  actionExecutionCapabilities: ReturnType<
    typeof createActionExecutionCapabilities
  >;
  extensionModules: Map<string, any>;
}

export function createActionRegistration(deps: ActionRegistrationDeps) {
  const {
    viewActionExecutionRecords,
    rootActionExecutionRecords,
    actionExecutionCapabilities,
    extensionModules,
  } = deps;
  const HOST_ACTION_OWNER_VERSION = crypto.randomUUID();

  function extensionExecutionOwner(action, entry?: any) {
    const inheritedRecord = action?.executionId
      ? viewActionExecutionRecords.get(String(action.executionId)) ||
        rootActionExecutionRecords.get(String(action.executionId))
      : undefined;
    const extensionId =
      entry?.extension?.id ||
      action?.extensionId ||
      inheritedRecord?.extensionId;
    const extensionPath =
      entry?.extension?.__filePath ||
      action?.extensionFile ||
      inheritedRecord?.extensionFile ||
      '';
    const extensionFile = extensionPath ? path.basename(extensionPath) : '';
    return {
      ...(extensionId ? { extensionId: String(extensionId) } : {}),
      ...(extensionFile ? { extensionFile } : {}),
    };
  }

  function actionCapabilityOwner(action, entry?: any) {
    const owner = extensionExecutionOwner(action, entry);
    if (!owner.extensionId)
      return {
        owner: HOST_ACTION_OWNER,
        ownerVersion: HOST_ACTION_OWNER_VERSION,
      };
    const extension =
      entry?.extension || extensionModules.get(String(owner.extensionId));
    return {
      owner: `${EXTENSION_ACTION_OWNER_PREFIX}${owner.extensionId}`,
      ownerVersion: String(extension?.__executionVersion || ''),
    };
  }

  function actionCapabilityOwnerIsCurrent(owner: string, ownerVersion: string) {
    if (owner === HOST_ACTION_OWNER)
      return ownerVersion === HOST_ACTION_OWNER_VERSION;
    if (!owner.startsWith(EXTENSION_ACTION_OWNER_PREFIX)) return false;
    return (
      extensionModules.get(owner.slice(EXTENSION_ACTION_OWNER_PREFIX.length))
        ?.__executionVersion === ownerVersion
    );
  }

  function registerViewActionForRenderer(action, entry?: any) {
    if (!action || typeof action !== 'object') return action;
    if (RENDERER_ONLY_VIEW_ACTION_TYPES.has(String(action.type || '')))
      return action;
    if (CHARACTER_INSERT_ACTION_TYPES.has(String(action.type || '')))
      return action;
    if (
      action.executionId &&
      actionMatchesExecutionRecord(
        action,
        viewActionExecutionRecords.get(String(action.executionId)),
      )
    )
      return action;
    pruneExecutionRecords(viewActionExecutionRecords);
    const stored = clonePlain(withoutExecutionId(action));
    const executionId = actionExecutionCapabilities.issue(stored, {
      scope: 'view',
      ...actionCapabilityOwner(action, entry),
      mutableFields: mutableViewActionFields(stored),
    });
    viewActionExecutionRecords.set(executionId, {
      action: stored,
      createdAt: Date.now(),
      ...extensionExecutionOwner(action, entry),
    });
    return { ...action, executionId };
  }

  function registerRootActionForRenderer(action) {
    if (!action || typeof action !== 'object') return action;
    if (
      action.executionId &&
      actionMatchesExecutionRecord(
        action,
        rootActionExecutionRecords.get(String(action.executionId)),
      )
    )
      return action;
    pruneExecutionRecords(rootActionExecutionRecords);
    const stored = clonePlain(withoutExecutionId(action));
    const executionId = actionExecutionCapabilities.issue(stored, {
      scope: 'root',
      ...actionCapabilityOwner(action),
    });
    rootActionExecutionRecords.set(executionId, {
      action: stored,
      createdAt: Date.now(),
      ...extensionExecutionOwner(action),
    });
    return { ...action, executionId };
  }

  return {
    registerViewActionForRenderer,
    registerRootActionForRenderer,
    actionCapabilityOwnerIsCurrent,
  };
}
