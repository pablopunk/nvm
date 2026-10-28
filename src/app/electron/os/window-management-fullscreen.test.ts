// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createHarness,
  fail,
  NO_OPTIONS,
  ORIGINAL_FRAME,
  shortcutInvocation,
} from './window-management.harness';
import { createSimulatedWindow } from './window-management.simulated-window';
import type { SetFrameData, WindowLayoutSpec } from './window-management-types';

const TOGGLE_FULL_SCREEN: WindowLayoutSpec = { kind: 'toggle-full-screen' };
const RESTORE: WindowLayoutSpec = { kind: 'restore' };

test('enters full screen from a normal window and reports the frame', async () => {
  const simulation = createSimulatedWindow({});
  const { service, calls } = createHarness({ simulation });
  const result = await service.apply(
    TOGGLE_FULL_SCREEN,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.deepEqual(result, {
    ok: true,
    outcome: 'exact',
    changed: true,
    frame: ORIGINAL_FRAME,
  });
  assert.equal(simulation.state.isFullScreen, true);
  assert.ok(calls.includes('fullscreen:true'));
});

test('leaves full screen from a full-screen window', async () => {
  const simulation = createSimulatedWindow({ isFullScreen: true });
  const { service, calls } = createHarness({ simulation });
  const result = await service.apply(
    TOGGLE_FULL_SCREEN,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(result.ok, true);
  assert.equal(simulation.state.isFullScreen, false);
  assert.ok(calls.includes('fullscreen:false'));
});

test('rejects a minimized window without changing its full-screen state', async () => {
  const simulation = createSimulatedWindow({ isMinimized: true });
  const { service, calls } = createHarness({ simulation });
  const result = await service.apply(
    TOGGLE_FULL_SCREEN,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(result.ok === false && result.reason, 'minimized');
  assert.equal(
    calls.some((call) => call.startsWith('fullscreen:')),
    false,
  );
});

test('reports unapplied with the helper reason when the window refuses full screen', async () => {
  const simulation = createSimulatedWindow({
    fullScreenResponse: fail<SetFrameData>(
      'unapplied',
      'Attribute is not settable',
    ),
  });
  const { service } = createHarness({ simulation });
  const result = await service.apply(
    TOGGLE_FULL_SCREEN,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.deepEqual(result, {
    ok: false,
    reason: 'unapplied',
    message: 'Attribute is not settable',
  });
});

test('records no restore frame for a full-screen toggle', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({ simulation });
  await service.apply(TOGGLE_FULL_SCREEN, NO_OPTIONS, shortcutInvocation());
  const restore = await service.apply(
    RESTORE,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(restore.ok === false && restore.reason, 'no-history');
});
