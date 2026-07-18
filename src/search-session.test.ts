import assert from 'node:assert/strict';
import test from 'node:test';
import type { SearchSnapshot } from './preload-api';
import { createSearchSession } from './search-session';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve;
  });
  return { promise, resolve };
}

function harness() {
  let listener: (snapshot: SearchSnapshot<string>) => void = () => {};
  const searches: Array<{
    query: string;
    generation: number;
    response: ReturnType<typeof deferred<SearchSnapshot<string>>>;
  }> = [];
  const cancelled: number[] = [];
  const accepted: SearchSnapshot<string>[] = [];
  let unsubscribed = false;
  let now = 0;
  const session = createSearchSession({
    transport: {
      search: (query, options) => {
        const response = deferred<SearchSnapshot<string>>();
        searches.push({ query, generation: options.generation, response });
        return response.promise;
      },
      cancelSearch: (generation) => cancelled.push(generation),
      onSearchUpdate: (callback) => {
        listener = callback;
        return () => {
          unsubscribed = true;
        };
      },
    },
    onSnapshot: (snapshot) => accepted.push(snapshot),
    now: () => now,
  });
  return {
    session,
    searches,
    cancelled,
    accepted,
    emit: (snapshot: SearchSnapshot<string>) => listener(snapshot),
    setNow: (value: number) => {
      now = value;
    },
    unsubscribed: () => unsubscribed,
  };
}

test('event before invoke response wins and revisions never move backwards', async () => {
  const state = harness();
  const generation = state.session.start('query');
  state.emit({
    generation,
    revision: 1,
    results: ['progressive'],
    complete: true,
  });
  state.searches[0].response.resolve({
    generation,
    revision: 0,
    results: ['initial'],
    complete: false,
  });
  await Promise.resolve();
  state.emit({
    generation,
    revision: 1,
    results: ['duplicate'],
    complete: true,
  });
  assert.deepEqual(
    state.accepted.map((snapshot) => snapshot.results),
    [['progressive']],
  );
});

test('rapid generations ignore stale updates and cancel exact ownership', () => {
  const state = harness();
  const first = state.session.start('a');
  state.session.cancel(first);
  const second = state.session.start('ab');
  state.session.cancel(second);
  const third = state.session.start('abc');
  state.emit({
    generation: first,
    revision: 1,
    results: ['a'],
    complete: true,
  });
  state.emit({
    generation: third,
    revision: 0,
    results: ['abc'],
    complete: false,
  });
  assert.deepEqual(state.cancelled, [first, second]);
  assert.deepEqual(
    state.accepted.map((snapshot) => snapshot.results),
    [['abc']],
  );
});

test('dispose cancels the active generation and unsubscribes once', () => {
  const state = harness();
  const generation = state.session.start('query');
  state.session.dispose();
  assert.deepEqual(state.cancelled, [generation]);
  assert.equal(state.unsubscribed(), true);
});
