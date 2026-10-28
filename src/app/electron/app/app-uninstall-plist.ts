import { execFile } from 'node:child_process';
import fs from 'node:fs/promises';
import { promisify } from 'node:util';
import { isMissing } from './app-uninstall-inspect';
import { isWithin, macPath } from './app-uninstall-paths';

const execFileAsync = promisify(execFile);
export const PLUTIL_PATH = '/usr/bin/plutil';
export const PLUTIL_OPTIONS = {
  shell: false,
  timeout: 5000,
  maxBuffer: 4096,
} as const;

type PlistExecutor = (
  command: string,
  arguments_: string[],
  options: typeof PLUTIL_OPTIONS,
) => Promise<{ stdout: unknown }>;

interface PlistDirectoryEntry {
  name: string;
  isDirectory: () => boolean;
  isSymbolicLink: () => boolean;
}

export interface PlistFileSystem {
  realpath: (value: string) => Promise<string>;
  readdir: (value: string) => Promise<PlistDirectoryEntry[]>;
}

const productionPlistFileSystem: PlistFileSystem = {
  realpath: (value) => fs.realpath(value),
  readdir: (value) =>
    fs.readdir(value, { withFileTypes: true }) as Promise<
      PlistDirectoryEntry[]
    >,
};

async function resolveProductionPlistPath(
  appPath: string,
  fileSystem: PlistFileSystem,
) {
  const canonicalAppPath = await fileSystem.realpath(appPath);
  const directPlistPath = macPath.join(appPath, 'Contents', 'Info.plist');
  try {
    const canonicalPlistPath = await fileSystem.realpath(directPlistPath);
    if (!isWithin(canonicalAppPath, canonicalPlistPath)) {
      throw new Error('App metadata is outside the selected app bundle');
    }
    return canonicalPlistPath;
  } catch (error) {
    if (!isMissing(error)) {
      throw error;
    }
  }

  const wrapperPath = macPath.join(appPath, 'Wrapper');
  const wrappedApps = (await fileSystem.readdir(wrapperPath)).filter(
    (entry) =>
      entry.name.endsWith('.app') &&
      entry.isDirectory() &&
      !entry.isSymbolicLink(),
  );
  if (wrappedApps.length !== 1) {
    throw new Error('App has no supported bundle metadata');
  }
  const wrappedPlistPath = macPath.join(
    wrapperPath,
    wrappedApps[0].name,
    'Info.plist',
  );
  const canonicalPlistPath = await fileSystem.realpath(wrappedPlistPath);
  if (!isWithin(canonicalAppPath, canonicalPlistPath)) {
    throw new Error('App metadata is outside the selected app bundle');
  }
  return canonicalPlistPath;
}

export function createProductionPlistReader(
  execute: PlistExecutor = execFileAsync as PlistExecutor,
  fileSystem: PlistFileSystem = productionPlistFileSystem,
) {
  return async (appPath: string): Promise<unknown> => {
    const plistPath = await resolveProductionPlistPath(appPath, fileSystem);
    const result = await execute(
      PLUTIL_PATH,
      ['-extract', 'CFBundleIdentifier', 'raw', '-o', '-', plistPath],
      PLUTIL_OPTIONS,
    );
    return String(result.stdout).trim();
  };
}
