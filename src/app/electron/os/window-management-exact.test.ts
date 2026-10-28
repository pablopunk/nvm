// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import { computeTargetFrame } from './window-layouts';
import {
  createHarness,
  NO_OPTIONS,
  shortcutInvocation,
} from './window-management.harness';
import {
  createSimulatedWindow,
  type SimulatedWindow,
} from './window-management.simulated-window';
import type { Frame, WindowLayoutSpec } from './window-management-types';

const ORIGINAL_FRAME: Frame = { x: 100, y: 100, width: 600, height: 400 };
const HALF_OF_MENU_BAR_AREA: Frame = { x: 0, y: 25, width: 720, height: 875 };
const CYCLED_LEFT_HALF: WindowLayoutSpec = {
  kind: 'fraction',
  x: 0,
  y: 0,
  width: 0.5,
  height: 1,
  cycleGroup: 'left',
};
const NO_FIELDS_MESSAGE =
  'Choose at least one window value: x, y, width, or height.';

type ExactFields = Omit<Extract<WindowLayoutSpec, { kind: 'exact' }>, 'kind'>;

function exactLayout(fields: ExactFields): WindowLayoutSpec {
  return { kind: 'exact', ...fields };
}

function realEngineHarness(simulation: SimulatedWindow) {
  return createHarness({ simulation, engine: { computeTargetFrame } });
}

test('refuses an exact layout that names no field before the helper runs', async () => {
  const { service, calls } = createHarness();
  const result = await service.apply(
    { kind: 'exact' },
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.deepEqual(result, {
    ok: false,
    reason: 'invalid-spec',
    message: NO_FIELDS_MESSAGE,
  });
  assert.deepEqual(calls, []);
});

test('places an exact layout through the layout path with the computed frame', async () => {
  const simulation = createSimulatedWindow({ frame: ORIGINAL_FRAME });
  const { service, setFrameRequests } = realEngineHarness(simulation);
  const result = await service.apply(
    exactLayout({ width: { value: 50, unit: 'percent' } }),
    NO_OPTIONS,
    shortcutInvocation(),
  );
  const expectedFrame = { x: 100, y: 100, width: 720, height: 400 };
  assert.deepEqual(result, {
    ok: true,
    outcome: 'exact',
    changed: true,
    frame: expectedFrame,
  });
  assert.deepEqual(setFrameRequests, [
    { pid: 42, windowKey: '42:7', frame: expectedFrame },
  ]);
});

test('remembers the pre-layout frame so restore returns an exact layout', async () => {
  const simulation = createSimulatedWindow({ frame: ORIGINAL_FRAME });
  const { service } = realEngineHarness(simulation);
  await service.apply(
    exactLayout({ width: { value: 720, unit: 'points' } }),
    NO_OPTIONS,
    shortcutInvocation(),
  );
  const restored = await service.apply(
    { kind: 'restore' },
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(restored.ok, true);
  assert.deepEqual(simulation.state.frame, ORIGINAL_FRAME);
});

test('clears the cycle group so the next half command restarts the cycle', async () => {
  const simulation = createSimulatedWindow({ frame: ORIGINAL_FRAME });
  const { service } = realEngineHarness(simulation);
  await service.apply(CYCLED_LEFT_HALF, NO_OPTIONS, shortcutInvocation());
  await service.apply(
    exactLayout({
      x: { value: 0, unit: 'points' },
      y: { value: 0, unit: 'points' },
      width: { value: 720, unit: 'points' },
      height: { value: 875, unit: 'points' },
    }),
    NO_OPTIONS,
    shortcutInvocation(),
  );
  const result = await service.apply(
    CYCLED_LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(result.ok, true);
  assert.deepEqual(simulation.state.frame, HALF_OF_MENU_BAR_AREA);
});

test('keeps the size of a non-resizable window for an exact layout', async () => {
  const simulation = createSimulatedWindow({
    frame: ORIGINAL_FRAME,
    isResizable: false,
  });
  const { service, setFrameRequests } = realEngineHarness(simulation);
  await service.apply(
    exactLayout({
      x: { value: 0, unit: 'points' },
      width: { value: 1000, unit: 'points' },
    }),
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.deepEqual(setFrameRequests[0]?.frame, {
    x: 0,
    y: 100,
    width: 600,
    height: 400,
  });
});
