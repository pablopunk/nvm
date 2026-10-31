import assert from 'node:assert/strict';
import path from 'node:path';
import test from 'node:test';
import {
  changedPathCanAffectFileIndex,
  changedPathCanCreateMissingRoot,
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

test('individual directory watchers include indexed subfolders and filter their events', () => {
  const fake = fakeWatchFactory();
  const nested = path.join(ROOT, 'photos', 'trip');
  let refreshes = 0;
  const watcher = createFileIndexWatcher({
    watchDirectoryTree: fake.watchDirectoryTree,
    watchDirectoriesIndividually: true,
    onRelevantChange: () => {
      refreshes += 1;
    },
  });
  watcher.sync(scope({ indexedDirectories: [ROOT, nested] }));
  fake.watchers.get(nested)?.emit('new.jpg');
  fake.watchers.get(nested)?.emit('.hidden');
  assert.equal(refreshes, 1);
  watcher.sync(scope({ indexedDirectories: [ROOT] }));
  assert.equal(fake.watchers.get(nested)?.closed, true);
  watcher.close();
});

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

const HOME = path.join(path.sep, 'home', 'me');
const DOCUMENTS = path.join(HOME, 'Documents');

test('only the child that leads to the missing root can create it', () => {
  assert.equal(
    changedPathCanCreateMissingRoot(HOME, DOCUMENTS, 'Documents'),
    true,
  );
  assert.equal(changedPathCanCreateMissingRoot(HOME, DOCUMENTS, null), true);
  assert.equal(
    changedPathCanCreateMissingRoot(HOME, DOCUMENTS, 'notes.txt'),
    false,
  );
  assert.equal(
    changedPathCanCreateMissingRoot(HOME, DOCUMENTS, '.cache'),
    false,
  );
});

test('a missing root is watched through its closest existing parent', () => {
  const fake = fakeWatchFactory();
  let refreshes = 0;
  const watcher = createFileIndexWatcher({
    watchDirectoryTree: fake.watchDirectoryTree,
    watchDirectoryShallow: fake.watchDirectoryTree,
    onRelevantChange: () => {
      refreshes += 1;
    },
  });
  watcher.sync(scope({ missingRoots: [DOCUMENTS] }));
  assert.deepEqual(watcher.watchedRoots(), [ROOT]);
  assert.deepEqual(watcher.watchedMissingRoots(), [DOCUMENTS]);
  fake.watchers.get(HOME)?.emit('unrelated.txt');
  assert.equal(refreshes, 0);
  fake.watchers.get(HOME)?.emit('Documents');
  assert.equal(refreshes, 1);
  watcher.close();
});

test('a missing root under missing folders is watched at the nearest existing ancestor', () => {
  const fake = fakeWatchFactory();
  const nested = path.join(HOME, 'Work', 'Documents');
  fake.missing.add(path.join(HOME, 'Work'));
  let refreshes = 0;
  const watcher = createFileIndexWatcher({
    watchDirectoryTree: fake.watchDirectoryTree,
    watchDirectoryShallow: fake.watchDirectoryTree,
    onRelevantChange: () => {
      refreshes += 1;
    },
  });
  watcher.sync(scope({ roots: [], missingRoots: [nested] }));
  fake.watchers.get(HOME)?.emit('Work');
  assert.equal(refreshes, 1);
  fake.missing.delete(path.join(HOME, 'Work'));
  watcher.sync(scope({ roots: [], missingRoots: [nested] }));
  fake.watchers.get(path.join(HOME, 'Work'))?.emit('Documents');
  assert.equal(refreshes, 2);
  watcher.close();
});

test('a missing root watcher is dropped once the root exists or is no longer wanted', () => {
  const fake = fakeWatchFactory();
  const watcher = createFileIndexWatcher({
    watchDirectoryTree: fake.watchDirectoryTree,
    watchDirectoryShallow: fake.watchDirectoryTree,
    onRelevantChange: () => {},
  });
  watcher.sync(scope({ roots: [], missingRoots: [DOCUMENTS] }));
  watcher.sync(scope({ roots: [DOCUMENTS], missingRoots: [] }));
  assert.equal(fake.watchers.get(HOME)?.closed, true);
  assert.deepEqual(watcher.watchedMissingRoots(), []);
  watcher.close();
});

test('a failed missing root watcher is dropped, requests a refresh, and is retried', () => {
  const fake = fakeWatchFactory();
  let refreshes = 0;
  const watcher = createFileIndexWatcher({
    watchDirectoryTree: fake.watchDirectoryTree,
    watchDirectoryShallow: fake.watchDirectoryTree,
    onRelevantChange: () => {
      refreshes += 1;
    },
  });
  const missing = scope({ roots: [], missingRoots: [DOCUMENTS] });
  watcher.sync(missing);
  fake.watchers.get(HOME)?.fail();
  assert.equal(refreshes, 1);
  assert.deepEqual(watcher.watchedMissingRoots(), []);
  watcher.sync(missing);
  assert.deepEqual(watcher.watchedMissingRoots(), [DOCUMENTS]);
  watcher.close();
});
