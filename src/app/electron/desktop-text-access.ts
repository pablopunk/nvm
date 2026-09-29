import { execFile } from 'node:child_process';
import { constants } from 'node:fs';
import { access } from 'node:fs/promises';
import path from 'node:path';
import { promisify } from 'node:util';

export type DesktopTextTarget = {
  bundleId: string;
  pid: number;
  windowId?: string;
};

export type DesktopTextAccess = {
  state: 'allowed' | 'denied' | 'unavailable' | 'unsupported' | 'unknown';
  message: string;
};

export class DesktopTextAccessError extends Error {
  constructor(
    readonly state: DesktopTextAccess['state'],
    message: string,
  ) {
    super(message);
    this.name = 'DesktopTextAccessError';
  }
}

const execFileAsync = promisify(execFile);

export async function runDesktopTextCommand(command: string, args: string[]) {
  const { stdout } = await execFileAsync(command, args, {
    timeout: 5000,
    maxBuffer: 1024 * 1024,
    windowsHide: true,
    encoding: 'utf8',
  });
  return stdout;
}

export async function findDesktopTextCommand(
  command: string,
  searchPath: string,
) {
  for (const directory of searchPath.split(path.delimiter).filter(Boolean)) {
    const executable = path.join(directory, command);
    try {
      await access(executable, constants.X_OK);
      return executable;
    } catch {}
  }
  return null;
}

export function desktopTextFailure(error: unknown): DesktopTextAccess {
  return error instanceof DesktopTextAccessError
    ? { state: error.state, message: error.message }
    : {
        state: 'unknown',
        message: 'Could not check selected-text access. Try again.',
      };
}

export function assertDesktopTextTarget(
  actual: DesktopTextTarget | null,
  expected: DesktopTextTarget,
) {
  if (!actual || actual.bundleId !== expected.bundleId) {
    throw new DesktopTextAccessError(
      'unavailable',
      'The source window lost focus. Select the text and try again.',
    );
  }
}
