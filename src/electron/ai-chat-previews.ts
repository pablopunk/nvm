import path from 'node:path';

interface AiChatPreviewState {
  builderPreviewFiles?: unknown;
  selectedBuilderPreviewFilename?: unknown;
  touchedExtensionFiles?: unknown;
  generatedExtensionFile?: unknown;
  contextExtensionFile?: unknown;
}

function extensionSourceFilename(value: unknown) {
  if (typeof value !== 'string') {
    return null;
  }
  const filename = path.basename(value);
  return filename.endsWith('.ts') && !filename.endsWith('.d.ts')
    ? filename
    : null;
}

function uniqueExtensionSourceFilenames(values: unknown[]) {
  return Array.from(
    new Set(
      values
        .map(extensionSourceFilename)
        .filter((filename): filename is string => Boolean(filename)),
    ),
  );
}

function withoutExecutionHandles(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(withoutExecutionHandles);
  }
  if (!value || typeof value !== 'object') {
    return value;
  }
  const { executionId: _executionId, ...rest } = value as Record<
    string,
    unknown
  >;
  return Object.fromEntries(
    Object.entries(rest).map(([key, item]) => [
      key,
      withoutExecutionHandles(item),
    ]),
  );
}

export function prepareAiChatPreview<
  T extends {
    preview: {
      rootItems: unknown[];
      commands: unknown[];
      actions: unknown[];
    };
  },
>(preview: T, prepareAction: (action: unknown) => unknown): T {
  return {
    ...preview,
    preview: {
      ...preview.preview,
      rootItems: preview.preview.rootItems
        .map(withoutExecutionHandles)
        .map(prepareAction)
        .filter(Boolean),
      commands: preview.preview.commands
        .map(withoutExecutionHandles)
        .map(prepareAction)
        .filter(Boolean),
      actions: preview.preview.actions
        .map(withoutExecutionHandles)
        .map(prepareAction)
        .filter(Boolean),
    },
  };
}

export function aiChatPreviewFiles(
  chat: AiChatPreviewState,
  existingFiles: Iterable<string>,
) {
  const existing = new Set(uniqueExtensionSourceFilenames([...existingFiles]));
  const ownedFiles = uniqueExtensionSourceFilenames([
    ...(Array.isArray(chat.touchedExtensionFiles)
      ? chat.touchedExtensionFiles
      : []),
    chat.generatedExtensionFile,
    chat.contextExtensionFile,
  ]).filter((filename) => existing.has(filename));
  const savedPreviewFiles = uniqueExtensionSourceFilenames(
    Array.isArray(chat.builderPreviewFiles) ? chat.builderPreviewFiles : [],
  ).filter((filename) => ownedFiles.includes(filename));
  const fallback = savedPreviewFiles.length === 0;
  const files = fallback ? ownedFiles : savedPreviewFiles;
  const contextFile = extensionSourceFilename(chat.contextExtensionFile);
  const selectedFile = extensionSourceFilename(
    chat.selectedBuilderPreviewFilename,
  );
  const preferredFile = fallback ? contextFile : selectedFile;
  return {
    files,
    selectedBuilderPreviewFilename:
      preferredFile && files.includes(preferredFile)
        ? preferredFile
        : files.at(-1),
  };
}
