// biome-ignore-all lint/style/noMagicNumbers: fixture geometry and app counts read best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createHarness,
  DISPLAY,
  FINDER,
  fail,
  focusedWindow,
  type HarnessOptions,
  LEFT_HALF,
  NO_OPTIONS,
  OTHER_APP,
  ok,
  paletteInvocation,
  REQUESTED_LEFT_HALF,
  writeOk,
} from './window-management.harness';
import { createSimulatedWindow } from './window-management.simulated-window';
import { UNKNOWN_WINDOW_MESSAGE } from './window-management-failures';
import { WINDOW_HANDLE_TTL_MS } from './window-management-handles';
import type {
  DisplayInfo,
  DisplayListResult,
  HelperEnvelope,
  SetFrameRequest,
  WindowApplyResult,
  WindowDescriptor,
  WindowFocusResult,
  WindowInfo,
  WindowLayoutSpec,
  WindowListQuery,
  WindowListResult,
} from './window-management-types';

type Harness = ReturnType<typeof createHarness>;

const RESTORE: WindowLayoutSpec = { kind: 'restore' };
const RIGHT_HALF: WindowLayoutSpec = {
  kind: 'fraction',
  x: 0.5,
  y: 0,
  width: 0.5,
  height: 1,
};
const SECOND_DISPLAY: DisplayInfo = {
  id: 'second',
  isPrimary: false,
  frame: { x: 1440, y: 0, width: 1440, height: 900 },
  workArea: { x: 1440, y: 25, width: 1440, height: 875 },
};
const ORIGINAL_WINDOW_FRAME = { x: 100, y: 100, width: 600, height: 400 };
const FINDER_NOTES = focusedWindow({ windowKey: '42:7', title: 'Notes' });
const FINDER_DOWNLOADS = focusedWindow({
  windowKey: '42:8',
  title: 'Downloads',
});
const OTHER_WINDOW = focusedWindow({
  bundleId: OTHER_APP.bundleId,
  windowKey: '43:9',
  title: 'Other Window',
});

function windowsOfPid(pid: number): Promise<HelperEnvelope<WindowInfo[]>> {
  if (pid === FINDER.pid) {
    return Promise.resolve(ok([FINDER_NOTES, FINDER_DOWNLOADS]));
  }
  if (pid === OTHER_APP.pid) {
    return Promise.resolve(ok([OTHER_WINDOW]));
  }
  return Promise.resolve(ok([]));
}

function flushPromises() {
  return new Promise((resolve) => setImmediate(resolve));
}

function reasonOf(
  result: WindowApplyResult | WindowFocusResult | DisplayListResult,
) {
  return result.ok === false ? result.reason : null;
}

function windowsOf(result: WindowListResult): WindowDescriptor[] {
  return result.ok ? result.windows : [];
}

function titlesOf(result: WindowListResult) {
  return windowsOf(result).map((window) => window.title);
}

function lastFrameOf(requests: SetFrameRequest[]) {
  return requests.at(-1)?.frame;
}

function countCalls(calls: string[], matches: (call: string) => boolean) {
  return calls.filter(matches).length;
}

async function listedHandle(
  service: Harness['service'],
  query: WindowListQuery,
) {
  const windows = windowsOf(await service.listWindows(query));
  const [only] = windows;
  if (windows.length !== 1 || !only) {
    throw new Error(`expected one listed window, got ${windows.length}`);
  }
  return only.handle;
}

test('lists displays when access is allowed', async () => {
  const { service } = createHarness({});
  assert.deepEqual(await service.listDisplays(), {
    ok: true,
    displays: [DISPLAY],
  });
});

test('refuses to list displays without window access', async () => {
  const { service, calls } = createHarness({ permission: 'denied' });
  assert.equal(reasonOf(await service.listDisplays()), 'denied');
  assert.deepEqual(calls, ['permissions']);
});

test('lists no displays when window management is unsupported', async () => {
  const { service, calls } = createHarness({ supported: false });
  assert.equal(reasonOf(await service.listDisplays()), 'unsupported');
  assert.deepEqual(calls, []);
});

test('filters windows by app identifier case-insensitively', async () => {
  const { service, calls } = createHarness({ windowsRead: windowsOfPid });
  const result = await service.listWindows({ appId: 'COM.APPLE.FINDER' });
  assert.deepEqual(titlesOf(result), ['Notes', 'Downloads']);
  assert.equal(calls.includes('windows:43'), false);
});

test('filters windows by app name substring case-insensitively', async () => {
  const { service, calls } = createHarness({ windowsRead: windowsOfPid });
  const result = await service.listWindows({ appName: 'other harness' });
  assert.deepEqual(titlesOf(result), ['Other Window']);
  assert.equal(calls.includes('windows:42'), false);
});

test('filters windows by title substring case-insensitively', async () => {
  const { service } = createHarness({ windowsRead: windowsOfPid });
  const result = await service.listWindows({ title: 'NOTES' });
  assert.deepEqual(titlesOf(result), ['Notes']);
});

test('skips an app whose window read fails and keeps the other apps', async () => {
  const { service } = createHarness({
    windowsRead: (pid) =>
      pid === OTHER_APP.pid
        ? Promise.resolve(fail<WindowInfo[]>('error', 'helper failed'))
        : windowsOfPid(pid),
  });
  assert.deepEqual(titlesOf(await service.listWindows({})), [
    'Notes',
    'Downloads',
  ]);
});

test('describes windows with an opaque handle and no process, window key or app bundle field', async () => {
  const { service } = createHarness({ windowsRead: windowsOfPid });
  const [first] = windowsOf(
    await service.listWindows({ appId: FINDER.bundleId }),
  );
  assert.deepEqual(Object.keys(first ?? {}).sort(), [
    'appId',
    'appName',
    'displayId',
    'frame',
    'handle',
    'isFullScreen',
    'isMinimized',
    'title',
  ]);
});

test('reuses the handle when the same window is listed again', async () => {
  const { service } = createHarness({ windowsRead: windowsOfPid });
  const first = await listedHandle(service, { title: 'Notes' });
  const second = await listedHandle(service, { title: 'Notes' });
  assert.equal(first, second);
});

test('reads at most forty apps when listing windows', async () => {
  const apps = Array.from({ length: 45 }, (_, index) => ({
    pid: 100 + index,
    bundleId: `app.number.${index}`,
    name: `App ${index}`,
  }));
  const { service, calls } = createHarness({
    runningApps: () => Promise.resolve(ok(apps)),
  });
  await service.listWindows({});
  assert.equal(
    countCalls(calls, (call) => call.startsWith('windows:')),
    40,
  );
});

test('applies a layout to a listed window by handle', async () => {
  const { service, setFrameRequests } = createHarness({
    windowsRead: windowsOfPid,
  });
  const handle = await listedHandle(service, {
    appId: FINDER.bundleId,
    title: 'Notes',
  });
  const result = await service.applyToWindow(handle, LEFT_HALF, NO_OPTIONS);
  assert.deepEqual(result, {
    ok: true,
    outcome: 'exact',
    changed: true,
    frame: REQUESTED_LEFT_HALF,
  });
  assert.deepEqual(lastFrameOf(setFrameRequests), REQUESTED_LEFT_HALF);
});

test('reports a partial write on a listed window with its reason', async () => {
  const partialFrame = { ...REQUESTED_LEFT_HALF, width: 700 };
  const { service } = createHarness({
    windowsRead: windowsOfPid,
    writes: [writeOk(partialFrame, 'partial', 'Minimum width reached')],
  });
  const handle = await listedHandle(service, { title: 'Notes' });
  assert.deepEqual(await service.applyToWindow(handle, LEFT_HALF, NO_OPTIONS), {
    ok: true,
    outcome: 'partial',
    changed: true,
    frame: partialFrame,
    reason: 'Minimum width reached',
  });
});

test('rejects an unknown handle without reading any window', async () => {
  const { service, calls } = createHarness({});
  assert.deepEqual(
    await service.applyToWindow('never-issued', LEFT_HALF, NO_OPTIONS),
    { ok: false, reason: 'unknown-window', message: UNKNOWN_WINDOW_MESSAGE },
  );
  assert.deepEqual(calls, []);
});

test('rejects an invalid spec on a listed window before writing', async () => {
  const { service, setFrameRequests } = createHarness({
    windowsRead: windowsOfPid,
  });
  const handle = await listedHandle(service, { title: 'Notes' });
  const result = await service.applyToWindow(
    handle,
    { kind: 'fraction', x: 2, y: 0, width: 1, height: 1 },
    NO_OPTIONS,
  );
  assert.equal(reasonOf(result), 'invalid-spec');
  assert.deepEqual(setFrameRequests, []);
});

test('expires a handle after its time to live', async () => {
  let current = 0;
  const { service } = createHarness({
    windowsRead: windowsOfPid,
    now: () => current,
  });
  const handle = await listedHandle(service, { title: 'Notes' });
  current += WINDOW_HANDLE_TTL_MS;
  assert.equal(
    reasonOf(await service.applyToWindow(handle, LEFT_HALF, NO_OPTIONS)),
    'unknown-window',
  );
});

test('forgets a handle whose window is gone and reports target-lost', async () => {
  let finderWindows: WindowInfo[] = [FINDER_NOTES];
  const { service } = createHarness({
    windowsRead: (pid) =>
      Promise.resolve(ok(pid === FINDER.pid ? finderWindows : [])),
  });
  const handle = await listedHandle(service, { title: 'Notes' });
  finderWindows = [];
  assert.equal(
    reasonOf(await service.applyToWindow(handle, LEFT_HALF, NO_OPTIONS)),
    'target-lost',
  );
  assert.equal(
    reasonOf(await service.applyToWindow(handle, LEFT_HALF, NO_OPTIONS)),
    'unknown-window',
  );
});

test('restores a listed window to the frame it had before a handle apply', async () => {
  const { service, setFrameRequests } = createHarness({
    windowsRead: windowsOfPid,
  });
  const handle = await listedHandle(service, {
    appId: FINDER.bundleId,
    title: 'Notes',
  });
  await service.applyToWindow(handle, LEFT_HALF, NO_OPTIONS);
  const restored = await service.applyToWindow(handle, RESTORE, NO_OPTIONS);
  assert.equal(restored.ok, true);
  assert.deepEqual(lastFrameOf(setFrameRequests), ORIGINAL_WINDOW_FRAME);
});

test('reports no-history when restoring a listed window that was never moved', async () => {
  const { service } = createHarness({ windowsRead: windowsOfPid });
  const handle = await listedHandle(service, { title: 'Notes' });
  assert.equal(
    reasonOf(await service.applyToWindow(handle, RESTORE, NO_OPTIONS)),
    'no-history',
  );
});

test('toggles full screen on a listed window through its handle', async () => {
  const simulation = createSimulatedWindow({});
  const { service } = createHarness({
    simulation,
    windowsRead: (pid) =>
      pid === FINDER.pid ? simulation.windowsRead() : windowsOfPid(pid),
  });
  const handle = await listedHandle(service, { title: 'Simulated window' });
  const result = await service.applyToWindow(
    handle,
    { kind: 'toggle-full-screen' },
    NO_OPTIONS,
  );
  assert.equal(result.ok, true);
  assert.equal(simulation.state.isFullScreen, true);
});

test('moves a listed window to the next display through its handle', async () => {
  const { service, setFrameRequests } = createHarness({
    windowsRead: windowsOfPid,
    displays: () => Promise.resolve(ok([DISPLAY, SECOND_DISPLAY])),
  });
  const handle = await listedHandle(service, { title: 'Notes' });
  const result = await service.applyToWindow(
    handle,
    { kind: 'display', direction: 'next' },
    NO_OPTIONS,
  );
  assert.equal(result.ok, true);
  assert.ok(
    (lastFrameOf(setFrameRequests)?.x ?? 0) >= SECOND_DISPLAY.workArea.x,
  );
});

test('serializes a handle apply behind an in-flight apply on the same pid', async () => {
  let release: () => void = () => undefined;
  const writeGate = new Promise<void>((resolve) => {
    release = resolve;
  });
  const { service, calls } = createHarness({
    windowsRead: windowsOfPid,
    writeGate,
  });
  const handle = await listedHandle(service, {
    appId: FINDER.bundleId,
    title: 'Notes',
  });
  const palette = service.apply(LEFT_HALF, NO_OPTIONS, paletteInvocation());
  await flushPromises();
  const byHandle = service.applyToWindow(handle, RIGHT_HALF, NO_OPTIONS);
  await flushPromises();
  const readsOfPid = (call: string) => call === 'windows:42';
  assert.equal(countCalls(calls, readsOfPid), 1);
  release();
  const [first, second] = await Promise.all([palette, byHandle]);
  assert.deepEqual([first.ok, second.ok], [true, true]);
  assert.equal(countCalls(calls, readsOfPid), 2);
});

test('raises a listed window through its handle', async () => {
  const { service, calls } = createHarness({ windowsRead: windowsOfPid });
  const handle = await listedHandle(service, { title: 'Notes' });
  assert.deepEqual(await service.focusWindow(handle), { ok: true });
  assert.ok(calls.includes('raise:42:7'));
});

test('forgets a handle when raising reports target-lost', async () => {
  const { service } = createHarness({
    windowsRead: windowsOfPid,
    raiseResponse: () =>
      fail<null>('target-lost', 'Window is no longer available'),
  });
  const handle = await listedHandle(service, { title: 'Notes' });
  assert.deepEqual(await service.focusWindow(handle), {
    ok: false,
    reason: 'target-lost',
    message: 'Window is no longer available',
  });
  assert.deepEqual(await service.focusWindow(handle), {
    ok: false,
    reason: 'unknown-window',
    message: UNKNOWN_WINDOW_MESSAGE,
  });
});

test('rejects raising an unknown handle without touching the window', async () => {
  const { service, calls } = createHarness({});
  assert.equal(
    reasonOf(await service.focusWindow('never-issued')),
    'unknown-window',
  );
  assert.deepEqual(calls, []);
});

test('refuses to raise a window without window access', async () => {
  const options: HarnessOptions = { windowsRead: windowsOfPid };
  const { service, calls } = createHarness(options);
  const handle = await listedHandle(service, { title: 'Notes' });
  options.permission = 'denied';
  assert.equal(reasonOf(await service.focusWindow(handle)), 'denied');
  assert.equal(
    countCalls(calls, (call) => call.startsWith('raise:')),
    0,
  );
});

test('refuses to raise a listed window when window management is unsupported', async () => {
  const options: HarnessOptions = { windowsRead: windowsOfPid };
  const { service } = createHarness(options);
  const handle = await listedHandle(service, { title: 'Notes' });
  options.supported = false;
  assert.equal(reasonOf(await service.focusWindow(handle)), 'unsupported');
});
