// biome-ignore-all lint/style/noMagicNumbers: helper exit codes and fixture geometry read best as literals
import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: { app: {}, systemPreferences: {} },
});
mock.module('electron-log/main', {
  defaultExport: {
    debug: () => undefined,
    info: () => undefined,
    warn: () => undefined,
  },
});

const { createMacosWindowManager } = await import('./macos-window-manager');

import type { HelperEnvelope } from './window-management-types';

const HELPER_CODES = [
  'denied',
  'no-window',
  'target-lost',
  'unsupported',
  'unapplied',
  'error',
] as const;
const UNREADABLE_OUTPUTS = [
  '',
  'not json',
  '{"code":"maybe"}',
  '{"ok":true}',
  '[]',
];
const HELPER_UNREADABLE = /helper returned an unreadable/;
const SAMPLE_FRAME = { x: 10, y: 20, width: 300, height: 400 };
const SAMPLE_REQUEST = { pid: 42, windowKey: '42:7', frame: SAMPLE_FRAME };

interface Run {
  exitCode?: number;
  stdout?: string;
}

function envelopeJson(code: string, extra: Record<string, unknown> = {}) {
  return JSON.stringify({ ok: code === 'ok', code, ...extra });
}

function manager(run: Run | Error, trusted = true) {
  const args: string[][] = [];
  const windowManager = createMacosWindowManager({
    trusted: () => trusted,
    run(commandArgs) {
      args.push(commandArgs);
      if (run instanceof Error) {
        return Promise.reject(run);
      }
      return Promise.resolve({
        exitCode: run.exitCode ?? 0,
        stdout: run.stdout ?? '',
        stderr: '',
      });
    },
  });
  return { windowManager, args };
}

for (const code of HELPER_CODES) {
  test(`returns the ${code} helper code from the JSON envelope`, async () => {
    const { windowManager } = manager({
      exitCode: 1,
      stdout: envelopeJson(code, { message: `${code} message` }),
    });
    const result = await windowManager.focusedWindow(42);
    assert.deepEqual(
      [result.ok, result.code, result.message],
      [false, code, `${code} message`],
    );
  });
}

test('trusts the JSON denied code over a zero exit code', async () => {
  const { windowManager } = manager({
    exitCode: 0,
    stdout: envelopeJson('denied'),
  });
  assert.equal((await windowManager.displays()).code, 'denied');
});

test('trusts the JSON ok code over a non-zero exit code', async () => {
  const { windowManager } = manager({
    exitCode: 2,
    stdout: envelopeJson('ok', { data: [] }),
  });
  assert.deepEqual(await windowManager.displays(), {
    ok: true,
    code: 'ok',
    data: [],
    message: undefined,
  });
});

for (const stdout of UNREADABLE_OUTPUTS) {
  test(`maps unreadable helper output ${JSON.stringify(stdout)} to an error envelope`, async () => {
    const { windowManager } = manager({ exitCode: 0, stdout });
    const result = await windowManager.focusedWindow(42);
    assert.equal(result.ok, false);
    assert.equal(result.code, 'error');
    assert.match(result.message ?? '', HELPER_UNREADABLE);
  });
}

test('maps a spawn failure or timeout to an error envelope', async () => {
  const { windowManager } = manager(new Error('spawn ETIMEDOUT'));
  assert.deepEqual(await windowManager.setFrame(SAMPLE_REQUEST), {
    ok: false,
    code: 'error',
    message: 'The window helper failed. Restart Nevermind and try again.',
  });
});

test('never spawns the helper when Accessibility is not trusted', async () => {
  const { windowManager, args } = manager(
    { stdout: envelopeJson('ok') },
    false,
  );
  const results = await Promise.all([
    windowManager.displays(),
    windowManager.focusedWindow(42),
    windowManager.setFrame(SAMPLE_REQUEST),
    windowManager.setFullScreen(42, '42:7', true),
  ]);
  for (const result of results) {
    assert.equal(result.ok, false);
    assert.equal(result.code, 'denied');
  }
  assert.equal(await windowManager.permissionState(), 'denied');
  assert.deepEqual(args, []);
});

test('builds window subcommands for reads and writes', async () => {
  const { windowManager, args } = manager({
    stdout: envelopeJson('ok', { data: {} }),
  });
  await windowManager.displays();
  await windowManager.focusedWindow(42);
  await windowManager.setFrame(SAMPLE_REQUEST);
  await windowManager.setFrame({ ...SAMPLE_REQUEST, positionOnly: true });
  await windowManager.setFullScreen(42, '42:7', true);
  assert.deepEqual(args, [
    ['displays'],
    ['focused', '42'],
    ['set-frame', '42', '42:7', '10', '20', '300', '400'],
    ['set-frame', '42', '42:7', '10', '20', '--position-only'],
    ['set-fullscreen', '42', '42:7', 'true'],
  ]);
});

test('reports allowed only when the permissions envelope says ok', async () => {
  const allowed = await manager({
    stdout: envelopeJson('ok'),
  }).windowManager.permissionState();
  const denied = await manager({
    exitCode: 2,
    stdout: envelopeJson('denied'),
  }).windowManager.permissionState();
  assert.equal(allowed, 'allowed');
  assert.equal(denied, 'denied');
});

test('reports an unknown permission state as unknown, never allowed', async () => {
  const unreadable = await manager({
    exitCode: 0,
    stdout: '',
  }).windowManager.permissionState();
  const errored = await manager({
    exitCode: 1,
    stdout: envelopeJson('error'),
  }).windowManager.permissionState();
  const spawnFailed = await manager(
    new Error('spawn failed'),
  ).windowManager.permissionState();
  assert.deepEqual(
    [unreadable, errored, spawnFailed],
    ['unknown', 'unknown', 'unknown'],
  );
});

test('reports an unavailable trust probe as unknown without running the helper', async () => {
  const throwingTrust = createMacosWindowManager({
    trusted() {
      throw new Error('System preferences unavailable');
    },
    run() {
      return Promise.reject(new Error('The helper must not run'));
    },
  });
  assert.equal(await throwingTrust.permissionState(), 'unknown');
  assert.equal((await throwingTrust.displays()).code, 'error');
});

type WindowManagerUnderTest = ReturnType<typeof manager>['windowManager'];

const WINDOW_LIST_METHODS: Record<
  string,
  (windowManager: WindowManagerUnderTest) => Promise<HelperEnvelope<unknown>>
> = {
  runningApps: (windowManager) => windowManager.runningApps(),
  allWindows: (windowManager) => windowManager.allWindows(),
  windows: (windowManager) => windowManager.windows(42),
  raiseWindow: (windowManager) => windowManager.raiseWindow(42, '42:7'),
};

for (const [methodName, callMethod] of Object.entries(WINDOW_LIST_METHODS)) {
  for (const code of HELPER_CODES) {
    test(`${methodName} returns the ${code} helper code from the JSON envelope`, async () => {
      const { windowManager } = manager({
        exitCode: 1,
        stdout: envelopeJson(code, { message: `${code} message` }),
      });
      const result = await callMethod(windowManager);
      assert.deepEqual(
        [result.ok, result.code, result.message],
        [false, code, `${code} message`],
      );
    });
  }

  for (const stdout of UNREADABLE_OUTPUTS) {
    test(`${methodName} maps unreadable output ${JSON.stringify(stdout)} to an error envelope`, async () => {
      const { windowManager } = manager({ exitCode: 0, stdout });
      const result = await callMethod(windowManager);
      assert.equal(result.code, 'error');
      assert.match(result.message ?? '', HELPER_UNREADABLE);
    });
  }

  test(`${methodName} never spawns the helper when Accessibility is not trusted`, async () => {
    const { windowManager, args } = manager(
      { stdout: envelopeJson('ok') },
      false,
    );
    const result = await callMethod(windowManager);
    assert.deepEqual([result.ok, result.code], [false, 'denied']);
    assert.deepEqual(args, []);
  });
}

test('lists running apps with the apps subcommand', async () => {
  const apps = [{ pid: 42, bundleId: 'com.apple.finder', name: 'Finder' }];
  const { windowManager, args } = manager({
    stdout: envelopeJson('ok', { data: apps }),
  });
  assert.deepEqual(await windowManager.runningApps(), {
    ok: true,
    code: 'ok',
    data: apps,
    message: undefined,
  });
  assert.deepEqual(args, [['apps']]);
});

test('lists every window across apps with the all subcommand', async () => {
  const listed = [
    {
      pid: 42,
      appName: 'Finder',
      zIndex: 0,
      bundleId: 'com.apple.finder',
      windowKey: '42:7',
      title: 'Downloads',
      frame: SAMPLE_FRAME,
      displayId: '1',
      isFullScreen: false,
      isMinimized: false,
      isMovable: true,
      isResizable: true,
    },
  ];
  const { windowManager, args } = manager({
    stdout: envelopeJson('ok', { data: listed }),
  });
  assert.deepEqual(await windowManager.allWindows(), {
    ok: true,
    code: 'ok',
    data: listed,
    message: undefined,
  });
  assert.deepEqual(args, [['all']]);
});

test('lists windows of one pid with the list subcommand', async () => {
  const { windowManager, args } = manager({
    stdout: envelopeJson('ok', { data: [] }),
  });
  assert.deepEqual((await windowManager.windows(42)).data, []);
  assert.deepEqual(args, [['list', '42']]);
});

test('raises a window with the raise subcommand and reports success without data', async () => {
  const { windowManager, args } = manager({
    stdout: envelopeJson('ok', { data: null }),
  });
  const result = await windowManager.raiseWindow(42, '42:7');
  assert.deepEqual([result.ok, result.code], [true, 'ok']);
  assert.deepEqual(args, [['raise', '42', '42:7']]);
});

test('maps a raise target-lost envelope with its message', async () => {
  const { windowManager } = manager({
    exitCode: 4,
    stdout: envelopeJson('target-lost', {
      message: 'Window is no longer available',
    }),
  });
  assert.deepEqual(await windowManager.raiseWindow(42, '42:7'), {
    ok: false,
    code: 'target-lost',
    data: undefined,
    message: 'Window is no longer available',
  });
});
