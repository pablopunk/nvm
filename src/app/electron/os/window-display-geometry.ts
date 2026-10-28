import type { WindowLayoutId } from './window-layout-catalog';
import { WINDOW_LAYOUT_SPECS } from './window-layout-catalog';
import type { AxisSpan, FractionLayoutSpec } from './window-layouts';
import { axesOf, frameFromAxes, placeSpan } from './window-layouts';
import type {
  AdjacentDisplay,
  CycleSequence,
  DisplayDirection,
  DisplayMoveInput,
  MoveFrameToDisplay,
} from './window-management-engine-port';
import type {
  DisplayInfo,
  Frame,
  WindowCycleGroup,
  WindowLayoutSpec,
} from './window-management-types';

const CYCLE_LAYOUT_IDS: Record<WindowCycleGroup, WindowLayoutId[]> = {
  left: ['left-half', 'first-two-thirds', 'first-third'],
  right: ['right-half', 'last-two-thirds', 'last-third'],
  top: ['top-half', 'top-two-thirds', 'top-third'],
  bottom: ['bottom-half', 'bottom-two-thirds', 'bottom-third'],
};

function compareDisplayPosition(a: DisplayInfo, b: DisplayInfo): number {
  return a.frame.x - b.frame.x || a.frame.y - b.frame.y;
}

function findAdjacentDisplay(
  displays: DisplayInfo[],
  from: DisplayInfo,
  direction: DisplayDirection,
): DisplayInfo | null {
  if (displays.length < 2) {
    return null;
  }
  const ordered = [...displays].sort(compareDisplayPosition);
  const currentIndex = ordered.findIndex((display) => display.id === from.id);
  if (currentIndex === -1) {
    return null;
  }
  const step = direction === 'next' ? 1 : -1;
  const wrappedIndex = (currentIndex + step + ordered.length) % ordered.length;
  return ordered[wrappedIndex] ?? null;
}

function scaledSpan(
  sourceArea: AxisSpan,
  targetArea: AxisSpan,
  windowSpan: AxisSpan,
): AxisSpan {
  const ratio = targetArea.length / sourceArea.length;
  return placeSpan(
    targetArea,
    Math.round(windowSpan.length * ratio),
    targetArea.origin +
      Math.round((windowSpan.origin - sourceArea.origin) * ratio),
  );
}

function relocateFrameToDisplay({ frame, from, to }: DisplayMoveInput): Frame {
  const source = axesOf(from.workArea);
  const target = axesOf(to.workArea);
  const windowAxes = axesOf(frame);
  return frameFromAxes({
    horizontal: scaledSpan(
      source.horizontal,
      target.horizontal,
      windowAxes.horizontal,
    ),
    vertical: scaledSpan(source.vertical, target.vertical, windowAxes.vertical),
  });
}

function fractionWithoutCycleGroup(spec: WindowLayoutSpec): FractionLayoutSpec {
  if (spec.kind !== 'fraction') {
    throw new Error(`${spec.kind} cannot join a cycle sequence`);
  }
  return {
    kind: 'fraction',
    x: spec.x,
    y: spec.y,
    width: spec.width,
    height: spec.height,
  };
}

function cycleSequenceFor(group: WindowCycleGroup): FractionLayoutSpec[] {
  return CYCLE_LAYOUT_IDS[group].map((layoutId) =>
    fractionWithoutCycleGroup(WINDOW_LAYOUT_SPECS[layoutId]),
  );
}

export const adjacentDisplay: AdjacentDisplay = findAdjacentDisplay;

export const moveFrameToDisplay: MoveFrameToDisplay = relocateFrameToDisplay;

export const cycleSequence: CycleSequence = cycleSequenceFor;
