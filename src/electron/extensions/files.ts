import { quickLookTitle, revealPathTitle } from '../os';
import { extensionContext } from './_context';

function buildPreviewItemActionForFile(input: any) {
  const kind =
    input?.kind ||
    (input?.text
      ? 'text'
      : input?.imageDataUrl || input?.imagePath
        ? 'image'
        : input?.videoUrl
          ? 'video'
          : input?.filePath
            ? 'file'
            : 'clipboard');
  return {
    type: 'previewClipboardItem',
    title: input?.title || 'Preview',
    description:
      input?.description ||
      (kind === 'file' || kind === 'image' || kind === 'video'
        ? 'Preview this file'
        : 'Preview clipboard item'),
    shortcut: input?.shortcut || 'Command+Y',
    clipboardType: input?.clipboardType || kind,
    text: input?.text,
    imageDataUrl: input?.imageDataUrl,
    imagePath: input?.imagePath,
    videoUrl: input?.videoUrl,
    filePath: input?.filePath,
    thumbnailUrl: input?.thumbnailUrl,
  };
}

function fileRootItem(item) {
  const openAction = {
    type: 'openPath' as const,
    title: `Open ${item.name}`,
    path: item.path,
    dismissAfterRun: 'auto' as const,
  };
  const revealAction = {
    type: 'revealPath' as const,
    title: revealPathTitle(),
    path: item.path,
    dismissAfterRun: 'auto' as const,
  };
  const quickLookAction = {
    type: 'quickLook' as const,
    title: quickLookTitle(),
    path: item.path,
  };
  const isMedia = item.kind === 'image' || item.kind === 'video';
  const text = isMedia ? undefined : item.displayPath;
  const previewAction = buildPreviewItemActionForFile({
    kind: item.kind || 'file',
    title: 'Preview',
    filePath: item.path,
    videoUrl: item.videoUrl,
    thumbnailUrl: item.thumbnailUrl,
    text,
  });
  return {
    id: `file:${item.path}`,
    title: item.name,
    subtitle: item.displayPath,
    icon: isMedia ? item.kind : 'folder',
    score: 4,
    dismissAfterRun: 'auto',
    filePath: item.path,
    videoUrl: item.videoUrl || undefined,
    thumbnailUrl: item.thumbnailUrl || undefined,
    text,
    primaryAction: openAction,
    actionPanel: {
      sections: [
        { actions: [previewAction, quickLookAction, revealAction, openAction] },
      ],
    },
  };
}

export function createFilesExtension() {
  return {
    id: 'nevermind.files',
    title: 'Files',
    capabilities: ['desktop.files'] as const,
    commands: [],
    rootItems(ctx) {
      return ctx.desktop.files
        .recent({ limit: extensionContext.FILE_RESULT_LIMIT })
        .map(fileRootItem);
    },
    searchItems(ctx, query) {
      return ctx.desktop.files
        .recent()
        .map(fileRootItem)
        .filter((item) => extensionContext.rankAction(item, query))
        .slice(0, extensionContext.FILE_RESULT_LIMIT);
    },
  };
}
