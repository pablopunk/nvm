import { quickLookTitle, revealPathTitle } from '../electron/os/os';
import { extensionContext } from './_context';
import { semanticFileSearch } from './files-semantic-search';

function isMediaFile(item) {
  return item.kind === 'image' || item.kind === 'video';
}

function inPalettePreviewAction(ctx, item) {
  if (isMediaFile(item)) {
    return ctx.ui.previewAction(item, { title: 'Preview' });
  }
  return ctx.actions.run(
    'Preview Contents',
    function previewFileContents(innerCtx) {
      return innerCtx.ui.previewFile(item);
    },
  );
}

function systemPreviewAction(item) {
  return {
    type: 'quickLook' as const,
    title: quickLookTitle(),
    path: item.path,
  };
}

function fileRootItem(ctx, item) {
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
  const isMedia = isMediaFile(item);
  const text = isMedia ? undefined : item.displayPath;
  return {
    id: `file:${item.path}`,
    title: item.name,
    subtitle: item.displayPath,
    icon: isMedia ? item.kind : 'folder',
    score: 4,
    dismissAfterRun: 'auto' as const,
    filePath: item.path,
    videoUrl: item.videoUrl || undefined,
    thumbnailUrl: item.thumbnailUrl || undefined,
    text,
    primaryAction: openAction,
    actionPanel: {
      sections: [
        {
          actions: [
            inPalettePreviewAction(ctx, item),
            systemPreviewAction(item),
            revealAction,
            openAction,
          ],
        },
      ],
    },
  };
}

export function createFilesExtension() {
  return {
    id: 'nevermind.files',
    title: 'Files',
    capabilities: ['desktop.files', 'ai'] as const,
    commands: [
      {
        id: 'find-files-by-meaning',
        title: 'Find Files by Meaning',
        subtitle: 'Search recent file names using AI',
        icon: 'sparkles',
        run(ctx) {
          return ctx.navigation.run(
            semanticFileSearch(ctx, (item) => fileRootItem(ctx, item)),
          );
        },
      },
    ],
    rootItems(ctx) {
      return ctx.desktop.files
        .recent({ limit: extensionContext.FILE_RESULT_LIMIT })
        .map((item) => fileRootItem(ctx, item));
    },
    searchItems(ctx, query) {
      return ctx.desktop.files
        .recent()
        .map((item) => fileRootItem(ctx, item))
        .filter((item) => extensionContext.rankAction(item, query))
        .slice(0, extensionContext.FILE_RESULT_LIMIT);
    },
  };
}
