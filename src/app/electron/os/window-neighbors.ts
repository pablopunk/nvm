import type {
  NeighborDirection,
  NeighborOptions,
} from './window-management-types';
import {
  areCentersCoincident,
  centerOf,
  coneWeightedDistance,
  coordinateOn,
  DIRECTION_AXES,
  hasArea,
  isHiddenBehindOrigin,
  type NeighborWindow,
  perpendicularTo,
} from './window-neighbor-geometry';

interface Measurement {
  id: string;
  side: 'ahead' | 'behind';
  displayRank: number;
  score: number;
  stackStep: number;
  axialDistance: number;
  perpendicularOffset: number;
  zIndex: number;
}

function displayRankOf(origin: NeighborWindow, candidate: NeighborWindow) {
  return candidate.displayId === origin.displayId ? 0 : 1;
}

function stackedMeasurement(
  origin: NeighborWindow,
  candidate: NeighborWindow,
  direction: NeighborDirection,
  displayRank: number,
): Measurement | null {
  const stackStep =
    DIRECTION_AXES[direction].sign * (candidate.zIndex - origin.zIndex);
  if (stackStep <= 0) {
    return null;
  }
  return {
    id: candidate.id,
    side: 'ahead',
    displayRank,
    score: 0,
    stackStep,
    axialDistance: 0,
    perpendicularOffset: 0,
    zIndex: candidate.zIndex,
  };
}

function geometricMeasurement(
  origin: NeighborWindow,
  candidate: NeighborWindow,
  direction: NeighborDirection,
  displayRank: number,
): Measurement | null {
  if (isHiddenBehindOrigin(origin, candidate)) {
    return null;
  }
  const { axis, sign } = DIRECTION_AXES[direction];
  const originCenter = centerOf(origin.frame);
  const candidateCenter = centerOf(candidate.frame);
  const signedAxial =
    sign *
    (coordinateOn(candidateCenter, axis) - coordinateOn(originCenter, axis));
  if (signedAxial === 0) {
    return null;
  }
  const perpendicular = perpendicularTo(axis);
  const axialDistance = Math.abs(signedAxial);
  const perpendicularOffset = Math.abs(
    coordinateOn(candidateCenter, perpendicular) -
      coordinateOn(originCenter, perpendicular),
  );
  return {
    id: candidate.id,
    side: signedAxial > 0 ? 'ahead' : 'behind',
    displayRank,
    score: coneWeightedDistance(axialDistance, perpendicularOffset),
    stackStep: 0,
    axialDistance,
    perpendicularOffset,
    zIndex: candidate.zIndex,
  };
}

function measureNeighbor(
  origin: NeighborWindow,
  candidate: NeighborWindow,
  direction: NeighborDirection,
): Measurement | null {
  if (candidate.id === origin.id || !hasArea(candidate.frame)) {
    return null;
  }
  const displayRank = displayRankOf(origin, candidate);
  if (origin.isFullScreen && displayRank === 0) {
    return null;
  }
  if (areCentersCoincident(centerOf(origin.frame), centerOf(candidate.frame))) {
    return stackedMeasurement(origin, candidate, direction, displayRank);
  }
  return geometricMeasurement(origin, candidate, direction, displayRank);
}

function compareIds(first: string, second: string) {
  if (first === second) {
    return 0;
  }
  return first < second ? -1 : 1;
}

function compareNearest(first: Measurement, second: Measurement) {
  return (
    first.displayRank - second.displayRank ||
    first.score - second.score ||
    first.stackStep - second.stackStep ||
    first.zIndex - second.zIndex ||
    compareIds(first.id, second.id)
  );
}

function compareFarthest(first: Measurement, second: Measurement) {
  return (
    first.displayRank - second.displayRank ||
    second.axialDistance - first.axialDistance ||
    first.perpendicularOffset - second.perpendicularOffset ||
    first.zIndex - second.zIndex ||
    compareIds(first.id, second.id)
  );
}

function bestOf(
  measurements: Measurement[],
  compare: (first: Measurement, second: Measurement) => number,
): string | null {
  return [...measurements].sort(compare)[0]?.id ?? null;
}

export const NEIGHBOR_DIRECTIONS: readonly NeighborDirection[] = [
  'left',
  'right',
  'up',
  'down',
];

export function nearestWindowInDirection(
  origin: NeighborWindow,
  candidates: NeighborWindow[],
  direction: NeighborDirection,
  options: NeighborOptions = {},
): string | null {
  if (!hasArea(origin.frame)) {
    return null;
  }
  const measured = candidates
    .map((candidate) => measureNeighbor(origin, candidate, direction))
    .filter((entry): entry is Measurement => entry !== null);
  const nearest = bestOf(
    measured.filter((entry) => entry.side === 'ahead'),
    compareNearest,
  );
  if (nearest !== null || !options.wrap) {
    return nearest;
  }
  return bestOf(
    measured.filter((entry) => entry.side === 'behind'),
    compareFarthest,
  );
}

export type { NeighborWindow } from './window-neighbor-geometry';
