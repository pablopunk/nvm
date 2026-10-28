// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createHarness,
  FINDER,
  fail,
  focusedWindow,
  LEFT_HALF,
  NO_OPTIONS,
  ORIGINAL_FRAME,
  OTHER_APP,
  OTHER_PID,
  ok,
  paletteInvocation,
  shortcutInvocation,
  writeOk,
} from './window-management.harness';
import type { SetFrameData, WindowLayoutSpec } from './window-management-types';

const OVERFLOWING_FRAME = { x: 900, y: 25, width: 900, height: 875 };
const REANCHORED_FRAME = { x: 0, y: 25, width: 900, height: 875 };
const MINIMUM_WIDTH_REASON = 'Minimum width is 900';
const CORRECTION_FAILED_REASON =
  'Part of the window stayed outside its display.';
const HELPER_EXPLODED = /helper exploded/;
const RIGHT_HALF: WindowLayoutSpec = {
  kind: 'fraction',
  x: 0.5,
  y: 0,
  width: 0.5,
  height: 1,
};

function createGate() {
  const gate: { open?: () => void } = {};
  const promise = new Promise<void>((resolve) => {
    gate.open = () => resolve();
  });
  return { promise, open: () => gate.open?.() };
}

async function flushPendingWork() {
  await new Promise((resolve) => setImmediate(resolve));
}

test('accepts a partial write that stays inside the display without a corrective write', async () => {
  const minimumWidthFrame = { x: 0, y: 25, width: 900, height: 875 };
  const { service, setFrameRequests } = createHarness({
    writes: [writeOk(minimumWidthFrame, 'partial', MINIMUM_WIDTH_REASON)],
  });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.deepEqual(result, {
    ok: true,
    outcome: 'partial',
    changed: true,
    frame: minimumWidthFrame,
    reason: MINIMUM_WIDTH_REASON,
  });
  assert.equal(setFrameRequests.length, 1);
});

test('re-anchors an overflowing partial write with exactly one position-only write', async () => {
  const { service, setFrameRequests } = createHarness({
    writes: [
      writeOk(OVERFLOWING_FRAME, 'partial', MINIMUM_WIDTH_REASON),
      writeOk(REANCHORED_FRAME, 'partial', MINIMUM_WIDTH_REASON),
    ],
  });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.deepEqual(result, {
    ok: true,
    outcome: 'partial',
    changed: true,
    frame: REANCHORED_FRAME,
    reason: MINIMUM_WIDTH_REASON,
  });
  assert.equal(setFrameRequests.length, 2);
  assert.equal(setFrameRequests[1]?.positionOnly, true);
  assert.deepEqual(setFrameRequests[1]?.frame, REANCHORED_FRAME);
});

test('reports unapplied when the corrective write only returns the original frame', async () => {
  const { service } = createHarness({
    writes: [
      writeOk(OVERFLOWING_FRAME, 'partial'),
      {
        ok: false,
        code: 'unapplied',
        message: 'AX ignored the move',
        data: { outcome: 'unapplied', changed: false, frame: ORIGINAL_FRAME },
      },
    ],
  });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(result.ok === false && result.reason, 'unapplied');
});

test('keeps the overflowing frame and a reason when the corrective write fails', async () => {
  const { service, setFrameRequests } = createHarness({
    writes: [
      writeOk(OVERFLOWING_FRAME, 'partial'),
      fail<SetFrameData>('error', 'helper crashed'),
    ],
  });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.deepEqual(result, {
    ok: true,
    outcome: 'partial',
    changed: true,
    frame: OVERFLOWING_FRAME,
    reason: CORRECTION_FAILED_REASON,
  });
  assert.equal(setFrameRequests.length, 2);
});

test('reports a helper unapplied envelope as unapplied with the helper message', async () => {
  const { service } = createHarness({
    writes: [fail<SetFrameData>('unapplied', 'AX refused')],
  });
  assert.deepEqual(
    await service.apply(LEFT_HALF, NO_OPTIONS, shortcutInvocation()),
    {
      ok: false,
      reason: 'unapplied',
      message: 'AX refused',
    },
  );
});

test('reports an ok envelope whose outcome is unapplied with its reason', async () => {
  const { service } = createHarness({
    writes: [
      ok<SetFrameData>({
        outcome: 'unapplied',
        changed: false,
        frame: ORIGINAL_FRAME,
        reason: 'Attribute is not settable',
      }),
    ],
  });
  assert.deepEqual(
    await service.apply(LEFT_HALF, NO_OPTIONS, shortcutInvocation()),
    {
      ok: false,
      reason: 'unapplied',
      message: 'Attribute is not settable',
    },
  );
});

test('serializes applies for the same pid so the second never reads before the first writes', async () => {
  const gate = createGate();
  const { service, calls } = createHarness({
    focusedRead: async (pid) => {
      if (pid === FINDER.pid) {
        await gate.promise;
      }
      return ok(focusedWindow());
    },
  });
  const first = service.apply(LEFT_HALF, NO_OPTIONS, paletteInvocation());
  const second = service.apply(RIGHT_HALF, NO_OPTIONS, paletteInvocation());
  await flushPendingWork();
  assert.equal(calls.filter((call) => call.startsWith('focused')).length, 1);
  gate.open();
  await Promise.all([first, second]);
  const firstWrite = calls.indexOf('setFrame');
  const secondRestore = calls.lastIndexOf(`restore:${FINDER.pid}`);
  assert.ok(firstWrite >= 0 && secondRestore > firstWrite);
});

test('lets applies for different pids overlap', async () => {
  const gate = createGate();
  let paletteTarget = FINDER;
  const { service, calls } = createHarness({
    paletteTarget: () => paletteTarget,
    focusedRead: async (pid) => {
      if (pid === FINDER.pid) {
        await gate.promise;
      }
      return ok(
        focusedWindow({
          bundleId: pid === OTHER_PID ? OTHER_APP.bundleId : FINDER.bundleId,
        }),
      );
    },
  });
  const blocked = service.apply(LEFT_HALF, NO_OPTIONS, paletteInvocation());
  await flushPendingWork();
  paletteTarget = OTHER_APP;
  const overlapping = service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    paletteInvocation(OTHER_APP),
  );
  await flushPendingWork();
  assert.ok(calls.includes(`focused:${OTHER_PID}`));
  gate.open();
  assert.equal((await overlapping).ok, true);
  assert.equal((await blocked).ok, true);
});

test('a rejected apply does not poison the queue for the same pid', async () => {
  let reads = 0;
  const { service } = createHarness({
    focusedRead: () => {
      reads += 1;
      return reads === 1
        ? Promise.reject(new Error('helper exploded'))
        : Promise.resolve(ok(focusedWindow()));
    },
  });
  await assert.rejects(
    service.apply(LEFT_HALF, NO_OPTIONS, shortcutInvocation()),
    HELPER_EXPLODED,
  );
  const recovered = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(recovered.ok, true);
});
