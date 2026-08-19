import assert from 'node:assert/strict';
import test from 'node:test';
import { visibleResultsForSearchSnapshot } from './use-search-results';

test('keeps visible results during a new incomplete initial snapshot', () => {
  const current = ['telegram', 'floating-notes'];
  assert.equal(
    visibleResultsForSearchSnapshot(current, {
      complete: false,
      results: ['local-only'],
      revision: 0,
    }),
    current,
  );
});

test('commits the first progressive provider update', () => {
  assert.deepEqual(
    visibleResultsForSearchSnapshot(['telegram'], {
      complete: false,
      results: ['telegram', 'web-search'],
      revision: 1,
    }),
    ['telegram', 'web-search'],
  );
});

test('commits complete initial snapshots', () => {
  assert.deepEqual(
    visibleResultsForSearchSnapshot(['telegram'], {
      complete: true,
      results: ['clipboard-result'],
      revision: 0,
    }),
    ['clipboard-result'],
  );
});
