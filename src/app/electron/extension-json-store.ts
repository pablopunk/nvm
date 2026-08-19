import fs, { type FileHandle } from 'node:fs/promises';
import path from 'node:path';
import { atomicallyReplaceFile } from './atomic-file';

type JsonObject = Record<string, unknown>;

interface ExtensionJsonStoreDeps {
  mkdir?: typeof fs.mkdir;
  open?: typeof fs.open;
  readFile?: typeof fs.readFile;
  realpath?: typeof fs.realpath;
  rename?: typeof fs.rename;
  unlink?: typeof fs.unlink;
  writeTemporaryFile?: (file: FileHandle, data: string) => Promise<void>;
}

function ignoreError() {
  return null;
}

function isMissingFileError(error: unknown) {
  return (error as NodeJS.ErrnoException | undefined)?.code === 'ENOENT';
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: The factory keeps each store's injected filesystem operations and queue state isolated.
function createExtensionJsonStore(deps: ExtensionJsonStoreDeps = {}) {
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
  const canonicalPathResolutions = new Map<string, Promise<string>>();
  const pendingOperations = new Map<string, Promise<void>>();
  let operationRegistrationTail: Promise<void> = Promise.resolve();

  async function canonicalPath(filePath: string) {
    const absolutePath = path.resolve(filePath);
    try {
      return await realpath(absolutePath);
    } catch (error) {
      if (!isMissingFileError(error)) {
        throw error;
      }
    }

    const directory = path.dirname(absolutePath);
    try {
      return path.join(await realpath(directory), path.basename(absolutePath));
    } catch (error) {
      if (!isMissingFileError(error)) {
        throw error;
      }
      return absolutePath;
    }
  }

  async function readFileData(filePath: string): Promise<JsonObject> {
    try {
      return JSON.parse(await readFile(filePath, 'utf8'));
    } catch (error) {
      if (isMissingFileError(error)) {
        return {};
      }
      throw error;
    }
  }

  function resolveCanonicalPath(filePath: string) {
    const absolutePath = path.resolve(filePath);
    const pendingResolution = canonicalPathResolutions.get(absolutePath);
    if (pendingResolution) {
      return pendingResolution;
    }

    const resolution = canonicalPath(absolutePath);
    canonicalPathResolutions.set(absolutePath, resolution);
    resolution.then(
      () => {
        if (canonicalPathResolutions.get(absolutePath) === resolution) {
          canonicalPathResolutions.delete(absolutePath);
        }
      },
      () => {
        if (canonicalPathResolutions.get(absolutePath) === resolution) {
          canonicalPathResolutions.delete(absolutePath);
        }
      },
    );
    return resolution;
  }

  async function enqueue<T>(
    filePath: string,
    operation: (canonicalFilePath: string) => Promise<T>,
  ): Promise<T> {
    // Resolve eagerly, but register in call order so aliases cannot enter the
    // same canonical queue in resolution order.
    const canonicalPathResolution = resolveCanonicalPath(filePath);
    const registration = operationRegistrationTail.then(async () => {
      const canonicalFilePath = await canonicalPathResolution;
      const previous = pendingOperations.get(canonicalFilePath);
      const operationPromise = (previous ?? Promise.resolve())
        .catch(ignoreError)
        .then(() => operation(canonicalFilePath));
      const settledOperation = operationPromise.then(
        () => undefined,
        () => undefined,
      );
      pendingOperations.set(canonicalFilePath, settledOperation);
      settledOperation.then(() => {
        if (pendingOperations.get(canonicalFilePath) === settledOperation) {
          pendingOperations.delete(canonicalFilePath);
        }
      });
      return { operationPromise };
    });
    operationRegistrationTail = registration.then(
      () => undefined,
      () => undefined,
    );
    const { operationPromise } = await registration;
    return operationPromise;
  }

  return {
    read(filePath: string) {
      return enqueue(filePath, readFileData);
    },
    replace(filePath: string, data: JsonObject) {
      return enqueue(filePath, async (canonicalFilePath) => {
        await atomicallyReplaceFile(
          canonicalFilePath,
          JSON.stringify(data, null, 2),
          { mkdir, open, rename, unlink, writeTemporaryFile },
        );
      });
    },
    mutate(
      filePath: string,
      update: (current: JsonObject) => JsonObject | Promise<JsonObject>,
    ) {
      return enqueue(filePath, async (canonicalFilePath) => {
        const current = await readFileData(canonicalFilePath);
        const next = await update(current);
        await atomicallyReplaceFile(
          canonicalFilePath,
          JSON.stringify(next, null, 2),
          { mkdir, open, rename, unlink, writeTemporaryFile },
        );
        return next;
      });
    },
    pendingOperationCount() {
      return pendingOperations.size;
    },
  };
}

export type { JsonObject };
export { createExtensionJsonStore };
