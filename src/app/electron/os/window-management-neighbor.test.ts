// biome-ignore-all lint/style/noMagicNumbers: fixture geometry reads best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  adjacentDisplay,
  cycleSequence,
  moveFrameToDisplay,
} from './window-display-geometry';
import { computeTargetFrame, pickDisplayForFrame } from './window-layouts';
import { createWindowManagementService } from './window-management';
import type { WindowManagementEngine } from './window-management-engine-port';
import {
  PALETTE_TARGET_CHANGED,
  SHORTCUT_FOCUS_CHANGED,
} from './window-management-failures';
import { NO_FOCUSED_WINDOW_MESSAGE } from './window-management-neighbor';
import type {
  DisplayInfo,
  Frame,
  HelperCode,
  HelperEnvelope,
  MacosWindowManager,
  NeighborDirection,
  RunningApp,
  WindowFocusResult,
  WindowFocusTarget,
  WindowInfo,
  WindowInvocation,
  WindowInvocationPorts,
} from './window-management-types';

const FINDER: WindowFocusTarget = { bundleId: 'com.apple.finder', pid: 42 };
const SAFARI: WindowFocusTarget = { bundleId: 'com.apple.Safari', pid: 43 };
const NOTES: WindowFocusTarget = { bundleId: 'com.apple.Notes', pid: 44 };
const SELF_PID = 99;

const DISPLAY: DisplayInfo = {
  id: 'main',
  isPrimary: true,
  frame: { x: 0, y: 0, width: 1440, height: 900 },
  workArea: { x: 0, y: 0, width: 1440, height: 900 },
};

const LEFT_FRAME: Frame = { x: 0, y: 0, width: 500, height: 900 };
const MIDDLE_FRAME: Frame = { x: 500, y: 0, width: 500, height: 900 };
const RIGHT_FRAME: Frame = { x: 1000, y: 0, width: 440, height: 900 };

const APPS: RunningApp[] = [
  { pid: FINDER.pid, bundleId: FINDER.bundleId, name: 'Finder' },
  { pid: SAFARI.pid, bundleId: SAFARI.bundleId, name: 'Safari' },
  { pid: NOTES.pid, bundleId: NOTES.bundleId, name: 'Notes' },
  { pid: SELF_PID, bundleId: 'app.nevermind', name: 'Nevermind' },
];

interface FakeDesktop {
  focused: Record<number, WindowInfo | undefined>;
  windows: Record<number, WindowInfo[]>;
  permission: 'allowed' | 'denied';
  raised: [number, string][];
  frontmost: WindowFocusTarget | null;
}

function ok<Data>(data: Data): HelperEnvelope<Data> {
  return { ok: true, code: 'ok', data };
}

function failedEnvelope<Data>(
  code: HelperCode,
  message: string,
): HelperEnvelope<Data> {
  return { ok: false, code, message };
}

function windowOf(
  target: WindowFocusTarget,
  windowKey: string,
  frame: Frame,
  overrides: Partial<WindowInfo> = {},
): WindowInfo {
  return {
    bundleId: target.bundleId,
    windowKey,
    title: windowKey,
    frame,
    displayId: 'main',
    isFullScreen: false,
    isMinimized: false,
    isMovable: true,
    isResizable: true,
    ...overrides,
  };
}

function desktopWith(overrides: Partial<FakeDesktop> = {}): FakeDesktop {
  return {
    focused: { [FINDER.pid]: windowOf(FINDER, '42:1', LEFT_FRAME) },
    windows: {},
    permission: 'allowed',
    raised: [],
    frontmost: FINDER,
    ...overrides,
  };
}

function fakeManager(desktop: FakeDesktop): MacosWindowManager {
  return {
    permissionState: () => Promise.resolve(desktop.permission),
    displays: () => Promise.resolve(ok([DISPLAY])),
    focusedWindow: (pid) => {
      const focused = desktop.focused[pid];
      return Promise.resolve(
        focused
          ? ok(focused)
          : failedEnvelope<WindowInfo>('no-window', 'no focused window'),
      );
    },
    setFrame: () => Promise.reject(new Error('focus does not move windows')),
    setFullScreen: () =>
      Promise.reject(new Error('focus does not change full screen')),
    runningApps: () => Promise.resolve(ok(APPS)),
    windows: (pid) => Promise.resolve(ok(desktop.windows[pid] ?? [])),
    raiseWindow: (pid, windowKey) => {
      desktop.raised.push([pid, windowKey]);
      return Promise.resolve(ok(null));
    },
  };
}

function fakePorts(
  paletteTarget: WindowFocusTarget | null,
  desktop: FakeDesktop,
): WindowInvocationPorts {
  return {
    paletteFocusTarget: () => paletteTarget,
    hidePaletteAndRestoreFocus: (target) => {
      desktop.frontmost = target;
      return Promise.resolve(true);
    },
    frontmostTarget: () => Promise.resolve(desktop.frontmost),
  };
}

const ENGINE: WindowManagementEngine = {
  computeTargetFrame,
  pickDisplayForFrame,
  moveFrameToDisplay,
  cycleSequence,
  adjacentDisplay,
};

function serviceFor(
  desktop: FakeDesktop,
  paletteTarget: WindowFocusTarget | null = FINDER,
  options: { supported?: boolean } = {},
) {
  return createWindowManagementService({
    manager: fakeManager(desktop),
    ports: fakePorts(paletteTarget, desktop),
    engine: ENGINE,
    supported: () => options.supported ?? true,
    selfPid: SELF_PID,
  });
}

function paletteFrom(target: WindowFocusTarget | null): WindowInvocation {
  return { source: 'palette', target };
}

function shortcutFrom(target: WindowFocusTarget | null): WindowInvocation {
  return { source: 'shortcut', target };
}

function focusNeighbor(
  desktop: FakeDesktop,
  direction: NeighborDirection,
  invocation: WindowInvocation = paletteFrom(FINDER),
  options: { wrap?: boolean } = {},
): Promise<WindowFocusResult> {
  return serviceFor(desktop).focusNeighbor(direction, options, invocation);
}

function otherAppWindows(desktop: FakeDesktop) {
  desktop.windows[SAFARI.pid] = [windowOf(SAFARI, '43:7', MIDDLE_FRAME)];
  desktop.windows[NOTES.pid] = [windowOf(NOTES, '44:1', RIGHT_FRAME)];
}

test('palette focus raises the nearest window of another app and never Nevermind', async () => {
  const desktop = desktopWith();
  desktop.windows[SELF_PID] = [
    windowOf(
      { bundleId: 'app.nevermind', pid: SELF_PID },
      '99:1',
      MIDDLE_FRAME,
    ),
  ];
  otherAppWindows(desktop);
  assert.deepEqual(await focusNeighbor(desktop, 'right'), { ok: true });
  assert.deepEqual(desktop.raised, [[SAFARI.pid, '43:7']]);
});

test('a neighbouring window of the same app is raised without deadlocking its queue', async () => {
  const desktop = desktopWith();
  desktop.windows[FINDER.pid] = [
    windowOf(FINDER, '42:1', LEFT_FRAME),
    windowOf(FINDER, '42:2', MIDDLE_FRAME),
  ];
  assert.deepEqual(await focusNeighbor(desktop, 'right'), { ok: true });
  assert.deepEqual(desktop.raised, [[FINDER.pid, '42:2']]);
});

test('minimized and full-screen windows are never chosen as neighbours', async () => {
  const desktop = desktopWith();
  desktop.windows[SAFARI.pid] = [
    windowOf(SAFARI, '43:7', MIDDLE_FRAME, { isMinimized: true }),
    windowOf(SAFARI, '43:8', MIDDLE_FRAME, { isFullScreen: true }),
  ];
  desktop.windows[NOTES.pid] = [windowOf(NOTES, '44:1', RIGHT_FRAME)];
  assert.deepEqual(await focusNeighbor(desktop, 'right'), { ok: true });
  assert.deepEqual(desktop.raised, [[NOTES.pid, '44:1']]);
});

test('no window in the direction fails with no-neighbor and raises nothing', async () => {
  const desktop = desktopWith();
  otherAppWindows(desktop);
  const result = await focusNeighbor(desktop, 'up');
  assert.equal(result.ok === false && result.reason, 'no-neighbor');
  assert.deepEqual(desktop.raised, []);
});

test('wrap raises the farthest window on the opposite side', async () => {
  const desktop = desktopWith();
  otherAppWindows(desktop);
  assert.deepEqual(
    await focusNeighbor(desktop, 'left', paletteFrom(FINDER), { wrap: true }),
    { ok: true },
  );
  assert.deepEqual(desktop.raised, [[NOTES.pid, '44:1']]);
});

test('palette focus restores the target app before reading its window', async () => {
  const desktop = desktopWith({ frontmost: SAFARI });
  otherAppWindows(desktop);
  assert.deepEqual(await focusNeighbor(desktop, 'right'), { ok: true });
  assert.equal(desktop.frontmost?.pid, FINDER.pid);
});

test('palette focus fails when the captured target changed and raises nothing', async () => {
  const desktop = desktopWith();
  otherAppWindows(desktop);
  const result = await serviceFor(desktop, SAFARI).focusNeighbor(
    'right',
    {},
    paletteFrom(FINDER),
  );
  assert.equal(result.ok === false && result.message, PALETTE_TARGET_CHANGED);
  assert.deepEqual(desktop.raised, []);
});

test('shortcut focus fails when the targeted app is no longer frontmost', async () => {
  const desktop = desktopWith({ frontmost: SAFARI });
  otherAppWindows(desktop);
  const result = await focusNeighbor(desktop, 'right', shortcutFrom(FINDER));
  assert.equal(result.ok === false && result.message, SHORTCUT_FOCUS_CHANGED);
  assert.deepEqual(desktop.raised, []);
});

test('shortcut focus uses the frontmost target it captured', async () => {
  const desktop = desktopWith();
  otherAppWindows(desktop);
  assert.deepEqual(
    await focusNeighbor(desktop, 'right', shortcutFrom(FINDER)),
    { ok: true },
  );
  assert.deepEqual(desktop.raised, [[SAFARI.pid, '43:7']]);
});

test('a target app without a focused window fails with a focus-specific message', async () => {
  const desktop = desktopWith({ focused: {} });
  const result = await focusNeighbor(desktop, 'right');
  assert.equal(result.ok === false && result.reason, 'no-window');
  assert.equal(
    result.ok === false && result.message,
    NO_FOCUSED_WINDOW_MESSAGE,
  );
});

test('missing target, Nevermind target, and unsupported platform fail before touching windows', async () => {
  const desktop = desktopWith();
  otherAppWindows(desktop);
  const noTarget = await serviceFor(desktop, null).focusNeighbor(
    'right',
    {},
    paletteFrom(null),
  );
  const selfTarget = await focusNeighbor(
    desktop,
    'right',
    paletteFrom({ bundleId: 'app.nevermind', pid: SELF_PID }),
  );
  const unsupported = await serviceFor(desktop, FINDER, {
    supported: false,
  }).focusNeighbor('right', {}, paletteFrom(FINDER));
  assert.equal(noTarget.ok === false && noTarget.reason, 'no-target');
  assert.equal(selfTarget.ok === false && selfTarget.reason, 'no-target');
  assert.equal(unsupported.ok === false && unsupported.reason, 'unsupported');
  assert.deepEqual(desktop.raised, []);
});

test('denied window access fails with the denied reason', async () => {
  const desktop = desktopWith({ permission: 'denied' });
  otherAppWindows(desktop);
  const result = await focusNeighbor(desktop, 'right');
  assert.equal(result.ok === false && result.reason, 'denied');
  assert.deepEqual(desktop.raised, []);
});
