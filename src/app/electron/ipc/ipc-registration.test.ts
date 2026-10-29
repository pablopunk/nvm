import assert from 'node:assert/strict';
import test from 'node:test';
import { createMeasuredIpcRegistrar } from './ipc-registration';
import { createMaintenanceActivity } from '../platform/maintenance-activity';

test('maintenance waits for an interactive IPC operation and rejects new invocations', async () => {
  const activity = createMaintenanceActivity();
  let handler!: (event: unknown) => unknown;
  let finish!: () => void;
  const pending = new Promise<void>((resolve) => {
    finish = resolve;
  });
  const register = createMeasuredIpcRegistrar({
    ipcMain: {
      handle: (_channel, invoke) => {
        handler = invoke;
      },
    },
    measure: (_name, _data, task) => task(),
    summarize: (value) => value,
    activity,
  });
  register('actions:execute', () => pending);
  const action = handler({});
  let drained = false;
  const quiescence = activity.quiesce().then(() => {
    drained = true;
  });
  assert.throws(() => handler({}), /state backup/);
  assert.throws(() => activity.resume(), /still draining/);
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(drained, false);
  finish();
  await Promise.all([action, quiescence]);
  activity.resume();
  await handler({});
});

test('createMeasuredIpcRegistrar registers measured ipc handlers', async () => {
  let registeredChannel = '';
  let registeredHandler:
    | ((event: unknown, ...args: unknown[]) => unknown)
    | null = null;
  const measured: Array<{ name: string; data: Record<string, unknown> }> = [];
  const register = createMeasuredIpcRegistrar({
    ipcMain: {
      handle(channel, handler) {
        registeredChannel = channel;
        registeredHandler = handler;
      },
    },
    measure: (name, data, fn) => {
      measured.push({ name, data });
      return fn();
    },
    summarize: (value) => `summary:${String(value)}`,
  });

  register('things:do', (_event, first, second) => `${first}:${second}`);

  assert.equal(registeredChannel, 'things:do');
  assert.ok(registeredHandler);
  assert.equal(await registeredHandler({ sender: true }, 'a', 2), 'a:2');
  assert.deepEqual(measured, [
    {
      name: 'ipc.things:do.handler',
      data: { args: ['summary:a', 'summary:2'], alwaysLog: true },
    },
  ]);
});
