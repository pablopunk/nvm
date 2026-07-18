import fs from 'node:fs/promises';
import path from 'node:path';
import { atomicallyReplaceFile } from './atomic-file';

type UserState = Record<string, unknown>;

interface UserStateFileDeps {
  readFile?: typeof fs.readFile;
  rename?: typeof fs.rename;
  now?: () => Date;
  replaceFile?: (filePath: string, contents: string) => Promise<void>;
  onCorrupt?: (error: unknown, backupPath: string) => void;
  onCorruptBackupError?: (error: unknown, backupPath: string) => void;
  onReadError?: (error: unknown) => void;
}

interface UserStateSaveSchedulerDeps {
  save: () => Promise<void>;
  onSaveError?: (error: unknown) => void;
  debounceMs?: number;
  setTimer?: typeof setTimeout;
  clearTimer?: typeof clearTimeout;
}

const DEFAULT_SAVE_DEBOUNCE_MS = 200;

function ignoreError() {
  return;
}

function isMissingFileError(error: unknown) {
  return (error as NodeJS.ErrnoException | undefined)?.code === 'ENOENT';
}

function isUserState(value: unknown): value is UserState {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value));
}

function corruptionTimestamp(now: Date) {
  return now
    .toISOString()
    .replaceAll('-', '')
    .replaceAll(':', '')
    .replace('.', '');
}

function corruptStateBackupPath(filePath: string, now: Date) {
  return path.join(
    path.dirname(filePath),
    `${path.basename(filePath)}.corrupt-${corruptionTimestamp(now)}`,
  );
}

async function readUserStateFile(
  filePath: string,
  deps: UserStateFileDeps = {},
): Promise<UserState | null> {
  const readFile = deps.readFile ?? fs.readFile;
  const rename = deps.rename ?? fs.rename;
  let contents: string;

  try {
    contents = await readFile(filePath, 'utf8');
  } catch (error) {
    if (isMissingFileError(error)) {
      return null;
    }
    deps.onReadError?.(error);
    return null;
  }

  try {
    const parsed: unknown = JSON.parse(contents);
    if (!isUserState(parsed)) {
      throw new SyntaxError('State must be an object');
    }
    return parsed;
  } catch (error) {
    const backupPath = corruptStateBackupPath(
      filePath,
      (deps.now ?? (() => new Date()))(),
    );
    deps.onCorrupt?.(error, backupPath);
    try {
      await rename(filePath, backupPath);
    } catch (backupError) {
      deps.onCorruptBackupError?.(backupError, backupPath);
    }
    return null;
  }
}

function writeUserStateFile(
  filePath: string,
  state: UserState,
  deps: UserStateFileDeps = {},
) {
  const replaceFile = deps.replaceFile ?? atomicallyReplaceFile;
  return replaceFile(filePath, JSON.stringify(state, null, 2));
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: The factory keeps one scheduler's timer, version, and shared promise state isolated.
function createUserStateSaveScheduler(deps: UserStateSaveSchedulerDeps) {
  const debounceMs = deps.debounceMs ?? DEFAULT_SAVE_DEBOUNCE_MS;
  const setTimer = deps.setTimer ?? setTimeout;
  const clearTimer = deps.clearTimer ?? clearTimeout;
  let requestedVersion = 0;
  let completedVersion = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let activeSave: Promise<void> | null = null;
  let activeFlush: Promise<void> | null = null;

  function cancelTimer() {
    if (timer === undefined) {
      return;
    }
    clearTimer(timer);
    timer = undefined;
  }

  function runLatestSave() {
    if (activeSave) {
      return activeSave;
    }
    if (completedVersion >= requestedVersion) {
      return Promise.resolve();
    }
    const savingVersion = requestedVersion;
    const save = Promise.resolve()
      .then(deps.save)
      .catch((error) => {
        deps.onSaveError?.(error);
        throw error;
      })
      .finally(() => {
        completedVersion = Math.max(completedVersion, savingVersion);
        activeSave = null;
      });
    activeSave = save;
    return save;
  }

  async function saveLatestPendingVersion() {
    if (activeSave) {
      await activeSave;
    }
    if (completedVersion < requestedVersion) {
      await runLatestSave();
    }
  }

  function schedule() {
    requestedVersion += 1;
    cancelTimer();
    timer = setTimer(() => {
      timer = undefined;
      saveLatestPendingVersion().catch(ignoreError);
    }, debounceMs);
    timer.unref?.();
  }

  function flushPendingSave() {
    if (activeFlush) {
      return activeFlush;
    }
    cancelTimer();
    const flushLatestRequestedVersion = async (): Promise<void> => {
      await saveLatestPendingVersion();
      cancelTimer();
      if (completedVersion < requestedVersion) {
        await flushLatestRequestedVersion();
      }
    };
    const flush = flushLatestRequestedVersion();
    activeFlush = flush.finally(() => {
      activeFlush = null;
    });
    return activeFlush;
  }

  return {
    schedule,
    flushPendingSave,
    hasPendingSave() {
      return completedVersion < requestedVersion;
    },
  };
}

export type { UserState, UserStateFileDeps, UserStateSaveSchedulerDeps };
export {
  corruptStateBackupPath,
  createUserStateSaveScheduler,
  readUserStateFile,
  writeUserStateFile,
};
