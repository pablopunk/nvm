// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import { NO_GAPS, uniformGaps } from './window-layout-gaps.fixture';
import { computeTargetFrame } from './window-layouts';
import type {
  Frame,
  WindowDimension,
  WindowLayoutSpec,
} from './window-management-types';

type ExactSpec = Extract<WindowLayoutSpec, { kind: 'exact' }>;

const MENU_BAR_WORK_AREA: Frame = { x: 0, y: 25, width: 1440, height: 875 };
const CURRENT_FRAME: Frame = { x: 100, y: 120, width: 800, height: 500 };
const NEGATIVE_ORIGIN_WORK_AREA: Frame = {
  x: -1920,
  y: 0,
  width: 1920,
  height: 1080,
};
const ABOVE_PRIMARY_WORK_AREA: Frame = {
  x: 0,
  y: -1080,
  width: 1920,
  height: 1080,
};
const PORTRAIT_WORK_AREA: Frame = { x: 0, y: 0, width: 1080, height: 1920 };
const ODD_WORK_AREA: Frame = { x: 0, y: 25, width: 1441, height: 875 };

function points(value: number): WindowDimension {
  return { value, unit: 'points' };
}

function percent(value: number): WindowDimension {
  return { value, unit: 'percent' };
}

interface ExactCase {
  name: string;
  spec: ExactSpec;
  expected: Frame;
  workArea?: Frame;
  currentFrame?: Frame;
  gap?: number;
}

const exactCases: ExactCase[] = [
  {
    name: 'absolute points place and size the window',
    spec: {
      kind: 'exact',
      x: points(100),
      y: points(50),
      width: points(400),
      height: points(300),
    },
    expected: { x: 100, y: 75, width: 400, height: 300 },
  },
  {
    name: 'percent sizes scale to the work area and round to whole points',
    spec: { kind: 'exact', width: percent(50), height: percent(50) },
    expected: { x: 100, y: 120, width: 720, height: 438 },
  },
  {
    name: 'percent offsets are measured from the work area origin',
    spec: { kind: 'exact', x: percent(10), y: percent(20) },
    expected: { x: 144, y: 200, width: 800, height: 500 },
  },
  {
    name: 'only x and y move the window and keep its size',
    spec: { kind: 'exact', x: points(0), y: points(0) },
    expected: { x: 0, y: 25, width: 800, height: 500 },
  },
  {
    name: 'only width and height resize the window and keep its position',
    spec: { kind: 'exact', width: points(1000), height: points(300) },
    expected: { x: 100, y: 120, width: 1000, height: 300 },
  },
  {
    name: 'a partial x and width keeps the current height and y',
    spec: { kind: 'exact', x: points(500), width: points(300) },
    expected: { x: 500, y: 120, width: 300, height: 500 },
  },
  {
    name: 'a position past the right edge clamps the window inside the work area',
    spec: {
      kind: 'exact',
      x: points(1300),
      width: points(400),
      height: points(300),
    },
    expected: { x: 1040, y: 120, width: 400, height: 300 },
  },
  {
    name: 'a kept position below the work area clamps the window back inside',
    spec: { kind: 'exact', width: points(400) },
    currentFrame: { x: 100, y: 2000, width: 800, height: 500 },
    expected: { x: 100, y: 400, width: 400, height: 500 },
  },
  {
    name: 'a width larger than the work area shrinks to fit before clamping',
    spec: { kind: 'exact', width: points(2000) },
    expected: { x: 0, y: 120, width: 1440, height: 500 },
  },
  {
    name: 'a height larger than the work area shrinks to fit and pins to the top',
    spec: { kind: 'exact', height: points(5000) },
    expected: { x: 100, y: 25, width: 800, height: 875 },
  },
  {
    name: 'a negative-origin work area places offsets from its own origin',
    spec: {
      kind: 'exact',
      x: percent(10),
      y: points(0),
      width: percent(50),
      height: percent(100),
    },
    workArea: NEGATIVE_ORIGIN_WORK_AREA,
    expected: { x: -1728, y: 0, width: 960, height: 1080 },
  },
  {
    name: 'an above-primary work area places offsets from its own origin',
    spec: { kind: 'exact', y: points(0), height: percent(50) },
    workArea: ABOVE_PRIMARY_WORK_AREA,
    expected: { x: 100, y: -1080, width: 800, height: 540 },
  },
  {
    name: 'a portrait work area scales each axis by its own extent',
    spec: { kind: 'exact', width: percent(100), height: percent(50) },
    workArea: PORTRAIT_WORK_AREA,
    expected: { x: 0, y: 120, width: 1080, height: 960 },
  },
  {
    name: 'odd work area sizes round each resolved dimension',
    spec: { kind: 'exact', width: percent(33), height: percent(33.3) },
    workArea: ODD_WORK_AREA,
    expected: { x: 100, y: 120, width: 476, height: 291 },
  },
  {
    name: 'a sub-point percent size rounds up to one point',
    spec: { kind: 'exact', width: percent(0.001) },
    expected: { x: 100, y: 120, width: 1, height: 500 },
  },
  {
    name: 'fractional point offsets round to whole points',
    spec: { kind: 'exact', x: points(10.4), y: points(10.6) },
    expected: { x: 10, y: 36, width: 800, height: 500 },
  },
  {
    name: 'the gap does not inset an exact frame',
    spec: {
      kind: 'exact',
      x: points(0),
      y: points(0),
      width: points(720),
      height: points(875),
    },
    gap: 40,
    expected: { x: 0, y: 25, width: 720, height: 875 },
  },
];

for (const exactCase of exactCases) {
  test(`exact layout: ${exactCase.name}`, () => {
    const frame = computeTargetFrame({
      spec: exactCase.spec,
      workArea: exactCase.workArea ?? MENU_BAR_WORK_AREA,
      currentFrame: exactCase.currentFrame ?? CURRENT_FRAME,
      gaps: uniformGaps(exactCase.gap ?? 0),
    });
    assert.deepEqual(frame, exactCase.expected);
  });
}

test('exact layout with the same work area and frame keeps the frame', () => {
  const frame = computeTargetFrame({
    spec: {
      kind: 'exact',
      x: points(0),
      y: points(0),
      width: points(800),
      height: points(500),
    },
    workArea: MENU_BAR_WORK_AREA,
    currentFrame: { x: 0, y: 25, width: 800, height: 500 },
    gaps: NO_GAPS,
  });
  assert.deepEqual(frame, { x: 0, y: 25, width: 800, height: 500 });
});
