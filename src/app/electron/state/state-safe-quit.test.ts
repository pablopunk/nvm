import assert from 'node:assert/strict';
import test from 'node:test';
import { createStateSafeQuit } from './state-safe-quit';

const INJECTED_FLUSH_FAILURE_PATTERN = /injected flush failure/;
const NEVER_SETTLES = new Promise<void>(() => undefined);

function noOp() {
  return;
}

function deferred() {
  let resolve!: () => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<void>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

function fakeTimers() {
  let nextId = 1;
  const timers = new Map<number, () => void>();
  return {
    setTimer: ((callback: () => void) => {
      const id = nextId++;
      timers.set(id, callback);
      return id;
    }) as unknown as typeof setTimeout,
    clearTimer: ((id: number) => {
      timers.delete(id);
    }) as unknown as typeof clearTimeout,
    runNext() {
      const entry = timers.entries().next().value as
        | [number, () => void]
        | undefined;
      if (!entry) {
        throw new Error('Expected a pending timer');
      }
      timers.delete(entry[0]);
      entry[1]();
    },
  };
}

test('flushes pending state before quit and cleanup', async () => {
  const flush = deferred();
  const calls: string[] = [];
  const lifecycle = createStateSafeQuit({
    flushPendingSave: async () => {
      calls.push('flush');
      await flush.promise;
    },
    quit: () => calls.push('quit'),
    cleanup: () => calls.push('cleanup'),
    exit: () => calls.push('exit'),
  });

  const request = lifecycle.requestQuit('menu');
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(calls, ['flush']);
  flush.resolve();
  await request;
  assert.deepEqual(calls, ['flush', 'quit']);

  lifecycle.handleWillQuit();
  assert.deepEqual(calls, ['flush', 'quit', 'cleanup']);
});

test('before-quit is prevented only until the shared flush completes', async () => {
  let prevented = 0;
  let quits = 0;
  const lifecycle = createStateSafeQuit({
    flushPendingSave: () => Promise.resolve(),
    quit: () => {
      quits += 1;
    },
    cleanup: noOp,
    exit: noOp,
  });

  lifecycle.handleBeforeQuit({
    preventDefault: () => {
      prevented += 1;
    },
  });
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(prevented, 1);
  assert.equal(quits, 1);

  lifecycle.handleBeforeQuit({
    preventDefault: () => {
      prevented += 1;
    },
  });
  assert.equal(prevented, 1);
});

test('flush failure is logged and does not block quit', async () => {
  const errors: unknown[] = [];
  let quits = 0;
  const lifecycle = createStateSafeQuit({
    flushPendingSave: () => Promise.reject(new Error('injected flush failure')),
    quit: () => {
      quits += 1;
    },
    cleanup: noOp,
    exit: noOp,
    onFlushError: (error) => errors.push(error),
  });

  await lifecycle.requestQuit('ipc');

  assert.equal(quits, 1);
  assert.equal(errors.length, 1);
  assert.match(String(errors[0]), INJECTED_FLUSH_FAILURE_PATTERN);
});

test('timeout permits quit and the hard fallback cleans up before exit', async () => {
  const timers = fakeTimers();
  const calls: string[] = [];
  const lifecycle = createStateSafeQuit({
    flushPendingSave: () => NEVER_SETTLES,
    quit: () => calls.push('quit'),
    cleanup: () => calls.push('cleanup'),
    exit: () => calls.push('exit'),
    onFlushTimeout: () => calls.push('timeout'),
    onFallbackExit: () => calls.push('fallback'),
    setTimer: timers.setTimer,
    clearTimer: timers.clearTimer,
  });

  const request = lifecycle.requestQuit('os');
  await new Promise((resolve) => setImmediate(resolve));
  timers.runNext();
  await request;
  assert.deepEqual(calls, ['timeout', 'quit']);

  timers.runNext();
  assert.deepEqual(calls, ['timeout', 'quit', 'fallback', 'cleanup', 'exit']);
});

test('multiple quit callers share one in-flight flush and quit action', async () => {
  const flush = deferred();
  let flushes = 0;
  let quits = 0;
  const lifecycle = createStateSafeQuit({
    flushPendingSave: async () => {
      flushes += 1;
      await flush.promise;
    },
    quit: () => {
      quits += 1;
    },
    cleanup: noOp,
    exit: noOp,
  });

  const first = lifecycle.requestQuit('menu');
  const second = lifecycle.requestQuit('ipc');
  assert.equal(first, second);
  flush.resolve();
  await first;

  assert.equal(flushes, 1);
  assert.equal(quits, 1);
});
