import assert from 'node:assert/strict';
import test from 'node:test';
import { WINDOW_LAYOUT_SPECS } from './window-layout-catalog';
import {
  computeTargetFrame,
  isServiceHandledLayoutSpec,
} from './window-layouts';
import type { Frame, WindowLayoutSpec } from './window-management-types';

const primaryWorkArea: Frame = { x: 0, y: 0, width: 1440, height: 900 };
const windowFrame: Frame = { x: 100, y: 120, width: 800, height: 500 };
const centeredWindow: Frame = { x: 320, y: 200, width: 800, height: 500 };
const negativeWorkArea: Frame = { x: -1920, y: 0, width: 1920, height: 1080 };

interface EngineCase {
  name: string;
  spec: WindowLayoutSpec;
  expected: Frame;
  workArea?: Frame;
  gap?: number;
  currentFrame?: Frame;
}

function frameForCase({
  spec,
  workArea = primaryWorkArea,
  gap = 0,
  currentFrame = windowFrame,
}: EngineCase): Frame {
  return computeTargetFrame({ spec, workArea, currentFrame, gap });
}

const maximizeWidthCases: EngineCase[] = [
  {
    name: 'maximize-width keeps vertical position and height',
    spec: { kind: 'maximize-width' },
    expected: { x: 0, y: 120, width: 1440, height: 500 },
  },
  {
    name: 'maximize-width insets horizontally by the gap',
    spec: { kind: 'maximize-width' },
    gap: 10,
    expected: { x: 10, y: 120, width: 1420, height: 500 },
  },
  {
    name: 'maximize-width pulls an offscreen window back vertically',
    spec: { kind: 'maximize-width' },
    currentFrame: { x: 5000, y: 5000, width: 400, height: 300 },
    expected: { x: 0, y: 600, width: 1440, height: 300 },
  },
  {
    name: 'maximize-width clamps a window taller than the work area',
    spec: { kind: 'maximize-width' },
    currentFrame: { x: 0, y: -300, width: 400, height: 1200 },
    expected: { x: 0, y: 0, width: 1440, height: 900 },
  },
  {
    name: 'maximize-width on a negative-origin display with gap 10',
    spec: { kind: 'maximize-width' },
    workArea: negativeWorkArea,
    gap: 10,
    expected: { x: -1910, y: 120, width: 1900, height: 500 },
  },
];

const maximizeHeightCases: EngineCase[] = [
  {
    name: 'maximize-height keeps horizontal position and width',
    spec: { kind: 'maximize-height' },
    expected: { x: 100, y: 0, width: 800, height: 900 },
  },
  {
    name: 'maximize-height insets vertically by the gap',
    spec: { kind: 'maximize-height' },
    gap: 10,
    expected: { x: 100, y: 10, width: 800, height: 880 },
  },
  {
    name: 'maximize-height pulls an offscreen window back horizontally',
    spec: { kind: 'maximize-height' },
    currentFrame: { x: 5000, y: 5000, width: 400, height: 300 },
    expected: { x: 1040, y: 0, width: 400, height: 900 },
  },
];

const scaledCenterCases: EngineCase[] = [
  {
    name: 'reasonable-size caps a 3000x2000 display at 1025x900',
    spec: WINDOW_LAYOUT_SPECS['reasonable-size'],
    workArea: { x: 0, y: 0, width: 3000, height: 2000 },
    expected: { x: 988, y: 550, width: 1025, height: 900 },
  },
  {
    name: 'reasonable-size scales a 1440x900 display to 60 percent',
    spec: WINDOW_LAYOUT_SPECS['reasonable-size'],
    expected: { x: 288, y: 180, width: 864, height: 540 },
  },
  {
    name: 'almost-maximize centers 90 percent of the work area',
    spec: WINDOW_LAYOUT_SPECS['almost-maximize'],
    expected: { x: 72, y: 45, width: 1296, height: 810 },
  },
  {
    name: 'almost-maximize ignores the gap',
    spec: WINDOW_LAYOUT_SPECS['almost-maximize'],
    gap: 10,
    expected: { x: 72, y: 45, width: 1296, height: 810 },
  },
  {
    name: 'almost-maximize on a negative-origin display',
    spec: WINDOW_LAYOUT_SPECS['almost-maximize'],
    workArea: negativeWorkArea,
    expected: { x: -1824, y: 54, width: 1728, height: 972 },
  },
  {
    name: 'reasonable-size on a portrait display',
    spec: WINDOW_LAYOUT_SPECS['reasonable-size'],
    workArea: { x: -1080, y: 0, width: 1080, height: 1920 },
    expected: { x: -864, y: 510, width: 648, height: 900 },
  },
  {
    name: 'almost-maximize rounds an odd work area width',
    spec: WINDOW_LAYOUT_SPECS['almost-maximize'],
    workArea: { x: 0, y: 0, width: 1441, height: 900 },
    expected: { x: 72, y: 45, width: 1297, height: 810 },
  },
];

const edgeCases: EngineCase[] = [
  {
    name: 'move-left keeps size at gap 0',
    spec: WINDOW_LAYOUT_SPECS['move-left'],
    expected: { x: 0, y: 120, width: 800, height: 500 },
  },
  {
    name: 'move-left insets by the gap',
    spec: WINDOW_LAYOUT_SPECS['move-left'],
    gap: 10,
    expected: { x: 10, y: 120, width: 800, height: 500 },
  },
  {
    name: 'move-right at gap 0',
    spec: WINDOW_LAYOUT_SPECS['move-right'],
    expected: { x: 640, y: 120, width: 800, height: 500 },
  },
  {
    name: 'move-right insets by the gap',
    spec: WINDOW_LAYOUT_SPECS['move-right'],
    gap: 10,
    expected: { x: 630, y: 120, width: 800, height: 500 },
  },
  {
    name: 'move-up at gap 0',
    spec: WINDOW_LAYOUT_SPECS['move-up'],
    expected: { x: 100, y: 0, width: 800, height: 500 },
  },
  {
    name: 'move-up insets by the gap',
    spec: WINDOW_LAYOUT_SPECS['move-up'],
    gap: 10,
    expected: { x: 100, y: 10, width: 800, height: 500 },
  },
  {
    name: 'move-down at gap 0',
    spec: WINDOW_LAYOUT_SPECS['move-down'],
    expected: { x: 100, y: 400, width: 800, height: 500 },
  },
  {
    name: 'move-down insets by the gap',
    spec: WINDOW_LAYOUT_SPECS['move-down'],
    gap: 10,
    expected: { x: 100, y: 390, width: 800, height: 500 },
  },
  {
    name: 'move-left on a negative-origin display with gap 10',
    spec: WINDOW_LAYOUT_SPECS['move-left'],
    workArea: negativeWorkArea,
    gap: 10,
    expected: { x: -1910, y: 120, width: 800, height: 500 },
  },
  {
    name: 'move-right on a negative-origin display with gap 10',
    spec: WINDOW_LAYOUT_SPECS['move-right'],
    workArea: negativeWorkArea,
    gap: 10,
    expected: { x: -810, y: 120, width: 800, height: 500 },
  },
  {
    name: 'move-up on a display above the primary with gap 10',
    spec: WINDOW_LAYOUT_SPECS['move-up'],
    workArea: { x: 0, y: -1080, width: 1920, height: 1080 },
    gap: 10,
    expected: { x: 100, y: -1070, width: 800, height: 500 },
  },
  {
    name: 'move-down on a display above the primary with gap 10',
    spec: WINDOW_LAYOUT_SPECS['move-down'],
    workArea: { x: 0, y: -1080, width: 1920, height: 1080 },
    gap: 10,
    expected: { x: 100, y: -510, width: 800, height: 500 },
  },
  {
    name: 'move-right shrinks an oversized window to the work area',
    spec: WINDOW_LAYOUT_SPECS['move-right'],
    currentFrame: { x: 0, y: 0, width: 2000, height: 1200 },
    gap: 10,
    expected: { x: 0, y: 0, width: 1440, height: 900 },
  },
  {
    name: 'move-left pulls an offscreen window back vertically',
    spec: WINDOW_LAYOUT_SPECS['move-left'],
    currentFrame: { x: 5000, y: 5000, width: 400, height: 300 },
    expected: { x: 0, y: 600, width: 400, height: 300 },
  },
  {
    name: 'move-up pulls an offscreen window back horizontally',
    spec: WINDOW_LAYOUT_SPECS['move-up'],
    currentFrame: { x: 5000, y: 5000, width: 400, height: 300 },
    expected: { x: 1040, y: 0, width: 400, height: 300 },
  },
];

const resizeStepCases: EngineCase[] = [
  {
    name: 'make-larger grows 5 percent of the work area around the center',
    spec: WINDOW_LAYOUT_SPECS['make-larger'],
    currentFrame: centeredWindow,
    expected: { x: 284, y: 178, width: 872, height: 545 },
  },
  {
    name: 'larger step 0.1 keeps the center',
    spec: { kind: 'resize-step', direction: 'larger', step: 0.1 },
    currentFrame: centeredWindow,
    expected: { x: 248, y: 155, width: 944, height: 590 },
  },
  {
    name: 'smaller step 0.1 keeps the center',
    spec: { kind: 'resize-step', direction: 'smaller', step: 0.1 },
    currentFrame: centeredWindow,
    expected: { x: 392, y: 245, width: 656, height: 410 },
  },
  {
    name: 'larger clamps to the work area',
    spec: { kind: 'resize-step', direction: 'larger', step: 0.1 },
    currentFrame: { x: 0, y: 0, width: 1400, height: 880 },
    expected: { x: 0, y: 0, width: 1440, height: 900 },
  },
  {
    name: 'larger shifts a grown window back inside the work area',
    spec: { kind: 'resize-step', direction: 'larger', step: 0.1 },
    currentFrame: { x: 1300, y: 700, width: 100, height: 100 },
    expected: { x: 1196, y: 655, width: 244, height: 190 },
  },
  {
    name: 'smaller never goes below 100 points',
    spec: { kind: 'resize-step', direction: 'smaller', step: 0.1 },
    currentFrame: { x: 500, y: 300, width: 120, height: 120 },
    expected: { x: 510, y: 310, width: 100, height: 100 },
  },
  {
    name: 'smaller never grows an already small window',
    spec: { kind: 'resize-step', direction: 'smaller', step: 0.1 },
    currentFrame: { x: 500, y: 300, width: 80, height: 80 },
    expected: { x: 500, y: 300, width: 80, height: 80 },
  },
  {
    name: 'larger on a negative-origin display',
    spec: WINDOW_LAYOUT_SPECS['make-larger'],
    workArea: negativeWorkArea,
    currentFrame: { x: -1500, y: 300, width: 600, height: 400 },
    expected: { x: -1548, y: 273, width: 696, height: 454 },
  },
];

const engineCases: EngineCase[] = [
  ...maximizeWidthCases,
  ...maximizeHeightCases,
  ...scaledCenterCases,
  ...edgeCases,
  ...resizeStepCases,
];

for (const engineCase of engineCases) {
  test(engineCase.name, () => {
    assert.deepEqual(frameForCase(engineCase), engineCase.expected);
  });
}

const serviceHandledSpecs: WindowLayoutSpec[] = [
  { kind: 'restore' },
  { kind: 'display', direction: 'next' },
  { kind: 'display', direction: 'previous' },
  { kind: 'toggle-full-screen' },
];

for (const spec of serviceHandledSpecs) {
  test(`${spec.kind} is rejected by the engine as service-handled`, () => {
    assert.equal(isServiceHandledLayoutSpec(spec), true);
    assert.throws(
      () =>
        computeTargetFrame({
          spec,
          workArea: primaryWorkArea,
          currentFrame: windowFrame,
          gap: 0,
        }),
      {
        message: `${spec.kind} is handled by the window management service`,
      },
    );
  });
}

test('geometry kinds are not service-handled', () => {
  assert.equal(isServiceHandledLayoutSpec({ kind: 'center' }), false);
  assert.equal(
    isServiceHandledLayoutSpec({ kind: 'edge', edge: 'left' }),
    false,
  );
});

test('fraction cycleGroup does not change the computed frame', () => {
  const withGroup = computeTargetFrame({
    spec: {
      kind: 'fraction',
      x: 0,
      y: 0,
      width: 0.5,
      height: 1,
      cycleGroup: 'left',
    },
    workArea: primaryWorkArea,
    currentFrame: windowFrame,
    gap: 0,
  });
  assert.deepEqual(withGroup, { x: 0, y: 0, width: 720, height: 900 });
});
