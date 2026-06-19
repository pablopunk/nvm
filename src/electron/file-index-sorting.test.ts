import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyDateAdded,
  findFilesNeedsStats,
  includeDimensionsForFindOptions,
  selectFindFiles,
  sortFoundFiles,
} from './file-index-sorting';

test('findFiles selection sorts globally before slicing to the limit', () => {
  const candidates = [
    { path: '/older-first.png', name: 'older-first.png', dateAddedMs: 10 },
    { path: '/older-second.png', name: 'older-second.png', dateAddedMs: 20 },
    { path: '/newest-third.png', name: 'newest-third.png', dateAddedMs: 300 },
    { path: '/newer-fourth.png', name: 'newer-fourth.png', dateAddedMs: 200 },
  ];

  const selected = selectFindFiles(candidates, { sortBy: 'added' }, 2);

  assert.deepEqual(
    selected.map((file) => file.path),
    ['/newest-third.png', '/newer-fourth.png'],
  );
});

test('sortBy added uses dateAddedMs with birthtime fallback applied before sorting', () => {
  const candidates = applyDateAdded(
    [
      { path: '/missing-date.png', name: 'missing-date.png', birthtimeMs: 150 },
      {
        path: '/explicit-date.png',
        name: 'explicit-date.png',
        birthtimeMs: 50,
      },
    ],
    new Map([['/explicit-date.png', 250]]),
  );

  const sorted = sortFoundFiles(candidates, { sortBy: 'added' });

  assert.deepEqual(
    sorted.map((file) => ({ path: file.path, dateAddedMs: file.dateAddedMs })),
    [
      { path: '/explicit-date.png', dateAddedMs: 250 },
      { path: '/missing-date.png', dateAddedMs: 150 },
    ],
  );
});

test('includeDimensions defaults to false and stats are required only for stat-based sorts', () => {
  assert.equal(includeDimensionsForFindOptions(), false);
  assert.equal(includeDimensionsForFindOptions({}), false);
  assert.equal(
    includeDimensionsForFindOptions({ includeDimensions: true }),
    true,
  );

  assert.equal(findFilesNeedsStats('added'), true);
  assert.equal(findFilesNeedsStats('size'), true);
  assert.equal(findFilesNeedsStats('name'), false);
});
