import assert from 'node:assert/strict';
import test from 'node:test';
import { filesNeedingDateAdded } from './file-index-commit';

test('known files reuse date added while new or replaced files need a lookup', () => {
  const previous = [
    { path: '/d/a.pdf', birthtimeMs: 1, dateAddedMs: 10 },
    { path: '/d/b.pdf', birthtimeMs: 2, dateAddedMs: 20 },
  ];
  const scanned: any[] = [
    { path: '/d/a.pdf', birthtimeMs: 1 },
    { path: '/d/b.pdf', birthtimeMs: 3 },
    { path: '/d/c.pdf', birthtimeMs: 4 },
  ];
  const needing = filesNeedingDateAdded(scanned, previous);
  assert.deepEqual(
    needing.map((file) => file.path),
    ['/d/b.pdf', '/d/c.pdf'],
  );
  assert.equal(scanned[0].dateAddedMs, 10);
});
