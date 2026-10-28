// biome-ignore-all lint/suspicious/noExplicitAny: leaking fixtures deliberately carry fields the host must strip.
// biome-ignore-all lint/style/noMagicNumbers: fixture geometry and timings are the asserted values.
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  DEFAULT_WINDOW_MANAGEMENT_OPTIONS,
  type DisplayInfo,
  type WindowApplyResult,
  type WindowDescriptor,
  type WindowLayoutSpec,
  type WindowManagementService,
} from '../os/window-management-types';
import {
  clampWaitTimeoutMs,
  createDesktopWindowManagement,
  type DesktopWindowManagementDeps,
} from './extension-desktop-window-management';
import {
  runInWindowInvocationScope,
  windowInvocationFromScope,
} from './window-invocation-scope';

const HALF: WindowLayoutSpec = {
  kind: 'fraction',
  x: 0,
  y: 0,
  width: 0.5,
  height: 1,
};
const FOCUS_TARGET = { bundleId: 'app.example.Browser', pid: 411 };
const LEFT_HALF_FRAME = { x: 0, y: 25, width: 720, height: 875 };
const APPLIED: WindowApplyResult = {
  ok: true,
  outcome: 'exact',
  changed: true,
  frame: LEFT_HALF_FRAME,
};
const DISPLAY: DisplayInfo = {
  id: 'display-1',
  isPrimary: true,
  frame: { x: 0, y: 0, width: 1440, height: 900 },
  workArea: { x: 0, y: 25, width: 1440, height: 875 },
};
const WINDOW: WindowDescriptor = {
  handle: 'opaque-handle-1',
  appId: 'app.example.Browser',
  appName: 'Browser',
  title: 'Start page',
  frame: LEFT_HALF_FRAME,
  displayId: 'display-1',
  isFullScreen: false,
  isMinimized: false,
};
const LEAKY_FIELDS = { pid: 411, windowKey: '411:9001' };

type StubCalls = Record<string, unknown[][]>;

function recordingStub(
  calls: StubCalls,
  name: string,
  implementation: (...args: any[]) => unknown,
) {
  return (...args: unknown[]) => {
    calls[name] = [...(calls[name] ?? []), args];
    return Promise.resolve(implementation(...args));
  };
}

function createServiceStub(
  overrides: Partial<Omit<WindowManagementService, 'available'>> = {},
  available = true,
) {
  const calls: StubCalls = {};
  const service: WindowManagementService = {
    available: () => available,
    apply: recordingStub(calls, 'apply', () => APPLIED),
    listDisplays: recordingStub(calls, 'listDisplays', () => ({
      ok: true,
      displays: [DISPLAY],
    })),
    listWindows: recordingStub(calls, 'listWindows', () => ({
      ok: true,
      windows: [WINDOW],
    })),
    applyToWindow: recordingStub(calls, 'applyToWindow', () => APPLIED),
    focusWindow: recordingStub(calls, 'focusWindow', () => ({ ok: true })),
    ...overrides,
  };
  return { service, calls };
}

function createFacet(service: WindowManagementService) {
  const deps: DesktopWindowManagementDeps = {
    service,
    resolveInvocation: (scope) =>
      windowInvocationFromScope(scope, () => FOCUS_TARGET),
  };
  return createDesktopWindowManagement(deps) as any;
}

function createServiceAndFacet(
  overrides: Partial<Omit<WindowManagementService, 'available'>> = {},
) {
  const harness = createServiceStub(overrides);
  return { ...harness, facet: createFacet(harness.service) };
}

function countCalls(calls: StubCalls, name: string) {
  return calls[name]?.length ?? 0;
}

test('facet is absent when no window management service is wired', () => {
  assert.equal(createDesktopWindowManagement(undefined), undefined);
});

test('facet is absent when the service reports unavailable', () => {
  const { service } = createServiceStub({}, false);
  assert.equal(
    createDesktopWindowManagement({
      service,
      resolveInvocation: () => null,
    }),
    undefined,
  );
});

test('apply without an invocation scope fails closed before reaching the service', async () => {
  const { facet, calls } = createServiceAndFacet();
  const result = await facet.apply(HALF);
  assert.equal(countCalls(calls, 'apply'), 0);
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'no-target');
});

test('apply inside a shortcut scope passes the snapshot and the default options', async () => {
  const { facet, calls } = createServiceAndFacet();
  await runInWindowInvocationScope(
    { source: 'shortcut', target: FOCUS_TARGET },
    () => facet.apply(HALF),
  );
  assert.deepEqual(calls.apply, [
    [
      HALF,
      DEFAULT_WINDOW_MANAGEMENT_OPTIONS,
      { source: 'shortcut', target: FOCUS_TARGET },
    ],
  ]);
});

test('apply merges explicit options over the defaults', async () => {
  const { facet, calls } = createServiceAndFacet();
  await runInWindowInvocationScope({ source: 'palette' }, () =>
    facet.apply(HALF, { innerGap: 12, outerGap: { top: 20 } }),
  );
  assert.deepEqual(calls.apply?.[0]?.[1], {
    ...DEFAULT_WINDOW_MANAGEMENT_OPTIONS,
    innerGap: 12,
    outerGap: { ...DEFAULT_WINDOW_MANAGEMENT_OPTIONS.outerGap, top: 20 },
  });
});

test('apply result never exposes pid or windowKey even if the service returns them', async () => {
  const leaking = { ...APPLIED, ...LEAKY_FIELDS } as WindowApplyResult;
  const { facet } = createServiceAndFacet({
    apply: () => Promise.resolve(leaking),
  });
  const result = await runInWindowInvocationScope({ source: 'palette' }, () =>
    facet.apply(HALF),
  );
  assert.deepEqual(result, {
    ok: true,
    outcome: 'exact',
    changed: true,
    frame: LEFT_HALF_FRAME,
  });
});

test('apply failure results keep only reason and message', async () => {
  const failure: WindowApplyResult = {
    ok: false,
    reason: 'unsupported',
    message: 'Window management is unavailable.',
  };
  const { facet } = createServiceAndFacet({
    apply: () => Promise.resolve(failure),
  });
  const result = await runInWindowInvocationScope(
    { source: 'shortcut', target: FOCUS_TARGET },
    () => facet.apply(HALF),
  );
  assert.deepEqual(result, failure);
});

test('displays rebuild each display from documented fields only', async () => {
  const leakingDisplay = { ...DISPLAY, debugName: 'internal' };
  const { facet } = createServiceAndFacet({
    listDisplays: () =>
      Promise.resolve({ ok: true, displays: [leakingDisplay] }),
  });
  assert.deepEqual(await facet.displays(), [DISPLAY]);
});

test('displays are empty when the service fails', async () => {
  const { facet } = createServiceAndFacet({
    listDisplays: () =>
      Promise.resolve({ ok: false, reason: 'denied', message: 'No access.' }),
  });
  assert.deepEqual(await facet.displays(), []);
});

test('windows map the handle string to { id } and drop pid and windowKey', async () => {
  const leaking = { ...WINDOW, ...LEAKY_FIELDS };
  const { facet } = createServiceAndFacet({
    listWindows: () => Promise.resolve({ ok: true, windows: [leaking] }),
  });
  const [window] = await facet.windows();
  assert.deepEqual(window, {
    handle: { id: 'opaque-handle-1' },
    appId: 'app.example.Browser',
    appName: 'Browser',
    title: 'Start page',
    frame: LEFT_HALF_FRAME,
    displayId: 'display-1',
    isFullScreen: false,
    isMinimized: false,
  });
  assert.equal(JSON.stringify(window).includes('windowKey'), false);
  assert.equal(JSON.stringify(window).includes('"pid"'), false);
});

test('windows forward only the appId, appName and title query fields that are strings', async () => {
  const { facet, calls } = createServiceAndFacet();
  await facet.windows({
    appId: 'app.example.Browser',
    appName: 'Browser',
    title: 'Start',
    extra: 1,
  });
  await facet.windows({ appId: 7 });
  await facet.windows();
  assert.deepEqual(calls.listWindows, [
    [{ appId: 'app.example.Browser', appName: 'Browser', title: 'Start' }],
    [{}],
    [{}],
  ]);
});

test('windows are empty when access is denied', async () => {
  const { facet } = createServiceAndFacet({
    listWindows: () =>
      Promise.resolve({ ok: false, reason: 'denied', message: 'No access.' }),
  });
  assert.deepEqual(await facet.windows(), []);
});

test('waitForWindow resolves to the first match once it appears', async () => {
  let listing = 0;
  const { facet, calls } = createServiceAndFacet({
    listWindows: () => {
      listing += 1;
      return Promise.resolve({
        ok: true,
        windows: listing < 3 ? [] : [WINDOW],
      });
    },
  });
  const window = await facet.waitForWindow(
    { appId: 'app.example.Browser' },
    { timeoutMs: 2000 },
  );
  assert.equal(window.handle.id, 'opaque-handle-1');
  assert.equal(countCalls(calls, 'listWindows'), 3);
});

test('waitForWindow resolves to null after the timeout with bounded polling', async () => {
  const { facet, calls } = createServiceAndFacet({
    listWindows: () => Promise.resolve({ ok: true, windows: [] }),
  });
  const started = Date.now();
  const window = await facet.waitForWindow({}, { timeoutMs: 600 });
  assert.equal(window, null);
  assert.ok(Date.now() - started < 2000);
  assert.ok(countCalls(calls, 'listWindows') <= 4);
});

test('waitForWindow with a zero timeout lists once and resolves to null', async () => {
  const { facet, calls } = createServiceAndFacet({
    listWindows: () => Promise.resolve({ ok: true, windows: [] }),
  });
  assert.equal(await facet.waitForWindow({}, { timeoutMs: 0 }), null);
  assert.equal(countCalls(calls, 'listWindows'), 1);
});

test('clampWaitTimeoutMs defaults missing or non-finite values and clamps to 0..30000', () => {
  assert.equal(clampWaitTimeoutMs(undefined), 5000);
  assert.equal(clampWaitTimeoutMs(Number.NaN), 5000);
  assert.equal(clampWaitTimeoutMs(Number.POSITIVE_INFINITY), 5000);
  assert.equal(clampWaitTimeoutMs(-10), 0);
  assert.equal(clampWaitTimeoutMs(90_000), 30_000);
  assert.equal(clampWaitTimeoutMs(1200), 1200);
});

test('applyTo rejects handles that are not { id: non-empty string } without calling the service', async () => {
  const invalidHandles = [undefined, null, {}, { id: '' }, { id: 42 }, 'h1'];
  await Promise.all(
    invalidHandles.map(async (handle) => {
      const { facet, calls } = createServiceAndFacet();
      const result = await facet.applyTo(handle, HALF);
      assert.equal(result.ok, false);
      assert.equal(result.reason, 'unknown-window');
      assert.equal(countCalls(calls, 'applyToWindow'), 0);
    }),
  );
});

test('applyTo targets the handle id with default options and works without an invocation scope', async () => {
  const { facet, calls } = createServiceAndFacet();
  const result = await facet.applyTo({ id: 'opaque-handle-1' }, HALF);
  assert.deepEqual(calls.applyToWindow, [
    ['opaque-handle-1', HALF, DEFAULT_WINDOW_MANAGEMENT_OPTIONS],
  ]);
  assert.deepEqual(result, {
    ok: true,
    outcome: 'exact',
    changed: true,
    frame: LEFT_HALF_FRAME,
  });
});

test('applyTo merges explicit options over the defaults', async () => {
  const { facet, calls } = createServiceAndFacet();
  await facet.applyTo({ id: 'opaque-handle-1' }, HALF, { innerGap: 12 });
  assert.deepEqual(calls.applyToWindow?.[0]?.[2], {
    ...DEFAULT_WINDOW_MANAGEMENT_OPTIONS,
    innerGap: 12,
  });
});

test('applyTo result strips leaking fields and keeps failure reason and message', async () => {
  const leaking = { ...APPLIED, ...LEAKY_FIELDS } as WindowApplyResult;
  const { facet } = createServiceAndFacet({
    applyToWindow: () => Promise.resolve(leaking),
  });
  const applied = await facet.applyTo({ id: 'opaque-handle-1' }, HALF);
  assert.equal(JSON.stringify(applied).includes('windowKey'), false);

  const failure: WindowApplyResult = {
    ok: false,
    reason: 'target-lost',
    message: 'The window closed.',
  };
  const { facet: failing } = createServiceAndFacet({
    applyToWindow: () => Promise.resolve(failure),
  });
  assert.deepEqual(
    await failing.applyTo({ id: 'opaque-handle-1' }, HALF),
    failure,
  );
});

test('focus rejects malformed handles without calling the service', async () => {
  const { facet, calls } = createServiceAndFacet();
  const result = await facet.focus({ id: '' });
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'unknown-window');
  assert.equal(countCalls(calls, 'focusWindow'), 0);
});

test('focus resolves to { ok: true } on success', async () => {
  const { facet, calls } = createServiceAndFacet();
  assert.deepEqual(await facet.focus({ id: 'opaque-handle-1' }), { ok: true });
  assert.deepEqual(calls.focusWindow, [['opaque-handle-1']]);
});

test('focus failure keeps only reason and message', async () => {
  const { facet } = createServiceAndFacet({
    focusWindow: () =>
      Promise.resolve({
        ok: false,
        reason: 'unknown-window',
        message: 'Window is gone.',
        ...LEAKY_FIELDS,
      }),
  });
  assert.deepEqual(await facet.focus({ id: 'opaque-handle-1' }), {
    ok: false,
    reason: 'unknown-window',
    message: 'Window is gone.',
  });
});
