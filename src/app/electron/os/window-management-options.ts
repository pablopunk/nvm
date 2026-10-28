import {
  DEFAULT_WINDOW_MANAGEMENT_OPTIONS,
  type WindowCycleOrder,
  type WindowDisplayMoveSizing,
  type WindowDisplayOrder,
  type WindowEdgeGaps,
  type WindowGaps,
  type WindowManagementOptions,
} from './window-management-types';

export interface PartialWindowManagementOptions {
  outerGap?: Partial<WindowEdgeGaps>;
  innerGap?: number;
  applyGapToMaximize?: boolean;
  cycle?: boolean;
  cycleOrder?: WindowCycleOrder;
  displayOrder?: WindowDisplayOrder;
  displayMoveSizing?: WindowDisplayMoveSizing;
}

const DEFAULT_OUTER_GAP = DEFAULT_WINDOW_MANAGEMENT_OPTIONS.outerGap;

function resolveEdgeGaps(
  partial: Partial<WindowEdgeGaps> | undefined,
): WindowEdgeGaps {
  return {
    top: partial?.top ?? DEFAULT_OUTER_GAP.top,
    right: partial?.right ?? DEFAULT_OUTER_GAP.right,
    bottom: partial?.bottom ?? DEFAULT_OUTER_GAP.bottom,
    left: partial?.left ?? DEFAULT_OUTER_GAP.left,
  };
}

export function resolveWindowManagementOptions(
  partial?: PartialWindowManagementOptions,
): WindowManagementOptions {
  const defaults = DEFAULT_WINDOW_MANAGEMENT_OPTIONS;
  return {
    outerGap: resolveEdgeGaps(partial?.outerGap),
    innerGap: partial?.innerGap ?? defaults.innerGap,
    applyGapToMaximize:
      partial?.applyGapToMaximize ?? defaults.applyGapToMaximize,
    cycle: partial?.cycle ?? defaults.cycle,
    cycleOrder: partial?.cycleOrder ?? defaults.cycleOrder,
    displayOrder: partial?.displayOrder ?? defaults.displayOrder,
    displayMoveSizing: partial?.displayMoveSizing ?? defaults.displayMoveSizing,
  };
}

export function windowGapsFrom(options: WindowManagementOptions): WindowGaps {
  return {
    outer: options.outerGap,
    inner: options.innerGap,
    applyToMaximize: options.applyGapToMaximize,
  };
}
