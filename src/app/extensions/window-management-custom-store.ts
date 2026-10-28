import type { WindowDimension } from '../electron/os/window-management-types';
import type { ExtensionContext } from '../resources/nevermind-extension-api';
import {
  EXACT_DIMENSION_KEYS,
  type ExactDimensionKey,
  exactDimensionValueIsInRange,
  isExactDimensionUnit,
} from './window-management-exact-values';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isAbsentValue(value: unknown) {
  return value === undefined || value === null;
}

function normalizeStoredDimension(
  key: ExactDimensionKey,
  raw: unknown,
): WindowDimension | null {
  if (!isRecord(raw) || typeof raw.value !== 'number') {
    return null;
  }
  if (!isExactDimensionUnit(raw.unit)) {
    return null;
  }
  const dimension = { value: raw.value, unit: raw.unit };
  return exactDimensionValueIsInRange(key, dimension) ? dimension : null;
}

function normalizeStoredDimensions(
  entry: Record<string, unknown>,
): CustomWindowDimensions | null {
  const dimensions: CustomWindowDimensions = {};
  for (const key of EXACT_DIMENSION_KEYS) {
    if (isAbsentValue(entry[key])) {
      continue;
    }
    const dimension = normalizeStoredDimension(key, entry[key]);
    if (!dimension) {
      return null;
    }
    dimensions[key] = dimension;
  }
  return dimensions;
}

function normalizeStoredText(value: unknown, maxLength: number) {
  if (typeof value !== 'string') {
    return '';
  }
  const trimmed = value.trim();
  return trimmed.length > maxLength ? '' : trimmed;
}

const MAX_CUSTOM_WINDOW_COMMAND_ID_LENGTH = 64;

const BASE_36_RADIX = 36;

const RANDOM_ID_SUFFIX_LENGTH = 8;

function randomIdSuffix() {
  return Math.random()
    .toString(BASE_36_RADIX)
    .slice(2, 2 + RANDOM_ID_SUFFIX_LENGTH);
}

export const CUSTOM_WINDOW_COMMANDS_KEY = 'window-management.custom-commands';

export const MAX_CUSTOM_WINDOW_COMMANDS = 100;

export const MAX_CUSTOM_WINDOW_COMMAND_TITLE_LENGTH = 80;

export type CustomWindowDimensions = Partial<
  Record<ExactDimensionKey, WindowDimension>
>;

export interface CustomWindowCommand extends CustomWindowDimensions {
  id: string;
  title: string;
}

export interface CustomWindowCommandsStore {
  load(): Promise<CustomWindowCommand[]>;
  save(commands: CustomWindowCommand[]): Promise<void>;
}

export function normalizeCustomWindowCommand(
  entry: unknown,
): CustomWindowCommand | null {
  if (!isRecord(entry)) {
    return null;
  }
  const id = normalizeStoredText(entry.id, MAX_CUSTOM_WINDOW_COMMAND_ID_LENGTH);
  const title = normalizeStoredText(
    entry.title,
    MAX_CUSTOM_WINDOW_COMMAND_TITLE_LENGTH,
  );
  if (!(id && title)) {
    return null;
  }
  const dimensions = normalizeStoredDimensions(entry);
  if (!dimensions || Object.keys(dimensions).length === 0) {
    return null;
  }
  return { id, title, ...dimensions };
}

export function normalizeCustomWindowCommands(
  raw: unknown,
): CustomWindowCommand[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  const seenIds = new Set<string>();
  const commands: CustomWindowCommand[] = [];
  for (const entry of raw) {
    const command = normalizeCustomWindowCommand(entry);
    if (!command || seenIds.has(command.id)) {
      continue;
    }
    seenIds.add(command.id);
    commands.push(command);
    if (commands.length === MAX_CUSTOM_WINDOW_COMMANDS) {
      break;
    }
  }
  return commands;
}

export function createCustomWindowCommandId() {
  return `custom-${Date.now().toString(BASE_36_RADIX)}-${randomIdSuffix()}`;
}

export function customWindowCommandDimensions(
  command: CustomWindowCommand,
): CustomWindowDimensions {
  const dimensions: CustomWindowDimensions = {};
  for (const key of EXACT_DIMENSION_KEYS) {
    if (command[key]) {
      dimensions[key] = command[key];
    }
  }
  return dimensions;
}

export function extensionWindowCustomCommandsStore(
  ctx: Pick<ExtensionContext, 'storage'>,
): CustomWindowCommandsStore {
  return {
    async load() {
      return normalizeCustomWindowCommands(
        await ctx.storage.get(CUSTOM_WINDOW_COMMANDS_KEY),
      );
    },
    async save(commands) {
      await ctx.storage.set(
        CUSTOM_WINDOW_COMMANDS_KEY,
        normalizeCustomWindowCommands(commands),
      );
    },
  };
}
