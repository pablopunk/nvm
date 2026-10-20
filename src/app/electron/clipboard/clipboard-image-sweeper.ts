import path from 'node:path';

const CLIPBOARD_IMAGE_FILENAME = /^[0-9a-f]{40}\.png$/;
export const CLIPBOARD_IMAGE_GRACE_MS = 10 * 60 * 1000;
export const CLIPBOARD_IMAGE_SWEEP_DEADLINE_MS = 15_000;
export const CLIPBOARD_IMAGE_SWEEP_MAX_DELETES = 500;

type FileStat = { isFile: () => boolean; mtimeMs: number };

export type ClipboardImageSweeperDeps = {
  getImagesDir: () => string;
  /** Live history, the authority for entries not saved yet. */
  getInMemoryHistory: () => unknown[];
  /**
   * History as last saved to disk, covering another instance sharing the same
   * user data folder. Returns null when it cannot be read, which aborts the sweep.
   */
  readPersistedHistory: () => Promise<unknown[] | null>;
  /** False whenever this process did not start from a valid saved state. */
  isStateTrusted: () => boolean;
  readdir: (dir: string) => Promise<string[]>;
  lstat: (filePath: string) => Promise<FileStat>;
  unlink: (filePath: string) => Promise<void>;
  now: () => number;
  logWarn: (message: string, data?: unknown) => void;
};

export type ClipboardImageSweepResult = {
  status: 'swept' | 'skipped';
  reason?: string;
  deleted: number;
};

/** Basenames of every PNG referenced by an image entry, keyed on imagePath, never on ids. */
export function referencedImageNames(history: unknown[]): Set<string> {
  const names = new Set<string>();
  for (const item of history) {
    const entry = item as { type?: unknown; imagePath?: unknown } | null;
    if (entry?.type === 'image' && typeof entry.imagePath === 'string')
      names.add(path.basename(entry.imagePath));
  }
  return names;
}

/**
 * Removes clipboard-image PNGs that no history entry references. Deletion is
 * eventual: a file is only removed after it is unreferenced in live and saved
 * history, older than the grace window, and re-verified right before unlink.
 */
export function createClipboardImageSweeper(deps: ClipboardImageSweeperDeps) {
  let inFlight: Promise<ClipboardImageSweepResult> | null = null;

  async function listCandidates(dir: string, referenced: Set<string>) {
    const names = await deps.readdir(dir);
    return names.filter(
      (name) => CLIPBOARD_IMAGE_FILENAME.test(name) && !referenced.has(name),
    );
  }

  async function isOldRegularFile(filePath: string) {
    const stat = await deps.lstat(filePath);
    return (
      stat.isFile() && deps.now() - stat.mtimeMs >= CLIPBOARD_IMAGE_GRACE_MS
    );
  }

  async function deleteIfStillOrphaned(dir: string, name: string) {
    const filePath = path.join(dir, name);
    if (!(await isOldRegularFile(filePath))) return false;
    // Synchronous re-check closes the readdir/lstat/unlink window against new copies.
    if (referencedImageNames(deps.getInMemoryHistory()).has(name)) return false;
    await deps.unlink(filePath);
    return true;
  }

  async function sweep(): Promise<ClipboardImageSweepResult> {
    if (!deps.isStateTrusted())
      return { status: 'skipped', reason: 'untrusted-state', deleted: 0 };
    const dir = deps.getImagesDir();
    if (!dir) return { status: 'skipped', reason: 'no-images-dir', deleted: 0 };

    const referenced = referencedImageNames(deps.getInMemoryHistory());
    const candidates = await listCandidates(dir, referenced);
    if (!candidates.length)
      return { status: 'swept', reason: 'no-candidates', deleted: 0 };

    const persisted = await deps.readPersistedHistory();
    if (!persisted)
      return { status: 'skipped', reason: 'persisted-unreadable', deleted: 0 };
    for (const name of referencedImageNames(persisted)) referenced.add(name);

    const deadline = deps.now() + CLIPBOARD_IMAGE_SWEEP_DEADLINE_MS;
    let deleted = 0;
    for (const name of candidates) {
      if (referenced.has(name)) continue;
      if (deleted >= CLIPBOARD_IMAGE_SWEEP_MAX_DELETES) break;
      if (deps.now() > deadline) break;
      try {
        if (await deleteIfStillOrphaned(dir, name)) deleted += 1;
      } catch (error) {
        deps.logWarn('clipboard.images.sweep.unlinkFailed', {
          name,
          error: error instanceof Error ? error.message : String(error),
        });
      }
    }
    return { status: 'swept', deleted };
  }

  function sweepOnce(): Promise<ClipboardImageSweepResult> {
    if (inFlight) return inFlight;
    inFlight = sweep()
      .catch((error): ClipboardImageSweepResult => {
        deps.logWarn('clipboard.images.sweep.failed', {
          error: error instanceof Error ? error.message : String(error),
        });
        return { status: 'skipped', reason: 'error', deleted: 0 };
      })
      .finally(() => {
        inFlight = null;
      });
    return inFlight;
  }

  return { sweep: sweepOnce };
}
