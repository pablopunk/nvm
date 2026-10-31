import type { LogLevel, LogSource } from '../platform/logger';
import type { BuilderReadPage } from './ai-builder-read-page';
import type { PreviewRunReader } from './ai-preview-run-observer';

export type GeneratedExtensionActivation = {
  filename: string;
  preview: {
    extensionId: string;
    rootItems: unknown[];
    commands?: unknown[];
    actions: unknown[];
  };
};

export type BuilderAiEvent = {
  type: string;
  name?: string;
  label?: string;
  data?: unknown;
  isError?: boolean;
};

export type BuilderActiveChat = {
  contextExtensionFile?: string;
  touchedExtensionFiles?: string[];
  generatedExtensionFile?: string;
};

export type BuilderToolOptions = {
  extensionsDir: string;
  internalExtensionsDir?: string;
  internalExtensionFiles?: readonly string[];
  extensionApiPath: string;
  getShortcuts?: () => Array<{
    actionId: string;
    title: string;
    subtitle?: string;
    accelerator: string;
    scope: 'global';
    source: 'user' | 'extension';
  }>;
  getPaletteShortcut?: () => {
    title: string;
    accelerator: string;
    scope: 'palette';
  };
  getExtensionRuntimeState?: (filename: string) => {
    loaded: boolean;
    extensionId?: string;
    commandIds: string[];
  };
  getActiveChat?: () => BuilderActiveChat | null;
  getCapabilities: () => unknown;
  markGeneratedExtension?: (filePath: string) => void;
  readPreviewRun?: PreviewRunReader;
  stageGeneratedExtension: (
    params: { filename: string; code: string },
    readFiles: ReadonlySet<string>,
  ) => Promise<{
    filePath: string;
    filename: string;
    preview: GeneratedExtensionActivation['preview'];
  }>;
  validateGeneratedExtension: (filePath: string) => Promise<void>;
  safeExtensionPath: (root: string, filename: string) => string;
  isExtensionSourceFile: (filename: string) => boolean;
  recordRead: (filename: string, page: BuilderReadPage) => void;
  canWriteExtension?: (filename: string) => boolean;
  removeExtension?: (
    filename: string,
  ) =>
    | Promise<{ removed: boolean; filePath?: string }>
    | { removed: boolean; filePath?: string };
  emitEvent?: (event: BuilderAiEvent) => void;
};

export function currentBuilderExtensionFile(chat?: BuilderActiveChat | null) {
  return (
    chat?.contextExtensionFile ||
    chat?.generatedExtensionFile ||
    chat?.touchedExtensionFiles?.[0]
  );
}

export function builderRuntimeDetails(
  getRuntimeState: BuilderToolOptions['getExtensionRuntimeState'],
  filename: string,
) {
  const runtime = getRuntimeState?.(filename);
  return {
    extensionId: runtime?.extensionId,
    commandIds: runtime?.commandIds || [],
    loaded: runtime?.loaded ?? false,
  };
}

export function observeBuilderTool<TParams, TResult>(
  emitEvent: BuilderToolOptions['emitEvent'],
  name: string,
  execute: (toolCallId: string, params: TParams) => Promise<TResult>,
) {
  return async (toolCallId: string, params: TParams) => {
    emitEvent?.({
      type: 'tool_trace_start',
      name,
      data: { toolCallId, input: summarizedToolInput(params) },
    });
    try {
      const result = await execute(toolCallId, params);
      emitEvent?.({
        type: 'tool_trace_end',
        name,
        data: { toolCallId, output: summarizedToolResult(result) },
        isError: false,
      });
      return result;
    } catch (error) {
      emitEvent?.({
        type: 'tool_trace_end',
        name,
        data: {
          toolCallId,
          error: summarizeBuilderText(
            error instanceof Error ? error.message : String(error),
            500,
          ),
        },
        isError: true,
      });
      throw error;
    }
  };
}

export function summarizedToolInput(params: unknown) {
  if (params == null || typeof params !== 'object') return params;
  const value = { ...(params as Record<string, unknown>) };
  for (const [key, field] of Object.entries(value)) {
    if (typeof field === 'string')
      value[key] = summarizeBuilderText(field, 500);
  }
  return value;
}

export function summarizedToolResult(result: unknown) {
  if (!result || typeof result !== 'object') return result;
  const resultRecord = result as {
    content?: Array<{ text?: string }>;
    details?: unknown;
  };
  const text = Array.isArray(resultRecord.content)
    ? resultRecord.content
        .map((item) => item?.text)
        .filter((item): item is string => Boolean(item))
        .join('\n\n')
    : '';
  let details: string | undefined;
  if (resultRecord.details && typeof resultRecord.details === 'object') {
    try {
      details = summarizeBuilderText(JSON.stringify(resultRecord.details), 500);
    } catch {
      details = '[Tool details could not be serialized]';
    }
  }
  return {
    ...(text
      ? {
          text: summarizeBuilderText(text, 1000),
          textBytes: Buffer.byteLength(text, 'utf8'),
        }
      : {}),
    ...(details ? { details } : {}),
  };
}

export function summarizeBuilderText(text: string, limit: number) {
  return String(text || '').slice(0, limit);
}

export function normalizeBuilderLogLevel(level?: string): LogLevel | undefined {
  return level === 'debug' ||
    level === 'info' ||
    level === 'warn' ||
    level === 'error'
    ? level
    : undefined;
}

export function normalizeBuilderLogSource(
  source?: string,
): LogSource | undefined {
  return source === 'main' ||
    source === 'renderer' ||
    source === 'extension' ||
    source === 'host'
    ? source
    : undefined;
}
