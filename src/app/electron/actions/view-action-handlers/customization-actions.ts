import type { createExtensionPrSubmitter } from '../../extension-host/extension-pr-submitter';

export type CustomizationActionHandlerDeps = {
  setShortcut: (action: any, accelerator: any) => Promise<any>;
  removeShortcut: (actionId: any) => Promise<any>;
  setAlias: (action: any, alias: any) => Promise<any>;
  removeAlias: (action: any, alias: any) => Promise<any>;
  clearOverride: (action: any) => Promise<any>;
  duplicateCreatedAction: (action: any) => Promise<any>;
  removeCreatedAction: (action: any) => Promise<any>;
  getExtensionPrSubmitter: () => ReturnType<
    typeof createExtensionPrSubmitter
  > | null;
};

export function createCustomizationActionHandlers(
  deps: CustomizationActionHandlerDeps,
) {
  const {
    setShortcut,
    removeShortcut,
    setAlias,
    removeAlias,
    clearOverride,
    duplicateCreatedAction,
    removeCreatedAction,
    getExtensionPrSubmitter,
  } = deps;

  async function handleSetActionShortcut(action) {
    const result = await setShortcut(
      action.targetAction || action.action,
      action.accelerator || action.shortcut,
    );
    return {
      toast: {
        message: result.message,
        tone: result.ok ? 'default' : 'error',
      },
      ok: result.ok,
    };
  }

  async function handleRemoveShortcut(action) {
    const result = await removeShortcut(action.actionId);
    if (!result.ok)
      return { toast: { message: result.message, tone: 'error' } };
    return {
      patch: { removeItemIds: [`shortcut:${action.actionId}`] },
      toast: { message: result.message },
      ok: true,
    };
  }

  async function handleSetActionAlias(action) {
    const result = await setAlias(
      action.targetAction || action.action,
      action.alias,
    );
    return {
      toast: {
        message: result.message,
        tone: result.ok ? 'default' : 'error',
      },
      ok: result.ok,
    };
  }

  async function handleRemoveActionAlias(action) {
    const result = await removeAlias(
      action.targetAction || action.action,
      action.alias,
    );
    return {
      toast: {
        message: result.message,
        tone: result.ok ? 'default' : 'error',
      },
      ok: result.ok,
    };
  }

  async function handleDuplicateCreatedAction(action) {
    const result = await duplicateCreatedAction(
      action.targetAction || action.action,
    );
    if (!result.ok)
      return { toast: { message: result.message, tone: 'error' } };
    return { toast: { message: result.message }, action: result.action };
  }

  async function handleRemoveCreatedAction(action) {
    const result = await removeCreatedAction(
      action.targetAction || action.action,
    );
    return {
      toast: {
        message: result.message,
        tone: result.ok ? 'default' : 'error',
      },
      ok: result.ok,
    };
  }

  async function handleSubmitExtensionPr(action) {
    const extensionPrSubmitter = getExtensionPrSubmitter();

    if (!extensionPrSubmitter) {
      return {
        toast: {
          message: 'Cannot submit extensions right now',
          tone: 'error',
        },
      };
    }
    const result = await extensionPrSubmitter.submitExtensionPr(action);
    return {
      toast: {
        message: result.prUrl ? `PR opened: ${result.prUrl}` : result.message,
        tone: result.ok ? 'default' : 'error',
      },
    };
  }

  async function handleClearActionOverride(action) {
    const result = await clearOverride(action.targetAction || action.action);
    return {
      toast: {
        message: result.message,
        tone: result.ok ? 'default' : 'error',
      },
      ok: result.ok,
    };
  }

  function handleRecordShortcut(action) {
    return {
      toast: { message: 'Shortcut recording is handled by the palette' },
    };
  }

  return {
    handleSetActionShortcut,
    handleRemoveShortcut,
    handleSetActionAlias,
    handleRemoveActionAlias,
    handleDuplicateCreatedAction,
    handleRemoveCreatedAction,
    handleSubmitExtensionPr,
    handleClearActionOverride,
    handleRecordShortcut,
  };
}
