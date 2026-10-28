import type {
  Frame,
  WindowInfo,
  WriteOutcome,
} from './window-management-types';

const EXACT_TOLERANCE_POINTS = 2;
const CYCLE_TOLERANCE_POINTS = 2;

function clampOffsetInside(
  offset: number,
  size: number,
  start: number,
  extent: number,
) {
  return Math.max(start, Math.min(offset, start + extent - size));
}

export function framesMatch(first: Frame, second: Frame, tolerance: number) {
  return (
    Math.abs(first.x - second.x) <= tolerance &&
    Math.abs(first.y - second.y) <= tolerance &&
    Math.abs(first.width - second.width) <= tolerance &&
    Math.abs(first.height - second.height) <= tolerance
  );
}

export function isFrameInside(frame: Frame, workArea: Frame) {
  const right = workArea.x + workArea.width + EXACT_TOLERANCE_POINTS;
  const bottom = workArea.y + workArea.height + EXACT_TOLERANCE_POINTS;
  return (
    frame.x >= workArea.x - EXACT_TOLERANCE_POINTS &&
    frame.y >= workArea.y - EXACT_TOLERANCE_POINTS &&
    frame.x + frame.width <= right &&
    frame.y + frame.height <= bottom
  );
}

export function clampFrameInside(frame: Frame, workArea: Frame): Frame {
  return {
    x: clampOffsetInside(frame.x, frame.width, workArea.x, workArea.width),
    y: clampOffsetInside(frame.y, frame.height, workArea.y, workArea.height),
    width: frame.width,
    height: frame.height,
  };
}

export function fitFrameInside(frame: Frame, workArea: Frame): Frame {
  return clampFrameInside(
    {
      ...frame,
      width: Math.min(frame.width, workArea.width),
      height: Math.min(frame.height, workArea.height),
    },
    workArea,
  );
}

export function fitRequestedFrame(
  computed: Frame,
  focused: WindowInfo,
  workArea: Frame,
): Frame {
  if (focused.isResizable) {
    return computed;
  }
  return clampFrameInside(
    {
      x: computed.x,
      y: computed.y,
      width: focused.frame.width,
      height: focused.frame.height,
    },
    workArea,
  );
}

export function classifyFinalOutcome(
  observed: Frame,
  requested: Frame,
  original: Frame,
): WriteOutcome {
  if (framesMatch(observed, requested, EXACT_TOLERANCE_POINTS)) {
    return 'exact';
  }
  if (framesMatch(observed, original, EXACT_TOLERANCE_POINTS)) {
    return 'unapplied';
  }
  return 'partial';
}

export { CYCLE_TOLERANCE_POINTS, EXACT_TOLERANCE_POINTS };
