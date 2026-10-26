import fs from 'node:fs/promises';
import path from 'node:path';
import {
  boundedBuilderReadPage,
  readPageDetails,
  type BuilderReadPageOptions,
} from './ai-builder-read-page';
import {
  builderRuntimeDetails,
  currentBuilderExtensionFile,
  normalizeBuilderLogLevel,
  normalizeBuilderLogSource,
  observeBuilderTool,
  type BuilderToolOptions,
} from './ai-builder-tool-shared';
import { readRecentLogs } from '../platform/logger';

type PiApi = typeof import('@earendil-works/pi-coding-agent');
type TypeApi = typeof import('@earendil-works/pi-ai').Type;

export function createBuilderReadTools(
  pi: PiApi,
  Type: TypeApi,
  options: BuilderToolOptions,
) {
  function readPageParameters() {
    return {
      offset: Type.Optional(
        Type.Number({
          description: 'Zero-based line offset. Defaults to the first line.',
        }),
      ),
      limit: Type.Optional(
        Type.Number({
          description: `Maximum lines to return, capped at ${pi.DEFAULT_MAX_LINES}.`,
        }),
      ),
      byteOffset: Type.Optional(
        Type.Number({
          description:
            'For a single oversized line only, continue from the returned UTF-8 byte offset.',
        }),
      ),
    };
  }

  return [
    pi.defineTool({
      name: 'read_extension_api',
      label: 'Read Extension API',
      description:
        'Read a bounded page of the typed Nevermind extension API reference. Repeat the call with the returned offset; use byteOffset only when a single line is oversized.',
      parameters: Type.Object(readPageParameters()),
      execute: observeBuilderTool(
        options.emitEvent,
        'read_extension_api',
        async (_toolCallId: string, params: BuilderReadPageOptions) => {
          const apiText = await fs.readFile(options.extensionApiPath, 'utf8');
          const page = boundedBuilderReadPage(pi, apiText, params);
          return {
            content: [{ type: 'text', text: page.text }],
            details: readPageDetails(page),
          };
        },
      ),
    }),
    pi.defineTool({
      name: 'list_capabilities',
      label: 'List Capabilities',
      description:
        'List a bounded page of UI views and OS capabilities available to generated Nevermind extensions. Repeat with the returned offset to continue.',
      parameters: Type.Object(readPageParameters()),
      execute: observeBuilderTool(
        options.emitEvent,
        'list_capabilities',
        async (_toolCallId: string, params: BuilderReadPageOptions) => {
          const page = boundedBuilderReadPage(
            pi,
            JSON.stringify(options.getCapabilities(), null, 2),
            params,
          );
          return {
            content: [{ type: 'text', text: page.text }],
            details: readPageDetails(page),
          };
        },
      ),
    }),
    pi.defineTool({
      name: 'list_shortcuts',
      label: 'List Shortcuts',
      description:
        'Read a bounded page of current Nevermind keyboard shortcuts, including the app shortcut used to open Nevermind and active global action shortcuts. Repeat with the returned offset to continue.',
      parameters: Type.Object(readPageParameters()),
      execute: observeBuilderTool(
        options.emitEvent,
        'list_shortcuts',
        async (_toolCallId: string, params: BuilderReadPageOptions) => {
          const palette = options.getPaletteShortcut?.();
          const shortcuts = options.getShortcuts?.() || [];
          const page = boundedBuilderReadPage(
            pi,
            JSON.stringify({ palette, shortcuts }, null, 2),
            params,
          );
          return {
            content: [{ type: 'text', text: page.text }],
            details: { count: shortcuts.length, ...readPageDetails(page) },
          };
        },
      ),
    }),
    pi.defineTool({
      name: 'read_app_logs',
      label: 'Read App Logs',
      description:
        'Read recent structured Nevermind app logs to debug host, renderer, extension, or API failures. Use this when an extension fails, an API helper behaves unexpectedly, or the user asks about app diagnostics.',
      parameters: Type.Object({
        limit: Type.Optional(
          Type.Number({
            description:
              'Maximum number of entries to return. Defaults to 200 and is capped by the host.',
          }),
        ),
        level: Type.Optional(
          Type.String({
            description: 'Optional level filter: debug, info, warn, or error.',
          }),
        ),
        source: Type.Optional(
          Type.String({
            description:
              'Optional source filter: main, renderer, extension, or host.',
          }),
        ),
        sinceMs: Type.Optional(
          Type.Number({
            description: 'Only return entries from the last N milliseconds.',
          }),
        ),
        beforeTimestamp: Type.Optional(
          Type.String({
            description: 'Continue with the returned older-page timestamp.',
          }),
        ),
        beforeCount: Type.Optional(
          Type.Number({
            description:
              'Pass the returned count with beforeTimestamp to continue through entries sharing the same timestamp.',
          }),
        ),
        query: Type.Optional(
          Type.String({
            description:
              'Case-insensitive text filter across the structured log entry.',
          }),
        ),
        extensionId: Type.Optional(
          Type.String({ description: 'Optional extension id filter.' }),
        ),
      }),
      execute: observeBuilderTool(
        options.emitEvent,
        'read_app_logs',
        async (
          _toolCallId: string,
          params: {
            limit?: number;
            level?: string;
            source?: string;
            sinceMs?: number;
            beforeTimestamp?: string;
            beforeCount?: number;
            query?: string;
            extensionId?: string;
          },
        ) => {
          const logs = await readRecentLogs({
            limit: params.limit,
            level: normalizeBuilderLogLevel(params.level),
            source: normalizeBuilderLogSource(params.source),
            sinceMs: params.sinceMs,
            beforeTimestamp: params.beforeTimestamp,
            beforeCount: params.beforeCount,
            query: params.query,
            extensionId: params.extensionId,
          });
          let startIndex = 0;
          let logText = logs.map((entry) => JSON.stringify(entry)).join('\n');
          const maxBytes = Math.max(pi.DEFAULT_MAX_BYTES - 512, 1);
          while (
            startIndex < logs.length - 1 &&
            Buffer.byteLength(logText, 'utf8') > maxBytes
          ) {
            startIndex += 1;
            logText = logs
              .slice(startIndex)
              .map((entry) => JSON.stringify(entry))
              .join('\n');
          }
          const boundedLogs = pi.truncateTail(logText, {
            maxBytes,
            maxLines: pi.DEFAULT_MAX_LINES,
          });
          const selectedLogs = logs.slice(startIndex);
          const hasOlderPage =
            startIndex > 0 ||
            logs.length >=
              Math.min(Math.max(Number(params.limit) || 200, 1), 1000);
          const nextBeforeTimestamp = hasOlderPage
            ? selectedLogs[0]?.timestamp
            : undefined;
          const nextBeforeCount = nextBeforeTimestamp
            ? selectedLogs.filter(
                (entry) => entry.timestamp === nextBeforeTimestamp,
              ).length
            : undefined;
          const notice = boundedLogs.truncated
            ? 'A matching log entry exceeds the output limit; only its newest tail is shown. '
            : startIndex > 0
              ? 'Older matching log entries were left out to fit the output limit. '
              : '';
          const continuation = nextBeforeTimestamp
            ? `Continue with beforeTimestamp=${nextBeforeTimestamp} and beforeCount=${nextBeforeCount}.`
            : 'No older matching entries were returned.';
          const output = logs.length
            ? `${notice}${continuation}\n${boundedLogs.content}`
            : 'No matching app logs.';
          return {
            content: [{ type: 'text', text: output }],
            details: {
              count: selectedLogs.length,
              truncated: boundedLogs.truncated || startIndex > 0,
              nextBeforeTimestamp,
              nextBeforeCount,
            },
          };
        },
      ),
    }),
    pi.defineTool({
      name: 'list_extensions',
      label: 'List Extensions',
      description:
        'List a bounded page of generated and built-in extension files available to inspect or reference. Repeat with the returned offset to continue.',
      parameters: Type.Object(readPageParameters()),
      execute: observeBuilderTool(
        options.emitEvent,
        'list_extensions',
        async (_toolCallId: string, params: BuilderReadPageOptions) => {
          const entries = await fs
            .readdir(options.extensionsDir, { withFileTypes: true })
            .catch(() => []);
          const files = entries
            .filter((entry) => options.isExtensionSourceFile(entry.name))
            .map((entry) => entry.name)
            .sort();
          const listed = files.map((filename) => ({
            filename,
            source: 'generated' as const,
            ...builderRuntimeDetails(
              options.getExtensionRuntimeState,
              filename,
            ),
          }));
          const builtins = (options.internalExtensionFiles || []).map(
            (filename) => ({
              filename: `builtin/${filename}`,
              source: 'builtin' as const,
            }),
          );
          const lines = [...listed, ...builtins].map((item) => {
            if (item.source === 'builtin')
              return `${item.filename} source=builtin`;
            const commands = item.commandIds.length
              ? ` commands=${item.commandIds.join(', ')}`
              : '';
            const extensionId = item.extensionId
              ? ` extensionId=${item.extensionId}`
              : '';
            return `${item.filename} loaded=${item.loaded}${extensionId}${commands}`;
          });
          const page = boundedBuilderReadPage(
            pi,
            lines.length ? lines.join('\n') : 'No extensions available.',
            params,
          );
          return {
            content: [{ type: 'text', text: page.text }],
            details: { count: lines.length, ...readPageDetails(page) },
          };
        },
      ),
    }),
    pi.defineTool({
      name: 'read_extension',
      label: 'Read Extension',
      description:
        'Read a bounded page of generated or built-in Nevermind TypeScript extension source by its list_extensions filename. Repeat with the returned offset; use byteOffset to continue one oversized line. A generated file counts as read for overwrite protection only after every page has been read. Built-in sources are read-only examples and can use privileged host APIs that generated extensions cannot access.',
      parameters: Type.Object({
        filename: Type.String({
          description:
            'Extension filename ending in .ts, including the builtin/ prefix shown by list_extensions when applicable',
        }),
        ...readPageParameters(),
      }),
      execute: observeBuilderTool(
        options.emitEvent,
        'read_extension',
        async (
          _toolCallId: string,
          params: { filename: string } & BuilderReadPageOptions,
        ) => {
          if (params.filename.startsWith('builtin/')) {
            const filename = params.filename.slice('builtin/'.length);
            if (
              !options.internalExtensionsDir ||
              !options.internalExtensionFiles?.includes(filename)
            )
              throw new Error('Unknown built-in extension');
            const filePath = options.safeExtensionPath(
              options.internalExtensionsDir,
              filename,
            );
            const code = await fs.readFile(filePath, 'utf8');
            const page = boundedBuilderReadPage(pi, code, params);
            return {
              content: [{ type: 'text', text: page.text }],
              details: {
                filename: params.filename,
                source: 'builtin',
                ...readPageDetails(page),
              },
            };
          }
          const filePath = options.safeExtensionPath(
            options.extensionsDir,
            params.filename,
          );
          const code = await fs.readFile(filePath, 'utf8');
          const page = boundedBuilderReadPage(pi, code, params);
          options.recordRead(filePath, page);
          return {
            content: [{ type: 'text', text: page.text }],
            details: { filePath, ...readPageDetails(page) },
          };
        },
      ),
    }),
    pi.defineTool({
      name: 'read_current_extension',
      label: 'Read Current Extension',
      description:
        'Read a bounded page of the active generated extension source for this chat/action before tweaking it. Repeat with the returned offset; use byteOffset to continue one oversized line. Every page must be read before overwriting.',
      parameters: Type.Object(readPageParameters()),
      execute: observeBuilderTool(
        options.emitEvent,
        'read_current_extension',
        async (_toolCallId: string, params: BuilderReadPageOptions) => {
          const filename = currentBuilderExtensionFile(
            options.getActiveChat?.(),
          );
          if (!filename)
            return {
              content: [
                {
                  type: 'text',
                  text: 'No focused generated extension for this chat.',
                },
              ],
              details: {},
            };
          const filePath = options.safeExtensionPath(
            options.extensionsDir,
            filename,
          );
          const code = await fs.readFile(filePath, 'utf8');
          const page = boundedBuilderReadPage(pi, code, params);
          options.recordRead(filePath, page);
          return {
            content: [{ type: 'text', text: page.text }],
            details: { filePath, ...readPageDetails(page) },
          };
        },
      ),
    }),
  ];
}
