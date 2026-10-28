import type {
  Frame,
  NeighborDirection,
  NeighborOptions,
} from './window-management-types';

interface DirectionAxis {
  axis: 'x' | 'y';
  sign: 1 | -1;
}

interface MeasuredCandidate {
  id: string;
  distance: number;
  overlaps: boolean;
  perpendicularGap: number;
}

const DIRECTION_AXES: Record<NeighborDirection, DirectionAxis> = {
  right: { axis: 'x', sign: 1 },
  left: { axis: 'x', sign: -1 },
  down: { axis: 'y', sign: 1 },
  up: { axis: 'y', sign: -1 },
};

const PERPENDICULAR_GAP_PENALTY = 2;

function hasArea(frame: Frame) {
  return frame.width > 0 && frame.height > 0;
}

function startOn(frame: Frame, axis: 'x' | 'y') {
  return axis === 'x' ? frame.x : frame.y;
}

function lengthOn(frame: Frame, axis: 'x' | 'y') {
  return axis === 'x' ? frame.width : frame.height;
}

function centerOn(frame: Frame, axis: 'x' | 'y') {
  return startOn(frame, axis) + lengthOn(frame, axis) / 2;
}

function perpendicularOf(axis: 'x' | 'y') {
  return axis === 'x' ? 'y' : 'x';
}

function perpendicularOverlap(
  current: Frame,
  candidate: Frame,
  axis: 'x' | 'y',
) {
  const perpendicular = perpendicularOf(axis);
  const currentStart = startOn(current, perpendicular);
  const candidateStart = startOn(candidate, perpendicular);
  return (
    Math.min(
      currentStart + lengthOn(current, perpendicular),
      candidateStart + lengthOn(candidate, perpendicular),
    ) - Math.max(currentStart, candidateStart)
  );
}

function measureCandidate(
  current: Frame,
  candidate: NeighborCandidate,
  { axis, sign }: DirectionAxis,
) {
  const signedDistance =
    sign * (centerOn(candidate.frame, axis) - centerOn(current, axis));
  const overlap = perpendicularOverlap(current, candidate.frame, axis);
  return {
    measured: {
      id: candidate.id,
      distance: Math.abs(signedDistance),
      overlaps: overlap > 0,
      perpendicularGap: Math.max(0, -overlap),
    },
    isAhead: signedDistance > 0,
    isBehind: signedDistance < 0,
  };
}

function penalisedDistance(candidate: MeasuredCandidate) {
  return (
    candidate.distance + PERPENDICULAR_GAP_PENALTY * candidate.perpendicularGap
  );
}

function overlapRank(candidate: MeasuredCandidate) {
  return candidate.overlaps ? 0 : 1;
}

function compareIds(first: string, second: string) {
  if (first === second) {
    return 0;
  }
  return first < second ? -1 : 1;
}

function compareNearest(first: MeasuredCandidate, second: MeasuredCandidate) {
  return (
    overlapRank(first) - overlapRank(second) ||
    penalisedDistance(first) - penalisedDistance(second) ||
    compareIds(first.id, second.id)
  );
}

function compareFarthest(first: MeasuredCandidate, second: MeasuredCandidate) {
  return (
    overlapRank(first) - overlapRank(second) ||
    second.distance - first.distance ||
    compareIds(first.id, second.id)
  );
}

function bestOf(
  candidates: MeasuredCandidate[],
  compare: (first: MeasuredCandidate, second: MeasuredCandidate) => number,
) {
  return [...candidates].sort(compare)[0]?.id ?? null;
}

export interface NeighborCandidate {
  id: string;
  frame: Frame;
}

export const NEIGHBOR_DIRECTIONS: readonly NeighborDirection[] = [
  'left',
  'right',
  'up',
  'down',
];

export function nearestWindowInDirection(
  current: Frame,
  candidates: NeighborCandidate[],
  direction: NeighborDirection,
  options: NeighborOptions = {},
): string | null {
  if (!hasArea(current)) {
    return null;
  }
  const axis = DIRECTION_AXES[direction];
  const measured = candidates
    .filter((candidate) => hasArea(candidate.frame))
    .map((candidate) => measureCandidate(current, candidate, axis));
  const ahead = measured
    .filter((entry) => entry.isAhead)
    .map((entry) => entry.measured);
  const nearest = bestOf(ahead, compareNearest);
  if (nearest !== null || !options.wrap) {
    return nearest;
  }
  const behind = measured
    .filter((entry) => entry.isBehind)
    .map((entry) => entry.measured);
  return bestOf(behind, compareFarthest);
}
