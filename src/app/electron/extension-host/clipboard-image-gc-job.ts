import fs from 'node:fs/promises';
import { createClipboardImageSweeper } from '../clipboard/clipboard-image-sweeper';
import { warn } from '../platform/logger';

export const CLIPBOARD_IMAGE_GC_JOB_ID = 'clipboard.images.gc';
export const CLIPBOARD_IMAGE_GC_INTERVAL_MS = 60 * 60 * 1000;
export const CLIPBOARD_IMAGE_GC_STARTUP_DELAY_MS = 30_000;

export type ClipboardImageGcDeps = {
  getClipboardHistory: () => unknown[];
  getClipboardImagesDir: () => string;
  /** Path of the state file this process loaded successfully, else null. */
  loadedStatePath: () => string | null;
};

async function readPersistedClipboardHistory(statePath: string) {
  try {
    const parsed = JSON.parse(await fs.readFile(statePath, 'utf8'));
    return Array.isArray(parsed?.clipboardHistory)
      ? (parsed.clipboardHistory as unknown[])
      : null;
  } catch {
    return null;
  }
}

export function createClipboardImageGcJob(deps: ClipboardImageGcDeps) {
  const sweeper = createClipboardImageSweeper({
    getImagesDir: deps.getClipboardImagesDir,
    getInMemoryHistory: deps.getClipboardHistory,
    isStateTrusted: () => deps.loadedStatePath() !== null,
    readPersistedHistory: async () => {
      const statePath = deps.loadedStatePath();
      return statePath ? readPersistedClipboardHistory(statePath) : null;
    },
    readdir: (dir) => fs.readdir(dir),
    lstat: (filePath) => fs.lstat(filePath),
    unlink: (filePath) => fs.unlink(filePath),
    now: Date.now,
    logWarn: (message, data) =>
      warn(message, data, { source: 'host', scope: 'cache' }),
  });
  return { run: sweeper.sweep };
}
