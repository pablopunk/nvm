// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import fs from 'node:fs';
import path from 'node:path';
import { showExtensionFeedback } from '../../extensions/feedback.ts';
import type { ExtensionContext } from '../../resources/nevermind-extension-api';
import { WATCH_FIXTURE_ROOT } from './watch-fixture-root.ts';

export function backgroundJobFixtureAction(ctx: ExtensionContext) {
  return ctx.action({
    id: 'background-job-fixture',
    title: 'Background Job Fixture',
    subtitle: 'Dev fixture for extension-owned host jobs and diagnostics',
    icon: 'activity',
    mode: 'background',
    triggers: [
      { type: 'startup', delayMs: 750 },
      { type: 'app.frontmost.changed', debounceMs: 1000 },
      { type: 'wake' },
      { type: 'login' },
    ],
    async run(innerCtx) {
      const count =
        (await innerCtx.storage.get<number>('backgroundJobRuns', 0)) || 0;
      await innerCtx.storage.set('backgroundJobRuns', count + 1);
    },
  });
}

export function folderWatchFixtureAction(ctx: ExtensionContext) {
  fs.mkdirSync(WATCH_FIXTURE_ROOT, { recursive: true });
  return ctx.action({
    id: 'folder-watch-fixture',
    title: 'Folder Watch Fixture Job',
    subtitle: 'Watches a temp folder and records ctx.launch.changedPaths/files',
    icon: 'folder-sync',
    mode: 'background',
    triggers: [
      {
        type: 'files.changed',
        roots: [WATCH_FIXTURE_ROOT],
        debounceMs: 250,
        extensions: ['txt', 'png'],
        ignore: ['ignored*'],
      },
    ],
    async run(innerCtx) {
      const count =
        (await innerCtx.storage.get<number>('folderWatchRuns', 0)) || 0;
      await innerCtx.storage.set('folderWatchRuns', count + 1);
      await innerCtx.storage.set('folderWatchLaunch', innerCtx.launch || null);
      showExtensionFeedback(
        innerCtx,
        'Folder Watch Fixture',
        `Folder watch fixture run #${count + 1}`,
      );
    },
  });
}

export async function folderWatchView(ctx: ExtensionContext) {
  fs.mkdirSync(WATCH_FIXTURE_ROOT, { recursive: true });
  const count = (await ctx.storage.get<number>('folderWatchRuns', 0)) || 0;
  const launch = await ctx.storage.get<any>('folderWatchLaunch', null);
  const touchAction = ctx.actions.run(
    'Touch Watched File',
    async (innerCtx) => {
      fs.mkdirSync(WATCH_FIXTURE_ROOT, { recursive: true });
      const filePath = path.join(
        WATCH_FIXTURE_ROOT,
        `changed-${Date.now()}.txt`,
      );
      fs.writeFileSync(
        filePath,
        `Nevermind folder watch fixture ${new Date().toISOString()}\n`,
      );
      showExtensionFeedback(
        innerCtx,
        'Folder Watch Fixture',
        `Touched ${path.basename(filePath)}`,
      );
    },
  );
  return ctx.ui.preview({
    id: 'dev-ui-folder-watch',
    title: 'Dev UI · Folder Watch',
    subtitle: WATCH_FIXTURE_ROOT,
    content: `# Folder Watch Fixture\n\nRoot: \`${WATCH_FIXTURE_ROOT}\`\n\nRuns: **${count}**\n\nLast launch context:\n\n\`\`\`json\n${JSON.stringify(launch, null, 2) || 'null'}\n\`\`\`\n\nUse the action panel to touch a watched .txt file, then reopen this view or check Background Tasks.`,
    actions: [touchAction],
    actionPanel: { sections: [{ actions: [touchAction] }] },
  });
}
