import type {
  DisplayInfo,
  EngineInput,
  Frame,
  WindowLayoutSpec,
} from './window-management-types';

type FractionLayoutSpec = Extract<WindowLayoutSpec, { kind: 'fraction' }>;

const EDGE_EPSILON = 1e-6;

function findPrimaryDisplay(displays: DisplayInfo[]): DisplayInfo | undefined {
  return displays.find((display) => display.isPrimary);
}

function intersectionArea(a: Frame, b: Frame): number {
  const width = Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x);
  const height = Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y);
  return width > 0 && height > 0 ? width * height : 0;
}

function centerFrameInWorkArea(workArea: Frame, currentFrame: Frame): Frame {
  const width = Math.min(currentFrame.width, workArea.width);
  const height = Math.min(currentFrame.height, workArea.height);
  return {
    x: Math.round(workArea.x + (workArea.width - width) / 2),
    y: Math.round(workArea.y + (workArea.height - height) / 2),
    width,
    height,
  };
}

function fractionFrameInWorkArea(
  spec: FractionLayoutSpec,
  workArea: Frame,
  gap: number,
): Frame {
  const left = Math.round(workArea.x + spec.x * workArea.width);
  const right = Math.round(workArea.x + (spec.x + spec.width) * workArea.width);
  const top = Math.round(workArea.y + spec.y * workArea.height);
  const bottom = Math.round(
    workArea.y + (spec.y + spec.height) * workArea.height,
  );

  const x = left + leadingInset(spec.x > EDGE_EPSILON, gap);
  const trailingX =
    right - trailingInset(spec.x + spec.width < 1 - EDGE_EPSILON, gap);
  const y = top + leadingInset(spec.y > EDGE_EPSILON, gap);
  const trailingY =
    bottom - trailingInset(spec.y + spec.height < 1 - EDGE_EPSILON, gap);

  return {
    x,
    y,
    width: Math.max(0, trailingX - x),
    height: Math.max(0, trailingY - y),
  };
}

function leadingInset(isInteriorEdge: boolean, gap: number): number {
  return isInteriorEdge ? gap - Math.floor(gap / 2) : gap;
}

function trailingInset(isInteriorEdge: boolean, gap: number): number {
  return isInteriorEdge ? Math.floor(gap / 2) : gap;
}

export function computeTargetFrame(input: EngineInput): Frame {
  if (input.spec.kind === 'center') {
    return centerFrameInWorkArea(input.workArea, input.currentFrame);
  }
  return fractionFrameInWorkArea(input.spec, input.workArea, input.gap);
}

export function pickDisplayForFrame(
  displays: DisplayInfo[],
  frame: Frame,
): DisplayInfo {
  const fallback = findPrimaryDisplay(displays) ?? displays[0];
  if (!fallback) {
    throw new Error('pickDisplayForFrame requires at least one display');
  }

  const overlaps = displays.map((display) => ({
    display,
    area: intersectionArea(display.workArea, frame),
  }));
  const largestArea = Math.max(0, ...overlaps.map(({ area }) => area));
  if (largestArea === 0) {
    return fallback;
  }

  const largestDisplays = overlaps
    .filter(({ area }) => area === largestArea)
    .map(({ display }) => display);
  return findPrimaryDisplay(largestDisplays) ?? largestDisplays[0] ?? fallback;
}

export const LAYOUT_SPECS = {
  'left-half': { kind: 'fraction', x: 0, y: 0, width: 0.5, height: 1 },
  'right-half': { kind: 'fraction', x: 0.5, y: 0, width: 0.5, height: 1 },
  'top-half': { kind: 'fraction', x: 0, y: 0, width: 1, height: 0.5 },
  'bottom-half': { kind: 'fraction', x: 0, y: 0.5, width: 1, height: 0.5 },
  maximize: { kind: 'fraction', x: 0, y: 0, width: 1, height: 1 },
  center: { kind: 'center' },
} satisfies Record<string, WindowLayoutSpec>;
