// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createHarness,
  NO_OPTIONS,
  shortcutInvocation,
} from './window-management.harness';
import { createSimulatedWindow } from './window-management.simulated-window';
import type {
  WindowLayoutSpec,
  WindowManagementOptions,
  WindowManagementService,
} from './window-management-types';

const LEFT_CYCLE: WindowLayoutSpec = {
  kind: 'fraction',
  x: 0,
  y: 0,
  width: 0.5,
  height: 1,
  cycleGroup: 'left',
};
const LEFT_PLAIN: WindowLayoutSpec = {
  kind: 'fraction',
  x: 0,
  y: 0,
  width: 0.5,
  height: 1,
};
const CYCLE_OFF = { ...NO_OPTIONS, cycle: false };
const MANUALLY_MOVED_FRAME = { x: 300, y: 25, width: 720, height: 875 };

async function pressLeft(
  service: WindowManagementService,
  simulation: ReturnType<typeof createSimulatedWindow>,
  spec: WindowLayoutSpec = LEFT_CYCLE,
  options = NO_OPTIONS,
) {
  await service.apply(spec, options, shortcutInvocation());
  return Math.round(simulation.state.frame.width);
}

test('three presses of the same half cycle through half, two-thirds and third', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({ simulation });
  const widths = [
    await pressLeft(service, simulation),
    await pressLeft(service, simulation),
    await pressLeft(service, simulation),
    await pressLeft(service, simulation),
  ];
  assert.deepEqual(widths, [720, 960, 480, 720]);
});

const THIRD_BEFORE_TWO_THIRDS: WindowManagementOptions = {
  ...NO_OPTIONS,
  cycleOrder: 'half-third-two-thirds',
};

test('cycleOrder half-third-two-thirds presses half, third and two-thirds', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({ simulation });
  const widths = [
    await pressLeft(service, simulation, LEFT_CYCLE, THIRD_BEFORE_TWO_THIRDS),
    await pressLeft(service, simulation, LEFT_CYCLE, THIRD_BEFORE_TWO_THIRDS),
    await pressLeft(service, simulation, LEFT_CYCLE, THIRD_BEFORE_TWO_THIRDS),
    await pressLeft(service, simulation, LEFT_CYCLE, THIRD_BEFORE_TWO_THIRDS),
  ];
  assert.deepEqual(widths, [720, 480, 960, 720]);
});

test('a plain half command without a cycle group ends the cycle', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({ simulation });
  await pressLeft(service, simulation);
  await pressLeft(service, simulation, LEFT_PLAIN);
  assert.equal(await pressLeft(service, simulation), 720);
});

test('moving the window by hand restarts the cycle at the half', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({ simulation });
  await pressLeft(service, simulation);
  simulation.state.frame = MANUALLY_MOVED_FRAME;
  assert.equal(await pressLeft(service, simulation), 720);
  assert.equal(simulation.state.frame.x, 0);
});

test('cycle:false keeps every press on the half', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({ simulation });
  assert.equal(
    await pressLeft(service, simulation, LEFT_CYCLE, CYCLE_OFF),
    720,
  );
  assert.equal(
    await pressLeft(service, simulation, LEFT_CYCLE, CYCLE_OFF),
    720,
  );
});
