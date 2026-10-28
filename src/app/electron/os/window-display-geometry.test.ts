import assert from 'node:assert/strict';
import test from 'node:test';
import {
  adjacentDisplay,
  cycleSequence,
  moveFrameToDisplay,
} from './window-display-geometry';
import type { FractionLayoutSpec } from './window-layouts';
import type {
  DisplayInfo,
  Frame,
  WindowCycleGroup,
  WindowDisplayMoveSizing,
  WindowDisplayOrder,
} from './window-management-types';

function displayAt(id: string, frame: Frame, isPrimary = false): DisplayInfo {
  return { id, isPrimary, frame, workArea: frame };
}

const primary = displayAt(
  'primary',
  { x: 0, y: 0, width: 1440, height: 900 },
  true,
);
const right = displayAt('right', { x: 1440, y: 0, width: 1920, height: 1080 });
const left = displayAt('left', { x: -1920, y: 0, width: 1920, height: 1080 });
const above = displayAt('above', { x: 0, y: -1080, width: 1920, height: 1080 });
const fullHd = displayAt('full-hd', { x: 0, y: 0, width: 1920, height: 1080 });
const smallLeft = displayAt('small-left', {
  x: -1440,
  y: 0,
  width: 1440,
  height: 900,
});

const adjacentCases: Array<{
  name: string;
  displays: DisplayInfo[];
  currentDisplayId: string;
  direction: 'next' | 'previous';
  order?: WindowDisplayOrder;
  expectedId: string | null;
}> = [
  {
    name: 'single display has no next neighbour',
    displays: [primary],
    currentDisplayId: 'primary',
    direction: 'next',
    expectedId: null,
  },
  {
    name: 'single display has no previous neighbour',
    displays: [primary],
    currentDisplayId: 'primary',
    direction: 'previous',
    expectedId: null,
  },
  {
    name: 'two displays: next from primary',
    displays: [primary, right],
    currentDisplayId: 'primary',
    direction: 'next',
    expectedId: 'right',
  },
  {
    name: 'two displays: next wraps from the rightmost',
    displays: [primary, right],
    currentDisplayId: 'right',
    direction: 'next',
    expectedId: 'primary',
  },
  {
    name: 'two displays: previous from primary',
    displays: [primary, right],
    currentDisplayId: 'primary',
    direction: 'previous',
    expectedId: 'right',
  },
  {
    name: 'three displays in shuffled input: next wraps from the rightmost to the leftmost',
    displays: [right, primary, left],
    currentDisplayId: 'right',
    direction: 'next',
    expectedId: 'left',
  },
  {
    name: 'three displays: previous wraps from the leftmost to the rightmost',
    displays: [right, primary, left],
    currentDisplayId: 'left',
    direction: 'previous',
    expectedId: 'right',
  },
  {
    name: 'three displays: next from the leftmost reaches the primary',
    displays: [primary, left, right],
    currentDisplayId: 'left',
    direction: 'next',
    expectedId: 'primary',
  },
  {
    name: 'ties on x are ordered by y: next from the display above wraps to primary',
    displays: [primary, right, above],
    currentDisplayId: 'above',
    direction: 'next',
    expectedId: 'primary',
  },
  {
    name: 'ties on x are ordered by y: previous from the display above wraps to the rightmost',
    displays: [right, primary, above],
    currentDisplayId: 'above',
    direction: 'previous',
    expectedId: 'right',
  },
  {
    name: 'ties on x are ordered by y: next from primary reaches the rightmost',
    displays: [above, right, primary],
    currentDisplayId: 'primary',
    direction: 'next',
    expectedId: 'right',
  },
  {
    name: 'unknown current display has no neighbour',
    displays: [primary, right],
    currentDisplayId: 'missing',
    direction: 'next',
    expectedId: null,
  },
];

function displayNamed(displays: DisplayInfo[], id: string): DisplayInfo {
  return (
    displays.find((display) => display.id === id) ??
    displayAt(id, primary.frame)
  );
}

for (const adjacentCase of adjacentCases) {
  test(adjacentCase.name, () => {
    const result = adjacentDisplay(
      adjacentCase.displays,
      displayNamed(adjacentCase.displays, adjacentCase.currentDisplayId),
      adjacentCase.direction,
      adjacentCase.order ?? 'left-to-right',
    );
    assert.equal(result?.id ?? null, adjacentCase.expectedId);
  });
}

const stackedDisplays = [primary, above, left, right];

test('previous display from primary is the display above it when ordered left-to-right', () => {
  assert.equal(
    adjacentDisplay(stackedDisplays, primary, 'previous', 'left-to-right')?.id,
    'above',
  );
});

test('previous display from primary is the display to its left when ordered top-to-bottom', () => {
  assert.equal(
    adjacentDisplay(stackedDisplays, primary, 'previous', 'top-to-bottom')?.id,
    'left',
  );
});

test('next display from the rightmost display wraps to the top when ordered top-to-bottom', () => {
  assert.equal(
    adjacentDisplay(stackedDisplays, right, 'next', 'top-to-bottom')?.id,
    'above',
  );
});

test('adjacent display does not reorder the caller array', () => {
  const displays = [right, primary, left];
  adjacentDisplay(displays, primary, 'next', 'left-to-right');
  assert.deepEqual(
    displays.map((display) => display.id),
    ['right', 'primary', 'left'],
  );
});

const moveCases: Array<{
  name: string;
  frame: Frame;
  from: DisplayInfo;
  to: DisplayInfo;
  sizing?: WindowDisplayMoveSizing;
  expected: Frame;
}> = [
  {
    name: 'left half moves to a larger display on the left keeping half size',
    frame: { x: 0, y: 0, width: 720, height: 900 },
    from: primary,
    to: left,
    expected: { x: -1920, y: 0, width: 960, height: 1080 },
  },
  {
    name: 'window right of center scales proportionally to a larger display on the right',
    frame: { x: 1000, y: 100, width: 400, height: 300 },
    from: primary,
    to: right,
    expected: { x: 2773, y: 120, width: 533, height: 360 },
  },
  {
    name: 'window moves to a smaller display and clamps fully inside it',
    frame: { x: 1700, y: 700, width: 400, height: 300 },
    from: fullHd,
    to: smallLeft,
    expected: { x: -300, y: 583, width: 300, height: 250 },
  },
  {
    name: 'window on a negative-origin display scales back to the primary',
    frame: { x: -1500, y: 100, width: 600, height: 400 },
    from: left,
    to: primary,
    expected: { x: 315, y: 83, width: 450, height: 333 },
  },
  {
    name: 'window moves to a display above the primary with a larger resolution',
    frame: { x: 100, y: 120, width: 800, height: 500 },
    from: primary,
    to: above,
    expected: { x: 133, y: -936, width: 1067, height: 600 },
  },
  {
    name: 'window larger than the target display is capped to its work area',
    frame: { x: 0, y: 0, width: 2000, height: 1200 },
    from: fullHd,
    to: displayAt('small', { x: 0, y: 0, width: 1000, height: 800 }),
    expected: { x: 0, y: 0, width: 1000, height: 800 },
  },
  {
    name: 'moving to the same display keeps the frame',
    frame: { x: 100, y: 120, width: 800, height: 500 },
    from: primary,
    to: primary,
    expected: { x: 100, y: 120, width: 800, height: 500 },
  },
];

for (const moveCase of moveCases) {
  test(moveCase.name, () => {
    assert.deepEqual(
      moveFrameToDisplay({
        frame: moveCase.frame,
        from: moveCase.from,
        to: moveCase.to,
        sizing: moveCase.sizing ?? 'proportional',
      }),
      moveCase.expected,
    );
  });
}

const keepSizeMoveCases: typeof moveCases = [
  {
    name: 'keep-size keeps the window size and its relative center on a larger display',
    frame: { x: 0, y: 0, width: 720, height: 900 },
    from: primary,
    to: right,
    sizing: 'keep-size',
    expected: { x: 1560, y: 90, width: 720, height: 900 },
  },
  {
    name: 'keep-size caps the window to a smaller display and clamps it inside',
    frame: { x: 0, y: 0, width: 1500, height: 1000 },
    from: fullHd,
    to: primary,
    sizing: 'keep-size',
    expected: { x: 0, y: 0, width: 1440, height: 900 },
  },
];

for (const moveCase of keepSizeMoveCases) {
  test(moveCase.name, () => {
    assert.deepEqual(
      moveFrameToDisplay({
        frame: moveCase.frame,
        from: moveCase.from,
        to: moveCase.to,
        sizing: moveCase.sizing ?? 'proportional',
      }),
      moveCase.expected,
    );
  });
}

const TILE_COUNT = 3;
const HALF = 1 / 2;
const THIRD = 1 / TILE_COUNT;
const TWO_THIRDS = 2 / TILE_COUNT;

const cycleCases: Array<{
  group: WindowCycleGroup;
  expected: FractionLayoutSpec[];
}> = [
  {
    group: 'left',
    expected: [
      { kind: 'fraction', x: 0, y: 0, width: HALF, height: 1 },
      { kind: 'fraction', x: 0, y: 0, width: TWO_THIRDS, height: 1 },
      { kind: 'fraction', x: 0, y: 0, width: THIRD, height: 1 },
    ],
  },
  {
    group: 'right',
    expected: [
      { kind: 'fraction', x: HALF, y: 0, width: HALF, height: 1 },
      { kind: 'fraction', x: THIRD, y: 0, width: TWO_THIRDS, height: 1 },
      { kind: 'fraction', x: TWO_THIRDS, y: 0, width: THIRD, height: 1 },
    ],
  },
  {
    group: 'top',
    expected: [
      { kind: 'fraction', x: 0, y: 0, width: 1, height: HALF },
      { kind: 'fraction', x: 0, y: 0, width: 1, height: TWO_THIRDS },
      { kind: 'fraction', x: 0, y: 0, width: 1, height: THIRD },
    ],
  },
  {
    group: 'bottom',
    expected: [
      { kind: 'fraction', x: 0, y: HALF, width: 1, height: HALF },
      { kind: 'fraction', x: 0, y: THIRD, width: 1, height: TWO_THIRDS },
      { kind: 'fraction', x: 0, y: TWO_THIRDS, width: 1, height: THIRD },
    ],
  },
];

for (const cycleCase of cycleCases) {
  test(`${cycleCase.group} cycle runs half, two-thirds, third without cycleGroup`, () => {
    assert.deepEqual(
      cycleSequence(cycleCase.group, 'half-two-thirds-third'),
      cycleCase.expected,
    );
  });
}

test('left cycle runs half, third, two-thirds when the third step comes first', () => {
  assert.deepEqual(cycleSequence('left', 'half-third-two-thirds'), [
    { kind: 'fraction', x: 0, y: 0, width: HALF, height: 1 },
    { kind: 'fraction', x: 0, y: 0, width: THIRD, height: 1 },
    { kind: 'fraction', x: 0, y: 0, width: TWO_THIRDS, height: 1 },
  ]);
});

test('bottom cycle runs half, third, two-thirds when the third step comes first', () => {
  assert.deepEqual(cycleSequence('bottom', 'half-third-two-thirds'), [
    { kind: 'fraction', x: 0, y: HALF, width: 1, height: HALF },
    { kind: 'fraction', x: 0, y: TWO_THIRDS, width: 1, height: THIRD },
    { kind: 'fraction', x: 0, y: THIRD, width: 1, height: TWO_THIRDS },
  ]);
});
