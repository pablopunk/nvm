// biome-ignore-all lint: Host-side extension registration handles dynamic extension shapes by design.
import crypto from 'node:crypto';
import path from 'node:path';
import {
  measureDebugPerformanceSync,
  performanceTraces,
} from '../diagnostics/debug-performance';
import { hasCapability } from '../os/os';
import { error as logError } from '../platform/logger';
import { extensionCommandAction } from './extension-command-action';
import type { ExtensionRuntimeRegistries } from './extension-runtime-state';
import { markInternalExtension } from './internal-extension';
import { missingRequiredInternalCommands } from './internal-extension-requirements';

const EXTENSION_ACTIONS_TIMEOUT_MS = 10_000;

function extensionCommandActionId(extension, command) {
  return `extension:${extension.id}:${command.id}`;
}

function declaredActionItemsFromResult(result) {
  if (Array.isArray(result)) return result;
  return Array.isArray(result?.actions) ? result.actions : [];
}

function isRegistrableDeclaredAction(item) {
  return Boolean(item?.id && item.title);
}

function withActionsTimeout<T>(pending: Promise<T>, extensionId: string) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timer = setTimeout(
      () => reject(new Error(`actions(ctx) for ${extensionId} timed out`)),
      EXTENSION_ACTIONS_TIMEOUT_MS,
    );
  });
  return Promise.race([pending, timeout]).finally(() => clearTimeout(timer));
}

function declaredActionEntry(extension) {
  return {
    extension,
    command: { id: 'actions', title: extension.title || extension.id },
    source: 'action',
  };
}

function isDeclaredActionEntryOf(registered, extension) {
  return (
    registered?.source === 'action' && registered.extension?.id === extension.id
  );
}

export type ExtensionContributionRefreshResult = {
  ok: boolean;
  error?: string;
};

export type ExtensionRegistration = ReturnType<
  typeof createExtensionRegistration
>;

export type ExtensionRegistrationDeps = Pick<
  ExtensionRuntimeRegistries,
  'extensionModules' | 'extensionActionRegistry'
> & {
  INTERNAL_EXTENSION_FACTORIES: Array<() => any>;
  REQUIRED_INTERNAL_EXTENSIONS: string[];
  REQUIRED_INTERNAL_COMMANDS: Parameters<
    typeof missingRequiredInternalCommands
  >[0];
  normalizeItemAppearance: (appearance: any) => any;
  normalizeViewItems: (items: any[], entry: any) => any[];
  createExtensionContext: (extension: any, command: any) => any;
  registerExtensionBackgroundJob: (entry: any, item: any) => void;
  unregisterExtensionBackgroundJob: (extension: any, item: any) => void;
  registerActionShortcuts: () => void;
  invalidateExtensionRootItems: () => void;
};

export function createExtensionRegistration(deps: ExtensionRegistrationDeps) {
  const {
    extensionModules,
    extensionActionRegistry,
    INTERNAL_EXTENSION_FACTORIES,
    REQUIRED_INTERNAL_EXTENSIONS,
    REQUIRED_INTERNAL_COMMANDS,
    normalizeItemAppearance,
    normalizeViewItems,
    createExtensionContext,
    registerExtensionBackgroundJob,
    unregisterExtensionBackgroundJob,
    registerActionShortcuts,
    invalidateExtensionRootItems,
  } = deps;
  const refreshGenerations = new Map<string, number>();

  function assertInternalExtensionsRegistered() {
    const missingExtensions = REQUIRED_INTERNAL_EXTENSIONS.filter(
      (extensionId) => !extensionModules.has(extensionId),
    );
    const missingCommands = missingRequiredInternalCommands(
      REQUIRED_INTERNAL_COMMANDS,
      extensionActionRegistry,
      hasCapability,
    );
    if (missingExtensions.length || missingCommands.length) {
      const details = [
        missingExtensions.length
          ? `extensions: ${missingExtensions.join(', ')}`
          : '',
        missingCommands.length
          ? `commands: ${missingCommands.map(({ extensionId, commandId }) => `${extensionId}:${commandId}`).join(', ')}`
          : '',
      ]
        .filter(Boolean)
        .join('; ');
      throw new Error(`Missing required internal extensions (${details})`);
    }
  }

  // Resolves `actions(ctx)` without touching live registries, so callers can
  // await it before any synchronous registry swap. Returns null on failure.
  async function declareExtensionActions(extension) {
    if (typeof extension.actions !== 'function') return [];
    try {
      const pending = Promise.resolve(
        extension.actions(createExtensionContext(extension, null)),
      );
      const result = await withActionsTimeout(pending, extension.id);
      return declaredActionItemsFromResult(result).filter(
        isRegistrableDeclaredAction,
      );
    } catch (error) {
      logError('extension.actions.failed', error, {
        source: 'host',
        scope: 'extension',
        extensionId: extension.id,
      });
      return null;
    }
  }

  function normalizeDeclaredActionItem(item, entry) {
    const action = item.run
      ? {
          type: 'runExtensionAction',
          title: item.title,
          __handler: item.run,
        }
      : item.action;
    return normalizeViewItems(
      [
        {
          ...item,
          background:
            item.background ||
            item.mode === 'background' ||
            item.mode === 'noView',
          primaryAction: action,
        },
      ],
      entry,
    )[0];
  }

  function registerDeclaredActionItem(entry, item) {
    const normalizedItem = normalizeDeclaredActionItem(item, entry);
    extensionActionRegistry.set(`${entry.extension.id}:${item.id}`, {
      ...entry,
      item: normalizedItem,
    });
    registerExtensionBackgroundJob(entry, normalizedItem);
  }

  function registerExtensionCommands(extension) {
    for (const command of extension.commands || []) {
      if (!(command?.id && command.title) || typeof command.run !== 'function')
        continue;
      const entry = { extension, command, source: 'command' };
      const action = {
        type: 'runExtensionAction',
        title: command.title,
        __handler: async (ctx, actionArg) => command.run(ctx, actionArg),
      };
      const item = {
        id: command.id,
        actionId:
          command.actionId || extensionCommandActionId(extension, command),
        title: command.title,
        subtitle: command.subtitle || extension.title || 'Extension command',
        aliases: command.aliases || [],
        icon: command.icon || 'sparkles',
        score: command.score || 12,
        shortcut:
          command.globalShortcut ||
          (command.shortcutScope === 'global' ? command.shortcut : undefined),
        shortcutScope:
          command.globalShortcut || command.shortcutScope === 'global'
            ? ('global' as const)
            : undefined,
        globalShortcut: command.globalShortcut,
        dismissAfterRun: command.dismissAfterRun,
        appearance: normalizeItemAppearance(command.appearance),
        background:
          command.background ||
          command.mode === 'background' ||
          command.mode === 'noView',
        mode: command.mode,
        triggers: command.triggers,
        primaryAction: extensionCommandAction(command, action),
      };
      const normalizedItem = normalizeViewItems([item], entry)[0];
      extensionActionRegistry.set(`${extension.id}:${command.id}`, {
        ...entry,
        item: normalizedItem,
      });
      registerExtensionBackgroundJob(entry, normalizedItem);
    }
  }

  // Synchronous commit: callers resolve `declaredItems` with
  // `declareExtensionActions` first, so no await happens while registries are staged.
  function registerDeclaredExtension(extension, declaredItems = []) {
    performanceTraces.run(
      'extension.register',
      { extensionId: extension?.id },
      () =>
        measureDebugPerformanceSync(
          'extension.register',
          {
            extensionId: extension?.id,
            commandCount: extension?.commands?.length || 0,
          },
          () => {
            if (!extension?.id) return;
            const existing = extensionModules.get(extension.id);
            if (existing) {
              const existingOwner = path.basename(
                existing.__filePath || `internal:${existing.id}`,
              );
              const incomingOwner = path.basename(
                extension.__filePath || `internal:${extension.id}`,
              );
              throw new Error(
                `Duplicate extension id "${extension.id}": ${existingOwner} is already registered; cannot also register ${incomingOwner}`,
              );
            }
            extension.__executionVersion ||= crypto.randomUUID();
            extensionModules.set(extension.id, extension);
            registerExtensionCommands(extension);
            const entry = declaredActionEntry(extension);
            for (const item of declaredItems)
              registerDeclaredActionItem(entry, item);
          },
        ),
    );
  }

  async function registerExtension(extension) {
    if (!extension?.id) return;
    const declaredItems = (await declareExtensionActions(extension)) ?? [];
    registerDeclaredExtension(extension, declaredItems);
  }

  async function registerInternalExtensions() {
    for (const createExtension of INTERNAL_EXTENSION_FACTORIES)
      await registerExtension(markInternalExtension(createExtension()));
    assertInternalExtensionsRegistered();
  }

  // Replaces this extension's durable `actions(ctx)` entries in place: unchanged
  // ids keep their registry key and persisted actionId, removed ids are dropped.
  function applyDeclaredActionItems(extension, declaredItems) {
    const nextIds = new Set(declaredItems.map((item) => item.id));
    for (const [key, registered] of Array.from(extensionActionRegistry)) {
      if (
        !isDeclaredActionEntryOf(registered, extension) ||
        nextIds.has(registered.item?.id)
      )
        continue;
      extensionActionRegistry.delete(key);
      unregisterExtensionBackgroundJob(extension, registered.item);
    }
    const entry = declaredActionEntry(extension);
    for (const item of declaredItems) {
      unregisterExtensionBackgroundJob(extension, item);
      registerDeclaredActionItem(entry, item);
    }
  }

  // Re-runs the calling extension's `actions(ctx)`. A failed or superseded run
  // leaves the previously registered set untouched.
  async function refreshExtensionContributions(
    extension,
  ): Promise<ExtensionContributionRefreshResult> {
    const generation = (refreshGenerations.get(extension.id) || 0) + 1;
    refreshGenerations.set(extension.id, generation);
    const declaredItems = await declareExtensionActions(extension);
    if (declaredItems === null)
      return {
        ok: false,
        error: `actions(ctx) failed; previous ${extension.id} actions are still active`,
      };
    const superseded =
      refreshGenerations.get(extension.id) !== generation ||
      extensionModules.get(extension.id) !== extension;
    if (superseded)
      return {
        ok: false,
        error: 'A newer reload or refresh replaced this extension',
      };
    applyDeclaredActionItems(extension, declaredItems);
    registerActionShortcuts();
    invalidateExtensionRootItems();
    return { ok: true };
  }

  return {
    registerExtension,
    registerDeclaredExtension,
    declareExtensionActions,
    registerInternalExtensions,
    refreshExtensionContributions,
  };
}
