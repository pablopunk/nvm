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
  portableUpdateHelperArguments,
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

test('helper arguments carry the target, checksum, version and both processes to await', () => {
  const args = portableUpdateHelperArguments(
    plan('C:\\work'),
    'C:\\work\\h.ps1',
  );
  const value = (name: string) => args[args.indexOf(name) + 1];
  assert.equal(value('-File'), 'C:\\work\\h.ps1');
  assert.equal(value('-TargetPath'), target.executablePath);
  assert.equal(value('-ExpectedSha512'), 'c2hhNTEy+/=');
  assert.equal(value('-ExpectedVersion'), '2026.1030.2');
  assert.equal(value('-WaitProcessIds'), '41,40');
  assert.equal(value('-ExecutionPolicy'), 'Bypass');
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
    assert.match(calls[0].command, /powershell\.exe$/);
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
