import assert from 'node:assert/strict';
import test from 'node:test';
import {
  boundsFittedToWorkArea,
  boundsResizedAroundCenter,
  paletteBoundsPlacedInWorkArea,
} from './palette-window-geometry';

test('palette mode resizes keep the window centered in one bounds update', () => {
  assert.deepEqual(
    boundsResizedAroundCenter(
      { x: 100, y: 200, width: 760, height: 480 },
      { width: 1000, height: 640 },
    ),
    { x: -20, y: 120, width: 1000, height: 640 },
  );
  assert.deepEqual(
    boundsResizedAroundCenter(
      { x: 101, y: 200, width: 760, height: 480 },
      { width: 761, height: 480 },
    ),
    { x: 101, y: 200, width: 761, height: 480 },
  );
});

test('large palette modes keep the visible palette inside a short work area', () => {
  const workArea = { x: 0, y: 0, width: 1280, height: 800 };
  const preview = boundsFittedToWorkArea(
    boundsResizedAroundCenter(
      { x: 184, y: 144, width: 912, height: 672 },
      { width: 1272, height: 952 },
    ),
    workArea,
    96,
  );

  assert.deepEqual(preview, { x: 4, y: -56, width: 1272, height: 952 });
  assert.ok(preview.y + 96 >= workArea.y);
  assert.ok(preview.y + preview.height - 96 <= workArea.height);
});

test('a palette taller than the work area shrinks to the visible height', () => {
  assert.deepEqual(
    boundsFittedToWorkArea(
      { x: 0, y: 100, width: 1272, height: 952 },
      { x: 0, y: 32, width: 1366, height: 736 },
      96,
    ),
    { x: 0, y: -64, width: 1272, height: 928 },
  );
});

test('a palette that already fits keeps its position', () => {
  const bounds = { x: 304, y: 144, width: 912, height: 672 };
  assert.deepEqual(
    boundsFittedToWorkArea(
      bounds,
      { x: 0, y: 0, width: 1512, height: 944 },
      96,
    ),
    bounds,
  );
});

test('opening on a short display places the palette fully on screen', () => {
  assert.deepEqual(
    paletteBoundsPlacedInWorkArea(
      { x: 0, y: 0, width: 1272, height: 952 },
      { x: 0, y: 0, width: 1280, height: 800 },
    ),
    { x: 4, y: -56, width: 1272, height: 952 },
  );
});
