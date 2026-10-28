import type { WindowManagementEngine } from './window-management-engine-port';
import {
  CYCLE_TOLERANCE_POINTS,
  framesMatch,
} from './window-management-frames';
import type { AppliedLayoutRecord } from './window-management-memory';
import type {
  Frame,
  WindowCycleGroup,
  WindowLayoutSpec,
} from './window-management-types';

const CYCLE_LENGTH = 3;

function isContinuingCycle(
  lastApplied: AppliedLayoutRecord | undefined,
  currentFrame: Frame,
  group: WindowCycleGroup,
) {
  return (
    lastApplied?.cycleGroup === group &&
    framesMatch(lastApplied.frame, currentFrame, CYCLE_TOLERANCE_POINTS)
  );
}

export interface CycleStep {
  spec: WindowLayoutSpec;
  cycleGroup?: WindowCycleGroup | undefined;
  cycleIndex: number;
}

export interface CycleStepInput {
  engine: WindowManagementEngine;
  lastApplied: AppliedLayoutRecord | undefined;
  currentFrame: Frame;
  spec: WindowLayoutSpec;
  cycleEnabled: boolean;
}

export function nextCycleIndex(
  lastApplied: AppliedLayoutRecord | undefined,
  currentFrame: Frame,
  group: WindowCycleGroup,
): number {
  if (!(lastApplied && isContinuingCycle(lastApplied, currentFrame, group))) {
    return 0;
  }
  return (lastApplied.cycleIndex + 1) % CYCLE_LENGTH;
}

export function resolveCycleStep({
  engine,
  lastApplied,
  currentFrame,
  spec,
  cycleEnabled,
}: CycleStepInput): CycleStep {
  if (!cycleEnabled || spec.kind !== 'fraction' || !spec.cycleGroup) {
    return { spec, cycleIndex: 0 };
  }
  const cycleIndex = nextCycleIndex(lastApplied, currentFrame, spec.cycleGroup);
  const sequence = engine.cycleSequence(spec.cycleGroup);
  return {
    spec: sequence[cycleIndex] ?? spec,
    cycleGroup: spec.cycleGroup,
    cycleIndex,
  };
}
