import assert from 'node:assert/strict';
import test from 'node:test';
import { JobRegistry } from './jobs';

test('maintenance waits for timed-out work and prevents new jobs until resumed', async () => {
  const registry = new JobRegistry();
  let finish!: () => void;
  const work = new Promise<void>((resolve) => {
    finish = resolve;
  });
  registry.register({
    id: 'write',
    title: 'Write',
    timeoutMs: 10,
    run: () => work,
  });
  const running = registry.run('write');
  // Keep the event loop alive while the job's unref'ed timeout fires.
  const keepAlive = setTimeout(() => {}, 1000);
  await assert.rejects(running, /timed out/);
  clearTimeout(keepAlive);
  let drained = false;
  const quiescence = registry.quiesce().then(() => {
    drained = true;
  });
  await new Promise<void>((resolve) => setImmediate(resolve));
  assert.equal(drained, false);
  assert.throws(() => registry.resume(), /still draining/);
  assert.equal(await registry.run('write'), null);
  assert.throws(
    () => registry.register({ id: 'other', title: 'Other', run: () => {} }),
    /paused/,
  );
  finish();
  await quiescence;
  assert.equal(drained, true);
  registry.resume();
  await registry.run('write');
  registry.clear();
});
