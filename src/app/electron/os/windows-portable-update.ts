import { spawn } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';

export type PortableExecutable = {
  executablePath: string;
  directory: string;
};

export type PortableUpdateHelperPlan = {
  target: PortableExecutable;
  sourcePath: string;
  sha512: string;
  version: string;
  currentVersion: string;
  workDirectory: string;
  waitProcessIds: number[];
};

export type PortableUpdateResult = {
  outcome: 'installed' | 'failed' | 'rolled-back' | 'unconfirmed';
  reason: string;
  fromVersion: string;
  toVersion: string;
};

type HelperFileSystem = Pick<
  typeof fs,
  'access' | 'mkdir' | 'rm' | 'writeFile'
>;

type HelperDependencies = {
  environment?: NodeJS.ProcessEnv;
  fileSystem?: HelperFileSystem;
  helperScript: () => Promise<string>;
  spawnProcess?: typeof spawn;
};

export const PORTABLE_UPDATE_HEALTH_FILE_ENV =
  'NVM_PORTABLE_UPDATE_HEALTH_FILE';
const PORTABLE_UPDATE_HEALTH_FILE_NAME =
  /^nevermind-update-health-[0-9a-f]{32}\.json$/;
const HELPER_SCRIPT_NAME = 'windows-portable-update.ps1';
const RESULT_FILE_NAME = 'result.json';
const PORTABLE_UPDATE_OUTCOMES = new Set([
  'installed',
  'failed',
  'rolled-back',
  'unconfirmed',
]);

/** The user's portable launcher, as reported by electron-builder's portable wrapper. */
export function portableExecutableFromEnvironment(
  environment: NodeJS.ProcessEnv = process.env,
): PortableExecutable | null {
  const executable = environment.PORTABLE_EXECUTABLE_FILE;
  if (!(executable && path.win32.isAbsolute(executable))) return null;
  const executablePath = path.win32.normalize(executable);
  return { executablePath, directory: path.win32.dirname(executablePath) };
}

export async function assertPortableDirectoryWritable(
  directory: string,
  fileSystem: Pick<typeof fs, 'writeFile' | 'rm'> = fs,
) {
  const probe = path.win32.join(
    directory,
    `.nevermind-update-probe-${process.pid}`,
  );
  try {
    await fileSystem.writeFile(probe, '');
    await fileSystem.rm(probe, { force: true });
  } catch {
    throw new Error(
      `Nevermind cannot update itself because ${directory} is not writable. Move Nevermind to a folder you can write to, then try again.`,
    );
  }
}

export function portableUpdateResultPath(workDirectory: string) {
  return path.join(workDirectory, RESULT_FILE_NAME);
}

function windowsPowerShellPath(environment: NodeJS.ProcessEnv) {
  return path.win32.join(
    environment.SystemRoot || 'C:\\Windows',
    'System32',
    'WindowsPowerShell',
    'v1.0',
    'powershell.exe',
  );
}

export function portableUpdateHelperArguments(
  plan: PortableUpdateHelperPlan,
  scriptPath: string,
) {
  return [
    '-NoProfile',
    '-NonInteractive',
    '-ExecutionPolicy',
    'Bypass',
    '-WindowStyle',
    'Hidden',
    '-File',
    scriptPath,
    '-TargetPath',
    plan.target.executablePath,
    '-SourcePath',
    plan.sourcePath,
    '-ExpectedSha512',
    plan.sha512,
    '-ExpectedVersion',
    plan.version,
    '-CurrentVersion',
    plan.currentVersion,
    '-WorkDirectory',
    plan.workDirectory,
    '-ResultPath',
    portableUpdateResultPath(plan.workDirectory),
    '-WaitProcessIds',
    plan.waitProcessIds.join(','),
  ];
}

/** The helper relaunches Nevermind, so it must not inherit the old wrapper's identity or one-shot update variables. */
export function portableUpdateHelperEnvironment(
  environment: NodeJS.ProcessEnv,
): NodeJS.ProcessEnv {
  return Object.fromEntries(
    Object.entries(environment).filter(
      ([name]) => !/^(PORTABLE_EXECUTABLE_|NVM_PORTABLE_UPDATE_)/i.test(name),
    ),
  );
}

async function requireFile(
  fileSystem: HelperFileSystem,
  filePath: string,
  message: string,
) {
  try {
    await fileSystem.access(filePath);
  } catch {
    throw new Error(message);
  }
}

/** Does every fallible file step up front so the final launch can run synchronously while Nevermind quits. */
export async function preparePortableUpdateHelper(
  plan: PortableUpdateHelperPlan,
  dependencies: HelperDependencies,
) {
  const environment = dependencies.environment ?? process.env;
  const fileSystem = dependencies.fileSystem ?? fs;
  const spawnProcess = dependencies.spawnProcess ?? spawn;
  const powershell = windowsPowerShellPath(environment);
  await requireFile(
    fileSystem,
    powershell,
    'Windows PowerShell is unavailable, so Nevermind cannot install this update.',
  );
  await requireFile(
    fileSystem,
    plan.sourcePath,
    'The downloaded update is missing. Download it again.',
  );
  await assertPortableDirectoryWritable(plan.target.directory, fileSystem);
  await fileSystem.mkdir(plan.workDirectory, { recursive: true });
  await fileSystem.rm(portableUpdateResultPath(plan.workDirectory), {
    force: true,
  });
  const scriptPath = path.join(plan.workDirectory, HELPER_SCRIPT_NAME);
  await fileSystem.writeFile(scriptPath, await dependencies.helperScript());
  return function launchPortableUpdateHelper() {
    const helper = spawnProcess(
      powershell,
      portableUpdateHelperArguments(plan, scriptPath),
      {
        detached: true,
        env: portableUpdateHelperEnvironment(environment),
        stdio: 'ignore',
        windowsHide: true,
      },
    );
    helper.unref();
  };
}

/** Tells the update helper that the relaunched app started; without this signal the helper restores the previous executable. */
export async function confirmPortableUpdateLaunch(
  version: string,
  environment: NodeJS.ProcessEnv = process.env,
  fileSystem: Pick<typeof fs, 'writeFile' | 'rename'> = fs,
) {
  const healthFile = environment[PORTABLE_UPDATE_HEALTH_FILE_ENV];
  delete environment[PORTABLE_UPDATE_HEALTH_FILE_ENV];
  if (
    !(
      healthFile &&
      path.isAbsolute(healthFile) &&
      PORTABLE_UPDATE_HEALTH_FILE_NAME.test(path.basename(healthFile))
    )
  )
    return false;
  const temporary = `${healthFile}.${process.pid}.tmp`;
  await fileSystem.writeFile(
    temporary,
    JSON.stringify({ version, pid: process.pid }),
  );
  await fileSystem.rename(temporary, healthFile);
  return true;
}

export async function consumePortableUpdateResult(
  workDirectory: string,
  fileSystem: Pick<typeof fs, 'readFile' | 'rm'> = fs,
): Promise<PortableUpdateResult | null> {
  const resultPath = portableUpdateResultPath(workDirectory);
  let text: string;
  try {
    text = await fileSystem.readFile(resultPath, 'utf8');
  } catch {
    return null;
  }
  await fileSystem.rm(resultPath, { force: true });
  try {
    const result = JSON.parse(text.replace(/^\uFEFF/, ''));
    if (!PORTABLE_UPDATE_OUTCOMES.has(result?.outcome)) return null;
    return {
      outcome: result.outcome,
      reason: String(result.reason ?? ''),
      fromVersion: String(result.fromVersion ?? ''),
      toVersion: String(result.toVersion ?? ''),
    };
  } catch {
    return null;
  }
}
