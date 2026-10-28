import type { WindowLayoutSpec } from './window-management-types';

const THIRD = 1 / 3;
const TWO_THIRDS = 2 / 3;
const SIXTH = 1 / 6;
const FOURTH = 1 / 4;
const HALF = 1 / 2;

function fraction(
  x: number,
  y: number,
  width: number,
  height: number,
  cycleGroup?: 'left' | 'right' | 'top' | 'bottom',
): WindowLayoutSpec {
  return cycleGroup
    ? { kind: 'fraction', x, y, width, height, cycleGroup }
    : { kind: 'fraction', x, y, width, height };
}

export const WINDOW_LAYOUT_SPECS = {
  'left-half': fraction(0, 0, HALF, 1, 'left'),
  'right-half': fraction(HALF, 0, HALF, 1, 'right'),
  'top-half': fraction(0, 0, 1, HALF, 'top'),
  'bottom-half': fraction(0, HALF, 1, HALF, 'bottom'),
  'center-half': fraction(FOURTH, 0, HALF, 1),
  maximize: fraction(0, 0, 1, 1),
  center: { kind: 'center' },
  'first-third': fraction(0, 0, THIRD, 1),
  'center-third': fraction(THIRD, 0, THIRD, 1),
  'last-third': fraction(TWO_THIRDS, 0, THIRD, 1),
  'top-third': fraction(0, 0, 1, THIRD),
  'middle-third': fraction(0, THIRD, 1, THIRD),
  'bottom-third': fraction(0, TWO_THIRDS, 1, THIRD),
  'first-two-thirds': fraction(0, 0, TWO_THIRDS, 1),
  'center-two-thirds': fraction(SIXTH, 0, TWO_THIRDS, 1),
  'last-two-thirds': fraction(THIRD, 0, TWO_THIRDS, 1),
  'top-two-thirds': fraction(0, 0, 1, TWO_THIRDS),
  'middle-two-thirds': fraction(0, SIXTH, 1, TWO_THIRDS),
  'bottom-two-thirds': fraction(0, THIRD, 1, TWO_THIRDS),
  'top-left-quarter': fraction(0, 0, HALF, HALF),
  'top-right-quarter': fraction(HALF, 0, HALF, HALF),
  'bottom-left-quarter': fraction(0, HALF, HALF, HALF),
  'bottom-right-quarter': fraction(HALF, HALF, HALF, HALF),
  'first-fourth': fraction(0, 0, FOURTH, 1),
  'second-fourth': fraction(FOURTH, 0, FOURTH, 1),
  'third-fourth': fraction(HALF, 0, FOURTH, 1),
  'last-fourth': fraction(HALF + FOURTH, 0, FOURTH, 1),
  'top-left-sixth': fraction(0, 0, THIRD, HALF),
  'top-center-sixth': fraction(THIRD, 0, THIRD, HALF),
  'top-right-sixth': fraction(TWO_THIRDS, 0, THIRD, HALF),
  'bottom-left-sixth': fraction(0, HALF, THIRD, HALF),
  'bottom-center-sixth': fraction(THIRD, HALF, THIRD, HALF),
  'bottom-right-sixth': fraction(TWO_THIRDS, HALF, THIRD, HALF),
  'maximize-width': { kind: 'maximize-width' },
  'maximize-height': { kind: 'maximize-height' },
  'almost-maximize': {
    kind: 'scaled-center',
    widthFraction: 0.9,
    heightFraction: 0.9,
  },
  'reasonable-size': {
    kind: 'scaled-center',
    widthFraction: 0.7,
    heightFraction: 0.7,
  },
  'move-left': { kind: 'edge', edge: 'left' },
  'move-right': { kind: 'edge', edge: 'right' },
  'move-up': { kind: 'edge', edge: 'top' },
  'move-down': { kind: 'edge', edge: 'bottom' },
  'make-larger': { kind: 'resize-step', direction: 'larger', step: 0.05 },
  'make-smaller': { kind: 'resize-step', direction: 'smaller', step: 0.05 },
  'next-display': { kind: 'display', direction: 'next' },
  'previous-display': { kind: 'display', direction: 'previous' },
  restore: { kind: 'restore' },
  'toggle-full-screen': { kind: 'toggle-full-screen' },
} as const satisfies Record<string, WindowLayoutSpec>;

export type WindowLayoutId = keyof typeof WINDOW_LAYOUT_SPECS;

export const WINDOW_LAYOUT_IDS = Object.keys(
  WINDOW_LAYOUT_SPECS,
) as WindowLayoutId[];
