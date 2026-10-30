import path from 'node:path';

type IndexedFile = {
  path: string;
  mtimeMs?: number;
  size?: number;
  dateAddedMs?: number;
};

export function pathIsInsideRoot(filePath: string, root: string) {
  return filePath === root || filePath.startsWith(`${root}${path.sep}`);
}

export function cachedEntriesUnderRoots<T extends IndexedFile>(
  cachedIndex: T[],
  roots: string[],
) {
  if (!roots.length) return [];
  return cachedIndex.filter((entry) =>
    roots.some((root) => pathIsInsideRoot(entry.path, root)),
  );
}

export function fileIndexSignature(files: IndexedFile[]) {
  return files
    .map(
      (file) =>
        `${file.path}\0${file.mtimeMs ?? ''}\0${file.size ?? ''}\0${file.dateAddedMs ?? ''}`,
    )
    .join('\n');
}

export function createLatestScanGate() {
  let latestStarted = 0;
  let latestCommitted = 0;

  function begin() {
    latestStarted += 1;
    return latestStarted;
  }

  function tryCommit(scan: number) {
    if (scan < latestCommitted) return false;
    latestCommitted = scan;
    return true;
  }

  return { begin, tryCommit };
}
