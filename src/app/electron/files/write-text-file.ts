import fs from 'node:fs/promises';
import path from 'node:path';
import { atomicWriteFile } from '../platform/atomic-file';
import { displayUserPath, expandUserPath } from './file-utils';

export type WriteTextFileOptions = { overwrite?: boolean };

async function requireExistingFolder(folder: string) {
  const stats = await fs.stat(folder).catch(() => null);
  if (!stats?.isDirectory())
    throw new Error(`Folder ${displayUserPath(folder)} does not exist.`);
}

async function createTextFileOnce(target: string, text: string) {
  try {
    await fs.writeFile(target, text, { encoding: 'utf8', flag: 'wx' });
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== 'EEXIST') throw error;
    throw new Error(
      `${displayUserPath(target)} already exists. Pass { overwrite: true } to replace it.`,
    );
  }
}

/** Writes UTF-8 text into an existing folder and returns the absolute path that was written. */
export async function writeTextFile(
  filePath: string,
  text: string,
  options: WriteTextFileOptions = {},
) {
  const target = expandUserPath(String(filePath || ''));
  if (!path.isAbsolute(target))
    throw new Error('writeText needs an absolute path or one starting with ~.');
  if (typeof text !== 'string') throw new Error('writeText needs text.');
  await requireExistingFolder(path.dirname(target));
  if (options.overwrite) await atomicWriteFile(target, text);
  else await createTextFileOnce(target, text);
  return target;
}
