import crypto from 'node:crypto';
import fs, { type FileHandle } from 'node:fs/promises';
import path from 'node:path';

export type JsonObject = Record<string, any>;

type ExtensionJsonStoreDeps = {
  mkdir?: typeof fs.mkdir;
  open?: typeof fs.open;
  readFile?: typeof fs.readFile;
  realpath?: typeof fs.realpath;
  rename?: typeof fs.rename;
  unlink?: typeof fs.unlink;
  writeTemporaryFile?: (file: FileHandle, data: string) => Promise<void>;
};

function isMissingFileError(error: unknown) {
  return (error as NodeJS.ErrnoException | undefined)?.code === 'ENOENT';
}

export function createExtensionJsonStore(deps: ExtensionJsonStoreDeps = {}) {
  const mkdir = deps.mkdir ?? fs.mkdir;
  const open = deps.open ?? fs.open;
  const readFile = deps.readFile ?? fs.readFile;
  const realpath = deps.realpath ?? fs.realpath;
  const rename = deps.rename ?? fs.rename;
  const unlink = deps.unlink ?? fs.unlink;
  const writeTemporaryFile =
    deps.writeTemporaryFile ??
    async function writeTemporaryFile(file, data) {
      await file.writeFile(data);
    };
  const pendingOperations = new Map<string, Promise<void>>();

  async function canonicalPath(filePath: string) {
    const absolutePath = path.resolve(filePath);
    try {
      return await realpath(absolutePath);
    } catch (error) {
      if (!isMissingFileError(error)) throw error;
    }

    const directory = path.dirname(absolutePath);
    try {
      return path.join(await realpath(directory), path.basename(absolutePath));
    } catch (error) {
      if (!isMissingFileError(error)) throw error;
      return absolutePath;
    }
  }

  async function readFileData(filePath: string): Promise<JsonObject> {
    try {
      return JSON.parse(await readFile(filePath, 'utf8'));
    } catch (error) {
      if (isMissingFileError(error)) return {};
      throw error;
    }
  }

  async function atomicallyReplaceFile(filePath: string, data: JsonObject) {
    await mkdir(path.dirname(filePath), { recursive: true });
    const temporaryPath = path.join(
      path.dirname(filePath),
      `.${path.basename(filePath)}.${crypto.randomUUID()}.tmp`,
    );
    let file: FileHandle | undefined;

    try {
      file = await open(temporaryPath, 'w', 0o600);
      await writeTemporaryFile(file, JSON.stringify(data, null, 2));
      await file.sync();
      await file.close();
      file = undefined;
      await rename(temporaryPath, filePath);
    } finally {
      await file?.close().catch(() => {});
      await unlink(temporaryPath).catch(() => {});
    }
  }

  async function enqueue<T>(
    filePath: string,
    operation: (canonicalFilePath: string) => Promise<T>,
  ): Promise<T> {
    const canonicalFilePath = await canonicalPath(filePath);
    const previous = pendingOperations.get(canonicalFilePath);
    const operationPromise = (previous ?? Promise.resolve())
      .catch(() => {})
      .then(() => operation(canonicalFilePath));
    const settledOperation = operationPromise.then(
      () => undefined,
      () => undefined,
    );
    pendingOperations.set(canonicalFilePath, settledOperation);
    void settledOperation.then(() => {
      if (pendingOperations.get(canonicalFilePath) === settledOperation)
        pendingOperations.delete(canonicalFilePath);
    });
    return operationPromise;
  }

  return {
    read(filePath: string) {
      return enqueue(filePath, readFileData);
    },
    replace(filePath: string, data: JsonObject) {
      return enqueue(filePath, async (canonicalFilePath) => {
        await atomicallyReplaceFile(canonicalFilePath, data);
      });
    },
    mutate(
      filePath: string,
      update: (current: JsonObject) => JsonObject | Promise<JsonObject>,
    ) {
      return enqueue(filePath, async (canonicalFilePath) => {
        const current = await readFileData(canonicalFilePath);
        const next = await update(current);
        await atomicallyReplaceFile(canonicalFilePath, next);
        return next;
      });
    },
    pendingOperationCount() {
      return pendingOperations.size;
    },
  };
}
