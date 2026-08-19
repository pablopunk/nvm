import crypto from 'node:crypto';
import fs, { type FileHandle } from 'node:fs/promises';
import path from 'node:path';

interface AtomicFileDeps {
  mkdir?: typeof fs.mkdir;
  open?: typeof fs.open;
  rename?: typeof fs.rename;
  unlink?: typeof fs.unlink;
  randomId?: () => string;
  writeTemporaryFile?: (file: FileHandle, data: string) => Promise<void>;
}

const PRIVATE_FILE_MODE = 0o600;

function ignoreError() {
  return null;
}

async function atomicallyReplaceFile(
  filePath: string,
  data: string,
  deps: AtomicFileDeps = {},
) {
  const mkdir = deps.mkdir ?? fs.mkdir;
  const open = deps.open ?? fs.open;
  const rename = deps.rename ?? fs.rename;
  const unlink = deps.unlink ?? fs.unlink;
  const randomId = deps.randomId ?? crypto.randomUUID;
  const writeTemporaryFile =
    deps.writeTemporaryFile ??
    async function writeTemporaryFile(file, contents) {
      await file.writeFile(contents);
    };

  await mkdir(path.dirname(filePath), { recursive: true });
  const temporaryPath = path.join(
    path.dirname(filePath),
    `.${path.basename(filePath)}.${randomId()}.tmp`,
  );
  let file: FileHandle | undefined;

  try {
    file = await open(temporaryPath, 'w', PRIVATE_FILE_MODE);
    await writeTemporaryFile(file, data);
    await file.sync();
    await file.close();
    file = undefined;
    await rename(temporaryPath, filePath);
  } finally {
    await file?.close().catch(ignoreError);
    await unlink(temporaryPath).catch(ignoreError);
  }
}

export type { AtomicFileDeps };
export { atomicallyReplaceFile };
