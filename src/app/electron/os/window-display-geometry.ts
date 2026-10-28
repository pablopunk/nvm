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
  WindowCycleOrder,
  WindowDisplayMoveSizing,
  WindowDisplayOrder,
  WindowLayoutSpec,
} from './window-management-types';

interface CycleStepLayoutIds {
  half: WindowLayoutId;
  twoThirds: WindowLayoutId;
  third: WindowLayoutId;
}

const CYCLE_STEP_LAYOUT_IDS: Record<WindowCycleGroup, CycleStepLayoutIds> = {
  left: {
    half: 'left-half',
    twoThirds: 'first-two-thirds',
    third: 'first-third',
  },
  right: {
    half: 'right-half',
    twoThirds: 'last-two-thirds',
    third: 'last-third',
  },
  top: { half: 'top-half', twoThirds: 'top-two-thirds', third: 'top-third' },
  bottom: {
    half: 'bottom-half',
    twoThirds: 'bottom-two-thirds',
    third: 'bottom-third',
  },
};

type SpanRelocator = (
  sourceArea: AxisSpan,
  targetArea: AxisSpan,
  windowSpan: AxisSpan,
) => AxisSpan;

function compareLeftToRight(a: DisplayInfo, b: DisplayInfo): number {
  return a.frame.x - b.frame.x || a.frame.y - b.frame.y;
}

function compareTopToBottom(a: DisplayInfo, b: DisplayInfo): number {
  return a.frame.y - b.frame.y || a.frame.x - b.frame.x;
}

const DISPLAY_COMPARATORS: Record<
  WindowDisplayOrder,
  (a: DisplayInfo, b: DisplayInfo) => number
> = {
  'left-to-right': compareLeftToRight,
  'top-to-bottom': compareTopToBottom,
};

function findAdjacentDisplay(
  displays: DisplayInfo[],
  from: DisplayInfo,
  direction: DisplayDirection,
  order: WindowDisplayOrder,
): DisplayInfo | null {
  if (displays.length < 2) {
    return null;
  }
  const ordered = [...displays].sort(DISPLAY_COMPARATORS[order]);
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

function keptSizeSpan(
  sourceArea: AxisSpan,
  targetArea: AxisSpan,
  windowSpan: AxisSpan,
): AxisSpan {
  const length = Math.min(windowSpan.length, targetArea.length);
  const sourceCenter = windowSpan.origin + windowSpan.length / 2;
  const relativeCenter = (sourceCenter - sourceArea.origin) / sourceArea.length;
  const targetCenter = targetArea.origin + relativeCenter * targetArea.length;
  return placeSpan(targetArea, length, targetCenter - length / 2);
}

const SPAN_RELOCATORS: Record<WindowDisplayMoveSizing, SpanRelocator> = {
  proportional: scaledSpan,
  'keep-size': keptSizeSpan,
};

function relocateFrameToDisplay({
  frame,
  from,
  to,
  sizing,
}: DisplayMoveInput): Frame {
  const relocate = SPAN_RELOCATORS[sizing];
  const source = axesOf(from.workArea);
  const target = axesOf(to.workArea);
  const windowAxes = axesOf(frame);
  return frameFromAxes({
    horizontal: relocate(
      source.horizontal,
      target.horizontal,
      windowAxes.horizontal,
    ),
    vertical: relocate(source.vertical, target.vertical, windowAxes.vertical),
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

function orderedCycleLayoutIds(
  steps: CycleStepLayoutIds,
  order: WindowCycleOrder,
): WindowLayoutId[] {
  return order === 'half-third-two-thirds'
    ? [steps.half, steps.third, steps.twoThirds]
    : [steps.half, steps.twoThirds, steps.third];
}

function cycleSequenceFor(
  group: WindowCycleGroup,
  order: WindowCycleOrder,
): FractionLayoutSpec[] {
  return orderedCycleLayoutIds(CYCLE_STEP_LAYOUT_IDS[group], order).map(
    (layoutId) => fractionWithoutCycleGroup(WINDOW_LAYOUT_SPECS[layoutId]),
  );
}

export const adjacentDisplay: AdjacentDisplay = findAdjacentDisplay;

export const moveFrameToDisplay: MoveFrameToDisplay = relocateFrameToDisplay;

export const cycleSequence: CycleSequence = cycleSequenceFor;
