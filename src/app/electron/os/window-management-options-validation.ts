import {
  MAX_WINDOW_GAP_POINTS,
  type WindowEdge,
  type WindowEdgeGaps,
  type WindowManagementOptions,
} from './window-management-types';

const WINDOW_EDGES: readonly WindowEdge[] = ['top', 'right', 'bottom', 'left'];
const CYCLE_ORDERS: readonly string[] = [
  'half-two-thirds-third',
  'half-third-two-thirds',
];
const DISPLAY_ORDERS: readonly string[] = ['left-to-right', 'top-to-bottom'];
const DISPLAY_MOVE_SIZINGS: readonly string[] = ['proportional', 'keep-size'];

const GAP_RANGE_TEXT = `a whole number from 0 to ${MAX_WINDOW_GAP_POINTS}`;

const OUTER_GAP_MESSAGES: Record<WindowEdge, string> = {
  top: `The outer top gap must be ${GAP_RANGE_TEXT}.`,
  right: `The outer right gap must be ${GAP_RANGE_TEXT}.`,
  bottom: `The outer bottom gap must be ${GAP_RANGE_TEXT}.`,
  left: `The outer left gap must be ${GAP_RANGE_TEXT}.`,
};
const INNER_GAP_MESSAGE = `The inner gap must be ${GAP_RANGE_TEXT}.`;
const APPLY_GAP_TO_MAXIMIZE_MESSAGE =
  'The apply gap to maximize option must be true or false.';
const CYCLE_MESSAGE = 'The cycle option must be true or false.';
const CYCLE_ORDER_MESSAGE =
  'The cycle order must be half-two-thirds-third or half-third-two-thirds.';
const DISPLAY_ORDER_MESSAGE =
  'The display order must be left-to-right or top-to-bottom.';
const DISPLAY_MOVE_SIZING_MESSAGE =
  'The display move sizing must be proportional or keep-size.';

function isWholeGapPoints(value: unknown): boolean {
  return (
    typeof value === 'number' &&
    Number.isInteger(value) &&
    value >= 0 &&
    value <= MAX_WINDOW_GAP_POINTS
  );
}

function isOneOf(allowed: readonly unknown[], value: unknown): boolean {
  return allowed.includes(value);
}

function describeWhen(isValid: boolean, message: string): string | null {
  return isValid ? null : message;
}

function describeInvalidOuterGap(
  outerGap: Partial<WindowEdgeGaps> | undefined,
): string | null {
  const invalidEdge = WINDOW_EDGES.find(
    (edge) => !isWholeGapPoints(outerGap?.[edge]),
  );
  return invalidEdge ? OUTER_GAP_MESSAGES[invalidEdge] : null;
}

export function describeInvalidWindowManagementOptions(
  options: WindowManagementOptions,
): string | null {
  return (
    describeInvalidOuterGap(options.outerGap) ??
    describeWhen(isWholeGapPoints(options.innerGap), INNER_GAP_MESSAGE) ??
    describeWhen(
      typeof options.applyGapToMaximize === 'boolean',
      APPLY_GAP_TO_MAXIMIZE_MESSAGE,
    ) ??
    describeWhen(typeof options.cycle === 'boolean', CYCLE_MESSAGE) ??
    describeWhen(
      isOneOf(CYCLE_ORDERS, options.cycleOrder),
      CYCLE_ORDER_MESSAGE,
    ) ??
    describeWhen(
      isOneOf(DISPLAY_ORDERS, options.displayOrder),
      DISPLAY_ORDER_MESSAGE,
    ) ??
    describeWhen(
      isOneOf(DISPLAY_MOVE_SIZINGS, options.displayMoveSizing),
      DISPLAY_MOVE_SIZING_MESSAGE,
    )
  );
}
