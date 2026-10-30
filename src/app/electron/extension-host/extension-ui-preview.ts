// biome-ignore-all lint: Host-side extension payload boundary keeps dynamic view shapes by design.
import {
  type TextFilePreview,
  textFilePreviewMarkdown,
} from '../files/text-file-preview';

export type ExtensionUiPreviewDeps = {
  buildPreviewItemAction: (item: unknown) => unknown;
  readTextFilePreview?: (filePath: string) => Promise<TextFilePreview>;
};

const PREVIEW_ITEM_KINDS = ['clipboard', 'image', 'video', 'file', 'text'];

function isPreviewItemDescriptor(value: any) {
  return value?.kind && PREVIEW_ITEM_KINDS.includes(value.kind);
}

// `path` and host-hydrated `fileUrl` only exist on ExtensionFile; `kind`,
// `videoUrl`, and `thumbnailUrl` are shared with preview-item descriptors.
function isExtensionFileInput(value: any) {
  return Boolean(value?.path || value?.fileUrl);
}

function isMediaFile(file: any) {
  return file.kind === 'image' || file.kind === 'video' || !!file.videoUrl;
}

function legacyFilePreviewView(file: any, view: any) {
  return {
    ...view,
    type: 'preview',
    presentation: view.presentation || 'preview',
    title: view.title || file.name || 'Preview',
    subtitle: view.subtitle || file.displayPath,
    content: view.content || file.displayPath || '',
    image: file.thumbnailUrl || file.url,
    video: file.videoUrl || undefined,
  };
}

function mediaFilePreviewView(file: any, view: any) {
  return {
    ...legacyFilePreviewView(file, view),
    image: file.thumbnailUrl || (file.kind === 'image' ? file.url : undefined),
  };
}

function previewItemDescriptorFromFile(file: any, options: any) {
  return {
    kind: isMediaFile(file) ? file.kind || 'video' : 'file',
    title: options.title,
    shortcut: options.shortcut,
    filePath: file.path,
    videoUrl: file.videoUrl || undefined,
    thumbnailUrl: file.thumbnailUrl || undefined,
  };
}

export function createExtensionUiPreview({
  buildPreviewItemAction,
  readTextFilePreview,
}: ExtensionUiPreviewDeps) {
  function previewView(view: any = {}) {
    return { ...view, type: 'preview' };
  }

  async function textFileContent(file: any) {
    if (!(readTextFilePreview && file.path)) return file.displayPath || '';
    return textFilePreviewMarkdown(
      file.path,
      await readTextFilePreview(file.path),
    );
  }

  async function previewFile(file: any, view: any = {}) {
    if (!isExtensionFileInput(file))
      throw new TypeError(
        'ctx.ui.previewFile expects a file from ctx.desktop.files.* (use metadata(path) for a path string)',
      );
    const base = mediaFilePreviewView(file, view);
    if (isMediaFile(file) || view.content) return base;
    return { ...base, content: await textFileContent(file) };
  }

  function previewAction(input: any = {}, options: any = {}) {
    if (isExtensionFileInput(input))
      return buildPreviewItemAction(
        previewItemDescriptorFromFile(input, options),
      );
    return buildPreviewItemAction({ ...input, ...options });
  }

  function legacyPreview(fileOrView: any, view: any = {}) {
    if (isExtensionFileInput(fileOrView))
      return legacyFilePreviewView(fileOrView, view);
    if (isPreviewItemDescriptor(fileOrView))
      return buildPreviewItemAction(fileOrView);
    return previewView(fileOrView);
  }

  return { previewView, previewFile, previewAction, legacyPreview };
}
