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

function extensionCommandActionId(extension, command) {
  return `extension:${extension.id}:${command.id}`;
}

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
  } = deps;

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

  function registerExtension(extension) {
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
            for (const command of extension.commands || []) {
              if (
                !(command?.id && command.title) ||
                typeof command.run !== 'function'
              )
                continue;
              const entry = { extension, command, source: 'command' };
              const action = {
                type: 'runExtensionAction',
                title: command.title,
                __handler: async (ctx, actionArg) =>
                  command.run(ctx, actionArg),
              };
              const item = {
                id: command.id,
                actionId:
                  command.actionId ||
                  extensionCommandActionId(extension, command),
                title: command.title,
                subtitle:
                  command.subtitle || extension.title || 'Extension command',
                aliases: command.aliases || [],
                icon: command.icon || 'sparkles',
                score: command.score || 12,
                shortcut:
                  command.globalShortcut ||
                  (command.shortcutScope === 'global'
                    ? command.shortcut
                    : undefined),
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
            if (typeof extension.actions === 'function') {
              try {
                const result = extension.actions(
                  createExtensionContext(extension, null),
                );
                const items = Array.isArray(result)
                  ? result
                  : Array.isArray(result?.actions)
                    ? result.actions
                    : [];
                const entry = {
                  extension,
                  command: {
                    id: 'actions',
                    title: extension.title || extension.id,
                  },
                  source: 'action',
                };
                for (const item of items) {
                  if (!(item?.id && item.title)) continue;
                  const action = item.run
                    ? {
                        type: 'runExtensionAction',
                        title: item.title,
                        __handler: item.run,
                      }
                    : item.action;
                  const normalizedItem = normalizeViewItems(
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
                  extensionActionRegistry.set(`${extension.id}:${item.id}`, {
                    ...entry,
                    item: normalizedItem,
                  });
                  registerExtensionBackgroundJob(entry, normalizedItem);
                }
              } catch (error) {
                logError('extension.actions.failed', error, {
                  source: 'host',
                  scope: 'extension',
                  extensionId: extension.id,
                });
              }
            }
          },
        ),
    );
  }

  function registerInternalExtensions() {
    for (const createExtension of INTERNAL_EXTENSION_FACTORIES)
      registerExtension(markInternalExtension(createExtension()));
    assertInternalExtensionsRegistered();
  }

  return { registerExtension, registerInternalExtensions };
}
