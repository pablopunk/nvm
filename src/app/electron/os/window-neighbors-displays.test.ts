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

const MAIN_WINDOW = windowAt('origin', rect(100, 100, 800, 600));
const MAIN_LEFT = windowAt('main-left', rect(0, 0, 50, 50));
const LEFT_NEAR = windowAt('left-near', rect(-200, 380, 100, 40), {
  displayId: 'left',
});
const LEFT_DISPLAY_WINDOW = windowAt(
  'left-display',
  rect(-1500, 200, 1000, 600),
  {
    displayId: 'left',
  },
);
const RIGHT_DISPLAY_WINDOW = windowAt(
  'right-display',
  rect(1500, 100, 1000, 600),
  {
    displayId: 'right',
  },
);
const ABOVE_WINDOW = windowAt('above', rect(100, -900, 800, 600), {
  displayId: 'above',
});

const FULL_SCREEN_ORIGIN = windowAt('full', rect(0, 0, 1440, 900), {
  isFullScreen: true,
});
const SAME_DISPLAY_SIDE = windowAt('same-side', rect(900, 0, 500, 900), {
  zIndex: 1,
});
const OTHER_DISPLAY_SIDE = windowAt('other', rect(1440, 0, 1440, 900), {
  displayId: 'right',
});

const TOP_LEFT = windowAt('tl', rect(0, 0, 100, 100));
const TOP_RIGHT = windowAt('tr', rect(100, 0, 100, 100));
const BOTTOM_LEFT = windowAt('bl', rect(0, 100, 100, 100));
const BOTTOM_RIGHT = windowAt('br', rect(100, 100, 100, 100));
const FAR_SAME_ROW = windowAt('row', rect(500, 0, 100, 100));

const STACK_MIDDLE = windowAt('mid', rect(0, 0, 100, 100), { zIndex: 1 });
const STACK_FRONT = windowAt('front', rect(0, 0, 100, 100));
const STACK_BACK = windowAt('back', rect(0, 0, 100, 100), { zIndex: 2 });
const STACK_FAR_BACK = windowAt('far-back', rect(0, 0, 100, 100), {
  zIndex: 3,
});

const DISPLAY_CASES: NeighborCase[] = [
  {
    name: 'second display: left reaches a window on the display to the left',
    origin: MAIN_WINDOW,
    candidates: [LEFT_DISPLAY_WINDOW],
    direction: 'left',
    expected: 'left-display',
  },
  {
    name: 'second display: the origin display is preferred over a closer window on another display',
    origin: MAIN_WINDOW,
    candidates: [LEFT_NEAR, MAIN_LEFT],
    direction: 'left',
    expected: 'main-left',
  },
  {
    name: 'second display: up reaches the window on the display above',
    origin: MAIN_WINDOW,
    candidates: [LEFT_DISPLAY_WINDOW, ABOVE_WINDOW],
    direction: 'up',
    expected: 'above',
  },
  {
    name: 'second display: down from the display above returns to the origin display',
    origin: ABOVE_WINDOW,
    candidates: [MAIN_WINDOW],
    direction: 'down',
    expected: 'origin',
  },
  {
    name: 'second display: a window on the right is not reached by left without wrap',
    origin: MAIN_WINDOW,
    candidates: [RIGHT_DISPLAY_WINDOW],
    direction: 'left',
    expected: null,
  },
  {
    name: 'second display: wrap reaches a window on the opposite display',
    origin: MAIN_WINDOW,
    candidates: [RIGHT_DISPLAY_WINDOW],
    direction: 'left',
    expected: 'right-display',
    options: { wrap: true },
  },
  {
    name: 'full-screen origin ignores windows on its own display',
    origin: FULL_SCREEN_ORIGIN,
    candidates: [SAME_DISPLAY_SIDE],
    direction: 'right',
    expected: null,
  },
  {
    name: 'full-screen origin reaches a window on another display',
    origin: FULL_SCREEN_ORIGIN,
    candidates: [SAME_DISPLAY_SIDE, OTHER_DISPLAY_SIDE],
    direction: 'right',
    expected: 'other',
  },
  {
    name: 'full-screen origin wrap still ignores windows on its own display',
    origin: FULL_SCREEN_ORIGIN,
    candidates: [windowAt('same-left', rect(0, 0, 200, 900), { zIndex: 1 })],
    direction: 'left',
    expected: null,
    options: { wrap: true },
  },
  {
    name: 'stacked: right from the middle of a stack selects the next window behind',
    origin: STACK_MIDDLE,
    candidates: [STACK_FRONT, STACK_BACK, STACK_FAR_BACK],
    direction: 'right',
    expected: 'back',
  },
  {
    name: 'stacked: left from the middle of a stack selects the window in front',
    origin: STACK_MIDDLE,
    candidates: [STACK_FRONT, STACK_BACK, STACK_FAR_BACK],
    direction: 'left',
    expected: 'front',
  },
  {
    name: 'stacked: a coincident window beats a farther geometric neighbour',
    origin: STACK_MIDDLE,
    candidates: [STACK_FRONT, STACK_BACK, FAR_SAME_ROW],
    direction: 'right',
    expected: 'back',
  },
  {
    name: 'stacked: a window whose centre is within two points counts as stacked',
    origin: TOP_LEFT,
    candidates: [
      windowAt('near-stack', rect(1, 1, 100, 100), { zIndex: 1 }),
      FAR_SAME_ROW,
    ],
    direction: 'right',
    expected: 'near-stack',
  },
  {
    name: 'stacked: a coincident stack elsewhere is entered at its front-most window',
    origin: TOP_LEFT,
    candidates: [
      windowAt('tr-back', rect(100, 0, 100, 100), { zIndex: 2 }),
      windowAt('tr-front', rect(100, 0, 100, 100), { zIndex: 1 }),
    ],
    direction: 'right',
    expected: 'tr-front',
  },
  {
    name: 'order: the answer does not depend on candidate order',
    origin: TOP_LEFT,
    candidates: [BOTTOM_RIGHT, BOTTOM_LEFT, TOP_RIGHT],
    direction: 'right',
    expected: 'tr',
  },
];

for (const testCase of DISPLAY_CASES) {
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
