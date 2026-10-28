import type { WindowEdgeGaps, WindowGaps } from './window-management-types';

export function uniformEdgeGaps(points: number): WindowEdgeGaps {
  return { top: points, right: points, bottom: points, left: points };
}

export function uniformGaps(points: number): WindowGaps {
  return {
    outer: uniformEdgeGaps(points),
    inner: points,
    applyToMaximize: true,
  };
}

export const NO_GAPS: WindowGaps = uniformGaps(0);
