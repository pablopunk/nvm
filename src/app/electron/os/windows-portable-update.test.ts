import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: { app: { isPackaged: true } },
});
mock.module('electron-log/main', {
  defaultExport: { error() {}, info() {}, warn() {} },
});

const {
  confirmPortableUpdateLaunch,
  consumePortableUpdateResult,
  PORTABLE_UPDATE_HEALTH_FILE_ENV,
  portableExecutableFromEnvironment,
  portableUpdateHelperCommand,
  portableUpdateHelperLaunchArguments,
  portableUpdateHelperEnvironment,
  preparePortableUpdateHelper,
} = await import('./windows-portable-update');

const target = {
  executablePath: String.raw`D:\Tools\My Nevermind.exe`,
  directory: String.raw`D:\Tools`,
};

function plan(workDirectory: string, sourcePath = 'update.exe') {
  return {
    target,
    sourcePath,
    sha512: 'c2hhNTEy+/=',
    version: '2026.1030.2',
    currentVersion: '2026.1030.1',
    workDirectory,
    waitProcessIds: [41, 40],
  };
}

test('reads the user launcher path, keeping the user-chosen filename', () => {
  assert.deepEqual(
    portableExecutableFromEnvironment({
      PORTABLE_EXECUTABLE_FILE: String.raw`D:\Tools\.\My Nevermind.exe`,
    }),
    target,
  );
  assert.equal(portableExecutableFromEnvironment({}), null);
  assert.equal(
    portableExecutableFromEnvironment({ PORTABLE_EXECUTABLE_FILE: 'x.exe' }),
    null,
  );
});

test('helper command quotes every value as a PowerShell literal', () => {
  const hostile = {
    ...plan('C:\\work'),
    target: {
      executablePath: String.raw`D:\Tools\Bob's & "x" %PATH% ’ $(calc).exe`,
      directory: String.raw`D:\Tools`,
    },
  };
  const command = portableUpdateHelperCommand(hostile, 'C:\\work\\h.ps1');
  assert.equal(command.startsWith("& 'C:\\work\\h.ps1' -TargetPath "), true);
  assert.match(
    command,
    /-TargetPath 'D:\\Tools\\Bob''s & "x" %PATH% ’’ \$\(calc\)\.exe'/,
  );
  assert.match(command, /-ExpectedSha512 'c2hhNTEy\+\/=' /);
  assert.match(command, /-WaitProcessIds '41,40'/);
  assert.match(command, /; exit \$LASTEXITCODE$/);
});

test('cmd.exe starts PowerShell in its own console and never sees a path', () => {
  const args = portableUpdateHelperLaunchArguments(
    { ...plan('C:\\work'), sourcePath: 'D:\\a&b.exe' },
    'C:\\work\\h.ps1',
    { SystemRoot: 'C:\\Windows' },
  );
  assert.deepEqual(args.slice(0, 6), [
    '/d',
    '/c',
    'start',
    '""',
    '/min',
    '"C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe"',
  ]);
  const encoded = args.at(-1) ?? '';
  assert.equal(args.at(-2), '-EncodedCommand');
  assert.match(encoded, /^[A-Za-z0-9+/=]+$/);
  assert.match(
    Buffer.from(encoded, 'base64').toString('utf16le'),
    /-SourcePath 'D:\\a&b\.exe'/,
  );
  assert.equal(args.join(' ').includes('a&b'), false);
});

test('helper environment drops wrapper identity and one-shot update variables', () => {
  assert.deepEqual(
    portableUpdateHelperEnvironment({
      PATH: 'p',
      PORTABLE_EXECUTABLE_FILE: 'f',
      PORTABLE_EXECUTABLE_DIR: 'd',
      NVM_PORTABLE_UPDATE_SMOKE_PACKAGE: 's',
      NVM_TEST_MODE: '1',
    }),
    { PATH: 'p', NVM_TEST_MODE: '1' },
  );
});

test('preparation fails before quitting when the update cannot be launched', async () => {
  const written: string[] = [];
  const fileSystem = {
    access: async (file: string) => {
      if (file.endsWith('powershell.exe')) throw new Error('missing');
    },
    mkdir: async () => undefined,
    open: async () => ({ fd: 1, close: async () => undefined }),
    rm: async () => undefined,
    writeFile: async (file: string) => {
      written.push(String(file));
    },
  };
  await assert.rejects(
    preparePortableUpdateHelper(plan('w'), {
      environment: { SystemRoot: 'C:\\Windows' },
      fileSystem: fileSystem as never,
      helperScript: async () => 'script',
    }),
    /PowerShell is unavailable/,
  );
  assert.deepEqual(written, []);
});

test('preparation rejects an unwritable portable folder', async () => {
  const fileSystem = {
    access: async () => undefined,
    mkdir: async () => undefined,
    open: async () => ({ fd: 1, close: async () => undefined }),
    rm: async () => undefined,
    writeFile: async (file: string) => {
      if (String(file).includes('.nevermind-update-probe-'))
        throw new Error('EPERM');
    },
  };
  await assert.rejects(
    preparePortableUpdateHelper(plan('w'), {
      fileSystem: fileSystem as never,
      helperScript: async () => 'script',
    }),
    /D:\\Tools is not writable/,
  );
});

test('the prepared launch spawns one detached hidden helper', async () => {
  const workDirectory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-portable-update-'),
  );
  const calls: Array<{ command: string; options: Record<string, unknown> }> =
    [];
  try {
    const launch = await preparePortableUpdateHelper(plan(workDirectory), {
      environment: { SystemRoot: 'C:\\Windows', PORTABLE_EXECUTABLE_FILE: 'x' },
      fileSystem: {
        access: async () => undefined,
        mkdir: fs.mkdir,
        open: fs.open,
        rm: fs.rm,
        writeFile: async (file: string, data: string) => {
          if (!String(file).includes('.nevermind-update-probe-'))
            await fs.writeFile(file, data);
        },
      } as never,
      helperScript: async () => 'param()',
      spawnProcess: ((command: string, _args: string[], options: never) => {
        calls.push({ command, options });
        return { pid: 7, once() {}, unref() {} };
      }) as never,
    });
    assert.equal(calls.length, 0);
    launch();
    assert.equal(calls.length, 1);
    assert.match(calls[0].command, /System32\\cmd\.exe$/);
    assert.equal(calls[0].options.windowsVerbatimArguments, true);
    assert.equal(calls[0].options.detached, true);
    assert.equal(calls[0].options.windowsHide, true);
    assert.equal(
      (calls[0].options.stdio as unknown[])[0],
      'ignore',
      'the helper output is kept for diagnosis',
    );
    assert.equal(
      (calls[0].options.env as NodeJS.ProcessEnv).PORTABLE_EXECUTABLE_FILE,
      undefined,
    );
    assert.equal(
      await fs.readFile(
        path.join(workDirectory, 'windows-portable-update.ps1'),
        'utf8',
      ),
      'param()',
    );
  } finally {
    await fs.rm(workDirectory, { recursive: true, force: true });
  }
});

test('launch confirmation writes only the helper-named health file once', async () => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-health-'));
  try {
    const healthFile = path.join(
      directory,
      `nevermind-update-health-${'a'.repeat(32)}.json`,
    );
    const environment = { [PORTABLE_UPDATE_HEALTH_FILE_ENV]: healthFile };
    assert.equal(await confirmPortableUpdateLaunch('1.2.3', environment), true);
    assert.equal(
      JSON.parse(await fs.readFile(healthFile, 'utf8')).version,
      '1.2.3',
    );
    assert.equal(
      await confirmPortableUpdateLaunch('1.2.3', environment),
      false,
    );
    assert.equal(
      await confirmPortableUpdateLaunch('1.2.3', {
        [PORTABLE_UPDATE_HEALTH_FILE_ENV]: path.join(directory, 'state.json'),
      }),
      false,
    );
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});

test('helper results are consumed once, tolerating a byte-order mark', async () => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-result-'));
  try {
    await fs.writeFile(
      path.join(directory, 'result.json'),
      `\uFEFF${JSON.stringify({ outcome: 'rolled-back', reason: 'startup-failed: x', fromVersion: '1', toVersion: '2' })}`,
    );
    assert.deepEqual(await consumePortableUpdateResult(directory), {
      outcome: 'rolled-back',
      reason: 'startup-failed: x',
      fromVersion: '1',
      toVersion: '2',
    });
    assert.equal(await consumePortableUpdateResult(directory), null);
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});
