import {
  measureDebugPerformance,
  performanceTraces,
} from '../../diagnostics/debug-performance';
import { error as logError, warn as logWarn } from '../../platform/logger';

export type ExtensionActionHandlerDeps = {
  resolveRegisteredActionRef: (action: any) => {
    error?: string;
    rootAction?: any;
  };
  executeExtensionRootItem: (action: any) => Promise<any>;
  executeViewActionResult: (
    result: any,
    entry: any,
    launchContext?: any,
  ) => Promise<any>;
  extensionModuleForAction: (action: any) => any;
  extensionModules: Map<string, any>;
  extensionActionRegistry: Map<string, any>;
  extensionActionHandlers: Map<string, any>;
  renameExtension: (extension: any, command: any, metadata: any) => unknown;
  invalidateExtensionRootItems: () => void;
  setDiagnosticExtensionTarget: (extension: any) => void;
  createExtensionContext: (
    extension: any,
    command?: any,
    launchContext?: any,
    options?: any,
  ) => any;
  extensionErrorView: (entry: any, error: unknown) => any;
};

export function createExtensionActionHandlers(
  deps: ExtensionActionHandlerDeps,
) {
  const {
    resolveRegisteredActionRef,
    executeExtensionRootItem,
    executeViewActionResult,
    extensionModuleForAction,
    extensionModules,
    extensionActionRegistry,
    extensionActionHandlers,
    renameExtension,
    invalidateExtensionRootItems,
    setDiagnosticExtensionTarget,
    createExtensionContext,
    extensionErrorView,
  } = deps;

  function handleRunExtensionRegisteredAction(action) {
    const resolved = resolveRegisteredActionRef(action);
    if (resolved.error)
      return { toast: { message: resolved.error, tone: 'error' } };
    return resolved.rootAction
      ? executeExtensionRootItem(resolved.rootAction)
      : { toast: { message: 'Action unavailable', tone: 'error' } };
  }

  function handleRenameExtensionPrompt(action) {
    const extAction = action.targetAction || action.action;
    const extension = extensionModuleForAction(extAction);
    if (!extension)
      return { toast: { message: 'Extension not found', tone: 'error' } };
    const itemTitle = extAction.title || extension.title || '';
    return {
      view: {
        type: 'prompt',
        title: `Rename "${itemTitle}"`,
        fields: [
          {
            id: 'title',
            type: 'text',
            label: 'Name',
            value: itemTitle,
            placeholder: 'Enter a new name',
            required: true,
          },
        ],
        submitAction: {
          type: 'renameExtension',
          title: 'Rename',
          extensionId: extension.id,
          commandId: extAction.commandId || '',
        },
      },
      navigation: 'push',
    };
  }

  async function handleRenameExtension(action) {
    const extension = Array.from(extensionModules.values()).find(
      (ext) => ext.id === action.extensionId,
    );
    if (!extension)
      return { toast: { message: 'Extension not found', tone: 'error' } };
    const newTitle = String(action.formValues?.title || '').trim();
    if (!newTitle)
      return { toast: { message: 'Name is required', tone: 'error' } };
    const commandId = action.commandId;
    const command = commandId
      ? extensionActionRegistry.get(`${extension.id}:${commandId}`)?.command ||
        null
      : null;
    const result = await renameExtension(extension, command, {
      title: newTitle,
      ...(command ? { commandTitle: newTitle } : {}),
    });
    invalidateExtensionRootItems();
    return {
      toast: { message: `Renamed to ${newTitle}` },
      navigation: 'pop',
    };
  }

  async function handleRunExtensionAction(action, launchContext) {
    const record = extensionActionHandlers.get(action.handlerId);
    if (!record)
      return {
        toast: { message: 'Action is no longer available', tone: 'error' },
      };
    if (!(record.entry && record.entry.extension)) {
      logWarn('extension.action.missingEntry', {
        handlerId: action.handlerId,
        actionTitle: action.title,
      });
      return {
        toast: {
          message: 'This action is not available in the current context.',
          tone: 'error',
        },
      };
    }
    try {
      setDiagnosticExtensionTarget(record.entry.extension);
      const result = await measureDebugPerformance(
        'extension.action.handler',
        {
          extensionId: record.entry.extension.id,
          commandId: record.entry.command?.id,
          actionTitle: action.title,
          alwaysLog: true,
        },
        () =>
          performanceTraces.run(
            'extension.action.handler',
            {
              extensionId: record.entry.extension.id,
              commandId: record.entry.command?.id,
            },
            () =>
              record.handler(
                createExtensionContext(
                  record.entry.extension,
                  record.entry.command || null,
                  launchContext,
                ),
                action,
              ),
            action.traceId ? { traceId: action.traceId } : undefined,
          ),
      );
      return executeViewActionResult(result, record.entry, launchContext);
    } catch (error) {
      logError('extension.action.failed', error, {
        source: 'host',
        scope: 'extension',
        extensionId: record.entry.extension?.id,
        commandId: record.entry.command?.id,
      });
      return {
        view: extensionErrorView(record.entry, error),
        navigation: 'push',
      };
    }
  }

  return {
    handleRunExtensionRegisteredAction,
    handleRenameExtensionPrompt,
    handleRenameExtension,
    handleRunExtensionAction,
  };
}
