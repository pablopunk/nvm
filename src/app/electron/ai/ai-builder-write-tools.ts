import path from 'node:path';
import { formatBuilderValidationError } from './ai-builder-read-page';
import {
  observeBuilderTool,
  type BuilderToolOptions,
} from './ai-builder-tool-shared';

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
          const { filePath, filename, preview } =
            await options.stageGeneratedExtension(params, readFiles);
          options.markGeneratedExtension?.(filePath);
          options.emitEvent?.({
            type: 'extension_activated',
            data: { filename, preview },
          });
          return {
            content: [
              { type: 'text', text: `Installed and enabled ${filename}.` },
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
