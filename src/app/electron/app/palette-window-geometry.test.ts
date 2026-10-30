import assert from 'node:assert/strict';
import test from 'node:test';
import { boundsResizedAroundCenter } from './palette-window-geometry';

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
