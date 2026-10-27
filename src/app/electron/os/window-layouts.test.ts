import assert from 'node:assert/strict';
import test from 'node:test';
import {
  computeTargetFrame,
  LAYOUT_SPECS,
  pickDisplayForFrame,
} from './window-layouts';
import type { DisplayInfo, Frame } from './window-management-types';

const primaryWorkArea: Frame = { x: 0, y: 0, width: 1440, height: 900 };
const windowFrame: Frame = { x: 100, y: 120, width: 800, height: 500 };

function targetFor(
  layoutId: keyof typeof LAYOUT_SPECS,
  workArea: Frame,
  gap = 0,
  currentFrame: Frame = windowFrame,
): Frame {
  return computeTargetFrame({
    spec: LAYOUT_SPECS[layoutId],
    workArea,
    currentFrame,
    gap,
  });
}

test('exposes the stable persisted P1 layout ids', () => {
  assert.deepEqual(Object.keys(LAYOUT_SPECS), [
    'left-half',
    'right-half',
    'top-half',
    'bottom-half',
    'maximize',
    'center',
  ]);
});

const primaryLayoutCases: Array<{
  layoutId: keyof typeof LAYOUT_SPECS;
  gap: number;
  expected: Frame;
}> = [
  {
    layoutId: 'left-half',
    gap: 0,
    expected: { x: 0, y: 0, width: 720, height: 900 },
  },
  {
    layoutId: 'right-half',
    gap: 0,
    expected: { x: 720, y: 0, width: 720, height: 900 },
  },
  {
    layoutId: 'top-half',
    gap: 0,
    expected: { x: 0, y: 0, width: 1440, height: 450 },
  },
  {
    layoutId: 'bottom-half',
    gap: 0,
    expected: { x: 0, y: 450, width: 1440, height: 450 },
  },
  {
    layoutId: 'maximize',
    gap: 0,
    expected: { x: 0, y: 0, width: 1440, height: 900 },
  },
  {
    layoutId: 'left-half',
    gap: 10,
    expected: { x: 10, y: 10, width: 705, height: 880 },
  },
  {
    layoutId: 'right-half',
    gap: 10,
    expected: { x: 725, y: 10, width: 705, height: 880 },
  },
  {
    layoutId: 'top-half',
    gap: 10,
    expected: { x: 10, y: 10, width: 1420, height: 435 },
  },
  {
    layoutId: 'bottom-half',
    gap: 10,
    expected: { x: 10, y: 455, width: 1420, height: 435 },
  },
  {
    layoutId: 'maximize',
    gap: 10,
    expected: { x: 10, y: 10, width: 1420, height: 880 },
  },
];

for (const { layoutId, gap, expected } of primaryLayoutCases) {
  test(`${layoutId} on a 1440x900 origin display with gap ${gap}`, () => {
    assert.deepEqual(targetFor(layoutId, primaryWorkArea, gap), expected);
  });
}

test('center keeps the window size and centers it in the work area', () => {
  assert.deepEqual(targetFor('center', primaryWorkArea), {
    x: 320,
    y: 200,
    width: 800,
    height: 500,
  });
});

test('center ignores the gap', () => {
  assert.deepEqual(
    targetFor('center', primaryWorkArea, 10),
    targetFor('center', primaryWorkArea, 0),
  );
});

test('center rounds the origin when the leftover space is odd', () => {
  const oddWindow: Frame = { x: 0, y: 0, width: 801, height: 501 };
  assert.deepEqual(targetFor('center', primaryWorkArea, 0, oddWindow), {
    x: 320,
    y: 200,
    width: 801,
    height: 501,
  });
});

test('center clamps a window larger than the work area to the work area', () => {
  const oversized: Frame = { x: 0, y: 0, width: 2000, height: 1200 };
  assert.deepEqual(targetFor('center', primaryWorkArea, 0, oversized), {
    x: 0,
    y: 0,
    width: 1440,
    height: 900,
  });
});

test('center clamps an oversized window on a negative-origin display', () => {
  const secondaryWorkArea: Frame = {
    x: -1920,
    y: 0,
    width: 1920,
    height: 1080,
  };
  const oversized: Frame = { x: 0, y: 0, width: 2560, height: 1440 };
  assert.deepEqual(targetFor('center', secondaryWorkArea, 0, oversized), {
    x: -1920,
    y: 0,
    width: 1920,
    height: 1080,
  });
});

const secondaryDisplayCases: Array<{
  name: string;
  workArea: Frame;
  layoutId: keyof typeof LAYOUT_SPECS;
  gap: number;
  expected: Frame;
}> = [
  {
    name: 'negative x origin, left half',
    workArea: { x: -1920, y: 0, width: 1920, height: 1080 },
    layoutId: 'left-half',
    gap: 0,
    expected: { x: -1920, y: 0, width: 960, height: 1080 },
  },
  {
    name: 'negative x origin, right half',
    workArea: { x: -1920, y: 0, width: 1920, height: 1080 },
    layoutId: 'right-half',
    gap: 0,
    expected: { x: -960, y: 0, width: 960, height: 1080 },
  },
  {
    name: 'negative x origin, left half with gap 10',
    workArea: { x: -1920, y: 0, width: 1920, height: 1080 },
    layoutId: 'left-half',
    gap: 10,
    expected: { x: -1910, y: 10, width: 945, height: 1060 },
  },
  {
    name: 'display above the primary, maximize',
    workArea: { x: 0, y: -1080, width: 1920, height: 1080 },
    layoutId: 'maximize',
    gap: 0,
    expected: { x: 0, y: -1080, width: 1920, height: 1080 },
  },
  {
    name: 'display above the primary, bottom half',
    workArea: { x: 0, y: -1080, width: 1920, height: 1080 },
    layoutId: 'bottom-half',
    gap: 0,
    expected: { x: 0, y: -540, width: 1920, height: 540 },
  },
  {
    name: 'display above the primary, top half',
    workArea: { x: 0, y: -1080, width: 1920, height: 1080 },
    layoutId: 'top-half',
    gap: 0,
    expected: { x: 0, y: -1080, width: 1920, height: 540 },
  },
  {
    name: 'portrait display left of the primary, top half',
    workArea: { x: -1080, y: 0, width: 1080, height: 1920 },
    layoutId: 'top-half',
    gap: 0,
    expected: { x: -1080, y: 0, width: 1080, height: 960 },
  },
  {
    name: 'portrait display left of the primary, left half',
    workArea: { x: -1080, y: 0, width: 1080, height: 1920 },
    layoutId: 'left-half',
    gap: 0,
    expected: { x: -1080, y: 0, width: 540, height: 1920 },
  },
];

for (const {
  name,
  workArea,
  layoutId,
  gap,
  expected,
} of secondaryDisplayCases) {
  test(name, () => {
    assert.deepEqual(targetFor(layoutId, workArea, gap), expected);
  });
}

const WORK_AREA_WIDTH_EVEN_LAPTOP = 1440;
const WORK_AREA_WIDTH_ODD_LAPTOP = 1441;
const WORK_AREA_WIDTH_ODD_FULL_HD = 1919;
const WORK_AREA_WIDTH_FULL_HD = 1920;
const GAP_NONE = 0;
const GAP_ODD_SMALL = 1;
const GAP_TEN = 10;
const GAP_ODD_LARGE = 11;

const abuttingWorkAreaWidths = [
  WORK_AREA_WIDTH_EVEN_LAPTOP,
  WORK_AREA_WIDTH_ODD_LAPTOP,
  WORK_AREA_WIDTH_ODD_FULL_HD,
  WORK_AREA_WIDTH_FULL_HD,
];
const abuttingGaps = [GAP_NONE, GAP_ODD_SMALL, GAP_TEN, GAP_ODD_LARGE];

for (const width of abuttingWorkAreaWidths) {
  for (const gap of abuttingGaps) {
    test(`left and right halves of a ${width}px work area with gap ${gap} abut without overlap`, () => {
      const workArea: Frame = { x: -width, y: 0, width, height: 900 };
      const left = targetFor('left-half', workArea, gap);
      const right = targetFor('right-half', workArea, gap);

      assert.equal(left.x, workArea.x + gap);
      assert.equal(right.x + right.width, workArea.x + workArea.width - gap);
      assert.equal(left.x + left.width + gap, right.x);
    });
  }
}

test('odd-width halves tile the work area exactly without gap', () => {
  const workArea: Frame = { x: 0, y: 0, width: 1441, height: 900 };
  assert.deepEqual(targetFor('left-half', workArea), {
    x: 0,
    y: 0,
    width: 721,
    height: 900,
  });
  assert.deepEqual(targetFor('right-half', workArea), {
    x: 721,
    y: 0,
    width: 720,
    height: 900,
  });
});

test('odd-width halves keep exactly the gap between them', () => {
  const workArea: Frame = { x: 0, y: 0, width: 1441, height: 900 };
  assert.deepEqual(targetFor('left-half', workArea, 10), {
    x: 10,
    y: 10,
    width: 706,
    height: 880,
  });
  assert.deepEqual(targetFor('right-half', workArea, 10), {
    x: 726,
    y: 10,
    width: 705,
    height: 880,
  });
});

const displayPrimary: DisplayInfo = {
  id: 'primary',
  isPrimary: true,
  frame: primaryWorkArea,
  workArea: primaryWorkArea,
};
const displayRight: DisplayInfo = {
  id: 'right',
  isPrimary: false,
  frame: { x: 1440, y: 0, width: 1920, height: 1080 },
  workArea: { x: 1440, y: 0, width: 1920, height: 1080 },
};
const displayLeftNegative: DisplayInfo = {
  id: 'left',
  isPrimary: false,
  frame: { x: -1920, y: 0, width: 1920, height: 1080 },
  workArea: { x: -1920, y: 0, width: 1920, height: 1080 },
};
const displayAbove: DisplayInfo = {
  id: 'above',
  isPrimary: false,
  frame: { x: 0, y: -1080, width: 1920, height: 1080 },
  workArea: { x: 0, y: -1080, width: 1920, height: 1080 },
};

const straddlingFrame: Frame = { x: 1000, y: 0, width: 1000, height: 500 };
const offScreenFrame: Frame = { x: 5000, y: 5000, width: 400, height: 300 };

test('picks the display with the largest overlap for a window straddling two displays', () => {
  assert.equal(
    pickDisplayForFrame([displayPrimary, displayRight], straddlingFrame),
    displayRight,
  );
});

test('picks the negative-origin display when the window sits on it', () => {
  const frame: Frame = { x: -1500, y: 100, width: 600, height: 400 };
  assert.equal(
    pickDisplayForFrame([displayPrimary, displayLeftNegative], frame),
    displayLeftNegative,
  );
});

test('falls back to the primary display for a fully off-screen window', () => {
  assert.equal(
    pickDisplayForFrame(
      [displayRight, displayPrimary, displayAbove],
      offScreenFrame,
    ),
    displayPrimary,
  );
});

test('falls back to the first display when none is primary and the window is off-screen', () => {
  const withoutPrimary = [
    { ...displayRight, isPrimary: false },
    { ...displayAbove, isPrimary: false },
  ];
  assert.equal(
    pickDisplayForFrame(withoutPrimary, offScreenFrame),
    withoutPrimary[0],
  );
});

test('returns the only display for any frame', () => {
  assert.equal(
    pickDisplayForFrame([displayPrimary], offScreenFrame),
    displayPrimary,
  );
  assert.equal(
    pickDisplayForFrame([displayPrimary], straddlingFrame),
    displayPrimary,
  );
});

test('prefers the primary display on an exact overlap tie regardless of list order', () => {
  const leftDisplay: DisplayInfo = {
    id: 'other',
    isPrimary: false,
    frame: { x: 1000, y: 0, width: 1000, height: 800 },
    workArea: { x: 1000, y: 0, width: 1000, height: 800 },
  };
  const tieFrame: Frame = { x: 500, y: 0, width: 1000, height: 800 };
  const primaryTieDisplay: DisplayInfo = {
    id: 'primary-tie',
    isPrimary: true,
    frame: { x: 0, y: 0, width: 1000, height: 800 },
    workArea: { x: 0, y: 0, width: 1000, height: 800 },
  };
  assert.equal(
    pickDisplayForFrame([leftDisplay, primaryTieDisplay], tieFrame),
    primaryTieDisplay,
  );
  assert.equal(
    pickDisplayForFrame([primaryTieDisplay, leftDisplay], tieFrame),
    primaryTieDisplay,
  );
});

test('throws when there are no displays', () => {
  assert.throws(() => pickDisplayForFrame([], straddlingFrame));
});
