import assert from 'node:assert/strict';
import test from 'node:test';
import { createMaintenanceActivity } from '../platform/maintenance-activity';
import { createPendingViewLoaderSpawner } from './pending-view-loader';

test('maintenance waits for spawned loaders and does not start new ones', async () => {
  const activity = createMaintenanceActivity();
  let finish!: () => void;
  const pending = new Promise<void>((resolve) => {
    finish = resolve;
  });
  let starts = 0;
  const spawn = createPendingViewLoaderSpawner({
    activity,
    registry: {
      has: () => true,
      spawn: () => {
        starts += 1;
        return pending;
      },
    },
    trace: { run: (_name, _attributes, task) => task() },
  });
  spawn({ view: { id: 'first' } });
  let drained = false;
  const maintenance = activity.quiesce().then(() => {
    drained = true;
  });
  spawn({ view: { id: 'second' } });
  assert.equal(starts, 1);
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(drained, false);
  finish();
  await maintenance;
  activity.resume();
  spawn({ view: { id: 'third' } });
  assert.equal(starts, 2);
});
