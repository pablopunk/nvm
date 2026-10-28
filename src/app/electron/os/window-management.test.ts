// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createHarness,
  fail,
  focusedWindow,
  gapOptions,
  LEFT_HALF,
  NO_OPTIONS,
  ok,
  paletteInvocation,
  REQUESTED_LEFT_HALF,
  shortcutInvocation,
  UNRELATED_FRONTMOST,
} from './window-management.harness';
import type {
  DisplayInfo,
  HelperCode,
  WindowApplyFailureReason,
  WindowInfo,
  WindowLayoutSpec,
  WindowPermissionState,
} from './window-management-types';

const INVALID_LAYOUTS: [WindowLayoutSpec, number][] = [
  [{ kind: 'fraction', x: 0, y: 0, width: 0, height: 1 }, 0],
  [{ kind: 'fraction', x: 0, y: 0, width: 1.2, height: 1 }, 0],
  [{ kind: 'fraction', x: 0.6, y: 0, width: 0.5, height: 1 }, 0],
  [{ kind: 'fraction', x: Number.NaN, y: 0, width: 0.5, height: 1 }, 0],
  [{ kind: 'fraction', x: 0, y: 1.5, width: 0.5, height: 1 }, 0],
  [{ kind: 'fraction', x: 0, y: 0, width: 0.5, height: -1 }, 0],
  [LEFT_HALF, 1.5],
  [LEFT_HALF, 101],
  [LEFT_HALF, -1],
  [{ kind: 'center' }, Number.NaN],
];

const MAPPED_HELPER_CODES: [HelperCode, WindowApplyFailureReason][] = [
  ['no-window', 'no-window'],
  ['target-lost', 'target-lost'],
  ['unsupported', 'unsupported'],
  ['denied', 'denied'],
  ['error', 'unapplied'],
  ['unapplied', 'unapplied'],
];

const PERMISSION_BLOCKS: WindowPermissionState[] = ['denied', 'unknown'];
const REOPEN_NEVERMIND = /Reopen Nevermind/;

test('reports availability from the supported probe', () => {
  assert.equal(createHarness().service.available(), true);
  assert.equal(createHarness({ supported: false }).service.available(), false);
});

for (const [spec, gap] of INVALID_LAYOUTS) {
  test(`rejects the malformed layout ${JSON.stringify(spec)} with gap ${gap} before touching any window`, async () => {
    const { service, calls } = createHarness();
    const result = await service.apply(
      spec,
      gapOptions(gap),
      paletteInvocation(),
    );
    assert.equal(result.ok === false && result.reason, 'invalid-spec');
    assert.deepEqual(calls, []);
  });
}

test('refuses to run when the platform is unsupported', async () => {
  const { service, calls } = createHarness({ supported: false });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    paletteInvocation(),
  );
  assert.deepEqual(result, {
    ok: false,
    reason: 'unsupported',
    message: 'Window management is not supported on this device.',
  });
  assert.deepEqual(calls, []);
});

test('tells the user to reopen Nevermind when a palette invocation has no captured target', async () => {
  const { service, calls } = createHarness();
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    paletteInvocation(null),
  );
  assert.equal(result.ok === false && result.reason, 'no-target');
  assert.match(result.ok === false ? result.message : '', REOPEN_NEVERMIND);
  assert.deepEqual(calls, []);
});

test('reports a shortcut invocation without a frontmost snapshot as no target', async () => {
  const { service, calls } = createHarness();
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(null),
  );
  assert.equal(result.ok === false && result.reason, 'no-target');
  assert.deepEqual(calls, []);
});

test('fails closed when the palette cannot restore focus to the captured app', async () => {
  const { service, calls } = createHarness({ restored: false });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    paletteInvocation(),
  );
  assert.equal(result.ok === false && result.reason, 'focus-not-restored');
  assert.deepEqual(calls, ['restore:42']);
});

test('refuses to act when focus is not on the captured app after the palette restores it', async () => {
  const { service, calls, setFrameRequests } = createHarness({
    frontmostAfterRestore: () => UNRELATED_FRONTMOST,
  });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    paletteInvocation(),
  );
  assert.equal(result.ok === false && result.reason, 'focus-not-restored');
  assert.deepEqual(calls, ['restore:42', 'frontmost']);
  assert.deepEqual(setFrameRequests, []);
});

test('moves the window on the palette path only after verified focus restoration', async () => {
  const { service, calls, setFrameRequests } = createHarness();
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    paletteInvocation(),
  );
  assert.deepEqual(result, {
    ok: true,
    outcome: 'exact',
    changed: true,
    frame: REQUESTED_LEFT_HALF,
  });
  assert.equal(calls[0], 'restore:42');
  assert.deepEqual(setFrameRequests, [
    { pid: 42, windowKey: '42:7', frame: REQUESTED_LEFT_HALF },
  ]);
});

test('moves the window on the shortcut path without restoring focus', async () => {
  const { service, calls, setFrameRequests } = createHarness();
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.deepEqual(result, {
    ok: true,
    outcome: 'exact',
    changed: true,
    frame: REQUESTED_LEFT_HALF,
  });
  assert.equal(
    calls.some((call) => call.startsWith('restore')),
    false,
  );
  assert.equal(setFrameRequests.length, 1);
});

test('refuses the shortcut path when the frontmost pid changed before mutation', async () => {
  const { service, calls, setFrameRequests } = createHarness({
    frontmost: UNRELATED_FRONTMOST,
  });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(result.ok === false && result.reason, 'focus-not-restored');
  assert.equal(
    calls.some((call) => call.startsWith('restore')),
    false,
  );
  assert.deepEqual(setFrameRequests, []);
});

for (const permission of PERMISSION_BLOCKS) {
  test(`denies the layout when the Accessibility probe reports ${permission}`, async () => {
    const { service, calls, setFrameRequests } = createHarness({ permission });
    const result = await service.apply(
      LEFT_HALF,
      NO_OPTIONS,
      shortcutInvocation(),
    );
    assert.equal(result.ok === false && result.reason, 'denied');
    assert.equal(
      calls.some((call) => call.startsWith('focused')),
      false,
    );
    assert.deepEqual(setFrameRequests, []);
  });
}

for (const [code, reason] of MAPPED_HELPER_CODES) {
  test(`maps focused-window helper code ${code} to ${reason} with the helper message`, async () => {
    const { service } = createHarness({
      focusedRead: async () => fail<WindowInfo>(code, 'helper says'),
    });
    const result = await service.apply(
      LEFT_HALF,
      NO_OPTIONS,
      shortcutInvocation(),
    );
    assert.deepEqual(result, { ok: false, reason, message: 'helper says' });
  });
}

test('refuses a focused window whose app no longer matches the captured bundle', async () => {
  const { service, setFrameRequests } = createHarness({
    focusedRead: async () => ok(focusedWindow({ bundleId: 'com.reused.pid' })),
  });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(result.ok === false && result.reason, 'target-lost');
  assert.deepEqual(setFrameRequests, []);
});

test('refuses full-screen and minimized windows with distinct reasons', async () => {
  const fullScreen = createHarness({
    focusedRead: async () => ok(focusedWindow({ isFullScreen: true })),
  });
  const minimized = createHarness({
    focusedRead: async () => ok(focusedWindow({ isMinimized: true })),
  });
  const fullScreenResult = await fullScreen.service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  const minimizedResult = await minimized.service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(
    fullScreenResult.ok === false && fullScreenResult.reason,
    'full-screen',
  );
  assert.equal(
    minimizedResult.ok === false && minimizedResult.reason,
    'minimized',
  );
  assert.deepEqual(fullScreen.setFrameRequests, []);
  assert.deepEqual(minimized.setFrameRequests, []);
});

test('maps display helper failures to their reason', async () => {
  const { service, setFrameRequests } = createHarness({
    displays: async () =>
      fail<DisplayInfo[]>('unsupported', 'no displays helper'),
  });
  assert.deepEqual(
    await service.apply(LEFT_HALF, NO_OPTIONS, shortcutInvocation()),
    {
      ok: false,
      reason: 'unsupported',
      message: 'no displays helper',
    },
  );
  assert.deepEqual(setFrameRequests, []);
});

test('refuses to place a window when no display is reported', async () => {
  const { service, setFrameRequests } = createHarness({
    displays: async () => ok<DisplayInfo[]>([]),
  });
  const result = await service.apply(
    LEFT_HALF,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(result.ok === false && result.reason, 'unsupported');
  assert.deepEqual(setFrameRequests, []);
});

test('keeps the size of a non-resizable window and clamps it inside the work area', async () => {
  const { service, setFrameRequests } = createHarness({
    focusedRead: async () => ok(focusedWindow({ isResizable: false })),
  });
  const rightEdge: WindowLayoutSpec = {
    kind: 'fraction',
    x: 0.75,
    y: 0,
    width: 0.25,
    height: 1,
  };
  const result = await service.apply(
    rightEdge,
    NO_OPTIONS,
    shortcutInvocation(),
  );
  assert.equal(result.ok, true);
  assert.deepEqual(setFrameRequests[0]?.frame, {
    x: 840,
    y: 263,
    width: 600,
    height: 400,
  });
});
