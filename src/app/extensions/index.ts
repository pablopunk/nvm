// biome-ignore-all lint: Internal extension registry intentionally accepts heterogeneous factory shapes.
import { createAccountExtension } from './account';
import { createAiCommandsExtension } from './ai-commands';
import { createAiBuilderExtension } from './ai-builder';
import { createAppsExtension } from './apps';
import { createBackgroundTasksExtension } from './background-tasks';
import { createCalculatorExtension } from './calculator';
import { createClipboardExtension } from './clipboard';
import { createDictationExtension } from './dictation';
import { createEmojiSymbolsExtension } from './emoji-symbols';
import { createExtensionsExtension } from './extensions';
import { createFilesExtension } from './files';
import { createFloatingNotesExtension } from './floating-notes';
import { createKeyboardShortcutsExtension } from './keyboard-shortcuts';
import { createQuitAppsExtension } from './quit-apps';
import { createSettingsExtension } from './settings';
import { createPlacesExtension, createSystemExtension } from './system';
import { createUpdatesExtension } from './updates';
import { createWebSearchExtension } from './web-search';

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
  createUpdatesExtension,
  createKeyboardShortcutsExtension,
  createQuitAppsExtension,
  createSettingsExtension,
  createBackgroundTasksExtension,
  createAccountExtension,
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
  'emoji-symbols.ts',
  'extensions.ts',
  'files.ts',
  'floating-notes.ts',
  'keyboard-shortcuts.ts',
  'quit-apps.ts',
  'settings.ts',
  'system.ts',
  'updates.ts',
  'web-search.ts',
] as const;
