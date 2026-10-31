import path from 'node:path';
import { formatBuilderValidationError } from './ai-builder-read-page';
import {
  observeBuilderTool,
  summarizeBuilderText,
  type BuilderToolOptions,
} from './ai-builder-tool-shared';
import type { PreviewAutoRun } from './ai-preview-auto-run';

const PREVIEW_FAILURE_MAX_CHARS = 2000;

function previewRunFailureMessage(filename: string, failureMessage: string) {
  return [
    `${filename} was installed and enabled, but running its first command in the preview failed, so it does NOT work yet.`,
    'Fix the extension and call write_extension again. Do not tell the user it works until a write_extension call succeeds.',
    `Error:\n${summarizeBuilderText(failureMessage, PREVIEW_FAILURE_MAX_CHARS)}`,
  ].join('\n\n');
}

function installedMessage(filename: string, autoRun?: PreviewAutoRun) {
  if (autoRun?.timedOut)
    return `Installed and enabled ${filename}. Its first command was still running when the preview check timed out, so it is not verified.`;
  return `Installed and enabled ${filename}.${autoRun ? ' Its first command ran in the preview without errors.' : ''}`;
}

type PiApi = typeof import('@earendil-works/pi-coding-agent');
type TypeApi = typeof import('@earendil-works/pi-ai').Type;

export function createBuilderWriteTools(
  pi: PiApi,
  Type: TypeApi,
  options: BuilderToolOptions,
  readFiles: ReadonlySet<string>,
) {
  return [
    pi.defineTool({
      name: 'write_extension',
      label: 'Write Extension',
      description:
        'Write a generated Nevermind TypeScript extension into the generated extensions directory.',
      parameters: Type.Object({
        filename: Type.String({
          description: 'Safe filename ending in .ts, for example image-grid.ts',
        }),
        code: Type.String({
          description:
            'Complete TypeScript extension source code using export default { ... } satisfies NevermindExtension',
        }),
      }),
      execute: observeBuilderTool(
        options.emitEvent,
        'write_extension',
        async (
          _toolCallId: string,
          params: { filename: string; code: string },
        ) => {
          const { filePath, filename, preview, autoRun } =
            await options.stageGeneratedExtension(params, readFiles);
          options.markGeneratedExtension?.(filePath);
          options.emitEvent?.({
            type: 'extension_activated',
            data: { filename, preview, autoRunResult: autoRun?.result },
          });
          if (autoRun?.failureMessage)
            throw new Error(
              previewRunFailureMessage(filename, autoRun.failureMessage),
            );
          return {
            content: [
              { type: 'text', text: installedMessage(filename, autoRun) },
            ],
            details: { filePath, filename },
          };
        },
      ),
    }),
    pi.defineTool({
      name: 'remove_extension',
      label: 'Remove Extension',
      description:
        'Remove a generated Nevermind TypeScript extension owned by this AI chat.',
      parameters: Type.Object({
        filename: Type.String({
          description: 'Safe generated extension filename ending in .ts',
        }),
      }),
      execute: observeBuilderTool(
        options.emitEvent,
        'remove_extension',
        async (_toolCallId: string, params: { filename: string }) => {
          const filePath = options.safeExtensionPath(
            options.extensionsDir,
            params.filename,
          );
          const filename = path.basename(filePath);
          if (!options.canWriteExtension?.(filename))
            throw new Error(
              `Refusing to remove ${filename}: this AI chat does not own that extension.`,
            );
          const result = await options.removeExtension?.(filename);
          if (!result?.removed)
            return {
              content: [
                { type: 'text', text: `${filename} is already removed.` },
              ],
              details: { filePath },
            };
          return {
            content: [
              {
                type: 'text',
                text: `Removed ${filename} from ${result.filePath || filePath}`,
              },
            ],
            details: { filePath: result.filePath || filePath },
          };
        },
      ),
    }),
    pi.defineTool({
      name: 'validate_extension',
      label: 'Validate Extension',
      description:
        'Typecheck and runtime-validate a generated TypeScript extension file.',
      parameters: Type.Object({ filename: Type.String() }),
      execute: observeBuilderTool(
        options.emitEvent,
        'validate_extension',
        async (_toolCallId: string, params: { filename: string }) => {
          const filePath = options.safeExtensionPath(
            options.extensionsDir,
            params.filename,
          );
          try {
            await options.validateGeneratedExtension(filePath);
          } catch (error) {
            throw new Error(
              formatBuilderValidationError(pi, path.basename(filePath), error),
            );
          }
          return {
            content: [
              { type: 'text', text: `Validated ${path.basename(filePath)}` },
            ],
            details: { filePath },
          };
        },
      ),
    }),
    pi.defineTool({
      name: 'install_extension',
      label: 'Install Extension',
      description:
        'No-op compatibility tool. write_extension installs and enables generated extensions immediately.',
      parameters: Type.Object({ filename: Type.String() }),
      execute: observeBuilderTool(
        options.emitEvent,
        'install_extension',
        async (_toolCallId: string, params: { filename: string }) => {
          const filePath = options.safeExtensionPath(
            options.extensionsDir,
            params.filename,
          );
          return {
            content: [
              {
                type: 'text',
                text: `${path.basename(filePath)} is already installed by write_extension.`,
              },
            ],
            details: { filePath, extensionsDir: options.extensionsDir },
          };
        },
      ),
    }),
  ];
}
