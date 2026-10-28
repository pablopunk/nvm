import type { WindowGaps } from './window-management-types';

export interface AxisInsets {
  leading: number;
  trailing: number;
}

export interface FractionInsets {
  horizontal: AxisInsets;
  vertical: AxisInsets;
}

export interface UnitRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

const EDGE_EPSILON = 1e-6;
const NO_AXIS_INSETS: AxisInsets = { leading: 0, trailing: 0 };
const NO_FRACTION_INSETS: FractionInsets = {
  horizontal: NO_AXIS_INSETS,
  vertical: NO_AXIS_INSETS,
};

function touchesLeadingEdge(start: number): boolean {
  return start <= EDGE_EPSILON;
}

function touchesTrailingEdge(start: number, length: number): boolean {
  return start + length >= 1 - EDGE_EPSILON;
}

function coversWholeWorkArea(rect: UnitRect): boolean {
  return (
    touchesLeadingEdge(rect.x) &&
    touchesTrailingEdge(rect.x, rect.width) &&
    touchesLeadingEdge(rect.y) &&
    touchesTrailingEdge(rect.y, rect.height)
  );
}

function interiorAxisInsets(innerGap: number): AxisInsets {
  return {
    leading: innerGap - Math.floor(innerGap / 2),
    trailing: Math.floor(innerGap / 2),
  };
}

function fractionAxisInsets(
  start: number,
  length: number,
  outer: { leading: number; trailing: number },
  innerGap: number,
): AxisInsets {
  const interior = interiorAxisInsets(innerGap);
  return {
    leading: touchesLeadingEdge(start) ? outer.leading : interior.leading,
    trailing: touchesTrailingEdge(start, length)
      ? outer.trailing
      : interior.trailing,
  };
}

export function fractionInsets(
  rect: UnitRect,
  gaps: WindowGaps,
): FractionInsets {
  if (!gaps.applyToMaximize && coversWholeWorkArea(rect)) {
    return NO_FRACTION_INSETS;
  }
  return {
    horizontal: fractionAxisInsets(
      rect.x,
      rect.width,
      { leading: gaps.outer.left, trailing: gaps.outer.right },
      gaps.inner,
    ),
    vertical: fractionAxisInsets(
      rect.y,
      rect.height,
      { leading: gaps.outer.top, trailing: gaps.outer.bottom },
      gaps.inner,
    ),
  };
}

export function maximizedAxisInsets(
  gaps: WindowGaps,
  leading: number,
  trailing: number,
): AxisInsets {
  return gaps.applyToMaximize ? { leading, trailing } : NO_AXIS_INSETS;
}
