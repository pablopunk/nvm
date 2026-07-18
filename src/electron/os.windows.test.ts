// biome-ignore-all lint/suspicious/useAwait lint/style/useBlockStatements lint/suspicious/noEmptyBlockStatements lint/complexity/noForEach lint/suspicious/useIterableCallbackReturn lint/performance/noAwaitInLoops lint/style/useNamingConvention: Injectable test doubles intentionally favor direct recordings and real Windows environment names.
import assert from 'node:assert/strict';
import path from 'node:path';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: false },
    shell: {},
  },
});

const { createOsAdapter, validatedWindowsImageName } = await import('./os');

function windowsAdapter(overrides: Record<string, unknown> = {}) {
  return createOsAdapter({
    environment: {
      ProgramData: String.raw`C:\ProgramData`,
      APPDATA: String.raw`C:\Users\Zoë\AppData\Roaming`,
    },
    homeDirectory: String.raw`C:\Users\Zoë`,
    pathFacade: path.win32,
    processPlatform: 'win32',
    ...overrides,
  });
}

test('simulates Windows labels, capabilities, and exact platform paths off-host', () => {
  const adapter = windowsAdapter();
  assert.equal(adapter.osLabel(), 'Windows');
  assert.equal(adapter.settingsTitle(), 'Open Settings');
  assert.equal(adapter.hasCapability('app-icons'), true);
  assert.equal(adapter.hasCapability('quick-look'), false);
  assert.equal(adapter.hasCapability('launch-at-login'), true);
  assert.deepEqual(adapter.appScanRoots(), [
    String.raw`C:\ProgramData\Microsoft\Windows\Start Menu\Programs`,
    String.raw`C:\Users\Zoë\AppData\Roaming\Microsoft\Windows\Start Menu\Programs`,
  ]);
  assert.equal(
    adapter.knownUserFolder('Downloads'),
    String.raw`C:\Users\Zoë\Downloads`,
  );
  assert.equal(
    createOsAdapter({
      environment: { ProgramData: String.raw`\\server\share` },
      homeDirectory: String.raw`C:\Users\Zoë`,
      pathFacade: path.win32,
      processPlatform: 'win32',
    }).appScanRoots()[0],
    String.raw`\\server\share\Microsoft\Windows\Start Menu\Programs`,
  );
});

test('recursively discovers Windows shortcuts and ignores inaccessible roots', async () => {
  const roots = new Map<string, Array<{ name: string; directory: boolean }>>([
    [
      String.raw`C:\ProgramData\Microsoft\Windows\Start Menu\Programs`,
      [
        { name: 'Tools', directory: true },
        { name: 'Nevermind.lnk', directory: false },
      ],
    ],
    [
      String.raw`C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Tools`,
      [{ name: 'Unicode 应用.LNK', directory: false }],
    ],
  ]);
  const adapter = windowsAdapter({
    fileSystem: {
      readdir: async (root: string) => {
        if (!roots.has(root)) throw new Error('access denied');
        return roots.get(root)?.map((entry) => ({
          name: entry.name,
          isDirectory: () => entry.directory,
        }));
      },
    },
  });

  assert.deepEqual(
    (await adapter.scanWindowsApps()).sort((a, b) =>
      a.name.localeCompare(b.name),
    ),
    [
      {
        id: String.raw`C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Nevermind.lnk`,
        name: 'Nevermind',
        path: String.raw`C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Nevermind.lnk`,
      },
      {
        id: String.raw`C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Tools\Unicode 应用.LNK`,
        name: 'Unicode 应用',
        path: String.raw`C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Tools\Unicode 应用.LNK`,
      },
    ],
  );
});

test('Windows watcher uses recursive mode and returns every cleanup handle', () => {
  const calls: Array<{ root: string; recursive: boolean }> = [];
  let closes = 0;
  const adapter = windowsAdapter({
    watch: (root: string, options: { recursive: boolean }) => {
      calls.push({ root, recursive: options.recursive });
      return {
        close: () => {
          closes += 1;
        },
        on: () => {},
      };
    },
  });

  const watchers = adapter.watchApps(() => {});
  assert.equal(watchers.length, 2);
  assert.equal(
    calls.every((call) => call.recursive),
    true,
  );
  watchers.forEach((watcher) => watcher.close());
  assert.equal(closes, 2);
});

test('Windows app launch passes the exact shortcut path to Electron shell', async () => {
  const opened: string[] = [];
  const adapter = windowsAdapter({
    shell: { openPath: async (candidate: string) => opened.push(candidate) },
  });
  const candidate = String.raw`\\server\share\Apps\Never mind 应用.lnk`;
  await adapter.launchWindowsApp(candidate);
  assert.deepEqual(opened, [candidate]);
});

test('force quit validates raw names before normalization and passes one argv element', async () => {
  const calls: Array<{ command: string; args: string[] }> = [];
  const adapter = windowsAdapter({
    execFile: (
      command: string,
      args: string[],
      _options: unknown,
      callback: (error: Error | null) => void,
    ) => {
      calls.push({ command, args });
      callback(null);
    },
  });

  assert.equal(
    validatedWindowsImageName('  Never mind 应用.EXE  '),
    'Never mind 应用.exe',
  );
  assert.deepEqual(await adapter.forceQuitWindowsApp('Never mind 应用.exe'), {
    ok: true,
  });
  assert.deepEqual(calls, [
    { command: 'taskkill', args: ['/F', '/IM', 'Never mind 应用.exe'] },
  ]);

  for (const rejected of [
    'bad\nname',
    'bad\tname',
    'bad/name',
    String.raw`bad\name`,
    'bad*name',
    'bad?name',
    'bad&name',
    'bad|name',
    'bad;name',
    'bad%name',
    'bad\u00a0name',
    '.',
    '..',
  ]) {
    assert.equal(validatedWindowsImageName(rejected), null, rejected);
    assert.deepEqual(await adapter.forceQuitWindowsApp(rejected), {
      ok: false,
      error: 'Invalid Windows process name',
    });
  }
  assert.equal(calls.length, 1);
});

test('unsupported capabilities and unpackaged launch-at-login fail safely', () => {
  const linux = createOsAdapter({ processPlatform: 'linux' });
  assert.deepEqual(linux.setLaunchAtLoginEnabled(true), {
    ok: false,
    message: 'Start at login is not available on Linux',
  });
  assert.deepEqual(windowsAdapter().setLaunchAtLoginEnabled(true), {
    ok: false,
    message: 'Start at login is only available in packaged builds',
  });
});
