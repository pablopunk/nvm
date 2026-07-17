import assert from 'node:assert/strict';
import test from 'node:test';
import path from 'node:path';
import {
  createAppUninstallService,
  createProductionPlistReader,
  NEVERMIND_BUNDLE_ID,
  PLUTIL_OPTIONS,
  PLUTIL_PATH,
  validateBundleId,
} from './app-uninstall-service';

const home = '/Users/tester';
const appPath = '/Applications/Example.app';
const bundleId = 'com.example.App';

function fixture(
  options: {
    running?: boolean;
    symlink?: string;
    owner?: number;
    failTrashAt?: string;
    onLstat?: (value: string) => void;
    isRunning?: () => boolean;
    nevermindBundleId?: string;
    canonicalPath?: (value: string) => string;
    readBundleIdError?: Error;
  } = {},
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
  const calls: string[] = [];
  let running = Boolean(options.running);
  let symlink = options.symlink;
  let currentBundleId = bundleId;
  let runningCalls = 0;
  const service = createAppUninstallService({
    platform: 'darwin',
    homeDirectory: home,
    currentUid: 501,
    lstat: async (value) => {
      calls.push(`lstat:${value}`);
      options.onLstat?.(value);
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
        isSymbolicLink: () => value === symlink,
      };
    },
    realpath: async (value) => options.canonicalPath?.(value) || value,
    access: async () => {},
    readBundleId: async () => {
      if (options.readBundleIdError) throw options.readBundleIdError;
      return currentBundleId;
    },
    trashItem: async (value) => {
      calls.push(`trash:${value}`);
      if (value === options.failTrashAt) throw new Error('Trash denied');
      trashed.push(value);
      present.delete(value);
    },
    nevermindAppPath: '/Applications/Nevermind.app',
    nevermindBundleId: options.nevermindBundleId || 'com.nevermind.app',
    runningAppPaths: async (value) => {
      runningCalls += 1;
      calls.push(`running:${runningCalls}`);
      return running || options.isRunning?.() ? new Set([value]) : new Set();
    },
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
    calls,
    setBundleId: (value: string) => {
      currentBundleId = value;
    },
    setSymlink: (value: string | undefined) => {
      symlink = value;
    },
    getRunningCalls: () => runningCalls,
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

test('production plist reader uses a fixed absolute command with bounded execution', async () => {
  let received: unknown[] = [];
  const reader = createProductionPlistReader(async (...input) => {
    received = input;
    return { stdout: 'com.example.App\n' };
  });
  assert.equal(await reader(appPath), bundleId);
  assert.equal(received[0], PLUTIL_PATH);
  assert.deepEqual(received[2], PLUTIL_OPTIONS);
  assert.deepEqual((received[1] as string[]).slice(0, 5), [
    '-extract',
    'CFBundleIdentifier',
    'raw',
    '-o',
    '-',
  ]);
});

test('redacts process failures when app metadata cannot be read', async () => {
  const { service } = fixture({
    readBundleIdError: new Error(
      'Command failed: /usr/bin/plutil -extract CFBundleIdentifier',
    ),
  });
  const result = await service.discover(appPath);

  assert.equal(result.status, 'unavailable');
  if (result.status !== 'unavailable') return;
  assert.equal(result.reasonCode, 'plist');
  assert.equal(result.message, 'This app’s metadata could not be read');
  assert.doesNotMatch(result.message, /plutil|\/usr\/bin/);
});

test('non-macOS discovery performs no host work and the trusted production ID rejects another Nevermind copy', async () => {
  let calls = 0;
  const unsupported = createAppUninstallService({
    platform: 'linux',
    homeDirectory: home,
    currentUid: 501,
    lstat: async () => {
      calls += 1;
      throw new Error('must not run');
    },
    realpath: async () => '',
    access: async () => {},
    readBundleId: async () => {
      calls += 1;
      return bundleId;
    },
    trashItem: async () => {
      calls += 1;
    },
    runningAppPaths: async () => {
      calls += 1;
      return new Set();
    },
  });
  assert.equal((await unsupported.discover(appPath)).status, 'unavailable');
  assert.equal(calls, 0);

  const selfCopy = fixture({ nevermindBundleId: NEVERMIND_BUNDLE_ID });
  selfCopy.setBundleId(NEVERMIND_BUNDLE_ID);
  const result = await selfCopy.service.discover(appPath);
  assert.equal(result.status, 'unavailable');
  if (result.status === 'unavailable') assert.equal(result.reasonCode, 'self');
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

  const rootSymlink = fixture({
    symlink: path.join(home, 'Library', 'Caches'),
  });
  const rootResult = await rootSymlink.service.discover(appPath);
  assert.equal(rootResult.status, 'ready');
  if (rootResult.status === 'ready')
    assert.equal(
      rootResult.candidates.some((item) => item.path === cachePath),
      false,
    );

  const escaped = fixture({
    canonicalPath: (value) =>
      value === cachePath ? '/private/tmp/not-allowlisted' : value,
  });
  const escapedResult = await escaped.service.discover(appPath);
  assert.equal(escapedResult.status, 'ready');
  if (escapedResult.status === 'ready')
    assert.equal(
      escapedResult.candidates.some((item) => item.path === cachePath),
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

test('preflights every selected item before the first move and reports partial Trash failures', async () => {
  const item = fixture({
    failTrashAt: path.join(home, 'Library', 'Caches', bundleId),
  });
  const discovery = await item.service.discover(appPath);
  assert.equal(discovery.status, 'ready');
  if (discovery.status !== 'ready') return;
  const app = discovery.candidates.find(
    (candidate) => candidate.kind === 'app',
  )!;
  const cache = discovery.candidates.find(
    (candidate) => candidate.slot === 'caches',
  )!;
  const preferences = discovery.candidates.find(
    (candidate) => candidate.slot === 'preferences',
  )!;
  const result = await item.service.trash(discovery.snapshot, {
    [app.id]: true,
    [cache.id]: true,
    [preferences.id]: true,
  });
  assert.equal(result.status, 'partial');
  assert.equal(result.untouched[0].code, 'trash-failed');
  assert.deepEqual(item.trashed, [preferences.path, app.path]);
  const firstTrash = item.calls.findIndex((call) => call.startsWith('trash:'));
  assert.equal(
    item.calls.slice(0, firstTrash).includes(`lstat:${preferences.path}`),
    true,
  );
});

test('rejects app bundle changes and a running transition after candidate revalidation before Trash', async () => {
  const replaced = fixture();
  const stale = await replaced.service.discover(appPath);
  assert.equal(stale.status, 'ready');
  if (stale.status !== 'ready') return;
  replaced.setBundleId('com.example.Replaced');
  const changed = await replaced.service.trash(stale.snapshot, {
    [stale.candidates[0].id]: true,
  });
  assert.equal(changed.status, 'failed');
  assert.equal(changed.untouched[0].code, 'app-changed');
  assert.equal(replaced.trashed.length, 0);

  let armed = false;
  let becameRunning = false;
  const running = fixture({
    isRunning: () => becameRunning,
    onLstat: (value) => {
      if (armed && value === path.join(home, 'Library', 'Caches', bundleId))
        becameRunning = true;
    },
  });
  const ready = await running.service.discover(appPath);
  assert.equal(ready.status, 'ready');
  if (ready.status !== 'ready') return;
  const cache = ready.candidates.find(
    (candidate) => candidate.slot === 'caches',
  )!;
  armed = true;
  const stopped = await running.service.trash(ready.snapshot, {
    [cache.id]: true,
  });
  assert.equal(stopped.status, 'failed');
  assert.equal(stopped.untouched[0].code, 'running');
  assert.equal(becameRunning, true);
  assert.equal(running.trashed.length, 0);
});
