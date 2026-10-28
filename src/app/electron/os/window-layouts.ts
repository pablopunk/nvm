import type {
  DisplayInfo,
  EngineInput,
  Frame,
  WindowEdge,
  WindowLayoutSpec,
} from './window-management-types';

type LayoutSpecOf<Kind extends WindowLayoutSpec['kind']> = Extract<
  WindowLayoutSpec,
  { kind: Kind }
>;

type FractionLayoutSpec = LayoutSpecOf<'fraction'>;

type ServiceHandledLayoutSpec =
  | LayoutSpecOf<'restore'>
  | LayoutSpecOf<'display'>
  | LayoutSpecOf<'toggle-full-screen'>;

interface AxisSpan {
  origin: number;
  length: number;
}

interface FrameAxes {
  horizontal: AxisSpan;
  vertical: AxisSpan;
}

type EdgeAlignment = 'start' | 'end' | 'keep';

const EDGE_EPSILON = 1e-6;
const MIN_RESIZED_LENGTH = 100;

const EDGE_ALIGNMENTS: Record<
  WindowEdge,
  { horizontal: EdgeAlignment; vertical: EdgeAlignment }
> = {
  left: { horizontal: 'start', vertical: 'keep' },
  right: { horizontal: 'end', vertical: 'keep' },
  top: { horizontal: 'keep', vertical: 'start' },
  bottom: { horizontal: 'keep', vertical: 'end' },
};

function findPrimaryDisplay(displays: DisplayInfo[]): DisplayInfo | undefined {
  return displays.find((display) => display.isPrimary);
}

function intersectionArea(a: Frame, b: Frame): number {
  const width = Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x);
  const height = Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y);
  return width > 0 && height > 0 ? width * height : 0;
}

function axesOf(frame: Frame): FrameAxes {
  return {
    horizontal: { origin: frame.x, length: frame.width },
    vertical: { origin: frame.y, length: frame.height },
  };
}

function frameFromAxes({ horizontal, vertical }: FrameAxes): Frame {
  return {
    x: Math.round(horizontal.origin),
    y: Math.round(vertical.origin),
    width: Math.round(horizontal.length),
    height: Math.round(vertical.length),
  };
}

function placeSpan(area: AxisSpan, length: number, origin: number): AxisSpan {
  const fittedLength = Math.min(length, area.length);
  const latestOrigin = area.origin + area.length - fittedLength;
  return {
    origin: Math.min(Math.max(origin, area.origin), latestOrigin),
    length: fittedLength,
  };
}

function centeredSpan(area: AxisSpan, length: number): AxisSpan {
  return placeSpan(area, length, area.origin + (area.length - length) / 2);
}

function insetSpan(area: AxisSpan, gap: number): AxisSpan {
  return {
    origin: area.origin + gap,
    length: Math.max(0, area.length - 2 * gap),
  };
}

function alignedSpan(
  area: AxisSpan,
  current: AxisSpan,
  alignment: EdgeAlignment,
  gap: number,
): AxisSpan {
  const origin = {
    start: area.origin + gap,
    end: area.origin + area.length - gap - current.length,
    keep: current.origin,
  }[alignment];
  return placeSpan(area, current.length, origin);
}

function resizedSpan(
  area: AxisSpan,
  current: AxisSpan,
  delta: number,
): AxisSpan {
  const length = Math.round(
    Math.max(
      current.length + delta,
      Math.min(MIN_RESIZED_LENGTH, current.length),
    ),
  );
  return placeSpan(
    area,
    length,
    current.origin + (current.length - length) / 2,
  );
}

function scaledLength(
  areaLength: number,
  fraction: number,
  maxLength: number | undefined,
): number {
  return Math.round(
    Math.min(fraction * areaLength, maxLength ?? Number.POSITIVE_INFINITY),
  );
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

function scaledCenterFrame(
  spec: LayoutSpecOf<'scaled-center'>,
  area: FrameAxes,
): Frame {
  return frameFromAxes({
    horizontal: centeredSpan(
      area.horizontal,
      scaledLength(area.horizontal.length, spec.widthFraction, spec.maxWidth),
    ),
    vertical: centeredSpan(
      area.vertical,
      scaledLength(area.vertical.length, spec.heightFraction, spec.maxHeight),
    ),
  });
}

function edgeFrame(
  edge: WindowEdge,
  area: FrameAxes,
  current: FrameAxes,
  gap: number,
): Frame {
  const alignment = EDGE_ALIGNMENTS[edge];
  return frameFromAxes({
    horizontal: alignedSpan(
      area.horizontal,
      current.horizontal,
      alignment.horizontal,
      gap,
    ),
    vertical: alignedSpan(
      area.vertical,
      current.vertical,
      alignment.vertical,
      gap,
    ),
  });
}

function resizedFrame(
  spec: LayoutSpecOf<'resize-step'>,
  area: FrameAxes,
  current: FrameAxes,
): Frame {
  const direction = spec.direction === 'larger' ? 1 : -1;
  return frameFromAxes({
    horizontal: resizedSpan(
      area.horizontal,
      current.horizontal,
      direction * spec.step * area.horizontal.length,
    ),
    vertical: resizedSpan(
      area.vertical,
      current.vertical,
      direction * spec.step * area.vertical.length,
    ),
  });
}

function isServiceHandledLayoutSpec(
  spec: WindowLayoutSpec,
): spec is ServiceHandledLayoutSpec {
  return (
    spec.kind === 'restore' ||
    spec.kind === 'display' ||
    spec.kind === 'toggle-full-screen'
  );
}

function unhandledLayoutSpec(spec: never): never {
  throw new Error(`Unhandled layout spec: ${JSON.stringify(spec)}`);
}

function computeTargetFrame(input: EngineInput): Frame {
  const { spec, workArea, currentFrame, gap } = input;
  if (isServiceHandledLayoutSpec(spec)) {
    throw new Error(`${spec.kind} is handled by the window management service`);
  }
  const area = axesOf(workArea);
  const current = axesOf(currentFrame);
  switch (spec.kind) {
    case 'fraction':
      return fractionFrameInWorkArea(spec, workArea, gap);
    case 'center':
      return frameFromAxes({
        horizontal: centeredSpan(area.horizontal, current.horizontal.length),
        vertical: centeredSpan(area.vertical, current.vertical.length),
      });
    case 'maximize-width':
      return frameFromAxes({
        horizontal: insetSpan(area.horizontal, gap),
        vertical: placeSpan(
          area.vertical,
          current.vertical.length,
          current.vertical.origin,
        ),
      });
    case 'maximize-height':
      return frameFromAxes({
        horizontal: placeSpan(
          area.horizontal,
          current.horizontal.length,
          current.horizontal.origin,
        ),
        vertical: insetSpan(area.vertical, gap),
      });
    case 'scaled-center':
      return scaledCenterFrame(spec, area);
    case 'edge':
      return edgeFrame(spec.edge, area, current, gap);
    case 'resize-step':
      return resizedFrame(spec, area, current);
    default:
      return unhandledLayoutSpec(spec);
  }
}

function pickDisplayForFrame(
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

export type { AxisSpan, FractionLayoutSpec, FrameAxes, LayoutSpecOf };
export {
  axesOf,
  computeTargetFrame,
  frameFromAxes,
  isServiceHandledLayoutSpec,
  pickDisplayForFrame,
  placeSpan,
};
