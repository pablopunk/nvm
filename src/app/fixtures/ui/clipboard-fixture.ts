// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import fs from 'node:fs';
import path from 'node:path';
import { showExtensionFeedback } from '../../extensions/feedback.ts';
import type { ExtensionContext } from '../../resources/nevermind-extension-api';
import { WATCH_FIXTURE_ROOT } from './watch-fixture-root.ts';

export function clipboardView(ctx: ExtensionContext) {
  const clipboardApi = ctx.desktop.clipboard;
  const preview = ctx.actions.run(
    'Preview Clipboard Item',
    (innerCtx) =>
      innerCtx.ui.preview({
        title: 'Clipboard Preview',
        content: 'This fixture action uses the selected-item shortcut.',
      }),
    { shortcut: 'Command+Y' },
  );
  const writeHtml = ctx.actions.run(
    'Write HTML Clipboard',
    async (innerCtx) => {
      innerCtx.desktop.clipboard?.write(
        {
          type: 'html',
          html: '<strong>Nevermind HTML Clipboard</strong>',
          text: 'Nevermind HTML Clipboard',
        },
        { concealed: true },
      );
      showExtensionFeedback(
        innerCtx,
        'Clipboard Fixture',
        'Wrote concealed HTML clipboard content',
      );
    },
  );
  const writeFiles = ctx.actions.run(
    'Write Files Clipboard',
    async (innerCtx) => {
      fs.mkdirSync(WATCH_FIXTURE_ROOT, { recursive: true });
      const filePath = path.join(WATCH_FIXTURE_ROOT, 'clipboard-file.txt');
      fs.writeFileSync(filePath, 'Nevermind clipboard file fixture\n');
      innerCtx.desktop.clipboard?.writeFiles([filePath], { concealed: true });
      showExtensionFeedback(
        innerCtx,
        'Clipboard Fixture',
        `Wrote ${path.basename(filePath)} as clipboard file`,
      );
    },
  );
  const read = ctx.actions.run('Read Clipboard', async (innerCtx) => {
    const value = await innerCtx.desktop.clipboard?.read({
      formats: ['files', 'html', 'text', 'image'],
    });
    return innerCtx.ui.preview({
      title: 'Clipboard Read Result',
      content: `\`\`\`json\n${JSON.stringify(value, null, 2)}\n\`\`\``,
    });
  });
  const pasteHtml = ctx.actions.paste(
    {
      type: 'html',
      html: '<em>Nevermind pasted HTML</em>',
      text: 'Nevermind pasted HTML',
    },
    'Paste HTML + Restore',
    { restoreClipboard: true, concealed: true, dismissAfterRun: 'auto' },
  );
  return ctx.ui.list({
    id: 'dev-ui-clipboard',
    title: 'Dev UI · Clipboard',
    subtitle: clipboardApi
      ? 'Read/write text, HTML, files, and generic paste content'
      : 'Clipboard history host API unavailable',
    items: [
      ctx.ui.item({
        id: 'preview',
        title: 'Preview clipboard item',
        subtitle: 'Selected-item shortcut: Command+Y',
        icon: 'eye',
        primaryAction: preview,
        actions: [preview],
      }),
      ctx.ui.item({
        id: 'write-html',
        title: 'Write concealed HTML',
        subtitle: 'Writes HTML with a plain-text fallback',
        icon: 'clipboard',
        primaryAction: writeHtml,
        actions: [writeHtml],
      }),
      ctx.ui.item({
        id: 'write-files',
        title: 'Write files to clipboard',
        subtitle: WATCH_FIXTURE_ROOT,
        icon: 'files',
        primaryAction: writeFiles,
        actions: [writeFiles],
      }),
      ctx.ui.item({
        id: 'read',
        title: 'Read current clipboard',
        subtitle: 'Prefers files, image, HTML, then text',
        icon: 'clipboard-list',
        primaryAction: read,
        actions: [read],
      }),
      ctx.ui.item({
        id: 'paste-html',
        title: 'Paste HTML and restore clipboard',
        subtitle: 'Uses ctx.actions.paste(...) with concealed restore behavior',
        icon: 'clipboard-paste',
        primaryAction: pasteHtml,
        actions: [pasteHtml],
      }),
    ],
  });
}
