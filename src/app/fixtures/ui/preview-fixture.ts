import fs from 'node:fs';
import path from 'node:path';
import type { ExtensionContext } from '../../resources/nevermind-extension-api';
import { WATCH_FIXTURE_ROOT } from './watch-fixture-root.ts';

const LONG_LOG_LINE_COUNT = 5000;
const NUL_BYTE = 0;
const TEXT_PREVIEW_FIXTURES = {
  'short-notes.md': '# Notes\n\nA short file with ```fenced``` backticks.\n',
  'long-log.txt': 'line of fixture log output\n'.repeat(LONG_LOG_LINE_COUNT),
  'binary.bin': Buffer.from([NUL_BYTE, NUL_BYTE]),
};

function writeTextPreviewFixture(name: keyof typeof TEXT_PREVIEW_FIXTURES) {
  fs.mkdirSync(WATCH_FIXTURE_ROOT, { recursive: true });
  const filePath = path.join(WATCH_FIXTURE_ROOT, name);
  fs.writeFileSync(filePath, TEXT_PREVIEW_FIXTURES[name]);
  return filePath;
}

function previewFileFixtureAction(
  ctx: ExtensionContext,
  name: keyof typeof TEXT_PREVIEW_FIXTURES,
) {
  return ctx.actions.run(
    `Preview ${name}`,
    async function previewFixtureFile(innerCtx) {
      const files = innerCtx.desktop.files;
      if (!files) {
        return innerCtx.ui.previewView({
          title: 'Files Unavailable',
          content: 'The `desktop.files` host API is not available.',
        });
      }
      return innerCtx.ui.previewFile(
        await files.metadata(writeTextPreviewFixture(name)),
      );
    },
  );
}

export function previewView(ctx: ExtensionContext) {
  const templateAction = ctx.actions.run(
    'Expand Template',
    async (innerCtx) => {
      const output = await innerCtx.text.template(
        'Today is {date} at {time}. 6 * 7 = {calculator:6*7}. Selection: {selectedText}',
        { name: 'Nevermind' },
      );
      const cursor = await innerCtx.text.template(
        'Snippet for {name}: {cursor}done. Missing {argument:topic}',
        {
          variables: { name: 'Nevermind' },
          returnCursor: true,
          promptMissing: true,
        },
      );
      return innerCtx.ui.previewView({
        title: 'Template Output',
        content: `# Template Output\n\n${output}\n\nCursor-aware result:\n\n\`\`\`json\n${JSON.stringify(cursor, null, 2)}\n\`\`\``,
      });
    },
  );
  const fileActions = (
    Object.keys(TEXT_PREVIEW_FIXTURES) as Array<
      keyof typeof TEXT_PREVIEW_FIXTURES
    >
  ).map((name) => previewFileFixtureAction(ctx, name));
  return ctx.ui.previewView({
    id: 'dev-ui-preview',
    title: 'Dev UI · Preview',
    content:
      '# Preview Fixture\n\nThis exercises `ctx.ui.previewView`, bounded text/binary `ctx.ui.previewFile`, action panels, and `ctx.text.template(...)`.',
    actions: [templateAction, ...fileActions],
    actionPanel: {
      sections: [
        { actions: [templateAction] },
        { title: 'ctx.ui.previewFile', actions: fileActions },
      ],
    },
  });
}
