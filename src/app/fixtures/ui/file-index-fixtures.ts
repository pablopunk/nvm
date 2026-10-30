// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import fs from 'node:fs';
import path from 'node:path';
import type { ExtensionContext } from '../../resources/nevermind-extension-api';
import { WATCH_FIXTURE_ROOT } from './watch-fixture-root.ts';

export function fileIndexControlsAction(ctx: ExtensionContext) {
  return ctx.actions.run('Show File Index Snapshot', async (innerCtx) => {
    const files = innerCtx.desktop.files;
    if (!files)
      return innerCtx.ui.previewView({
        title: 'File Index Unavailable',
        content: 'The `desktop.files` host API is not available.',
      });
    const roots = files.indexedRoots();
    const snapshot = files.indexSnapshot({
      roots: ['~/Downloads'],
      extensions: ['png', 'jpg', 'jpeg', 'mp4', 'mov'],
      limit: 8,
      ignore: ['*.tmp'],
    });
    return innerCtx.ui.previewView({
      title: 'File Index Snapshot',
      content: [
        '# File Index Snapshot',
        '',
        `Default roots: ${roots.join(', ')}`,
        '',
        ...snapshot.map(
          (file) =>
            `- **${file.name}** · ${file.kind || 'file'} · ${file.displayPath || file.path}`,
        ),
      ].join('\n'),
    });
  });
}

export function reindexDownloadsAction(ctx: ExtensionContext) {
  return ctx.actions.run('Reindex Downloads Media', async (innerCtx) => {
    const files = innerCtx.desktop.files;
    if (!files)
      return innerCtx.ui.previewView({
        title: 'File Index Unavailable',
        content: 'The `desktop.files` host API is not available.',
      });
    const result = await files.reindex({
      roots: ['~/Downloads'],
      kind: 'media',
      depth: 2,
      limit: 500,
      ignore: ['*.tmp'],
    });
    return innerCtx.ui.previewView({
      title: 'Reindex Complete',
      content: `# Reindex Complete\n\nIndexed ${result.count} files from ${result.roots.join(', ')}.`,
    });
  });
}

export function openWithAppsAction(ctx: ExtensionContext) {
  return ctx.actions.run('Show Open With Apps', async (innerCtx) => {
    const filePath = path.join(WATCH_FIXTURE_ROOT, 'open-with-fixture.png');
    fs.mkdirSync(WATCH_FIXTURE_ROOT, { recursive: true });
    fs.writeFileSync(
      filePath,
      Buffer.from(
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
        'base64',
      ),
    );
    const apps = (await innerCtx.desktop.files?.openWithApps(filePath)) || [];
    return innerCtx.ui.list({
      id: 'dev-ui-open-with-apps',
      title: 'Dev UI · Open With Apps',
      subtitle: `${apps.length} available apps`,
      items: apps.map((app) => ({
        id: app.id || app.path || app.name,
        title: app.name,
        subtitle: app.path,
        primaryAction: innerCtx.actions.openWith(filePath, app),
      })),
      emptyView: innerCtx.ui.empty(
        'No apps available',
        'The host did not discover an application for this file.',
      ),
    });
  });
}
