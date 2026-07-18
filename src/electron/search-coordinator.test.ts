import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createStableSearchResultPreparer,
  createSearchCoordinator,
  type SearchRequest,
  type SearchSnapshot,
  type SearchWork,
  searchResultsFingerprint,
} from './search-coordinator';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve;
  });
  return { promise, resolve };
}

function drainMicrotasks() {
  return new Promise<void>((resolve) => setImmediate(resolve));
}

class FakeSender {
  destroyed = false;
  sent: SearchSnapshot<string>[] = [];
  listeners = new Map<string, Set<() => void>>();

  isDestroyed() {
    return this.destroyed;
  }

  send(_channel: string, payload: unknown) {
    this.sent.push(payload as SearchSnapshot<string>);
  }

  on(event: 'destroyed' | 'render-process-gone', listener: () => void) {
    const listeners = this.listeners.get(event) || new Set();
    listeners.add(listener);
    this.listeners.set(event, listeners);
  }

  removeListener(
    event: 'destroyed' | 'render-process-gone',
    listener: () => void,
  ) {
    this.listeners.get(event)?.delete(listener);
  }

  emit(event: 'destroyed' | 'render-process-gone') {
    if (event === 'destroyed') this.destroyed = true;
    for (const listener of [...(this.listeners.get(event) || [])]) listener();
  }
}

function harness(createWork: (request: SearchRequest) => SearchWork<string>) {
  const scheduled = new Set<() => void>();
  const coordinator = createSearchCoordinator({
    createWork,
    fingerprint: searchResultsFingerprint,
    scheduleFlush: (callback) => {
      scheduled.add(callback);
      return callback;
    },
    cancelFlush: (callback) => scheduled.delete(callback as () => void),
  });
  return {
    coordinator,
    flush: () => {
      for (const callback of [...scheduled]) {
        scheduled.delete(callback);
        callback();
      }
    },
    scheduled: () => scheduled.size,
  };
}

test('returns local results immediately and coalesces progressive providers', async () => {
  const first = deferred<string[]>();
  const second = deferred<string[]>();
  const { coordinator, flush, scheduled } = harness(() => ({
    initialResults: ['local'],
    providers: [
      { key: 'first', run: () => first.promise },
      { key: 'second', run: () => second.promise },
    ],
    buildResults: (results) => [
      'local',
      ...(results.get('first') || []),
      ...(results.get('second') || []),
    ],
  }));
  const sender = new FakeSender();
  assert.deepEqual(coordinator.search(sender, { query: 'a', generation: 1 }), {
    generation: 1,
    revision: 0,
    results: ['local'],
    complete: false,
  });
  await drainMicrotasks();
  first.resolve(['one']);
  second.resolve(['two']);
  await drainMicrotasks();
  assert.equal(scheduled(), 1);
  flush();
  assert.deepEqual(sender.sent, [
    {
      generation: 1,
      revision: 1,
      results: ['local', 'one', 'two'],
      complete: true,
    },
  ]);
});

test('forces a terminal update when providers do not change visible results', async () => {
  const { coordinator, flush } = harness(() => ({
    initialResults: ['local'],
    providers: [{ key: 'empty', run: async () => [] }],
    buildResults: () => ['local'],
  }));
  const sender = new FakeSender();
  coordinator.search(sender, { query: 'a', generation: 1 });
  await drainMicrotasks();
  flush();
  assert.deepEqual(sender.sent[0], {
    generation: 1,
    revision: 1,
    results: ['local'],
    complete: true,
  });
});

test('forces terminal delivery for empty and failed provider sets', async () => {
  for (const providers of [
    [],
    [{ key: 'failed', run: async () => Promise.reject(new Error('failed')) }],
  ]) {
    const { coordinator, flush } = harness(() => ({
      initialResults: ['local'],
      providers,
      buildResults: () => ['local'],
    }));
    const sender = new FakeSender();
    coordinator.search(sender, { query: 'a', generation: 1 });
    await drainMicrotasks();
    flush();
    assert.equal(sender.sent.length, 1);
    assert.equal(sender.sent[0].complete, true);
    assert.deepEqual(sender.sent[0].results, ['local']);
  }
});

test('suppresses stale cooperative and non-cooperative provider results', async () => {
  const oldProvider = deferred<string[]>();
  let observedAbort = false;
  const { coordinator, flush } = harness((request) => ({
    initialResults: [`local:${request.query}`],
    providers: [
      {
        key: 'provider',
        run: (signal) => {
          signal.addEventListener('abort', () => {
            observedAbort = true;
          });
          return request.query === 'old'
            ? oldProvider.promise
            : Promise.resolve([`provider:${request.query}`]);
        },
      },
    ],
    buildResults: (results) => [
      `local:${request.query}`,
      ...(results.get('provider') || []),
    ],
  }));
  const sender = new FakeSender();
  coordinator.search(sender, { query: 'old', generation: 1 });
  await drainMicrotasks();
  coordinator.search(sender, { query: 'new', generation: 2 });
  assert.equal(observedAbort, true);
  oldProvider.resolve(['stale']);
  await drainMicrotasks();
  flush();
  assert.deepEqual(
    sender.sent.map((snapshot) => snapshot.generation),
    [2],
  );
  assert.deepEqual(sender.sent[0].results, ['local:new', 'provider:new']);
});

test('rejects duplicate generations and ignores late or future cancels', () => {
  const { coordinator } = harness(() => ({
    initialResults: [],
    providers: [],
    buildResults: () => [],
  }));
  const sender = new FakeSender();
  coordinator.search(sender, { query: '', generation: 2 });
  assert.throws(
    () => coordinator.search(sender, { query: '', generation: 2 }),
    /increase monotonically/,
  );
  assert.throws(
    () => coordinator.search(sender, { query: '', generation: 1 }),
    /increase monotonically/,
  );
  coordinator.cancel(sender, 1);
  coordinator.cancel(sender, 3);
  coordinator.cancel(sender, 2);
  coordinator.cancel(sender, 2);
});

test('isolates senders and removes lifecycle listeners on destruction', async () => {
  const { coordinator, flush } = harness((request) => ({
    initialResults: [request.query],
    providers: [],
    buildResults: () => [request.query],
  }));
  const first = new FakeSender();
  const second = new FakeSender();
  coordinator.search(first, { query: 'first', generation: 1 });
  coordinator.search(second, { query: 'second', generation: 1 });
  await drainMicrotasks();
  first.emit('destroyed');
  flush();
  assert.equal(first.sent.length, 0);
  assert.equal(second.sent.length, 1);
  assert.equal(coordinator.senderCount(), 1);
  assert.equal(first.listeners.get('destroyed')?.size, 0);
  assert.equal(first.listeners.get('render-process-gone')?.size, 0);
  second.emit('render-process-gone');
  assert.equal(coordinator.senderCount(), 0);
});

test('fingerprint includes visible changes but excludes execution tokens', () => {
  const first = [{ id: 'same', subtitle: 'old', executionId: 'one' }];
  const tokenChanged = [{ id: 'same', subtitle: 'old', executionId: 'two' }];
  const subtitleChanged = [{ id: 'same', subtitle: 'new', executionId: 'two' }];
  assert.equal(
    searchResultsFingerprint(first),
    searchResultsFingerprint(tokenChanged),
  );
  assert.notEqual(
    searchResultsFingerprint(first),
    searchResultsFingerprint(subtitleChanged),
  );
});

test('stable preparation preserves tokens until renderer-visible data changes', () => {
  let token = 0;
  const prepare = createStableSearchResultPreparer<
    { id: string; score: number; subtitle: string; action: { value: string } },
    {
      id: string;
      score: number;
      subtitle: string;
      action: { value: string };
      executionId: string;
    }
  >({
    logicalKey: (source) => source.id,
    prepare: (source) => ({ ...source, executionId: `token-${++token}` }),
  });
  const original = {
    id: 'same',
    score: 10,
    subtitle: 'old',
    action: { value: 'old' },
  };
  const first = prepare([original])[0];
  const unchanged = prepare([{ ...original }])[0];
  assert.equal(unchanged.executionId, first.executionId);
  const changed = prepare([
    {
      ...original,
      score: 11,
      subtitle: 'new',
      action: { value: 'new' },
    },
  ])[0];
  assert.notEqual(changed.executionId, first.executionId);
  assert.doesNotThrow(() => structuredClone(changed));
});
