// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import type {
  ExtensionContext,
  ExtensionFile,
} from '../../resources/nevermind-extension-api';

function fileMetadataMarkdown(file: ExtensionFile) {
  return [
    `# ${file.name}`,
    '',
    `- Path: ${file.displayPath || file.path}`,
    `- Kind: ${file.kind || 'file'}`,
    `- MIME: ${file.mimeType || 'unknown'}`,
    `- Size: ${file.size || 0} bytes`,
    file.width && file.height
      ? `- Dimensions: ${file.width} × ${file.height}`
      : '',
    file.mtime ? `- Modified: ${file.mtime}` : '',
  ]
    .filter(Boolean)
    .join('\n');
}

export async function gridView(ctx: ExtensionContext) {
  const media = ctx.desktop.files
    ? await ctx.desktop.files.findMedia(
        ['~/Pictures', '~/Desktop', '~/Downloads'],
        { limit: 6, depth: 2, sortBy: 'recent' },
      )
    : [];
  if (media.length) {
    const files = await Promise.all(
      media.map((file) => ctx.desktop.files!.metadata(file.path)),
    );
    return ctx.ui.grid({
      id: 'dev-ui-grid',
      title: 'Dev UI · Grid',
      subtitle:
        'Grid tiles using ctx.desktop.files.metadata(...) and thumbnail(...)',
      layout: 'wide',
      aspectRatio: '16 / 9',
      columns: 3,
      actions: [
        ctx.actions.copyText('grid action', 'Copy Grid Action', {
          shortcut: 'Control+N',
        }),
      ],
      sections: [
        {
          title: 'Recent Media',
          items: files.map((file) => ({
            id: file.path,
            title: file.name,
            subtitle:
              file.width && file.height
                ? `${file.kind} · ${file.width} × ${file.height}`
                : file.displayPath,
            icon: file.kind === 'video' ? 'video' : 'image',
            image: ctx.desktop.files!.thumbnail(file.path) || file.url,
            accessories: [{ text: file.extension || file.kind || 'file' }],
            primaryAction: ctx.actions.push(
              'Show Metadata',
              ctx.ui.previewView({
                title: file.name,
                content: fileMetadataMarkdown(file),
                image: file.thumbnailUrl || file.url,
              }),
            ),
            actions: [
              ctx.ui.previewAction(file, { title: 'Quick Preview' }),
              ctx.actions.run(
                'Open Full Preview',
                function openFullPreview(innerCtx) {
                  return innerCtx.ui.previewFile(file, {
                    content: fileMetadataMarkdown(file),
                  });
                },
              ),
              ctx.actions.revealPath(file.path, 'Reveal File'),
              ctx.actions.copyText(file.path, 'Copy Path'),
            ],
          })),
        },
      ],
    });
  }

  const colors = [
    'yellow',
    'blue',
    'purple',
    'green',
    'red',
    'orange',
  ] as const;
  return ctx.ui.grid({
    id: 'dev-ui-grid',
    title: 'Dev UI · Grid',
    subtitle: 'Grid tiles, sections, layout, aspect ratio, and action hints',
    layout: 'wide',
    aspectRatio: '16 / 9',
    columns: 3,
    actions: [
      ctx.actions.copyText('grid action', 'Copy Grid Action', {
        shortcut: 'Control+N',
      }),
    ],
    maxVisibleItems: 24,
    sections: [
      {
        title: 'Fallback Tiles',
        items: [
          {
            id: 'glyph',
            title: 'Copy Sparkles',
            subtitle: 'Primary native glyph tile',
            glyph: '✨',
            appearance: { prominence: 'primary' },
            primaryAction: ctx.actions.copyText('✨', 'Copy Sparkles'),
            actions: [ctx.actions.copyText('✨', 'Copy Sparkles')],
          },
          ...colors.map((color) => ({
            id: color,
            title: `${color[0].toUpperCase()}${color.slice(1)} Tile`,
            subtitle:
              'Generated placeholder tile; add media to Pictures/Desktop/Downloads to exercise file helpers',
            icon: 'image',
            image: `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><rect width="100%" height="100%" rx="28" fill="${color}"/><text x="50%" y="54%" text-anchor="middle" font-family="system-ui" font-size="42" fill="black">${color}</text></svg>`)}`,
            appearance: { foreground: color },
            primaryAction: ctx.actions.copyText(color, `Copy ${color}`),
            actions: [ctx.actions.copyText(color, `Copy ${color}`)],
          })),
        ],
      },
    ],
  });
}
