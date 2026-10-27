// biome-ignore-all lint: Internal extension registry intentionally accepts heterogeneous factory shapes.
import { createAccountExtension } from './account';
import { createAiBuilderExtension } from './ai-builder';
import { createAiCommandsExtension } from './ai-commands';
import { createAppsExtension } from './apps';
import { createBackgroundTasksExtension } from './background-tasks';
import { createCalculatorExtension } from './calculator';
import { createClipboardExtension } from './clipboard';
import { createDiagnosticsExtension } from './diagnostics';
import { createDictationExtension } from './dictation';
import { createEmojiSymbolsExtension } from './emoji-symbols';
import { createExtensionsExtension } from './extensions';
import { createFilesExtension } from './files';
import { createFloatingNotesExtension } from './floating-notes';
import { createGettingStartedExtension } from './getting-started';
import { createKeyboardShortcutsExtension } from './keyboard-shortcuts';
import { createPermissionsExtension } from './permissions';
import { createQuitAppsExtension } from './quit-apps';
import { createSettingsExtension } from './settings';
import { createPlacesExtension, createSystemExtension } from './system';
import { createUpdatesExtension } from './updates';
import { createWebSearchExtension } from './web-search';
import { createWindowManagementExtension } from './window-management';

export const INTERNAL_EXTENSION_FACTORIES: Array<() => any> = [
  createSystemExtension,
  createPlacesExtension,
  createCalculatorExtension,
  createWebSearchExtension,
  createClipboardExtension,
  createDictationExtension,
  createAiCommandsExtension,
  createEmojiSymbolsExtension,
  createAppsExtension,
  createFilesExtension,
  createFloatingNotesExtension,
  createExtensionsExtension,
  createAiBuilderExtension,
  createPermissionsExtension,
  createUpdatesExtension,
  createKeyboardShortcutsExtension,
  createQuitAppsExtension,
  createWindowManagementExtension,
  createSettingsExtension,
  createBackgroundTasksExtension,
  createDiagnosticsExtension,
  createAccountExtension,
  createGettingStartedExtension,
];

export const INTERNAL_EXTENSION_SOURCE_FILES = [
  'account.ts',
  'ai-builder.ts',
  'ai-commands.ts',
  'apps.ts',
  'background-tasks.ts',
  'calculator.ts',
  'clipboard.ts',
  'dictation.ts',
  'diagnostics.ts',
  'emoji-symbols.ts',
  'extensions.ts',
  'files.ts',
  'floating-notes.ts',
  'getting-started.ts',
  'permissions.ts',
  'keyboard-shortcuts.ts',
  'quit-apps.ts',
  'settings.ts',
  'system.ts',
  'updates.ts',
  'web-search.ts',
  'window-management.ts',
] as const;
