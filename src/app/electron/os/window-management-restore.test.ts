// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createHarness,
  DISPLAY,
  fail,
  LEFT_HALF,
  NO_OPTIONS,
  ORIGINAL_FRAME,
  ok,
  REQUESTED_LEFT_HALF,
  shortcutInvocation,
  writeOk,
} from './window-management.harness';
import { createSimulatedWindow } from './window-management.simulated-window';
import type {
  DisplayInfo,
  SetFrameData,
  WindowLayoutSpec,
} from './window-management-types';

const RESTORE: WindowLayoutSpec = { kind: 'restore' };
const RIGHT_HALF: WindowLayoutSpec = {
  kind: 'fraction',
  x: 0.5,
  y: 0,
  width: 0.5,
  height: 1,
};
const MOVED_FRAME = { x: 200, y: 200, width: 600, height: 400 };
const SECOND_DISPLAY: DisplayInfo = {
  id: 'second',
  isPrimary: false,
  frame: { x: 1440, y: 0, width: 1440, height: 900 },
  workArea: { x: 1440, y: 25, width: 1440, height: 875 },
};
const SHIFTED_ORIGINAL_FRAME = { x: 1600, y: 100, width: 600, height: 400 };
const CLAMPED_ONTO_PRIMARY = { x: 840, y: 100, width: 600, height: 400 };

test('restores the frame from before the last layout, then has nothing left to restore', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({ simulation });
  await service.apply(LEFT_HALF, NO_OPTIONS, shortcutInvocation());
  assert.deepEqual(simulation.state.frame, REQUESTED_LEFT_HALF);

  const restored = await service.apply(
    RESTORE,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.deepEqual(restored, {
    ok: true,
    outcome: 'exact',
    changed: true,
    frame: ORIGINAL_FRAME,
  });
  assert.deepEqual(simulation.state.frame, ORIGINAL_FRAME);

  const again = await service.apply(RESTORE, NO_OPTIONS, shortcutInvocation());
  assert.equal(again.ok === false && again.reason, 'no-history');
});

test('reports no-history without touching the window when nothing was recorded', async () => {
  const simulation = createSimulatedWindow({});
  const { service, setFrameRequests } = createHarness({ simulation });
  const result = await service.apply(RESTORE, NO_OPTIONS, shortcutInvocation());
  assert.deepEqual(result, {
    ok: false,
    reason: 'no-history',
    message: 'There is no earlier position to restore for this window.',
  });
  assert.deepEqual(setFrameRequests, []);
});

test('records nothing for a layout that leaves the window where it already was', async () => {
  const simulation = createSimulatedWindow({ frame: REQUESTED_LEFT_HALF });
  const { service } = createHarness({ simulation });
  const noop = await service.apply(LEFT_HALF, NO_OPTIONS, shortcutInvocation());
  assert.equal(noop.ok && noop.changed, false);
  const restore = await service.apply(
    RESTORE,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(restore.ok === false && restore.reason, 'no-history');
});

test('captures a new restore frame when the window was moved since the last layout', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({ simulation });
  await service.apply(LEFT_HALF, NO_OPTIONS, shortcutInvocation());
  simulation.state.frame = MOVED_FRAME;

  await service.apply(RIGHT_HALF, NO_OPTIONS, shortcutInvocation());
  await service.apply(RESTORE, NO_OPTIONS, shortcutInvocation());
  assert.deepEqual(simulation.state.frame, MOVED_FRAME);
});

test('keeps the stored frame when the restore write fails and restores after a retry', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({
    simulation,
    writes: [
      writeOk(REQUESTED_LEFT_HALF),
      fail<SetFrameData>('unapplied', 'AX refused'),
    ],
  });
  await service.apply(LEFT_HALF, NO_OPTIONS, shortcutInvocation());

  const failed = await service.apply(RESTORE, NO_OPTIONS, shortcutInvocation());
  assert.deepEqual(failed, {
    ok: false,
    reason: 'unapplied',
    message: 'AX refused',
  });

  const retried = await service.apply(
    RESTORE,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(retried.ok, true);
  assert.deepEqual(simulation.state.frame, ORIGINAL_FRAME);
});

test('clamps the restore frame onto a remaining display when its original display is gone', async () => {
  const simulation = createSimulatedWindow({ frame: SHIFTED_ORIGINAL_FRAME });
  let connected: DisplayInfo[] = [DISPLAY, SECOND_DISPLAY];
  const { service } = createHarness({
    simulation,
    displays: () => Promise.resolve(ok(connected)),
  });
  await service.apply(LEFT_HALF, NO_OPTIONS, shortcutInvocation());

  connected = [DISPLAY];
  const restored = await service.apply(
    RESTORE,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(restored.ok, true);
  assert.deepEqual(simulation.state.frame, CLAMPED_ONTO_PRIMARY);
});
