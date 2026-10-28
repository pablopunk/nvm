import type { Frame, NeighborDirection } from './window-management-types';

const COINCIDENT_CENTER_TOLERANCE = 2;
const HIDDEN_COVERAGE_RATIO = 0.95;

export interface NeighborWindow {
  id: string;
  frame: Frame;
  zIndex: number;
  displayId: string;
  isFullScreen: boolean;
}

export interface Point {
  x: number;
  y: number;
}

export type Axis = 'x' | 'y';

export interface DirectionAxis {
  axis: Axis;
  sign: 1 | -1;
}

export const DIRECTION_AXES: Record<NeighborDirection, DirectionAxis> = {
  right: { axis: 'x', sign: 1 },
  left: { axis: 'x', sign: -1 },
  down: { axis: 'y', sign: 1 },
  up: { axis: 'y', sign: -1 },
};

export function hasArea(frame: Frame) {
  return frame.width > 0 && frame.height > 0;
}

export function centerOf(frame: Frame): Point {
  return { x: frame.x + frame.width / 2, y: frame.y + frame.height / 2 };
}

export function coordinateOn(point: Point, axis: Axis) {
  return axis === 'x' ? point.x : point.y;
}

export function perpendicularTo(axis: Axis): Axis {
  return axis === 'x' ? 'y' : 'x';
}

export function areCentersCoincident(first: Point, second: Point) {
  return (
    Math.hypot(first.x - second.x, first.y - second.y) <=
    COINCIDENT_CENTER_TOLERANCE
  );
}

export function containsPoint(frame: Frame, point: Point) {
  return (
    point.x >= frame.x &&
    point.x <= frame.x + frame.width &&
    point.y >= frame.y &&
    point.y <= frame.y + frame.height
  );
}

export function coveredRatio(covered: Frame, cover: Frame) {
  const width =
    Math.min(covered.x + covered.width, cover.x + cover.width) -
    Math.max(covered.x, cover.x);
  const height =
    Math.min(covered.y + covered.height, cover.y + cover.height) -
    Math.max(covered.y, cover.y);
  return (
    (Math.max(0, width) * Math.max(0, height)) /
    (covered.width * covered.height)
  );
}

export function isHiddenBehindOrigin(
  origin: NeighborWindow,
  candidate: NeighborWindow,
) {
  return (
    candidate.zIndex > origin.zIndex &&
    (containsPoint(candidate.frame, centerOf(origin.frame)) ||
      coveredRatio(candidate.frame, origin.frame) >= HIDDEN_COVERAGE_RATIO)
  );
}

export function coneWeightedDistance(
  axialDistance: number,
  perpendicularOffset: number,
) {
  return (
    (axialDistance * axialDistance +
      perpendicularOffset * perpendicularOffset) /
    axialDistance
  );
}
