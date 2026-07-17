import assert from 'node:assert/strict';
import test from 'node:test';
import path from 'node:path';
import {
  createAppUninstallService,
  validateBundleId,
} from './app-uninstall-service';

const home = '/Users/tester';
const appPath = '/Applications/Example.app';
const bundleId = 'com.example.App';

function fixture(
  options: { running?: boolean; symlink?: string; owner?: number } = {},
) {
  const present = new Set<string>();
  const ids = new Map<string, number>();
  let nextIno = 1;
  const add = (value: string) => {
    let current = path.parse(value).root;
    for (const part of path
      .relative(current, value)
      .split(path.sep)
      .filter(Boolean)) {
      current = path.join(current, part);
      present.add(current);
    }
  };
  add(appPath);
  add(path.join(home, 'Library', 'Caches', bundleId));
  add(path.join(home, 'Library', 'Preferences', `${bundleId}.plist`));
  const trashed: string[] = [];
  let running = Boolean(options.running);
  const service = createAppUninstallService({
    platform: 'darwin',
    homeDirectory: home,
    currentUid: 501,
    lstat: async (value) => {
      if (!present.has(value)) {
        const error = Object.assign(new Error('missing'), { code: 'ENOENT' });
        throw error;
      }
      const ino = ids.get(value) || nextIno++;
      ids.set(value, ino);
      return {
        dev: 1,
        ino,
        uid: value.includes('Caches') ? (options.owner ?? 501) : 501,
        isDirectory: () => !value.endsWith('.plist'),
        isSymbolicLink: () => value === options.symlink,
      };
    },
    realpath: async (value) => value,
    access: async () => {},
    readBundleId: async () => bundleId,
    trashItem: async (value) => {
      trashed.push(value);
      present.delete(value);
    },
    nevermindAppPath: '/Applications/Nevermind.app',
    nevermindBundleId: 'com.nevermind.app',
    runningAppPaths: async (value) => (running ? new Set([value]) : new Set()),
    randomId: (() => {
      let number = 0;
      return () => `candidate-${++number}`;
    })(),
  });
  return {
    service,
    trashed,
    ids,
    present,
    setRunning: (value: boolean) => {
      running = value;
    },
  };
}

test('rejects unsafe bundle IDs before any associated path is constructed', () => {
  for (const value of [
    '',
    'com..example',
    'com.example/escape',
    'com.example. ',
    'com.example\napp',
    'com.example..',
    'a'.repeat(256),
  ])
    assert.equal(validateBundleId(value), null);
  assert.equal(validateBundleId(bundleId), bundleId);
});

test('discovers only exact bundle-ID paths, reports missing locations, and keeps the app selected by default', async () => {
  const { service } = fixture();
  const result = await service.discover(appPath);
  assert.equal(result.status, 'ready');
  if (result.status !== 'ready') return;
  assert.deepEqual(
    result.candidates.map((item) => item.path),
    [
      appPath,
      path.join(home, 'Library', 'Caches', bundleId),
      path.join(home, 'Library', 'Preferences', `${bundleId}.plist`),
    ],
  );
  assert.equal(result.notes[0].code, 'missing-associated');
  assert.deepEqual(
    service
      .selected(result.snapshot, { [result.candidates[0].id]: true })
      .map((item) => item.path),
    [appPath],
  );
});

test('rejects symlinks and ownership mismatches without treating them as deletable candidates', async () => {
  const cachePath = path.join(home, 'Library', 'Caches', bundleId);
  const symlinked = fixture({ symlink: cachePath });
  const symlinkResult = await symlinked.service.discover(appPath);
  assert.equal(symlinkResult.status, 'ready');
  if (symlinkResult.status === 'ready')
    assert.equal(
      symlinkResult.candidates.some((item) => item.path === cachePath),
      false,
    );

  const wrongOwner = fixture({ owner: 502 });
  const ownerResult = await wrongOwner.service.discover(appPath);
  assert.equal(ownerResult.status, 'ready');
  if (ownerResult.status === 'ready')
    assert.equal(
      ownerResult.candidates.some((item) => item.path === cachePath),
      false,
    );
});

test('binds Trash to snapshot IDs, ignores injected input, revalidates identities, and moves app last', async () => {
  const { service, trashed, ids } = fixture();
  const discovery = await service.discover(appPath);
  assert.equal(discovery.status, 'ready');
  if (discovery.status !== 'ready') return;
  const [app, cache, preferences] = discovery.candidates;
  const complete = await service.trash(discovery.snapshot, {
    [app.id]: true,
    [cache.id]: true,
    injectedPath: true,
  });
  assert.equal(complete.status, 'complete');
  assert.deepEqual(trashed, [cache.path, app.path]);

  const second = fixture();
  const stale = await second.service.discover(appPath);
  assert.equal(stale.status, 'ready');
  if (stale.status !== 'ready') return;
  const staleCache = stale.candidates.find(
    (item) => item.kind === 'associated',
  )!;
  second.ids.set(staleCache.path, 9999);
  const partial = await second.service.trash(stale.snapshot, {
    [staleCache.id]: true,
  });
  assert.equal(partial.status, 'failed');
  assert.equal(second.trashed.length, 0);
  assert.equal(partial.untouched[0].code, 'changed');
  assert.ok(preferences);
});

test('does not probe or trash on zero selection and stops safely when an app becomes running', async () => {
  const item = fixture();
  const discovery = await item.service.discover(appPath);
  assert.equal(discovery.status, 'ready');
  if (discovery.status !== 'ready') return;
  const empty = await item.service.trash(discovery.snapshot, {});
  assert.equal(empty.status, 'failed');
  assert.equal(item.trashed.length, 0);
  item.setRunning(true);
  const failed = await item.service.trash(discovery.snapshot, {
    [discovery.candidates[0].id]: true,
  });
  assert.equal(failed.status, 'failed');
  assert.equal(failed.untouched[0].code, 'running');
});
