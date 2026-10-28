import {
  CYCLE_TOLERANCE_POINTS,
  framesMatch,
} from './window-management-frames';
import type {
  WindowMemory,
  WindowMemoryEntry,
} from './window-management-memory';
import type {
  Frame,
  WindowApplyResult,
  WindowCycleGroup,
  WindowInfo,
} from './window-management-types';

type OkResult = Extract<WindowApplyResult, { ok: true }>;

function hasMovedSinceLastApplied(
  entry: WindowMemoryEntry | undefined,
  current: Frame,
) {
  const lastApplied = entry?.lastApplied;
  return !(
    lastApplied &&
    framesMatch(lastApplied.frame, current, CYCLE_TOLERANCE_POINTS)
  );
}

function shouldCaptureRestoreFrame(
  entry: WindowMemoryEntry | undefined,
  current: Frame,
  changed: boolean,
) {
  return (
    changed &&
    (!entry?.restoreFrame || hasMovedSinceLastApplied(entry, current))
  );
}

export interface CycleRecord {
  cycleGroup?: WindowCycleGroup | undefined;
  cycleIndex: number;
}

export function rememberAppliedLayout(
  memory: WindowMemory,
  focused: WindowInfo,
  applied: OkResult,
  cycle: CycleRecord,
) {
  const previous = memory.read(focused.windowKey);
  const restoreFrame = shouldCaptureRestoreFrame(
    previous,
    focused.frame,
    applied.changed,
  )
    ? focused.frame
    : previous?.restoreFrame;
  memory.write(focused.windowKey, {
    restoreFrame,
    lastApplied: {
      frame: applied.frame,
      cycleGroup: cycle.cycleGroup,
      cycleIndex: cycle.cycleIndex,
    },
  });
}

export function forgetCycleGroup(memory: WindowMemory, windowKey: string) {
  const previous = memory.read(windowKey);
  if (!previous?.lastApplied) {
    return;
  }
  memory.write(windowKey, {
    restoreFrame: previous.restoreFrame,
    lastApplied: { ...previous.lastApplied, cycleGroup: undefined },
  });
}
