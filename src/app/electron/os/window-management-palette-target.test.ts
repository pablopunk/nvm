import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createHarness,
  LEFT_HALF,
  NO_OPTIONS,
  OTHER_APP,
  paletteInvocation,
} from './window-management.harness';
import { PALETTE_FOCUS_NOT_RESTORED } from './window-management-failures';

const TARGET_CHANGED_MESSAGE =
  'The target app changed. Reopen Nevermind from the window you want to move.';

test('refuses the palette path when the captured app changed before consumption', async () => {
  const { service, calls, setFrameRequests } = createHarness({
    paletteTarget: OTHER_APP,
  });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    paletteInvocation(),
  );
  assert.deepEqual(result, {
    ok: false,
    reason: 'focus-not-restored',
    message: TARGET_CHANGED_MESSAGE,
  });
  assert.deepEqual(calls, []);
  assert.deepEqual(setFrameRequests, []);
});

test('keeps the focus message when the palette still fails to restore the captured app', async () => {
  const { service } = createHarness({ restored: false });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    paletteInvocation(),
  );
  assert.deepEqual(result, {
    ok: false,
    reason: 'focus-not-restored',
    message: PALETTE_FOCUS_NOT_RESTORED,
  });
});
