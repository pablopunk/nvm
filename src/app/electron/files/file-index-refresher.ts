import {
  cachedEntriesUnderRoots,
  createLatestScanGate,
  fileIndexSignature,
} from './file-index-commit';
import { sortFoundFiles } from './file-index-sorting';
import {
  createFileIndexWatcher,
  type FileIndexWatcherDeps,
  type FileIndexWatchScope,
} from './file-index-watcher';
import { normalizedIgnorePatterns } from './ignore-patterns';

export type FileIndexScan = {
  files: any[];
  existingRoots: string[];
  missingRoots: string[];
  unreadableRoots: string[];
  indexedDirectories?: string[];
};

export type FileIndexRefresherDeps = FileIndexWatcherDeps & {
  scan: (options: any, previousIndex: any[]) => Promise<FileIndexScan>;
  notifyChanged: () => void;
  mark?: (operation: string, attributes: Record<string, unknown>) => void;
};

function fileIndexWatchScope(
  options: any,
  existingRoots: string[],
  indexedDirectories: string[] = [],
): FileIndexWatchScope {
  return {
    roots: existingRoots,
    indexedDirectories,
    depth: options.depth ?? 2,
    includeHidden: Boolean(options.includeHidden),
    ignore: normalizedIgnorePatterns(options.ignore),
  };
}

export function createFileIndexRefresher(deps: FileIndexRefresherDeps) {
  let fileIndex: any[] = [];
  const scanGate = createLatestScanGate();
  const watcher = createFileIndexWatcher(deps);

  function indexKeepingCachedUnreadableRoots(scan: FileIndexScan, options) {
    const kept = cachedEntriesUnderRoots(fileIndex, scan.unreadableRoots);
    if (!kept.length) return scan.files;
    const scannedPaths = new Set(scan.files.map((file) => file.path));
    return sortFoundFiles(
      [...scan.files, ...kept.filter((entry) => !scannedPaths.has(entry.path))],
      {
        sortBy: options.sortBy || options.sort || 'added',
        order: options.order,
      },
    );
  }

  async function refresh(options: any = {}) {
    const scanId = scanGate.begin();
    const scan = await deps.scan(options, fileIndex);
    if (!scanGate.tryCommit(scanId)) {
      deps.mark?.('files.index.stale-scan-dropped', { scanId });
      return { scan, committed: false, changed: false };
    }
    const nextIndex = indexKeepingCachedUnreadableRoots(scan, options);
    const changed =
      fileIndexSignature(nextIndex) !== fileIndexSignature(fileIndex);
    fileIndex = nextIndex;
    watcher.sync(
      fileIndexWatchScope(options, scan.existingRoots, scan.indexedDirectories),
    );
    if (changed) deps.notifyChanged();
    deps.mark?.('files.index.result', {
      indexedCount: fileIndex.length,
      changed,
      missingRootCount: scan.missingRoots.length,
      unreadableRootCount: scan.unreadableRoots.length,
      watchedRootCount: watcher.watchedRoots().length,
    });
    return { scan, committed: true, changed };
  }

  function current() {
    return fileIndex;
  }

  return {
    refresh,
    current,
    watchedRoots: watcher.watchedRoots,
    close: watcher.close,
  };
}
