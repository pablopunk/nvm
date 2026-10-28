// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import type {
  Frame,
  NeighborDirection,
  NeighborOptions,
} from './window-management-types';
import {
  type NeighborCandidate,
  nearestWindowInDirection,
} from './window-neighbors';

interface NeighborCase {
  name: string;
  current: Frame;
  candidates: NeighborCandidate[];
  direction: NeighborDirection;
  expected: string | null;
  options?: NeighborOptions;
}

function rect(x: number, y: number, width: number, height: number): Frame {
  return { x, y, width, height };
}

function named(id: string, frame: Frame): NeighborCandidate {
  return { id, frame };
}

const TOP_LEFT = rect(0, 0, 100, 100);
const TOP_RIGHT = rect(100, 0, 100, 100);
const BOTTOM_LEFT = rect(0, 100, 100, 100);
const BOTTOM_RIGHT = rect(100, 100, 100, 100);
const GRID = [
  named('tl', TOP_LEFT),
  named('tr', TOP_RIGHT),
  named('bl', BOTTOM_LEFT),
  named('br', BOTTOM_RIGHT),
];

const LEFT_HALF = rect(0, 0, 500, 900);
const RIGHT_HALF = rect(500, 0, 500, 900);
const HALVES = [named('left', LEFT_HALF), named('right', RIGHT_HALF)];

const STACK_TOP = rect(0, 0, 800, 400);
const STACK_BOTTOM = rect(0, 400, 800, 400);
const STACKED = [named('top', STACK_TOP), named('bottom', STACK_BOTTOM)];

const CASCADE_FIRST = rect(0, 0, 600, 400);
const CASCADE_SECOND = rect(30, 30, 600, 400);
const CASCADE_THIRD = rect(60, 60, 600, 400);
const CASCADE = [
  named('w1', CASCADE_FIRST),
  named('w2', CASCADE_SECOND),
  named('w3', CASCADE_THIRD),
];

const BIG = rect(0, 0, 1000, 1000);
const TINY_INSIDE_BIG = rect(100, 100, 40, 40);

const MAIN_WINDOW = rect(100, 100, 800, 600);
const LEFT_DISPLAY_WINDOW = rect(-1500, 200, 1000, 600);
const ABOVE_MAIN_WINDOW = rect(100, -900, 800, 600);

const ROW = [
  named('x1', rect(0, 0, 100, 100)),
  named('x2', rect(100, 0, 100, 100)),
  named('x3', rect(200, 0, 100, 100)),
];

const NEAR_DIAGONAL = named('diagonal', rect(110, 150, 100, 100));
const FAR_SAME_ROW = named('row', rect(500, 0, 100, 100));
const FAR_SLIVER_OVERLAP = named('sliver', rect(500, 99, 100, 100));
const STEEP_NEAR = named('steep', rect(120, 250, 100, 100));
const FLAT_FARTHER = named('flat', rect(400, 100, 100, 100));

const NEIGHBOR_CASES: NeighborCase[] = [
  {
    name: 'grid: right from top-left picks top-right',
    current: TOP_LEFT,
    candidates: GRID,
    direction: 'right',
    expected: 'tr',
  },
  {
    name: 'grid: down from top-left picks bottom-left',
    current: TOP_LEFT,
    candidates: GRID,
    direction: 'down',
    expected: 'bl',
  },
  {
    name: 'grid: left from bottom-right picks bottom-left over the diagonal',
    current: BOTTOM_RIGHT,
    candidates: GRID,
    direction: 'left',
    expected: 'bl',
  },
  {
    name: 'grid: up from bottom-right picks top-right',
    current: BOTTOM_RIGHT,
    candidates: GRID,
    direction: 'up',
    expected: 'tr',
  },
  {
    name: 'grid: right from top-right has no neighbour without wrap',
    current: TOP_RIGHT,
    candidates: GRID,
    direction: 'right',
    expected: null,
  },
  {
    name: 'grid: right from top-right wraps to top-left',
    current: TOP_RIGHT,
    candidates: GRID,
    direction: 'right',
    expected: 'tl',
    options: { wrap: true },
  },
  {
    name: 'grid: up from top-left wraps to bottom-left',
    current: TOP_LEFT,
    candidates: GRID,
    direction: 'up',
    expected: 'bl',
    options: { wrap: true },
  },
  {
    name: 'halves: right from left half selects right half',
    current: LEFT_HALF,
    candidates: HALVES,
    direction: 'right',
    expected: 'right',
  },
  {
    name: 'halves: left from left half wraps to right half',
    current: LEFT_HALF,
    candidates: HALVES,
    direction: 'left',
    expected: 'right',
    options: { wrap: true },
  },
  {
    name: 'stacked: down from top selects the window below',
    current: STACK_TOP,
    candidates: STACKED,
    direction: 'down',
    expected: 'bottom',
  },
  {
    name: 'stacked: right ignores a window that only sits below',
    current: STACK_TOP,
    candidates: STACKED,
    direction: 'right',
    expected: null,
  },
  {
    name: 'cascade: right from first layer selects the next layer',
    current: CASCADE_FIRST,
    candidates: CASCADE,
    direction: 'right',
    expected: 'w2',
  },
  {
    name: 'cascade: left from last layer selects the previous layer',
    current: CASCADE_THIRD,
    candidates: CASCADE,
    direction: 'left',
    expected: 'w2',
  },
  {
    name: 'tiny window nested in a big one: right selects the enclosing window (awkward)',
    current: TINY_INSIDE_BIG,
    candidates: [named('big', BIG), named('tiny', TINY_INSIDE_BIG)],
    direction: 'right',
    expected: 'big',
  },
  {
    name: 'second display with negative origin: left selects the window on it',
    current: MAIN_WINDOW,
    candidates: [
      named('left-display', LEFT_DISPLAY_WINDOW),
      named('above', ABOVE_MAIN_WINDOW),
    ],
    direction: 'left',
    expected: 'left-display',
  },
  {
    name: 'display above with negative y: up selects the window above',
    current: MAIN_WINDOW,
    candidates: [
      named('left-display', LEFT_DISPLAY_WINDOW),
      named('above', ABOVE_MAIN_WINDOW),
    ],
    direction: 'up',
    expected: 'above',
  },
  {
    name: 'display above with negative y: down from above returns to main',
    current: ABOVE_MAIN_WINDOW,
    candidates: [named('main', MAIN_WINDOW)],
    direction: 'down',
    expected: 'main',
  },
  {
    name: 'lone window has no neighbour in any direction',
    current: TOP_LEFT,
    candidates: [named('self', TOP_LEFT)],
    direction: 'right',
    expected: null,
    options: { wrap: true },
  },
  {
    name: 'identical frames are not neighbours of each other',
    current: TOP_LEFT,
    candidates: [named('a', TOP_LEFT), named('b', TOP_LEFT)],
    direction: 'right',
    expected: null,
  },
  {
    name: 'identical frames do not hide a real neighbour',
    current: TOP_LEFT,
    candidates: [
      named('a', TOP_LEFT),
      named('b', TOP_LEFT),
      named('c', TOP_RIGHT),
    ],
    direction: 'right',
    expected: 'c',
  },
  {
    name: 'zero-area candidates are ignored',
    current: TOP_LEFT,
    candidates: [named('sliver', rect(150, 0, 0, 100))],
    direction: 'right',
    expected: null,
  },
  {
    name: 'zero-area current window has no neighbour',
    current: rect(0, 0, 0, 100),
    candidates: [named('tr', TOP_RIGHT)],
    direction: 'right',
    expected: null,
  },
  {
    name: 'equal candidates tie-break on the smaller id',
    current: TOP_LEFT,
    candidates: [named('z', TOP_RIGHT), named('b', TOP_RIGHT)],
    direction: 'right',
    expected: 'b',
  },
  {
    name: 'overlapping same-row window beats a closer diagonal window',
    current: TOP_LEFT,
    candidates: [NEAR_DIAGONAL, FAR_SAME_ROW],
    direction: 'right',
    expected: 'row',
  },
  {
    name: 'a 1pt perpendicular overlap far away beats a closer diagonal (awkward)',
    current: TOP_LEFT,
    candidates: [NEAR_DIAGONAL, FAR_SLIVER_OVERLAP],
    direction: 'right',
    expected: 'sliver',
  },
  {
    name: 'gap penalty: a flat farther window beats a steep nearer one',
    current: TOP_LEFT,
    candidates: [STEEP_NEAR, FLAT_FARTHER],
    direction: 'right',
    expected: 'flat',
  },
  {
    name: 'wrap disabled returns null when nothing lies in the direction',
    current: TOP_LEFT,
    candidates: GRID,
    direction: 'left',
    expected: null,
    options: { wrap: false },
  },
  {
    name: 'wrap picks the farthest window on the same row',
    current: ROW[2].frame,
    candidates: ROW,
    direction: 'right',
    expected: 'x1',
    options: { wrap: true },
  },
  {
    name: 'wrap is ignored when a neighbour exists in the direction',
    current: ROW[1].frame,
    candidates: ROW,
    direction: 'right',
    expected: 'x3',
    options: { wrap: true },
  },
];

for (const testCase of NEIGHBOR_CASES) {
  test(`nearestWindowInDirection ${testCase.name}`, () => {
    assert.equal(
      nearestWindowInDirection(
        testCase.current,
        testCase.candidates,
        testCase.direction,
        testCase.options,
      ),
      testCase.expected,
    );
  });
}
