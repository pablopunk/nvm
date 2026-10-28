// biome-ignore-all lint: End-to-end Electron harness intentionally drives the real macOS pasteboard through osascript and polls the production clipboard watcher job.
import { execFile as execFileCallback } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import { createRequire } from 'node:module';
import os from 'node:os';
import path from 'node:path';
import { promisify } from 'node:util';
import { expect, test } from '@playwright/test';
import { type ElectronApplication, _electron as electron } from 'playwright';
import { createClipboardHistory } from '../../src/app/electron/clipboard/clipboard-history';
import {
  expandUserPath,
  fileUrlForPath,
  isVideoPath,
  thumbnailUrlForPath,
} from '../../src/app/electron/files/file-utils';
import { createElectronClipboardApi } from '../../src/app/electron/os/electron-clipboard';
import { hashValue } from '../../src/app/electron/search/search-utils';

const require = createRequire(import.meta.url);
const execFile = promisify(execFileCallback);
const concealedType = 'org.nspasteboard.ConcealedType';
const pollIntervalMilliseconds = 1000;
const idleMainScript = `
const { app } = require('electron');
app.on('window-all-closed', () => {});
`;

test.skip(
  process.platform !== 'darwin',
  'NSPasteboard concealed markers only exist on macOS',
);

function pasteboardScript(text: string, concealed: boolean) {
  const types = concealed
    ? `[$.NSPasteboardTypeString, ${JSON.stringify(concealedType)}]`
    : '[$.NSPasteboardTypeString]';
  const concealedWrite = concealed
    ? `if (!pasteboard.setStringForType('1', ${JSON.stringify(concealedType)})) throw new Error('concealed marker write failed');`
    : '';
  return `
ObjC.import('AppKit');
const pasteboard = $.NSPasteboard.generalPasteboard;
pasteboard.clearContents();
pasteboard.declareTypesOwner($(${types}), null);
if (!pasteboard.setStringForType(${JSON.stringify(text)}, $.NSPasteboardTypeString)) throw new Error('text write failed');
${concealedWrite}
`;
}

async function writePasteboardText(text: string, concealed: boolean) {
  await execFile('osascript', [
    '-l',
    'JavaScript',
    '-e',
    pasteboardScript(text, concealed),
  ]);
}

function mainProcessClipboard(app: ElectronApplication) {
  return {
    has: (format: string) =>
      app.evaluate(({ clipboard }, value) => clipboard.has(value), format),
    readText: () => app.evaluate(({ clipboard }) => clipboard.readText()),
    read: async () =>
      (
        await app.evaluate(async ({ clipboard }) =>
          (await clipboard.read()).map((item) => ({ types: item.types })),
        )
      ).map((item) => ({
        types: item.types,
        getType: async () => {
          throw new Error('pasteboard payloads are not read by this probe');
        },
      })),
    clear: () => app.evaluate(({ clipboard }) => clipboard.clear()),
  };
}

function productionClipboardWatcher(app: ElectronApplication) {
  const history: any[] = [];
  const emptyImage = { isEmpty: () => true, toPNG: () => Buffer.alloc(0) };
  const clipboard = createElectronClipboardApi({
    clipboard: mainProcessClipboard(app) as any,
    nativeImage: {
      createFromBuffer: () => emptyImage as any,
      createEmpty: () => emptyImage as any,
    },
    ClipboardItem: class {} as any,
  });
  const noop = () => undefined;
  const deps = new Proxy(
    {
      getHistory: () => history,
      setHistory: (next: any[]) => history.splice(0, history.length, ...next),
      getSuppressedItemIds: () => new Map<string, number>(),
      getImagesDir: () => os.tmpdir(),
      clipboard,
      nativeImage: { createEmpty: () => emptyImage },
      hashValue,
      fileUrlForPath,
      thumbnailUrlForPath,
      isVideoPath,
      isImagePath: () => false,
      expandUserPath,
      measureSync: (_name: string, _data: unknown, run: () => unknown) => run(),
      CLIPBOARD_LIMIT: 300,
      CLIPBOARD_POLL_INTERVAL_MS: pollIntervalMilliseconds,
      CLIPBOARD_LAST_5_MINUTES_MS: 0,
      CLIPBOARD_LAST_HOUR_MS: 0,
      CLIPBOARD_LAST_DAY_MS: 0,
      CLIPBOARD_LAST_WEEK_MS: 0,
    } as Record<string, unknown>,
    { get: (target, name: string) => (name in target ? target[name] : noop) },
  );
  const service = createClipboardHistory(deps as any);
  return { clipboard, history, service };
}

async function runWatcherTicks(
  job: { run: () => Promise<void> },
  ticks: number,
) {
  for (let tick = 0; tick < ticks; tick += 1) {
    await new Promise((resolve) =>
      setTimeout(resolve, pollIntervalMilliseconds),
    );
    await job.run();
  }
}

test('clipboard watcher skips concealed pasteboard items but records ordinary text', async () => {
  test.setTimeout(60_000);
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-concealed-'));
  const mainScript = path.join(directory, 'main.js');
  await fs.writeFile(mainScript, idleMainScript);
  const app = await electron.launch({
    executablePath: require('electron') as string,
    args: [mainScript, `--user-data-dir=${path.join(directory, 'user-data')}`],
    timeout: 20_000,
  });
  try {
    const { clipboard, history, service } = productionClipboardWatcher(app);
    await writePasteboardText(`nvm-baseline-${crypto.randomUUID()}`, false);
    const job = await service.startClipboardWatcher();
    expect(job.triggers[0].everyMs).toBe(pollIntervalMilliseconds);

    const concealedText = `nvm-concealed-${crypto.randomUUID()}`;
    await writePasteboardText(concealedText, true);
    await expect.poll(() => clipboard.readText()).toBe(concealedText);
    expect(await clipboard.hasFormat(concealedType)).toBe(true);
    await runWatcherTicks(job, 3);
    expect(history.map((entry) => entry.text)).not.toContain(concealedText);

    const plainText = `nvm-plain-${crypto.randomUUID()}`;
    await writePasteboardText(plainText, false);
    expect(await clipboard.hasFormat(concealedType)).toBe(false);
    await runWatcherTicks(job, 3);
    expect(history.map((entry) => entry.text)).toContain(plainText);
  } finally {
    await app.close().catch(() => undefined);
    await fs.rm(directory, { recursive: true, force: true });
  }
});
