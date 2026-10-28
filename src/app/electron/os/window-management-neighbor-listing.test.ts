// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createHarness,
  FINDER,
  fail,
  focusedWindow,
  ORIGINAL_FRAME,
  OTHER_APP,
  ok,
  shortcutInvocation,
} from './window-management.harness';
import type { Frame, ListedWindow } from './window-management-types';

const NEIGHBOR_FRAME: Frame = { x: 800, y: 100, width: 600, height: 400 };

function listedWindow(
  pid: number,
  windowKey: string,
  frame: Frame,
  zIndex: number,
): ListedWindow {
  return {
    ...focusedWindow({ windowKey, frame }),
    pid,
    appName: 'Harness App',
    zIndex,
  };
}

const ORIGIN = listedWindow(FINDER.pid, '42:7', ORIGINAL_FRAME, 0);
const NEIGHBOR = listedWindow(OTHER_APP.pid, '43:7', NEIGHBOR_FRAME, 1);
const COINCIDENT_BEHIND = listedWindow(
  OTHER_APP.pid,
  '43:7',
  ORIGINAL_FRAME,
  1,
);

test('neighbour search reads one window snapshot instead of listing each app', async () => {
  const { service, calls } = createHarness({
    allWindowsRead: () => Promise.resolve(ok([ORIGIN, NEIGHBOR])),
  });
  assert.deepEqual(
    await service.focusNeighbor('right', {}, shortcutInvocation()),
    { ok: true },
  );
  assert.equal(calls.filter((call) => call === 'allWindows').length, 1);
  assert.equal(
    calls.some((call) => call.startsWith('windows:')),
    false,
  );
  assert.ok(calls.includes('raise:43:43:7'));
});

test('a window coincident with the origin is raised by its z-order in the snapshot', async () => {
  const { service, calls } = createHarness({
    allWindowsRead: () => Promise.resolve(ok([ORIGIN, COINCIDENT_BEHIND])),
  });
  assert.deepEqual(
    await service.focusNeighbor('right', {}, shortcutInvocation()),
    { ok: true },
  );
  assert.ok(calls.includes('raise:43:43:7'));
});

test('a failed snapshot read fails the focus with the helper message and raises nothing', async () => {
  const { service, calls } = createHarness({
    allWindowsRead: () =>
      Promise.resolve(fail<ListedWindow[]>('error', 'listing broke')),
  });
  const result = await service.focusNeighbor('right', {}, shortcutInvocation());
  assert.equal(result.ok === false && result.reason, 'unapplied');
  assert.equal(result.ok === false && result.message, 'listing broke');
  assert.equal(
    calls.some((call) => call.startsWith('raise:')),
    false,
  );
});

test('an origin missing from the snapshot falls back to its focused frame', async () => {
  const { service, calls } = createHarness({
    allWindowsRead: () => Promise.resolve(ok([NEIGHBOR])),
  });
  assert.deepEqual(
    await service.focusNeighbor('right', {}, shortcutInvocation()),
    { ok: true },
  );
  assert.ok(calls.includes('raise:43:43:7'));
});
