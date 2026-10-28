// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import type {
  Frame,
  NeighborDirection,
  NeighborOptions,
} from './window-management-types';
import {
  type NeighborWindow,
  nearestWindowInDirection,
} from './window-neighbors';

interface NeighborCase {
  name: string;
  origin: NeighborWindow;
  candidates: NeighborWindow[];
  direction: NeighborDirection;
  expected: string | null;
  options?: NeighborOptions;
}

function rect(x: number, y: number, width: number, height: number): Frame {
  return { x, y, width, height };
}

function windowAt(
  id: string,
  frame: Frame,
  overrides: Partial<NeighborWindow> = {},
): NeighborWindow {
  return {
    id,
    frame,
    zIndex: 0,
    displayId: 'main',
    isFullScreen: false,
    ...overrides,
  };
}

const TOP_LEFT = windowAt('tl', rect(0, 0, 100, 100));
const TOP_RIGHT = windowAt('tr', rect(100, 0, 100, 100));
const BOTTOM_LEFT = windowAt('bl', rect(0, 100, 100, 100));
const BOTTOM_RIGHT = windowAt('br', rect(100, 100, 100, 100));
const GRID = [TOP_LEFT, TOP_RIGHT, BOTTOM_LEFT, BOTTOM_RIGHT];

const LEFT_HALF = windowAt('left', rect(0, 0, 500, 900));
const RIGHT_HALF = windowAt('right', rect(500, 0, 500, 900));
const HALVES = [LEFT_HALF, RIGHT_HALF];

const STACK_TOP = windowAt('top', rect(0, 0, 800, 400));
const STACK_BOTTOM = windowAt('bottom', rect(0, 400, 800, 400));
const STACKED = [STACK_TOP, STACK_BOTTOM];

const CASCADE_FRONT = windowAt('w1', rect(0, 0, 600, 400));
const CASCADE_MIDDLE = windowAt('w2', rect(30, 30, 600, 400), { zIndex: 1 });
const CASCADE_BACK = windowAt('w3', rect(60, 60, 600, 400), { zIndex: 2 });

const ROW = [
  windowAt('x1', rect(0, 0, 100, 100)),
  windowAt('x2', rect(100, 0, 100, 100)),
  windowAt('x3', rect(200, 0, 100, 100)),
];

const MAIN_WINDOW = windowAt('origin', rect(100, 100, 800, 600));

const NEAR_DIAGONAL = windowAt('diagonal', rect(110, 150, 100, 100));
const FAR_SAME_ROW = windowAt('row', rect(500, 0, 100, 100));
const FAR_SLIVER_OVERLAP = windowAt('sliver', rect(500, 99, 100, 100));
const STEEP_NEAR = windowAt('steep', rect(120, 250, 100, 100));
const FLAT_FARTHER = windowAt('flat', rect(400, 100, 100, 100));
const DRIFTED_BELOW = windowAt('drifted', rect(1, 100, 100, 100));

const NESTED_ORIGIN = windowAt('big', rect(0, 0, 1000, 1000), { zIndex: 1 });
const CORNER_IN_FRONT = windowAt('corner', rect(0, 0, 20, 20));
const CORNER_BEHIND = windowAt('corner', rect(0, 0, 20, 20), { zIndex: 2 });
const BIG_BEHIND_CONTAINS_ORIGIN = windowAt(
  'big-behind',
  rect(300, 0, 1000, 1000),
  {
    zIndex: 1,
  },
);
const BIG_FRONT_CONTAINS_ORIGIN = windowAt(
  'big-front',
  rect(300, 0, 1000, 1000),
);
const ROW_FAR = windowAt('row-far', rect(1000, 100, 100, 100));

const NEIGHBOR_CASES: NeighborCase[] = [
  {
    name: 'grid: right from top-left picks top-right',
    origin: TOP_LEFT,
    candidates: GRID,
    direction: 'right',
    expected: 'tr',
  },
  {
    name: 'grid: down from top-left picks bottom-left',
    origin: TOP_LEFT,
    candidates: GRID,
    direction: 'down',
    expected: 'bl',
  },
  {
    name: 'grid: left from bottom-right picks bottom-left over the diagonal',
    origin: BOTTOM_RIGHT,
    candidates: GRID,
    direction: 'left',
    expected: 'bl',
  },
  {
    name: 'grid: up from bottom-right picks top-right',
    origin: BOTTOM_RIGHT,
    candidates: GRID,
    direction: 'up',
    expected: 'tr',
  },
  {
    name: 'grid: right from top-right has no neighbour without wrap',
    origin: TOP_RIGHT,
    candidates: GRID,
    direction: 'right',
    expected: null,
  },
  {
    name: 'grid: right from top-right wraps to the farthest aligned window',
    origin: TOP_RIGHT,
    candidates: GRID,
    direction: 'right',
    expected: 'tl',
    options: { wrap: true },
  },
  {
    name: 'grid: up from top-left wraps to bottom-left, not bottom-right',
    origin: TOP_LEFT,
    candidates: GRID,
    direction: 'up',
    expected: 'bl',
    options: { wrap: true },
  },
  {
    name: 'grid: a window one point off the row does not beat the true neighbour',
    origin: TOP_LEFT,
    candidates: [...GRID, DRIFTED_BELOW],
    direction: 'right',
    expected: 'tr',
  },
  {
    name: 'halves: right from left half selects right half',
    origin: LEFT_HALF,
    candidates: HALVES,
    direction: 'right',
    expected: 'right',
  },
  {
    name: 'halves: left from left half wraps to right half',
    origin: LEFT_HALF,
    candidates: HALVES,
    direction: 'left',
    expected: 'right',
    options: { wrap: true },
  },
  {
    name: 'stacked: down from top selects the window below',
    origin: STACK_TOP,
    candidates: STACKED,
    direction: 'down',
    expected: 'bottom',
  },
  {
    name: 'stacked: right ignores a window that only sits below',
    origin: STACK_TOP,
    candidates: STACKED,
    direction: 'right',
    expected: null,
  },
  {
    name: 'cascade: right from the front layer skips the layer behind that covers its centre',
    origin: CASCADE_FRONT,
    candidates: [CASCADE_MIDDLE, CASCADE_BACK],
    direction: 'right',
    expected: null,
  },
  {
    name: 'cascade: left from the back layer selects the nearer front layer',
    origin: CASCADE_BACK,
    candidates: [CASCADE_FRONT, CASCADE_MIDDLE],
    direction: 'left',
    expected: 'w2',
  },
  {
    name: 'nested: a small window in front of a big origin is a neighbour',
    origin: NESTED_ORIGIN,
    candidates: [CORNER_IN_FRONT],
    direction: 'left',
    expected: 'corner',
  },
  {
    name: 'nested: a small window hidden behind a big origin is skipped',
    origin: NESTED_ORIGIN,
    candidates: [CORNER_BEHIND],
    direction: 'left',
    expected: null,
  },
  {
    name: 'nested: a big window behind the origin that covers its centre is skipped',
    origin: MAIN_WINDOW,
    candidates: [BIG_BEHIND_CONTAINS_ORIGIN, ROW_FAR],
    direction: 'right',
    expected: 'row-far',
  },
  {
    name: 'nested: a big window in front of the origin that covers its centre stays a candidate',
    origin: { ...MAIN_WINDOW, zIndex: 1 },
    candidates: [BIG_FRONT_CONTAINS_ORIGIN, ROW_FAR],
    direction: 'right',
    expected: 'big-front',
  },
  {
    name: 'diagonal: a near diagonal window beats a far window on the same row',
    origin: TOP_LEFT,
    candidates: [NEAR_DIAGONAL, FAR_SAME_ROW],
    direction: 'right',
    expected: 'diagonal',
  },
  {
    name: 'diagonal: a near diagonal window beats a far window with a sliver of overlap',
    origin: TOP_LEFT,
    candidates: [NEAR_DIAGONAL, FAR_SLIVER_OVERLAP],
    direction: 'right',
    expected: 'diagonal',
  },
  {
    name: 'cone: a flatter farther window beats a steep nearer one',
    origin: TOP_LEFT,
    candidates: [STEEP_NEAR, FLAT_FARTHER],
    direction: 'right',
    expected: 'flat',
  },
  {
    name: 'wrap disabled returns null when nothing lies in the direction',
    origin: TOP_LEFT,
    candidates: GRID,
    direction: 'left',
    expected: null,
    options: { wrap: false },
  },
  {
    name: 'wrap picks the farthest window on the same row',
    origin: ROW[2],
    candidates: ROW,
    direction: 'right',
    expected: 'x1',
    options: { wrap: true },
  },
  {
    name: 'wrap is ignored when a neighbour exists in the direction',
    origin: ROW[1],
    candidates: ROW,
    direction: 'right',
    expected: 'x3',
    options: { wrap: true },
  },
  {
    name: 'lone window has no neighbour in any direction',
    origin: TOP_LEFT,
    candidates: [TOP_LEFT],
    direction: 'right',
    expected: null,
    options: { wrap: true },
  },
  {
    name: 'zero-area candidates are ignored',
    origin: TOP_LEFT,
    candidates: [windowAt('sliver', rect(150, 0, 0, 100))],
    direction: 'right',
    expected: null,
  },
  {
    name: 'zero-area origin has no neighbour',
    origin: windowAt('flat', rect(0, 0, 0, 100)),
    candidates: [TOP_RIGHT],
    direction: 'right',
    expected: null,
  },
  {
    name: 'equal scores tie-break on the front-most window before the smaller id',
    origin: TOP_LEFT,
    candidates: [
      windowAt('a', rect(100, 0, 100, 100), { zIndex: 1 }),
      windowAt('b', rect(100, 0, 100, 100), { zIndex: 0 }),
    ],
    direction: 'right',
    expected: 'b',
  },
  {
    name: 'equal scores with equal z tie-break on the smaller id',
    origin: TOP_LEFT,
    candidates: [
      windowAt('z', rect(100, 0, 100, 100)),
      windowAt('b', rect(100, 0, 100, 100)),
    ],
    direction: 'right',
    expected: 'b',
  },
];

for (const testCase of NEIGHBOR_CASES) {
  test(`nearestWindowInDirection ${testCase.name}`, () => {
    assert.equal(
      nearestWindowInDirection(
        testCase.origin,
        testCase.candidates,
        testCase.direction,
        testCase.options,
      ),
      testCase.expected,
    );
  });
}
