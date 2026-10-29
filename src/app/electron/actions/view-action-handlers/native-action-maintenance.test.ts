import assert from 'node:assert/strict';
import test, { mock } from 'node:test';
import { createMaintenanceActivity } from '../../platform/maintenance-activity';

mock.module('../../diagnostics/debug-performance', {
  namedExports: {
    performanceTraces: {
      run: (_name: string, _data: unknown, task: () => unknown) => task(),
    },
  },
});
const { createNativeActionHandlers } = await import('./native-action');

test('maintenance waits for a detached view retry before proceeding', async () => {
  const activity = createMaintenanceActivity();
  let finish!: () => void;
  const retry = new Promise<{ ok: true; items: unknown[] }>((resolve) => {
    finish = () => resolve({ ok: true, items: [] });
  });
  const handler = createNativeActionHandlers({
    activity,
    viewLoaderRegistry: { has: () => true, retry: () => retry },
    executeAction: async () => {},
    verifyStateView: () => ({}),
  });
  handler.handleNativeAction({
    nativeAction: { kind: 'view-hydrate-retry', viewId: 'view' },
  });
  let drained = false;
  const maintenance = activity.quiesce().then(() => {
    drained = true;
  });
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(drained, false);
  finish();
  await maintenance;
  activity.resume();
});

test('state verification is routed to the host view without executing a root action', () => {
  const executed: unknown[] = [];
  const requested: unknown[] = [];
  const handler = createNativeActionHandlers({
    activity: createMaintenanceActivity(),
    viewLoaderRegistry: { has: () => false, retry: async () => ({}) as never },
    executeAction: async (action) => void executed.push(action),
    verifyStateView: (nativeAction) => {
      requested.push(nativeAction);
      return { view: 'updates' };
    },
  });
  const result = handler.handleNativeAction({
    nativeAction: { kind: 'state-verification', confirmation: 'token' },
  });
  assert.deepEqual(result, { view: 'updates' });
  assert.deepEqual(requested, [
    { kind: 'state-verification', confirmation: 'token' },
  ]);
  assert.deepEqual(executed, []);
});
