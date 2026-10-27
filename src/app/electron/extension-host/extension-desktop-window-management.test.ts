import assert from 'node:assert/strict';
import test from 'node:test';
import type {
  WindowApplyResult,
  WindowLayoutSpec,
  WindowManagementService,
} from '../os/window-management-types';
import {
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
const SAFARI = { bundleId: 'com.apple.Safari', pid: 411 };
const LEFT_HALF_FRAME = { x: 0, y: 25, width: 720, height: 875 };
const APPLIED: WindowApplyResult = {
  ok: true,
  outcome: 'exact',
  changed: true,
  frame: LEFT_HALF_FRAME,
};

function createServiceStub(
  result: WindowApplyResult = APPLIED,
  available = true,
) {
  const calls: unknown[][] = [];
  const service: WindowManagementService = {
    available: () => available,
    apply: (...args) => {
      calls.push(args);
      return Promise.resolve(result);
    },
  };
  return { service, calls };
}

function createDeps(
  service: WindowManagementService,
): DesktopWindowManagementDeps {
  return {
    service,
    resolveInvocation: (scope) =>
      windowInvocationFromScope(scope, () => SAFARI),
  };
}

test('facet is absent when no window management service is wired', () => {
  assert.equal(createDesktopWindowManagement(undefined), undefined);
});

test('facet is absent when the service reports unavailable', () => {
  const { service } = createServiceStub(APPLIED, false);
  assert.equal(createDesktopWindowManagement(createDeps(service)), undefined);
});

test('apply without an invocation scope fails closed before reaching the service', async () => {
  const { service, calls } = createServiceStub();
  const facet = createDesktopWindowManagement(createDeps(service));
  const result = await facet?.apply(HALF);
  assert.equal(calls.length, 0);
  assert.equal(result?.ok, false);
  assert.equal(result?.reason, 'no-target');
});

test('apply inside a shortcut scope passes the snapshot and a zero default gap', async () => {
  const { service, calls } = createServiceStub();
  const facet = createDesktopWindowManagement(createDeps(service));
  await runInWindowInvocationScope({ source: 'shortcut', target: SAFARI }, () =>
    facet?.apply(HALF),
  );
  assert.deepEqual(calls, [
    [HALF, { gap: 0 }, { source: 'shortcut', target: SAFARI }],
  ]);
});

test('apply forwards an explicit gap from options', async () => {
  const { service, calls } = createServiceStub();
  const facet = createDesktopWindowManagement(createDeps(service));
  await runInWindowInvocationScope({ source: 'palette' }, () =>
    facet?.apply(HALF, { gap: 8 }),
  );
  assert.deepEqual(calls[0]?.[1], { gap: 8 });
  assert.deepEqual(calls[0]?.[2], { source: 'palette', target: SAFARI });
});

test('result never exposes pid or windowKey even if the service returns them', async () => {
  const leaking = {
    ...APPLIED,
    frame: { ...APPLIED.frame, pid: 411, windowKey: '411:9001' },
    pid: 411,
    windowKey: '411:9001',
  } as unknown as WindowApplyResult;
  const { service } = createServiceStub(leaking);
  const facet = createDesktopWindowManagement(createDeps(service));
  const result = await runInWindowInvocationScope({ source: 'palette' }, () =>
    facet?.apply(HALF),
  );
  assert.deepEqual(result, {
    ok: true,
    outcome: 'exact',
    changed: true,
    frame: LEFT_HALF_FRAME,
  });
  assert.equal(JSON.stringify(result).includes('windowKey'), false);
});

test('failure results keep only reason and message', async () => {
  const failure: WindowApplyResult = {
    ok: false,
    reason: 'unsupported',
    message: 'Window management is unavailable.',
  };
  const { service } = createServiceStub(failure);
  const facet = createDesktopWindowManagement(createDeps(service));
  const result = await runInWindowInvocationScope(
    { source: 'shortcut', target: SAFARI },
    () => facet?.apply(HALF),
  );
  assert.deepEqual(result, failure);
});
