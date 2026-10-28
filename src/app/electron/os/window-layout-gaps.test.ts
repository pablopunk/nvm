import assert from 'node:assert/strict';
import test from 'node:test';
import {
  WINDOW_LAYOUT_SPECS,
  type WindowLayoutId,
} from './window-layout-catalog';
import { NO_GAPS, uniformGaps } from './window-layout-gaps.fixture';
import { computeTargetFrame } from './window-layouts';
import type {
  Frame,
  WindowEdgeGaps,
  WindowGaps,
  WindowLayoutSpec,
} from './window-management-types';

const WORK_AREA: Frame = { x: 0, y: 0, width: 1440, height: 900 };
const CURRENT_FRAME: Frame = { x: 100, y: 120, width: 800, height: 500 };

function gapsWith(
  outer: WindowEdgeGaps,
  inner: number,
  applyToMaximize = true,
): WindowGaps {
  return { outer, inner, applyToMaximize };
}

function frameFor(
  spec: WindowLayoutSpec,
  gaps: WindowGaps,
  workArea: Frame = WORK_AREA,
  currentFrame: Frame = CURRENT_FRAME,
): Frame {
  return computeTargetFrame({ spec, workArea, currentFrame, gaps });
}

function layoutFrame(
  layoutId: WindowLayoutId,
  gaps: WindowGaps,
  workArea: Frame = WORK_AREA,
): Frame {
  return frameFor(WINDOW_LAYOUT_SPECS[layoutId], gaps, workArea);
}

const OUTER_8: WindowEdgeGaps = { top: 8, right: 8, bottom: 8, left: 8 };
const PER_EDGE_OUTER: WindowEdgeGaps = {
  top: 10,
  right: 20,
  bottom: 30,
  left: 40,
};

test('outer 8 and inner 4 leave 8px at the display edges and exactly 4px between halves', () => {
  const gaps = gapsWith(OUTER_8, 4);
  assert.deepEqual(layoutFrame('left-half', gaps), {
    x: 8,
    y: 8,
    width: 710,
    height: 884,
  });
  assert.deepEqual(layoutFrame('right-half', gaps), {
    x: 722,
    y: 8,
    width: 710,
    height: 884,
  });
});

test('per-edge outer gaps inset each display edge by its own value', () => {
  const gaps = gapsWith(PER_EDGE_OUTER, 6);
  assert.deepEqual(layoutFrame('left-half', gaps), {
    x: 40,
    y: 10,
    width: 677,
    height: 860,
  });
  assert.deepEqual(layoutFrame('right-half', gaps), {
    x: 723,
    y: 10,
    width: 697,
    height: 860,
  });
});

test('quarters split the inner gap on both axes and keep outer gaps on the display corners', () => {
  const gaps = gapsWith(OUTER_8, 4);
  assert.deepEqual(layoutFrame('top-left-quarter', gaps), {
    x: 8,
    y: 8,
    width: 710,
    height: 440,
  });
  assert.deepEqual(layoutFrame('bottom-right-quarter', gaps), {
    x: 722,
    y: 452,
    width: 710,
    height: 440,
  });
});

test('first and last thirds keep the inner gap to the center third', () => {
  const gaps = gapsWith(OUTER_8, 4);
  const first = layoutFrame('first-third', gaps);
  const center = layoutFrame('center-third', gaps);
  const last = layoutFrame('last-third', gaps);
  assert.deepEqual(first, { x: 8, y: 8, width: 470, height: 884 });
  assert.deepEqual(last, { x: 962, y: 8, width: 470, height: 884 });
  assert.equal(first.x + first.width + 4, center.x);
  assert.equal(center.x + center.width + 4, last.x);
});

test('top and bottom halves use the vertical outer gaps and the inner gap between them', () => {
  const gaps = gapsWith(PER_EDGE_OUTER, 6);
  assert.deepEqual(layoutFrame('top-half', gaps), {
    x: 40,
    y: 10,
    width: 1380,
    height: 437,
  });
  assert.deepEqual(layoutFrame('bottom-half', gaps), {
    x: 40,
    y: 453,
    width: 1380,
    height: 417,
  });
});

test('two-thirds tiles touch the outer gap on their display edge only', () => {
  const gaps = gapsWith(OUTER_8, 4);
  assert.deepEqual(layoutFrame('top-two-thirds', gaps), {
    x: 8,
    y: 8,
    width: 1424,
    height: 590,
  });
});

test('maximize applies every outer gap when applyToMaximize is true', () => {
  assert.deepEqual(layoutFrame('maximize', gapsWith(PER_EDGE_OUTER, 6)), {
    x: 40,
    y: 10,
    width: 1380,
    height: 860,
  });
});

test('maximize has no gaps when applyToMaximize is false', () => {
  assert.deepEqual(layoutFrame('maximize', gapsWith(OUTER_8, 4, false)), {
    x: 0,
    y: 0,
    width: 1440,
    height: 900,
  });
});

test('halves keep their outer gaps when applyToMaximize is false', () => {
  assert.deepEqual(layoutFrame('left-half', gapsWith(OUTER_8, 4, false)), {
    x: 8,
    y: 8,
    width: 710,
    height: 884,
  });
});

test('maximize-width uses the left and right outer gaps and keeps vertical position', () => {
  assert.deepEqual(
    frameFor({ kind: 'maximize-width' }, gapsWith(PER_EDGE_OUTER, 6)),
    {
      x: 40,
      y: 120,
      width: 1380,
      height: 500,
    },
  );
});

test('maximize-width has no horizontal gap when applyToMaximize is false', () => {
  assert.deepEqual(
    frameFor({ kind: 'maximize-width' }, gapsWith(OUTER_8, 4, false)),
    { x: 0, y: 120, width: 1440, height: 500 },
  );
});

test('maximize-height uses the top and bottom outer gaps and keeps horizontal position', () => {
  assert.deepEqual(
    frameFor({ kind: 'maximize-height' }, gapsWith(PER_EDGE_OUTER, 6)),
    {
      x: 100,
      y: 10,
      width: 800,
      height: 860,
    },
  );
});

test('maximize-height has no vertical gap when applyToMaximize is false', () => {
  assert.deepEqual(
    frameFor({ kind: 'maximize-height' }, gapsWith(OUTER_8, 4, false)),
    { x: 100, y: 0, width: 800, height: 900 },
  );
});

test('edge moves use the outer gap of the edge they move to', () => {
  const gaps = gapsWith(PER_EDGE_OUTER, 6);
  assert.deepEqual(frameFor({ kind: 'edge', edge: 'left' }, gaps), {
    x: 40,
    y: 120,
    width: 800,
    height: 500,
  });
  assert.deepEqual(frameFor({ kind: 'edge', edge: 'right' }, gaps), {
    x: 620,
    y: 120,
    width: 800,
    height: 500,
  });
  assert.deepEqual(frameFor({ kind: 'edge', edge: 'top' }, gaps), {
    x: 100,
    y: 10,
    width: 800,
    height: 500,
  });
  assert.deepEqual(frameFor({ kind: 'edge', edge: 'bottom' }, gaps), {
    x: 100,
    y: 370,
    width: 800,
    height: 500,
  });
});

test('gaps larger than a tiny work area clamp the tile to zero width inside the area', () => {
  const tinyArea: Frame = { x: 0, y: 0, width: 20, height: 20 };
  const frame = layoutFrame('left-half', uniformGaps(40), tinyArea);
  assert.equal(frame.width, 0);
  assert.ok(frame.x >= tinyArea.x && frame.x <= tinyArea.x + tinyArea.width);
  assert.ok(frame.y >= tinyArea.y && frame.y <= tinyArea.y + tinyArea.height);
});

const ODD_WORK_AREA: Frame = { x: 0, y: 0, width: 1441, height: 900 };
const ODD_INNER_GAPS = [0, 1, 4, 5, 11];
const ODD_OUTER_GAPS: WindowEdgeGaps[] = [
  { top: 0, right: 0, bottom: 0, left: 0 },
  { top: 8, right: 8, bottom: 8, left: 8 },
  { top: 3, right: 7, bottom: 5, left: 9 },
];

for (const outer of ODD_OUTER_GAPS) {
  for (const inner of ODD_INNER_GAPS) {
    test(`odd-width halves with outer ${outer.left}/${outer.right} and inner ${inner} keep exactly ${inner}px between them`, () => {
      const gaps = gapsWith(outer, inner);
      const left = layoutFrame('left-half', gaps, ODD_WORK_AREA);
      const right = layoutFrame('right-half', gaps, ODD_WORK_AREA);
      assert.equal(left.x, outer.left);
      assert.equal(right.x + right.width, ODD_WORK_AREA.width - outer.right);
      assert.equal(left.x + left.width + inner, right.x);
    });
  }
}

test('per-edge gaps on a negative-origin display keep the outer gaps on the display edges', () => {
  const negativeArea: Frame = { x: -1920, y: 0, width: 1920, height: 1080 };
  const gaps = gapsWith(PER_EDGE_OUTER, 6);
  assert.deepEqual(layoutFrame('right-half', gaps, negativeArea), {
    x: -957,
    y: 10,
    width: 937,
    height: 1040,
  });
  assert.equal(
    layoutFrame('left-half', gaps, negativeArea).x,
    negativeArea.x + PER_EDGE_OUTER.left,
  );
});

test('zero gaps reproduce the abutting halves of the plain work area', () => {
  assert.deepEqual(layoutFrame('left-half', NO_GAPS), {
    x: 0,
    y: 0,
    width: 720,
    height: 900,
  });
});
