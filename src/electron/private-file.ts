import fs from 'node:fs/promises';

type PrivateFileSystem = Pick<typeof fs, 'writeFile' | 'chmod'>;

export type PrivateFileDependencies = {
  fileSystem?: PrivateFileSystem;
  processPlatform?: NodeJS.Platform;
};

/**
 * Writes sensitive user data with restrictive creation-time metadata.
 *
 * Node's `mode` is a POSIX/best-effort creation hint on Windows, not an ACL
 * guarantee. Windows privacy relies on the inherited ACL of Electron's
 * per-user `userData` directory and is checked separately in packaging CI.
 */
export async function writePrivateFile(
  filePath: string,
  contents: string,
  dependencies: PrivateFileDependencies = {},
) {
  const fileSystem = dependencies.fileSystem || fs;
  const processPlatform = dependencies.processPlatform || process.platform;
  await fileSystem.writeFile(filePath, contents, { mode: 0o600 });
  if (processPlatform !== 'win32') await fileSystem.chmod(filePath, 0o600);
}
