import { createSystemExtension, createPlacesExtension } from './system';
import { createCalculatorExtension } from './calculator';
import { createWebSearchExtension } from './web-search';
import { createClipboardExtension } from './clipboard';
import { createAppsExtension } from './apps';
import { createFilesExtension } from './files';
import { createAiBuilderExtension } from './ai-builder';
import { createUpdatesExtension } from './updates';
import { createKeyboardShortcutsExtension } from './keyboard-shortcuts';
import { createSettingsExtension } from './settings';
import { createBackgroundTasksExtension } from './background-tasks';
import { createAccountExtension } from './account';

export const INTERNAL_EXTENSION_FACTORIES: Array<() => any> = [
  createSystemExtension,
  createPlacesExtension,
  createCalculatorExtension,
  createWebSearchExtension,
  createClipboardExtension,
  createAppsExtension,
  createFilesExtension,
  createAiBuilderExtension,
  createUpdatesExtension,
  createKeyboardShortcutsExtension,
  createSettingsExtension,
  createBackgroundTasksExtension,
  createAccountExtension,
];
