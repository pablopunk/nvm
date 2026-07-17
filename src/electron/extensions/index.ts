import { createAccountExtension } from './account';
import { createAiBuilderExtension } from './ai-builder';
import { createAppsExtension } from './apps';
import { createBackgroundTasksExtension } from './background-tasks';
import { createCalculatorExtension } from './calculator';
import { createClipboardExtension } from './clipboard';
import { createExtensionsExtension } from './extensions';
import { createFilesExtension } from './files';
import { createKeyboardShortcutsExtension } from './keyboard-shortcuts';
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
  createAppsExtension,
  createFilesExtension,
  createExtensionsExtension,
  createAiBuilderExtension,
  createUpdatesExtension,
  createKeyboardShortcutsExtension,
  createSettingsExtension,
  createBackgroundTasksExtension,
  createAccountExtension,
];
