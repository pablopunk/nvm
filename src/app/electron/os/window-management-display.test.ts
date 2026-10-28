// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createHarness,
  DISPLAY,
  NO_OPTIONS,
  ORIGINAL_FRAME,
  ok,
  shortcutInvocation,
} from './window-management.harness';
import { createSimulatedWindow } from './window-management.simulated-window';
import type { DisplayMoveInput } from './window-management-engine-port';
import type {
  DisplayInfo,
  Frame,
  WindowLayoutSpec,
} from './window-management-types';

const PRIMARY: DisplayInfo = {
  id: 'primary',
  isPrimary: true,
  frame: { x: 0, y: 0, width: 1440, height: 900 },
  workArea: { x: 0, y: 25, width: 1440, height: 875 },
};
const SECONDARY: DisplayInfo = {
  id: 'secondary',
  isPrimary: false,
  frame: { x: 1440, y: 0, width: 1440, height: 900 },
  workArea: { x: 1440, y: 25, width: 1440, height: 875 },
};
const TWO_DISPLAYS = [PRIMARY, SECONDARY];
const NEXT_DISPLAY: WindowLayoutSpec = { kind: 'display', direction: 'next' };
const PREVIOUS_DISPLAY: WindowLayoutSpec = {
  kind: 'display',
  direction: 'previous',
};
const RESTORE: WindowLayoutSpec = { kind: 'restore' };
const ONLY_ONE_DISPLAY_PATTERN = /Only one display/;
const ON_SECONDARY_FRAME = { x: 1540, y: 100, width: 600, height: 400 };
const ENGINE_FRAME: Frame = { x: 7, y: 8, width: 300, height: 200 };

function displaysResponse(displays: DisplayInfo[]) {
  return () => Promise.resolve(ok(displays));
}

test('moves the window to the next display, wrapping back to the first', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({
    simulation,
    displays: displaysResponse(TWO_DISPLAYS),
  });
  const moved = await service.apply(
    NEXT_DISPLAY,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(moved.ok, true);
  assert.deepEqual(simulation.state.frame, ON_SECONDARY_FRAME);

  await service.apply(NEXT_DISPLAY, NO_OPTIONS, shortcutInvocation());
  assert.deepEqual(simulation.state.frame, ORIGINAL_FRAME);
});

test('moves the window to the previous display, wrapping to the last', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({
    simulation,
    displays: displaysResponse(TWO_DISPLAYS),
  });
  await service.apply(PREVIOUS_DISPLAY, NO_OPTIONS, shortcutInvocation());
  assert.deepEqual(simulation.state.frame, ON_SECONDARY_FRAME);
});

test('applies the frame returned by moveFrameToDisplay and passes it the source and target', async () => {
  const simulation = createSimulatedWindow({});
  const moveInputs: DisplayMoveInput[] = [];
  function recordMoveInput(input: DisplayMoveInput): Frame {
    moveInputs.push(input);
    return ENGINE_FRAME;
  }
  const { service, setFrameRequests } = createHarness({
    simulation,
    displays: displaysResponse(TWO_DISPLAYS),
    engine: { moveFrameToDisplay: recordMoveInput },
  });
  const result = await service.apply(
    NEXT_DISPLAY,
    { gap: 20, cycle: true },
    shortcutInvocation(),
  );
  assert.deepEqual(result, {
    ok: true,
    outcome: 'exact',
    changed: true,
    frame: ENGINE_FRAME,
  });
  assert.deepEqual(setFrameRequests[0]?.frame, ENGINE_FRAME);
  assert.equal(moveInputs[0]?.from.id, PRIMARY.id);
  assert.equal(moveInputs[0]?.to.id, SECONDARY.id);
  assert.deepEqual(moveInputs[0]?.frame, ORIGINAL_FRAME);
});

test('records the pre-move frame so restore brings the window back', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({
    simulation,
    displays: displaysResponse(TWO_DISPLAYS),
  });
  await service.apply(NEXT_DISPLAY, NO_OPTIONS, shortcutInvocation());
  await service.apply(RESTORE, NO_OPTIONS, shortcutInvocation());
  assert.deepEqual(simulation.state.frame, ORIGINAL_FRAME);
});

test('reports a single display without touching the window', async () => {
  const simulation = createSimulatedWindow({});
  const { service, setFrameRequests } = createHarness({
    simulation,
    displays: displaysResponse([DISPLAY]),
  });
  const result = await service.apply(
    NEXT_DISPLAY,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(result.ok === false && result.reason, 'single-display');
  assert.match(
    result.ok === false ? result.message : '',
    ONLY_ONE_DISPLAY_PATTERN,
  );
  assert.deepEqual(setFrameRequests, []);
});
