import crypto from 'node:crypto';
import fs, { type FileHandle } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import {
  type PortableConfig,
  type PortableSettingKinds,
  parsePortableConfigText,
} from './portable-config-schema';

const PRIVATE_FILE_MODE = 0o600;
const PORTABLE_CONFIG_FILENAME = 'config.json';

export type PortableDiskReading =
  | { kind: 'missing' }
  | { kind: 'inaccessible'; reason: string }
  | { kind: 'invalid'; reason: string }
  | {
      kind: 'valid';
      text: string;
      raw: Record<string, unknown>;
      config: PortableConfig;
    };

function errorCode(error: unknown) {
  return (error as NodeJS.ErrnoException | undefined)?.code ?? 'UNKNOWN';
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function ignoreError() {
  return undefined;
}

export function resolvePortableConfigPath(options: {
  isTestMode: boolean;
  userDataDirectory: string;
  homeDirectory?: string;
}) {
  if (options.isTestMode)
    return path.join(options.userDataDirectory, PORTABLE_CONFIG_FILENAME);
  return path.join(
    options.homeDirectory ?? os.homedir(),
    '.config',
    'nevermind',
    PORTABLE_CONFIG_FILENAME,
  );
}

async function isDanglingSymbolicLink(filePath: string) {
  const stat = await fs.lstat(filePath).catch(() => null);
  return Boolean(stat?.isSymbolicLink());
}

async function readMissingOrDangling(
  filePath: string,
): Promise<PortableDiskReading> {
  if (!(await isDanglingSymbolicLink(filePath))) return { kind: 'missing' };
  return {
    kind: 'inaccessible',
    reason: 'Config symlink points to a missing file',
  };
}

export async function readPortableConfigFile(
  filePath: string,
  kinds: PortableSettingKinds,
): Promise<PortableDiskReading> {
  let text: string;
  try {
    text = await fs.readFile(filePath, 'utf8');
  } catch (error) {
    if (errorCode(error) === 'ENOENT') return readMissingOrDangling(filePath);
    return {
      kind: 'inaccessible',
      reason: `${errorCode(error)}: ${errorMessage(error)}`,
    };
  }
  const parsed = parsePortableConfigText(text, kinds);
  if (!parsed.ok) return { kind: 'invalid', reason: parsed.reason };
  return { kind: 'valid', text, raw: parsed.raw, config: parsed.config };
}

export async function portableConfigSignature(filePath: string) {
  try {
    const stat = await fs.stat(filePath);
    return `${stat.dev}:${stat.ino}:${stat.size}:${stat.mtimeMs}:${stat.ctimeMs}`;
  } catch (error) {
    if (errorCode(error) !== 'ENOENT') return `error:${errorCode(error)}`;
    const link = await fs.lstat(filePath).catch(() => null);
    return link ? `dangling:${link.ino}:${link.mtimeMs}` : 'missing';
  }
}

async function resolveWriteTarget(filePath: string) {
  const stat = await fs.lstat(filePath).catch(() => null);
  return stat?.isSymbolicLink() ? fs.realpath(filePath) : filePath;
}

async function existingFileMode(filePath: string) {
  const stat = await fs.stat(filePath).catch(() => null);
  return stat ? stat.mode & 0o777 : null;
}

export async function writePortableConfigText(filePath: string, text: string) {
  const target = await resolveWriteTarget(filePath);
  await fs.mkdir(path.dirname(target), { recursive: true });
  const mode = (await existingFileMode(target)) ?? PRIVATE_FILE_MODE;
  const temporaryPath = path.join(
    path.dirname(target),
    `.${path.basename(target)}.${crypto.randomUUID()}.tmp`,
  );
  let file: FileHandle | undefined;
  try {
    file = await fs.open(temporaryPath, 'wx', mode);
    await file.writeFile(text);
    await file.sync();
    await file.close();
    file = undefined;
    await fs.chmod(temporaryPath, mode);
    await fs.rename(temporaryPath, target);
  } finally {
    await file?.close().catch(ignoreError);
    await fs.unlink(temporaryPath).catch(ignoreError);
  }
}
