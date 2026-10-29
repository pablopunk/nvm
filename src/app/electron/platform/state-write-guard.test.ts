import assert from 'node:assert/strict';
import test from 'node:test';
import { createMaintenanceActivity } from './maintenance-activity';
import { guardStateWrite, installStateWriteGuard } from './state-write-guard';

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
