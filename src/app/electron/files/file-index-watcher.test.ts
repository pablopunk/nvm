import assert from 'node:assert/strict';
import path from 'node:path';
import test from 'node:test';
import {
  changedPathCanAffectFileIndex,
  createFileIndexWatcher,
  type FileIndexWatchScope,
} from './file-index-watcher';
import { normalizedIgnorePatterns } from './ignore-patterns';

const ROOT = path.join(path.sep, 'home', 'me', 'Downloads');

function scope(overrides: Partial<FileIndexWatchScope> = {}) {
  return {
    roots: [ROOT],
    depth: 2,
    includeHidden: false,
    ignore: normalizedIgnorePatterns([]),
    ...overrides,
  };
}

test('changes inside the indexed depth can affect the file index', () => {
  assert.equal(changedPathCanAffectFileIndex(ROOT, 'a.pdf', scope()), true);
  assert.equal(
    changedPathCanAffectFileIndex(ROOT, path.join('x', 'y', 'b.png'), scope()),
    true,
  );
  assert.equal(changedPathCanAffectFileIndex(ROOT, null, scope()), true);
});

test('changes below the indexed depth, hidden, or ignored paths are skipped', () => {
  assert.equal(
    changedPathCanAffectFileIndex(
      ROOT,
      path.join('x', 'y', 'z', 'deep.txt'),
      scope(),
    ),
    false,
  );
  assert.equal(
    changedPathCanAffectFileIndex(ROOT, '.DS_Store', scope()),
    false,
  );
  assert.equal(
    changedPathCanAffectFileIndex(
      ROOT,
      path.join('app', 'node_modules', 'a'),
      scope(),
    ),
    false,
  );
  assert.equal(
    changedPathCanAffectFileIndex(
      ROOT,
      '.hidden',
      scope({ includeHidden: true }),
    ),
    true,
  );
});

function fakeWatchFactory() {
  const watchers = new Map<
    string,
    {
      emit: (relativePath: string | null) => void;
      fail: () => void;
      closed: boolean;
    }
  >();
  const missing = new Set<string>();
  function watchDirectoryTree(root: string, onChange) {
    if (missing.has(root)) throw new Error(`ENOENT: ${root}`);
    let onError = () => {};
    const handle = {
      emit: onChange,
      fail: () => onError(),
      closed: false,
    };
    watchers.set(root, handle);
    return {
      close: () => {
        handle.closed = true;
      },
      on: (event: string, listener: () => void) => {
        if (event === 'error') onError = listener;
      },
    };
  }
  return { watchers, missing, watchDirectoryTree };
}

test('watcher requests refresh only for relevant changes', () => {
  const fake = fakeWatchFactory();
  let refreshes = 0;
  const watcher = createFileIndexWatcher({
    watchDirectoryTree: fake.watchDirectoryTree,
    onRelevantChange: () => {
      refreshes += 1;
    },
  });
  watcher.sync(scope());
  fake.watchers.get(ROOT)?.emit('new.pdf');
  fake.watchers.get(ROOT)?.emit('.DS_Store');
  fake.watchers.get(ROOT)?.emit(path.join('a', 'b', 'c', 'deep.txt'));
  assert.equal(refreshes, 1);
});

test('watcher retargets roots and retries missing roots on the next sync', () => {
  const fake = fakeWatchFactory();
  const other = path.join(path.sep, 'home', 'me', 'Desktop');
  const watcher = createFileIndexWatcher({
    watchDirectoryTree: fake.watchDirectoryTree,
    onRelevantChange: () => {},
  });
  fake.missing.add(other);
  watcher.sync(scope({ roots: [ROOT, other] }));
  assert.deepEqual(watcher.watchedRoots(), [ROOT]);
  fake.missing.delete(other);
  watcher.sync(scope({ roots: [ROOT, other] }));
  assert.deepEqual(watcher.watchedRoots().sort(), [other, ROOT].sort());
  watcher.sync(scope({ roots: [other] }));
  assert.equal(fake.watchers.get(ROOT)?.closed, true);
  assert.deepEqual(watcher.watchedRoots(), [other]);
  watcher.close();
  assert.deepEqual(watcher.watchedRoots(), []);
});

test('a failed root watcher is dropped and requests a refresh', () => {
  const fake = fakeWatchFactory();
  let refreshes = 0;
  const watcher = createFileIndexWatcher({
    watchDirectoryTree: fake.watchDirectoryTree,
    onRelevantChange: () => {
      refreshes += 1;
    },
  });
  watcher.sync(scope());
  fake.watchers.get(ROOT)?.fail();
  assert.equal(refreshes, 1);
  assert.equal(fake.watchers.get(ROOT)?.closed, true);
  assert.deepEqual(watcher.watchedRoots(), []);
  watcher.sync(scope());
  assert.deepEqual(watcher.watchedRoots(), [ROOT]);
});
