import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isPromiseAbortError,
  isPromiseTimeoutError,
  type PromiseTimeoutTimers,
  withAbortableTimeout,
} from './promise-timeout';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });
  return { promise, resolve, reject };
}

function fakeTimers() {
  const callbacks = new Map<object, () => void>();
  const timers: PromiseTimeoutTimers = {
    setTimeout: (callback) => {
      const handle = { unref() {} };
      callbacks.set(handle, callback);
      return handle as ReturnType<typeof setTimeout>;
    },
    clearTimeout: (handle) => callbacks.delete(handle),
  };
  return {
    timers,
    active: () => callbacks.size,
    fire: () => [...callbacks.values()].forEach((callback) => callback()),
  };
}

test('withAbortableTimeout clears its timer after resolve and reject', async () => {
  const resolvedTimers = fakeTimers();
  assert.equal(
    await withAbortableTimeout(Promise.resolve('done'), 100, {
      timers: resolvedTimers.timers,
    }),
    'done',
  );
  assert.equal(resolvedTimers.active(), 0);

  const rejectedTimers = fakeTimers();
  await assert.rejects(
    withAbortableTimeout(Promise.reject(new Error('failed')), 100, {
      timers: rejectedTimers.timers,
    }),
    /failed/,
  );
  assert.equal(rejectedTimers.active(), 0);
});

test('withAbortableTimeout distinguishes timeout and abort and cleans up', async () => {
  const timeoutTimers = fakeTimers();
  const pending = deferred<void>();
  const timed = withAbortableTimeout(pending.promise, 100, {
    timers: timeoutTimers.timers,
  });
  assert.equal(timeoutTimers.active(), 1);
  timeoutTimers.fire();
  await assert.rejects(timed, isPromiseTimeoutError);
  assert.equal(timeoutTimers.active(), 0);
  pending.reject(new Error('ignored late rejection'));
  await Promise.resolve();

  const abortTimers = fakeTimers();
  const controller = new AbortController();
  const aborted = withAbortableTimeout(deferred<void>().promise, 100, {
    signal: controller.signal,
    timers: abortTimers.timers,
  });
  controller.abort();
  await assert.rejects(aborted, isPromiseAbortError);
  assert.equal(abortTimers.active(), 0);
});

test('withAbortableTimeout rejects immediately for a pre-aborted signal', async () => {
  const timers = fakeTimers();
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(
    withAbortableTimeout(deferred<void>().promise, 100, {
      signal: controller.signal,
      timers: timers.timers,
    }),
    isPromiseAbortError,
  );
  assert.equal(timers.active(), 0);
});
