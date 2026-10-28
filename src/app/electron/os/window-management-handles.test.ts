// biome-ignore-all lint/style/noMagicNumbers: handle counts and TTL fixtures read best as literals
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createWindowHandleRegistry,
  MAX_WINDOW_HANDLES,
  WINDOW_HANDLE_TTL_MS,
} from './window-management-handles';

const TARGET = { pid: 42, appId: 'com.apple.finder', windowKey: '42:7' };

function sequentialIds() {
  let next = 0;
  return () => `handle-${++next}`;
}

function clock(start = 0) {
  let current = start;
  return {
    now: () => current,
    advance: (ms: number) => {
      current += ms;
    },
  };
}

test('reuses the handle for the same pid and window key and refreshes its expiry', () => {
  const time = clock();
  const registry = createWindowHandleRegistry({
    now: time.now,
    createId: sequentialIds(),
  });
  const first = registry.issue(TARGET);
  time.advance(WINDOW_HANDLE_TTL_MS - 1);
  assert.equal(registry.issue(TARGET), first);
  time.advance(WINDOW_HANDLE_TTL_MS - 1);
  assert.deepEqual(registry.resolve(first), TARGET);
});

test('issues distinct handles for different windows of the same pid', () => {
  const registry = createWindowHandleRegistry({ createId: sequentialIds() });
  const other = { ...TARGET, windowKey: '42:8' };
  assert.notEqual(registry.issue(TARGET), registry.issue(other));
});

test('resolves a handle to its pid, app id and window key', () => {
  const registry = createWindowHandleRegistry({ createId: sequentialIds() });
  const handle = registry.issue(TARGET);
  assert.deepEqual(registry.resolve(handle), TARGET);
});

test('stops resolving a handle once its time to live has passed', () => {
  const time = clock();
  const registry = createWindowHandleRegistry({
    now: time.now,
    createId: sequentialIds(),
  });
  const handle = registry.issue(TARGET);
  time.advance(WINDOW_HANDLE_TTL_MS);
  assert.equal(registry.resolve(handle), undefined);
});

test('expired handles are not reused, a fresh handle is issued', () => {
  const time = clock();
  const registry = createWindowHandleRegistry({
    now: time.now,
    createId: sequentialIds(),
  });
  const expired = registry.issue(TARGET);
  time.advance(WINDOW_HANDLE_TTL_MS);
  const fresh = registry.issue(TARGET);
  assert.notEqual(fresh, expired);
  assert.equal(registry.resolve(expired), undefined);
  assert.deepEqual(registry.resolve(fresh), TARGET);
});

test('forgets a handle explicitly', () => {
  const registry = createWindowHandleRegistry({ createId: sequentialIds() });
  const handle = registry.issue(TARGET);
  registry.forget(handle);
  assert.equal(registry.resolve(handle), undefined);
});

test('an unknown handle resolves to nothing', () => {
  const registry = createWindowHandleRegistry({ createId: sequentialIds() });
  assert.equal(registry.resolve('never-issued'), undefined);
});

test('evicts the oldest handle once the cap is exceeded', () => {
  const registry = createWindowHandleRegistry({ createId: sequentialIds() });
  const handles = Array.from({ length: MAX_WINDOW_HANDLES + 1 }, (_, index) =>
    registry.issue({ ...TARGET, windowKey: `42:${index}` }),
  );
  assert.equal(registry.resolve(handles[0] as string), undefined);
  assert.deepEqual(registry.resolve(handles[1] as string), {
    ...TARGET,
    windowKey: '42:1',
  });
  assert.notEqual(
    registry.resolve(handles[MAX_WINDOW_HANDLES] as string),
    undefined,
  );
});
