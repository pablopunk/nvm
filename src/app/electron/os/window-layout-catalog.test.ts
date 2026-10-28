import assert from 'node:assert/strict';
import test from 'node:test';
import {
  WINDOW_LAYOUT_IDS,
  WINDOW_LAYOUT_SPECS,
  type WindowLayoutId,
} from './window-layout-catalog';
import { NO_GAPS } from './window-layout-gaps.fixture';
import {
  computeTargetFrame,
  isServiceHandledLayoutSpec,
} from './window-layouts';
import type { Frame, WindowLayoutSpec } from './window-management-types';

const EPSILON = 1e-9;
const tilingWorkAreas: Frame[] = [
  { x: 0, y: 0, width: 1441, height: 901 },
  { x: -1441, y: -901, width: 1441, height: 901 },
];
const probeFrame: Frame = { x: 100, y: 120, width: 800, height: 500 };
const probeWorkArea: Frame = { x: 0, y: 0, width: 1440, height: 900 };

const PERSISTED_LAYOUT_IDS = [
  'left-half',
  'right-half',
  'top-half',
  'bottom-half',
  'center-half',
  'maximize',
  'center',
  'first-third',
  'center-third',
  'last-third',
  'top-third',
  'middle-third',
  'bottom-third',
  'first-two-thirds',
  'center-two-thirds',
  'last-two-thirds',
  'top-two-thirds',
  'middle-two-thirds',
  'bottom-two-thirds',
  'top-left-quarter',
  'top-right-quarter',
  'bottom-left-quarter',
  'bottom-right-quarter',
  'first-fourth',
  'second-fourth',
  'third-fourth',
  'last-fourth',
  'top-left-sixth',
  'top-center-sixth',
  'top-right-sixth',
  'bottom-left-sixth',
  'bottom-center-sixth',
  'bottom-right-sixth',
  'maximize-width',
  'maximize-height',
  'almost-maximize',
  'reasonable-size',
  'move-left',
  'move-right',
  'move-up',
  'move-down',
  'make-larger',
  'make-smaller',
  'next-display',
  'previous-display',
  'restore',
  'toggle-full-screen',
];

function overlapArea(a: Frame, b: Frame): number {
  const width = Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x);
  const height = Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y);
  return width > 0 && height > 0 ? width * height : 0;
}

function isInside(frame: Frame, area: Frame): boolean {
  return (
    frame.x >= area.x &&
    frame.y >= area.y &&
    frame.x + frame.width <= area.x + area.width &&
    frame.y + frame.height <= area.y + area.height
  );
}

function tiledFramesFor(layoutIds: WindowLayoutId[], workArea: Frame): Frame[] {
  return layoutIds.map((layoutId) =>
    computeTargetFrame({
      spec: WINDOW_LAYOUT_SPECS[layoutId],
      workArea,
      currentFrame: probeFrame,
      gaps: NO_GAPS,
    }),
  );
}

function totalArea(frames: Frame[]): number {
  return frames.reduce((sum, frame) => sum + frame.width * frame.height, 0);
}

function totalPairwiseOverlap(frames: Frame[]): number {
  return frames
    .flatMap((frame, index) =>
      frames.slice(index + 1).map((other) => overlapArea(frame, other)),
    )
    .reduce((sum, area) => sum + area, 0);
}

const tilingGroups: Array<{ name: string; layoutIds: WindowLayoutId[] }> = [
  {
    name: 'first, center and last third',
    layoutIds: ['first-third', 'center-third', 'last-third'],
  },
  {
    name: 'top, middle and bottom third',
    layoutIds: ['top-third', 'middle-third', 'bottom-third'],
  },
  {
    name: 'four fourths',
    layoutIds: ['first-fourth', 'second-fourth', 'third-fourth', 'last-fourth'],
  },
  {
    name: 'six sixths',
    layoutIds: [
      'top-left-sixth',
      'top-center-sixth',
      'top-right-sixth',
      'bottom-left-sixth',
      'bottom-center-sixth',
      'bottom-right-sixth',
    ],
  },
  {
    name: 'four quarters',
    layoutIds: [
      'top-left-quarter',
      'top-right-quarter',
      'bottom-left-quarter',
      'bottom-right-quarter',
    ],
  },
];

test('persisted layout ids are unique and stable', () => {
  assert.equal(new Set(WINDOW_LAYOUT_IDS).size, WINDOW_LAYOUT_IDS.length);
  assert.deepEqual(WINDOW_LAYOUT_IDS, PERSISTED_LAYOUT_IDS);
});

test('every catalog spec is accepted by the engine or is service-handled', () => {
  for (const layoutId of WINDOW_LAYOUT_IDS) {
    const spec: WindowLayoutSpec = WINDOW_LAYOUT_SPECS[layoutId];
    if (isServiceHandledLayoutSpec(spec)) {
      assert.throws(() =>
        computeTargetFrame({
          spec,
          workArea: probeWorkArea,
          currentFrame: probeFrame,
          gaps: NO_GAPS,
        }),
      );
      continue;
    }
    const frame = computeTargetFrame({
      spec,
      workArea: probeWorkArea,
      currentFrame: probeFrame,
      gaps: NO_GAPS,
    });
    assert.ok(
      Number.isFinite(frame.x) && Number.isFinite(frame.width),
      layoutId,
    );
  }
});

test('service-handled catalog specs are exactly restore, display and full screen', () => {
  const serviceHandledIds = WINDOW_LAYOUT_IDS.filter((layoutId) =>
    isServiceHandledLayoutSpec(WINDOW_LAYOUT_SPECS[layoutId]),
  );
  assert.deepEqual(serviceHandledIds, [
    'next-display',
    'previous-display',
    'restore',
    'toggle-full-screen',
  ]);
});

test('every fraction spec lies inside the unit square', () => {
  for (const layoutId of WINDOW_LAYOUT_IDS) {
    const spec = WINDOW_LAYOUT_SPECS[layoutId];
    if (spec.kind !== 'fraction') {
      continue;
    }
    assert.ok(spec.x >= 0 && spec.x <= 1, `${layoutId} x`);
    assert.ok(spec.y >= 0 && spec.y <= 1, `${layoutId} y`);
    assert.ok(spec.width > 0 && spec.width <= 1, `${layoutId} width`);
    assert.ok(spec.height > 0 && spec.height <= 1, `${layoutId} height`);
    assert.ok(spec.x + spec.width <= 1 + EPSILON, `${layoutId} right edge`);
    assert.ok(spec.y + spec.height <= 1 + EPSILON, `${layoutId} bottom edge`);
  }
});

for (const group of tilingGroups) {
  for (const workArea of tilingWorkAreas) {
    test(`${group.name} tile a ${workArea.width}x${workArea.height} work area at ${workArea.x},${workArea.y} exactly`, () => {
      const frames = tiledFramesFor(group.layoutIds, workArea);
      assert.equal(totalArea(frames), workArea.width * workArea.height);
      assert.equal(totalPairwiseOverlap(frames), 0);
      assert.equal(
        frames.every((frame) => isInside(frame, workArea)),
        true,
      );
    });
  }
}
