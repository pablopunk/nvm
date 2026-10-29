import assert from 'node:assert/strict';
import test from 'node:test';
import { createMaintenanceActivity } from './maintenance-activity';
import {
  freezeStateWrites,
  guardStateWrite,
  installStateWriteGuard,
  untilStateWritesThaw,
} from './state-write-guard';

test('guarded writes are drained by maintenance and rejected while paused', async (t) => {
  const activity = createMaintenanceActivity();
  installStateWriteGuard(activity.run);
  t.after(() => installStateWriteGuard((write) => write()));
  let finish!: () => void;
  const write = guardStateWrite(
    () => new Promise<void>((resolve) => (finish = resolve)),
  );
  let drained = false;
  const maintenance = activity.quiesce().then(() => {
    drained = true;
  });
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(drained, false);
  await assert.rejects(
    guardStateWrite(async () => {}),
    /state backup/,
  );
  finish();
  await write;
  await maintenance;
  activity.resume();
  assert.equal(await guardStateWrite(async () => 'ok'), 'ok');
});

test('frozen state writes wait until released and cannot be frozen twice', async () => {
  const thaw = freezeStateWrites();
  assert.throws(() => freezeStateWrites(), /already frozen/);
  let resumed = false;
  const waiting = untilStateWritesThaw().then(() => {
    resumed = true;
  });
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(resumed, false);
  thaw();
  await waiting;
  assert.equal(resumed, true);
  await untilStateWritesThaw();
});
