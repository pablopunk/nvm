// biome-ignore-all lint/complexity/noExcessiveLinesPerFunction: the shared host-service fixture deliberately exposes all security seams.
import assert from 'node:assert/strict';
import nodePath from 'node:path';
import test from 'node:test';
import {
  createAppUninstallService,
  createProductionPlistReader,
  NEVERMIND_BUNDLE_ID,
  PLUTIL_OPTIONS,
  PLUTIL_PATH,
  validateBundleId,
} from './app-uninstall-service';

const path = nodePath.posix;

const home = '/Users/tester';
const appPath = '/Applications/Example.app';
const bundleId = 'com.example.App';
const currentUid = 501;
const maxBundleIdBytes = 255;
const replacementInode = 9999;
const cachePath = path.join(home, 'Library', 'Caches', bundleId);
const preferencesPath = path.join(
  home,
  'Library',
  'Preferences',
  `${bundleId}.plist`,
);
const applicationSupportRoot = path.join(
  home,
  'Library',
  'Application Support',
);
const cachesRoot = path.join(home, 'Library', 'Caches');

function missingError() {
  return Object.assign(new Error('missing'), { code: 'ENOENT' });
}

function plistFileSystem(
  paths: Record<string, string>,
  directories: Record<
    string,
    Array<{
      name: string;
      isDirectory: () => boolean;
      isSymbolicLink: () => boolean;
    }>
  > = {},
) {
  return {
    realpath: (value: string) => {
      const canonical = paths[value];
      return canonical
        ? Promise.resolve(canonical)
        : Promise.reject(missingError());
    },
    readdir: (value: string) =>
      directories[value]
        ? Promise.resolve(directories[value])
        : Promise.reject(missingError()),
  };
}

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
    readBundleId?: (value: string) => Promise<unknown>;
    otherApps?: Record<string, string | null>;
    entries?: Record<string, string[]>;
    quitWorks?: boolean;
    runningCheckFails?: boolean;
    findMatchingApps?: (bundleId: string) => Promise<string[]>;
  } = {},
) {
  const present = new Set<string>();
  const ids = new Map<string, number>();
  const listings = new Map<string, string[]>();
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
  const list = (directory: string, name: string) => {
    listings.set(directory, [...(listings.get(directory) ?? []), name]);
  };
  add(appPath);
  list('/Applications', path.basename(appPath));
  add(cachePath);
  add(preferencesPath);
  const otherApps = options.otherApps ?? {};
  for (const otherPath of Object.keys(otherApps)) {
    add(otherPath);
    list(path.dirname(otherPath), path.basename(otherPath));
  }
  for (const [directory, names] of Object.entries(options.entries ?? {})) {
    for (const name of names) {
      add(path.join(directory, name));
      list(directory, name);
    }
  }
  const calls: string[] = [];
  const trashed: string[] = [];
  let running = Boolean(options.running);
  let symlink = options.symlink;
  let currentBundleId = bundleId;
  let runningCalls = 0;
  const service = createAppUninstallService({
    platform: 'darwin',
    homeDirectory: home,
    currentUid,
    lstat: (value) => {
      calls.push(`lstat:${value}`);
      options.onLstat?.(value);
      if (!present.has(value)) {
        throw missingError();
      }
      const ino = ids.get(value) || nextIno++;
      ids.set(value, ino);
      return Promise.resolve({
        dev: 1,
        ino,
        uid: value.includes('Caches')
          ? (options.owner ?? currentUid)
          : currentUid,
        isDirectory: () => !value.endsWith('.plist'),
        isSymbolicLink: () => value === symlink,
      });
    },
    realpath: async (value) => options.canonicalPath?.(value) || value,
    access: async () => {},
    listDirectory: (value) => {
      const names = listings.get(value);
      return names
        ? Promise.resolve([...names])
        : Promise.reject(missingError());
    },
    readBundleId: async (value) => {
      if (options.readBundleIdError) throw options.readBundleIdError;
      if (value in otherApps) {
        const otherBundleId = otherApps[value];
        if (otherBundleId === null) throw new Error('unreadable');
        return otherBundleId;
      }
      return options.readBundleId
        ? options.readBundleId(value)
        : currentBundleId;
    },
    findMatchingApps: options.findMatchingApps,
    trashItem: async (value) => {
      calls.push(`trash:${value}`);
      if (value === options.failTrashAt) {
        throw new Error('Trash denied');
      }
      trashed.push(value);
      present.delete(value);
      return Promise.resolve();
    },
    requestQuit: async (value) => {
      calls.push(`quit:${value}`);
      if (options.quitWorks !== false) {
        running = false;
      }
    },
    nevermindAppPath: '/Applications/Nevermind.app',
    nevermindBundleId: options.nevermindBundleId || 'com.nevermind.app',
    runningAppPaths: (value) => {
      runningCalls += 1;
      calls.push(`running:${runningCalls}`);
      if (options.runningCheckFails) {
        return Promise.reject(new Error('process inspection failed'));
      }
      return Promise.resolve(
        running || options.isRunning?.() ? new Set([value]) : new Set(),
      );
    },
    randomId: (() => {
      let number = 0;
      return () => `candidate-${++number}`;
    })(),
    wait: () => Promise.resolve(),
    quitTimeoutMs: 1000,
    quitPollMs: 250,
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

function ready(
  result: Awaited<
    ReturnType<ReturnType<typeof fixture>['service']['discover']>
  >,
) {
  assert.equal(result.status, 'ready');
  if (result.status !== 'ready') {
    throw new Error('expected ready discovery');
  }
  return result;
}

function candidateFor(result: ReturnType<typeof ready>, candidatePath: string) {
  const candidate = result.candidates.find(
    (item) => item.path === candidatePath,
  );
  assert.ok(candidate, `expected candidate ${candidatePath}`);
  return candidate;
}

test('rejects unsafe bundle IDs before any associated path is constructed', () => {
  for (const value of [
    '',
    'com..example',
    'com.example/escape',
    'com.example. ',
    'com.example\napp',
    'com.example..',
    'a'.repeat(maxBundleIdBytes + 1),
  ]) {
    assert.equal(validateBundleId(value), null);
  }
  assert.equal(validateBundleId(bundleId), bundleId);
});

test('production plist reader uses a fixed direct-bundle command with bounded execution', async () => {
  let received: unknown[] = [];
  const spacedAppPath = '/Applications/Visual Studio Code.app';
  const directPlistPath = path.join(spacedAppPath, 'Contents', 'Info.plist');
  const reader = createProductionPlistReader(
    (...input) => {
      received = input;
      return Promise.resolve({ stdout: 'com.example.App\n' });
    },
    plistFileSystem({
      [spacedAppPath]: spacedAppPath,
      [directPlistPath]: directPlistPath,
    }),
  );
  assert.equal(await reader(spacedAppPath), bundleId);
  assert.equal(received[0], PLUTIL_PATH);
  assert.deepEqual(received[2], PLUTIL_OPTIONS);
  assert.deepEqual((received[1] as string[]).slice(0, -1), [
    '-extract',
    'CFBundleIdentifier',
    'raw',
    '-o',
    '-',
  ]);
  assert.deepEqual(received[1], [
    '-extract',
    'CFBundleIdentifier',
    'raw',
    '-o',
    '-',
    directPlistPath,
  ]);
});

test('production plist reader resolves a contained Wrapper app when the top-level plist is missing', async () => {
  let received: unknown[] = [];
  const wrappedAppPath = appPath;
  const wrapperPath = path.join(wrappedAppPath, 'Wrapper');
  const innerAppPath = path.join(wrapperPath, 'ios_prod.app');
  const innerPlistPath = path.join(innerAppPath, 'Info.plist');
  const reader = createProductionPlistReader(
    (...input) => {
      received = input;
      return Promise.resolve({ stdout: `${bundleId}\n` });
    },
    plistFileSystem(
      {
        [wrappedAppPath]: wrappedAppPath,
        [innerPlistPath]: innerPlistPath,
      },
      {
        [wrapperPath]: [
          {
            name: 'ios_prod.app',
            isDirectory: () => true,
            isSymbolicLink: () => false,
          },
        ],
      },
    ),
  );
  const wrapped = fixture({ readBundleId: reader });
  const result = await wrapped.service.discover(appPath);
  assert.equal(result.status, 'ready');
  assert.equal(await reader(wrappedAppPath), bundleId);
  assert.equal((received[1] as string[]).at(-1), innerPlistPath);
});

test('production plist reader rejects Wrapper metadata that canonically escapes the selected app', async () => {
  const wrapperPath = path.join(appPath, 'Wrapper');
  const innerPlistPath = path.join(wrapperPath, 'ios_prod.app', 'Info.plist');
  const reader = createProductionPlistReader(
    () => Promise.resolve({ stdout: bundleId }),
    plistFileSystem(
      {
        [appPath]: appPath,
        [innerPlistPath]: '/private/tmp/escaped-Info.plist',
      },
      {
        [wrapperPath]: [
          {
            name: 'ios_prod.app',
            isDirectory: () => true,
            isSymbolicLink: () => false,
          },
        ],
      },
    ),
  );
  const unavailable = fixture({ readBundleId: reader });
  const result = await unavailable.service.discover(appPath);
  assert.equal(result.status, 'unavailable');
  if (result.status === 'unavailable') {
    assert.equal(result.reasonCode, 'plist');
  }
});

test('unsupported missing-top-level-plist layouts produce concise actionable text', async () => {
  const reader = createProductionPlistReader(
    () => Promise.resolve({ stdout: bundleId }),
    plistFileSystem({ [appPath]: appPath }),
  );
  const unavailable = fixture({ readBundleId: reader });
  const result = await unavailable.service.discover(appPath);
  assert.equal(result.status, 'unavailable');
  if (result.status === 'unavailable') {
    assert.equal(result.reasonCode, 'plist');
    assert.equal(result.message, 'This app’s metadata could not be read');
    assert.equal(result.message.includes('plutil'), false);
  }
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
    currentUid,
    lstat: () => {
      calls += 1;
      throw new Error('must not run');
    },
    realpath: async () => '',
    access: () => Promise.resolve(),
    listDirectory: () => {
      calls += 1;
      return Promise.resolve([]);
    },
    readBundleId: () => {
      calls += 1;
      return Promise.resolve(bundleId);
    },
    trashItem: () => {
      calls += 1;
      return Promise.resolve();
    },
    requestQuit: () => {
      calls += 1;
      return Promise.resolve();
    },
    runningAppPaths: () => {
      calls += 1;
      return Promise.resolve(new Set());
    },
  });
  assert.equal((await unsupported.discover(appPath)).status, 'unavailable');
  assert.equal(calls, 0);

  const selfCopy = fixture({ nevermindBundleId: NEVERMIND_BUNDLE_ID });
  selfCopy.setBundleId(NEVERMIND_BUNDLE_ID);
  const result = await selfCopy.service.discover(appPath);
  assert.equal(result.status, 'unavailable');
  if (result.status === 'unavailable') {
    assert.equal(result.reasonCode, 'self');
  }
});

test('selects exact bundle-ID data when no conflicting app is found', async () => {
  const { service } = fixture();
  const result = ready(await service.discover(appPath));
  assert.deepEqual(
    result.candidates.map((item) => item.path),
    [appPath, cachePath, preferencesPath],
  );
  assert.equal(result.notes[0].code, 'missing-associated');
  assert.equal(result.appRunning, false);
  assert.deepEqual(
    result.candidates
      .filter((item) => item.selectedByDefault)
      .map((item) => item.path),
    [appPath, cachePath, preferencesPath],
  );
  assert.deepEqual(
    service
      .selected(result.snapshot, { [result.candidates[0].id]: true })
      .map((item) => item.path),
    [appPath],
  );
});

test('discovery stays available while the app runs and reports the running state on the app item', async () => {
  const { service } = fixture({ running: true });
  const result = ready(await service.discover(appPath));
  assert.equal(result.appRunning, true);
  const app = candidateFor(result, appPath);
  assert.equal(app.selectedByDefault, true);
  assert.match(app.reason, /quit gracefully/);
});

test('rejects symlinks and ownership mismatches without treating them as deletable candidates', async () => {
  const symlinked = fixture({ symlink: cachePath });
  const symlinkResult = await symlinked.service.discover(appPath);
  assert.equal(symlinkResult.status, 'ready');
  if (symlinkResult.status === 'ready') {
    assert.equal(
      symlinkResult.candidates.some((item) => item.path === cachePath),
      false,
    );
  }

  const wrongOwner = fixture({ owner: 502 });
  const ownerResult = await wrongOwner.service.discover(appPath);
  assert.equal(ownerResult.status, 'ready');
  if (ownerResult.status === 'ready') {
    assert.equal(
      ownerResult.candidates.some((item) => item.path === cachePath),
      false,
    );
  }

  const rootSymlink = fixture({
    symlink: path.join(home, 'Library', 'Caches'),
  });
  const rootResult = await rootSymlink.service.discover(appPath);
  assert.equal(rootResult.status, 'ready');
  if (rootResult.status === 'ready') {
    assert.equal(
      rootResult.candidates.some((item) => item.path === cachePath),
      false,
    );
  }

  const escaped = fixture({
    canonicalPath: (value) =>
      value === cachePath ? '/private/tmp/not-allowlisted' : value,
  });
  const escapedResult = await escaped.service.discover(appPath);
  assert.equal(escapedResult.status, 'ready');
  if (escapedResult.status === 'ready') {
    assert.equal(
      escapedResult.candidates.some((item) => item.path === cachePath),
      false,
    );
  }
});

test('demotes exact data when another installed app shares or overlaps the bundle ID', async () => {
  const shared = fixture({
    otherApps: { '/Applications/Helper.app': 'com.example.App.Helper' },
  });
  const sharedResult = ready(await shared.service.discover(appPath));
  const sharedCache = candidateFor(sharedResult, cachePath);
  assert.equal(sharedCache.selectedByDefault, false);
  assert.match(sharedCache.reason, /May be shared with Helper/);
  assert.equal(candidateFor(sharedResult, appPath).selectedByDefault, true);

  const unrelated = fixture({
    otherApps: {
      '/Applications/Appetite.app': 'com.example.Appetite',
      '/Applications/Tool.app': 'com.other.Tool',
    },
  });
  const unrelatedResult = ready(await unrelated.service.discover(appPath));
  assert.equal(
    candidateFor(unrelatedResult, cachePath).selectedByDefault,
    true,
  );
});

test('demotes exact data when another installed app cannot be read to rule out ownership', async () => {
  const unreadable = fixture({
    otherApps: { '/Applications/Broken.app': null },
  });
  const result = ready(await unreadable.service.discover(appPath));
  const cache = candidateFor(result, cachePath);
  assert.equal(cache.selectedByDefault, false);
  assert.equal(
    cache.reason,
    'Other installed apps could not be ruled out; left unchecked',
  );
});

test('uses the application index to find nested conflicting apps and handles incomplete results', async () => {
  const nested = fixture({
    findMatchingApps: async () => [appPath, '/Applications/Vendor/Helper.app'],
  });
  const shared = ready(await nested.service.discover(appPath));
  assert.equal(candidateFor(shared, cachePath).selectedByDefault, false);

  const incomplete = fixture({ findMatchingApps: async () => [] });
  const unverified = ready(await incomplete.service.discover(appPath));
  assert.equal(candidateFor(unverified, cachePath).selectedByDefault, false);

  const unavailable = fixture({
    findMatchingApps: () => Promise.reject(new Error('index unavailable')),
  });
  const unavailableResult = ready(await unavailable.service.discover(appPath));
  assert.equal(
    candidateFor(unavailableResult, cachePath).selectedByDefault,
    false,
  );

  const unique = fixture({ findMatchingApps: async () => [appPath] });
  const verified = ready(await unique.service.discover(appPath));
  assert.equal(candidateFor(verified, cachePath).selectedByDefault, true);
});

test('lists bounded prefix and name matches unchecked with reasons and never includes group containers', async () => {
  const helperPath = path.join(
    applicationSupportRoot,
    'com.example.App.Helper',
  );
  const namePath = path.join(applicationSupportRoot, 'Example');
  const shipItPath = path.join(cachesRoot, 'com.example.App.ShipIt');
  const groupPath = path.join(
    home,
    'Library',
    'Group Containers',
    `group.${bundleId}`,
  );
  const { service } = fixture({
    entries: {
      [applicationSupportRoot]: [
        'com.example.App.Helper',
        'Example',
        'com.example.Other',
        'com.example.AppOther',
      ],
      [cachesRoot]: ['com.example.App.ShipIt'],
      [path.join(home, 'Library', 'Group Containers')]: [`group.${bundleId}`],
    },
  });
  const result = ready(await service.discover(appPath));
  for (const matched of [helperPath, namePath, shipItPath]) {
    const candidate = candidateFor(result, matched);
    assert.equal(candidate.kind, 'associated');
    assert.equal(candidate.selectedByDefault, false);
  }
  assert.match(candidateFor(result, helperPath).reason, /prefix match/);
  assert.match(candidateFor(result, namePath).reason, /Name match/);
  assert.equal(
    result.candidates.some((item) => item.path === groupPath),
    false,
  );
  assert.equal(
    result.candidates.some((item) => item.path.endsWith('com.example.Other')),
    false,
  );
  assert.equal(
    result.candidates.some((item) =>
      item.path.endsWith('com.example.AppOther'),
    ),
    false,
  );
});

test('caps pattern matches and reports the limit', async () => {
  const names = Array.from(
    { length: 25 },
    (_, index) => `com.example.App.Helper${index}`,
  );
  const { service } = fixture({ entries: { [applicationSupportRoot]: names } });
  const result = ready(await service.discover(appPath));
  const patternCandidates = result.candidates.filter((item) =>
    item.path.startsWith(applicationSupportRoot),
  );
  assert.equal(patternCandidates.length, 20);
  assert.ok(result.notes.some((note) => note.code === 'pattern-limit'));
});

test('binds Trash to snapshot IDs, ignores injected input, revalidates identities, and moves app last', async () => {
  const { service, trashed } = fixture();
  const discovery = ready(await service.discover(appPath));
  const [app, cache, preferences] = discovery.candidates;
  const complete = await service.trash(discovery.snapshot, {
    [app.id]: true,
    [cache.id]: true,
    injectedPath: true,
  });
  assert.equal(complete.status, 'complete');
  assert.deepEqual(trashed, [cache.path, app.path]);
  assert.ok(preferences);

  const second = fixture();
  const stale = ready(await second.service.discover(appPath));
  const staleCache = stale.candidates.find(
    (item) => item.kind === 'associated',
  );
  assert.ok(staleCache);
  second.ids.set(staleCache.path, replacementInode);
  const partial = await second.service.trash(stale.snapshot, {
    [staleCache.id]: true,
  });
  assert.equal(partial.status, 'failed');
  assert.equal(second.trashed.length, 0);
  assert.equal(partial.untouched[0].code, 'changed');
});

test('does not probe or trash on zero selection and keeps everything when a selected running app will not quit', async () => {
  const item = fixture({ running: true, quitWorks: false });
  const discovery = ready(await item.service.discover(appPath));
  const empty = await item.service.trash(discovery.snapshot, {});
  assert.equal(empty.status, 'failed');
  assert.equal(item.trashed.length, 0);

  const failed = await item.service.trash(discovery.snapshot, {
    [candidateFor(discovery, appPath).id]: true,
    [candidateFor(discovery, cachePath).id]: true,
  });
  assert.equal(failed.status, 'failed');
  assert.deepEqual(
    failed.untouched.map((untouched) => untouched.code),
    ['quit-timeout', 'quit-timeout'],
  );
  assert.equal(item.trashed.length, 0);
  assert.ok(item.calls.includes(`quit:${appPath}`));
});

test('does not move items when running process inspection fails', async () => {
  const item = fixture({ runningCheckFails: true });
  const discovery = ready(await item.service.discover(appPath));
  const result = await item.service.trash(discovery.snapshot, {
    [candidateFor(discovery, appPath).id]: true,
    [candidateFor(discovery, cachePath).id]: true,
  });
  assert.equal(result.status, 'failed');
  assert.deepEqual(item.trashed, []);
});

test('quits a selected running app gracefully before moving associated data and the app itself', async () => {
  const item = fixture({ running: true });
  const discovery = ready(await item.service.discover(appPath));
  const result = await item.service.trash(discovery.snapshot, {
    [candidateFor(discovery, appPath).id]: true,
    [candidateFor(discovery, cachePath).id]: true,
  });
  assert.equal(result.status, 'complete');
  assert.deepEqual(item.trashed, [cachePath, appPath]);
  const quitIndex = item.calls.indexOf(`quit:${appPath}`);
  const firstTrash = item.calls.findIndex((call) => call.startsWith('trash:'));
  assert.ok(quitIndex >= 0 && quitIndex < firstTrash);
  assert.ok(item.calls.every((call) => !call.includes('kill')));
});

test('refuses to move associated data of a running app unless the app itself is selected', async () => {
  const item = fixture({ running: true });
  const discovery = ready(await item.service.discover(appPath));
  const result = await item.service.trash(discovery.snapshot, {
    [candidateFor(discovery, cachePath).id]: true,
  });
  assert.equal(result.status, 'failed');
  assert.equal(result.untouched[0].code, 'running');
  assert.equal(item.trashed.length, 0);
  assert.equal(
    item.calls.some((call) => call.startsWith('quit:')),
    false,
  );
});

test('keeps the app when a selected associated item fails so the uninstall can be retried', async () => {
  const item = fixture({ failTrashAt: cachePath });
  const discovery = ready(await item.service.discover(appPath));
  const app = candidateFor(discovery, appPath);
  const cache = candidateFor(discovery, cachePath);
  const preferences = candidateFor(discovery, preferencesPath);
  const result = await item.service.trash(discovery.snapshot, {
    [app.id]: true,
    [cache.id]: true,
    [preferences.id]: true,
  });
  assert.equal(result.status, 'partial');
  assert.deepEqual(item.trashed, [preferencesPath]);
  assert.deepEqual(
    result.untouched.map((untouched) => [untouched.path, untouched.code]),
    [
      [cachePath, 'trash-failed'],
      [appPath, 'associated-failed'],
    ],
  );
  assert.equal(item.present.has(appPath), true);
});

test('reports moved data and retains the app when the app cannot be moved', async () => {
  const item = fixture({ failTrashAt: appPath });
  const discovery = ready(await item.service.discover(appPath));
  const result = await item.service.trash(discovery.snapshot, {
    [candidateFor(discovery, appPath).id]: true,
    [candidateFor(discovery, cachePath).id]: true,
  });
  assert.equal(result.status, 'partial');
  assert.deepEqual(result.moved, [cachePath]);
  assert.equal(result.untouched[0].path, appPath);
  assert.equal(item.present.has(appPath), true);
});

test('rejects app bundle changes and a running transition after candidate revalidation before Trash', async () => {
  const replaced = fixture();
  const stale = ready(await replaced.service.discover(appPath));
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
      if (armed && value === cachePath) {
        becameRunning = true;
      }
    },
  });
  const discovered = ready(await running.service.discover(appPath));
  const cache = candidateFor(discovered, cachePath);
  armed = true;
  const stopped = await running.service.trash(discovered.snapshot, {
    [cache.id]: true,
  });
  assert.equal(stopped.status, 'failed');
  assert.equal(stopped.untouched[0].code, 'running');
  assert.equal(becameRunning, true);
  assert.equal(running.trashed.length, 0);
});
