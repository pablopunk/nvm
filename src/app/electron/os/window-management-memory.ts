import type { Frame, WindowCycleGroup } from './window-management-types';

const MAX_REMEMBERED_WINDOWS = 200;

function isKeyOfPid(windowKey: string, pid: number) {
  return windowKey.startsWith(`${pid}:`);
}

function evictOldestWhenFull(entries: Map<string, WindowMemoryEntry>) {
  if (entries.size <= MAX_REMEMBERED_WINDOWS) {
    return;
  }
  const oldest = entries.keys().next();
  if (!oldest.done) {
    entries.delete(oldest.value);
  }
}

export interface AppliedLayoutRecord {
  frame: Frame;
  cycleGroup?: WindowCycleGroup | undefined;
  cycleIndex: number;
}

export interface WindowMemoryEntry {
  restoreFrame?: Frame | undefined;
  lastApplied?: AppliedLayoutRecord | undefined;
}

export interface WindowMemory {
  read(windowKey: string): WindowMemoryEntry | undefined;
  write(windowKey: string, entry: WindowMemoryEntry): void;
  forget(windowKey: string): void;
  dropPid(pid: number): void;
  size(): number;
}

export function createWindowMemory(): WindowMemory {
  const entries = new Map<string, WindowMemoryEntry>();

  return {
    read: (windowKey) => entries.get(windowKey),
    write(windowKey, entry) {
      entries.delete(windowKey);
      entries.set(windowKey, entry);
      evictOldestWhenFull(entries);
    },
    forget(windowKey) {
      entries.delete(windowKey);
    },
    dropPid(pid) {
      for (const windowKey of [...entries.keys()]) {
        if (isKeyOfPid(windowKey, pid)) {
          entries.delete(windowKey);
        }
      }
    },
    size: () => entries.size,
  };
}

export { MAX_REMEMBERED_WINDOWS };
