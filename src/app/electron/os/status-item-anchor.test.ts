import assert from 'node:assert/strict';
import test from 'node:test';
import {
  lastStatusItemAnchor,
  rememberStatusItemAnchor,
  statusItemAnchorFrom,
} from './status-item-anchor';

test('icon bounds win over the cursor when the tray reports them', () => {
  assert.deepEqual(
    statusItemAnchorFrom(
      { x: 10, y: 0, width: 22, height: 24 },
      { x: 1, y: 1 },
    ),
    { x: 10, y: 0, width: 22, height: 24 },
  );
});

test('trays without bounds anchor at the cursor', () => {
  assert.deepEqual(
    statusItemAnchorFrom(
      { x: 0, y: 0, width: 0, height: 0 },
      { x: 500, y: 12 },
    ),
    {
      x: 500,
      y: 12,
      width: 1,
      height: 1,
    },
  );
});

test('the last clicked anchor is remembered for popover placement', () => {
  rememberStatusItemAnchor({ x: 1, y: 2, width: 3, height: 4 });
  assert.deepEqual(lastStatusItemAnchor(), { x: 1, y: 2, width: 3, height: 4 });
  rememberStatusItemAnchor(null);
});
