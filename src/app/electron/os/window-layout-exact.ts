import { fitFrameInside } from './window-management-frames';
import type {
  Frame,
  WindowDimension,
  WindowLayoutSpec,
} from './window-management-types';

type ExactLayoutSpec = Extract<WindowLayoutSpec, { kind: 'exact' }>;

const PERCENT_SCALE = 100;
const MIN_RESIZED_POINTS = 1;

function dimensionInPoints(dimension: WindowDimension, extent: number) {
  return dimension.unit === 'percent'
    ? (dimension.value / PERCENT_SCALE) * extent
    : dimension.value;
}

function offsetFromOrigin(
  dimension: WindowDimension,
  origin: number,
  extent: number,
) {
  return origin + Math.round(dimensionInPoints(dimension, extent));
}

function sizeInPoints(dimension: WindowDimension, extent: number) {
  return Math.max(
    MIN_RESIZED_POINTS,
    Math.round(dimensionInPoints(dimension, extent)),
  );
}

function exactOriginOnAxis(
  dimension: WindowDimension | undefined,
  workAreaOrigin: number,
  workAreaExtent: number,
  currentOrigin: number,
) {
  return dimension === undefined
    ? currentOrigin
    : offsetFromOrigin(dimension, workAreaOrigin, workAreaExtent);
}

function exactSizeOnAxis(
  dimension: WindowDimension | undefined,
  workAreaExtent: number,
  currentSize: number,
) {
  return dimension === undefined
    ? currentSize
    : sizeInPoints(dimension, workAreaExtent);
}

export function exactFrame(
  spec: ExactLayoutSpec,
  workArea: Frame,
  currentFrame: Frame,
): Frame {
  return fitFrameInside(
    {
      x: exactOriginOnAxis(spec.x, workArea.x, workArea.width, currentFrame.x),
      y: exactOriginOnAxis(spec.y, workArea.y, workArea.height, currentFrame.y),
      width: exactSizeOnAxis(spec.width, workArea.width, currentFrame.width),
      height: exactSizeOnAxis(
        spec.height,
        workArea.height,
        currentFrame.height,
      ),
    },
    workArea,
  );
}
