import assert from 'node:assert/strict';
import test from 'node:test';
import { DebounceDeadlines } from './job-helpers';
import { JobRegistry } from './jobs';

test('debounce deadlines keep trailing debounce without a max wait', () => {
  const deadlines = new DebounceDeadlines();
  assert.equal(deadlines.delayFor('job', 500, undefined, 0), 500);
  assert.equal(deadlines.delayFor('job', 500, undefined, 400), 500);
});

test('debounce deadlines shrink the delay so a burst runs within max wait', () => {
  const deadlines = new DebounceDeadlines();
  assert.equal(deadlines.delayFor('job', 1500, 10_000, 0), 1500);
  assert.equal(deadlines.delayFor('job', 1500, 10_000, 9000), 1000);
  assert.equal(deadlines.delayFor('job', 1500, 10_000, 12_000), 0);
  deadlines.settle('job');
  assert.equal(deadlines.delayFor('job', 1500, 10_000, 12_000), 1500);
});

test('settling a job prefix forgets only that job', () => {
  const deadlines = new DebounceDeadlines();
  deadlines.delayFor('a\0event:x', 100, 1000, 0);
  deadlines.delayFor('b\0event:x', 100, 1000, 0);
  deadlines.settleWithPrefix('a\0');
  assert.equal(deadlines.delayFor('a\0event:x', 100, 1000, 950), 100);
  assert.equal(deadlines.delayFor('b\0event:x', 100, 1000, 950), 50);
});

function wait(ms: number) {
  return new Promise<void>((resolve) => setTimeout(resolve, ms));
}

test('a continuous event burst still runs the job within max wait', async () => {
  const registry = new JobRegistry();
  const runs: number[] = [];
  const startedAt = Date.now();
  registry.register({
    id: 'refresh',
    title: 'Refresh',
    triggers: [
      { type: 'event', event: 'changed', debounceMs: 80, maxWaitMs: 200 },
    ],
    run: () => {
      runs.push(Date.now() - startedAt);
    },
  });
  while (Date.now() - startedAt < 600) {
    registry.emit('changed');
    await wait(20);
  }
  await wait(150);
  registry.clear();
  assert.ok(runs.length >= 2, `expected bounded runs, got ${runs}`);
  assert.ok(runs[0] < 450, `first run waited ${runs[0]}ms`);
});

test('a debounced burst without max wait coalesces into one run', async () => {
  const registry = new JobRegistry();
  let runs = 0;
  registry.register({
    id: 'refresh',
    title: 'Refresh',
    triggers: [{ type: 'event', event: 'changed', debounceMs: 60 }],
    run: () => {
      runs += 1;
    },
  });
  for (let index = 0; index < 5; index += 1) {
    registry.emit('changed');
    await wait(10);
  }
  await wait(200);
  registry.clear();
  assert.equal(runs, 1);
});
