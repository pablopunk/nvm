import { describeInvalidExactSpec } from './window-management-exact-validation';
import {
  INVALID_OPTIONS_MESSAGE,
  INVALID_SPEC_MESSAGE,
} from './window-management-failures';
import type {
  WindowCycleGroup,
  WindowEdge,
  WindowLayoutSpec,
  WindowManagementOptions,
} from './window-management-types';

type FractionSpec = Extract<WindowLayoutSpec, { kind: 'fraction' }>;

const FRACTION_EDGE_EPSILON = 1e-6;
const MAX_GAP_POINTS = 100;
const MIN_RESIZE_STEP = 0.01;
const MAX_RESIZE_STEP = 0.25;
const WINDOW_EDGES: readonly WindowEdge[] = ['left', 'right', 'top', 'bottom'];
const RESIZE_DIRECTIONS: readonly string[] = ['larger', 'smaller'];
const DISPLAY_DIRECTIONS: readonly string[] = ['next', 'previous'];
const INVALID_GAP_MESSAGE = `The window gap must be a whole number from 0 to ${MAX_GAP_POINTS}.`;

function isWindowEdge(value: unknown): value is WindowEdge {
  return (WINDOW_EDGES as readonly unknown[]).includes(value);
}

function isOptionalCycleGroup(
  value: unknown,
): value is WindowCycleGroup | undefined {
  return value === undefined || isWindowEdge(value);
}

function isValidGap(gap: number) {
  return Number.isInteger(gap) && gap >= 0 && gap <= MAX_GAP_POINTS;
}

function isValidCycleOption(cycle: unknown) {
  return typeof cycle === 'boolean';
}

function isValidFractionRect(spec: FractionSpec) {
  const { x, y, width, height } = spec;
  return (
    [x, y, width, height].every(Number.isFinite) &&
    x >= 0 &&
    y >= 0 &&
    x <= 1 &&
    y <= 1 &&
    width > 0 &&
    width <= 1 &&
    height > 0 &&
    height <= 1 &&
    x + width <= 1 + FRACTION_EDGE_EPSILON &&
    y + height <= 1 + FRACTION_EDGE_EPSILON
  );
}

function isUnitFraction(value: number) {
  return Number.isFinite(value) && value > 0 && value <= 1;
}

function isOptionalPositiveLength(value: number | undefined) {
  return value === undefined || (Number.isFinite(value) && value > 0);
}

function isValidResizeStep(step: number) {
  return (
    Number.isFinite(step) && step >= MIN_RESIZE_STEP && step <= MAX_RESIZE_STEP
  );
}

function isValidFractionSpec(spec: FractionSpec) {
  return isValidFractionRect(spec) && isOptionalCycleGroup(spec.cycleGroup);
}

function isValidScaledCenterSpec(
  spec: Extract<WindowLayoutSpec, { kind: 'scaled-center' }>,
) {
  return (
    isUnitFraction(spec.widthFraction) &&
    isUnitFraction(spec.heightFraction) &&
    isOptionalPositiveLength(spec.maxWidth) &&
    isOptionalPositiveLength(spec.maxHeight)
  );
}

function isValidEdgeSpec(spec: Extract<WindowLayoutSpec, { kind: 'edge' }>) {
  return isWindowEdge(spec.edge);
}

function isValidResizeStepSpec(
  spec: Extract<WindowLayoutSpec, { kind: 'resize-step' }>,
) {
  return (
    RESIZE_DIRECTIONS.includes(spec.direction) && isValidResizeStep(spec.step)
  );
}

function isValidDisplaySpec(
  spec: Extract<WindowLayoutSpec, { kind: 'display' }>,
) {
  return DISPLAY_DIRECTIONS.includes(spec.direction);
}

function rejectUnknownSpecKind(_spec: never): string {
  return INVALID_SPEC_MESSAGE;
}

export function describeInvalidLayoutSpec(
  spec: WindowLayoutSpec,
): string | null {
  switch (spec.kind) {
    case 'exact':
      return describeInvalidExactSpec(spec);
    case 'fraction':
      return isValidFractionSpec(spec) ? null : INVALID_SPEC_MESSAGE;
    case 'scaled-center':
      return isValidScaledCenterSpec(spec) ? null : INVALID_SPEC_MESSAGE;
    case 'edge':
      return isValidEdgeSpec(spec) ? null : INVALID_SPEC_MESSAGE;
    case 'resize-step':
      return isValidResizeStepSpec(spec) ? null : INVALID_SPEC_MESSAGE;
    case 'display':
      return isValidDisplaySpec(spec) ? null : INVALID_SPEC_MESSAGE;
    case 'center':
    case 'maximize-width':
    case 'maximize-height':
    case 'restore':
    case 'toggle-full-screen':
      return null;
    default:
      return rejectUnknownSpecKind(spec);
  }
}

export function describeInvalidRequest(
  spec: WindowLayoutSpec,
  options: WindowManagementOptions,
): string | null {
  if (!isValidGap(options.gap)) {
    return INVALID_GAP_MESSAGE;
  }
  if (!isValidCycleOption(options.cycle)) {
    return INVALID_OPTIONS_MESSAGE;
  }
  return describeInvalidLayoutSpec(spec);
}
