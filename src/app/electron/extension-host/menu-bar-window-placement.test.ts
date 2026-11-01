import assert from 'node:assert/strict';
import test from 'node:test';
import {
  fallbackMenuBarAnchor,
  menuBarWindowBounds,
} from './menu-bar-window-placement';

const macWorkArea = { x: 0, y: 25, width: 1512, height: 957 };
const size = { width: 320, height: 240 };

test('popovers open centered under a top menu bar icon', () => {
  assert.deepEqual(
    menuBarWindowBounds(
      { x: 1200, y: 0, width: 24, height: 24 },
      size,
      macWorkArea,
    ),
    { x: 1052, y: 31, width: 320, height: 240 },
  );
});

test('popovers stay on screen near the display edge', () => {
  const bounds = menuBarWindowBounds(
    { x: 1500, y: 0, width: 24, height: 24 },
    size,
    macWorkArea,
  );
  assert.equal(bounds.x, 1512 - 320 - 6);
});

test('a bottom taskbar opens popovers upward', () => {
  const windowsWorkArea = { x: 0, y: 0, width: 1920, height: 1032 };
  const bounds = menuBarWindowBounds(
    { x: 1700, y: 1040, width: 24, height: 40 },
    size,
    windowsWorkArea,
  );
  assert.equal(bounds.y, 1040 - 240 - 6);
});

test('without a click anchor popovers open at the top right', () => {
  const anchor = fallbackMenuBarAnchor(macWorkArea);
  const bounds = menuBarWindowBounds(anchor, size, macWorkArea);
  assert.equal(bounds.y, 31);
  assert.ok(bounds.x + bounds.width <= macWorkArea.width);
});
