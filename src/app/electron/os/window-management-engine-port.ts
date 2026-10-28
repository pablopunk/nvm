import type {
  ComputeTargetFrame,
  DisplayInfo,
  Frame,
  WindowCycleGroup,
  WindowCycleOrder,
  WindowDisplayMoveSizing,
  WindowDisplayOrder,
  WindowLayoutSpec,
} from './window-management-types';

export type DisplayDirection = Extract<
  WindowLayoutSpec,
  { kind: 'display' }
>['direction'];

export interface DisplayMoveInput {
  frame: Frame;
  from: DisplayInfo;
  to: DisplayInfo;
  sizing: WindowDisplayMoveSizing;
}

export type MoveFrameToDisplay = (input: DisplayMoveInput) => Frame;

export type AdjacentDisplay = (
  displays: DisplayInfo[],
  from: DisplayInfo,
  direction: DisplayDirection,
  order: WindowDisplayOrder,
) => DisplayInfo | null;

export type CycleSequence = (
  group: WindowCycleGroup,
  order: WindowCycleOrder,
) => WindowLayoutSpec[];

export interface WindowManagementEngine {
  computeTargetFrame: ComputeTargetFrame;
  pickDisplayForFrame(displays: DisplayInfo[], frame: Frame): DisplayInfo;
  moveFrameToDisplay: MoveFrameToDisplay;
  adjacentDisplay: AdjacentDisplay;
  cycleSequence: CycleSequence;
}
