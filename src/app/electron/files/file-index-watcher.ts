import path from 'node:path';
import type {
  DirectoryTreeChangeListener,
  DirectoryTreeWatcher,
} from '../os/directory-tree-watch';
import { requiresIndividualDirectoryWatches } from '../os/directory-tree-watch';
import { warn as logWarn } from '../platform/logger';
import { ignoredByPattern } from './ignore-patterns';

export const FILE_INDEX_CHANGED_EVENT = 'files.index.changed';
const MAX_INDIVIDUAL_DIRECTORY_WATCHES = 2048;

export type FileIndexWatchScope = {
  roots: string[];
  indexedDirectories?: string[];
  depth: number;
  includeHidden: boolean;
  ignore: string[];
};

export type FileIndexWatcherDeps = {
  watchDirectoryTree: (
    root: string,
    onChange: DirectoryTreeChangeListener,
  ) => DirectoryTreeWatcher;
  onRelevantChange: () => void;
  watchDirectoriesIndividually?: boolean;
};

export function changedPathCanAffectFileIndex(
  root: string,
  relativePath: string | null,
  scope: FileIndexWatchScope,
) {
  if (!relativePath) return true;
  const segments = relativePath.split(/[\\/]+/).filter(Boolean);
  if (!segments.length) return true;
  if (segments.length > scope.depth + 1) return false;
  if (!scope.includeHidden && segments.some((name) => name.startsWith('.')))
    return false;
  const changedPath = path.join(root, ...segments);
  return !segments.some((name) =>
    ignoredByPattern(changedPath, name, scope.ignore),
  );
}

export function createFileIndexWatcher(deps: FileIndexWatcherDeps) {
  const watchers = new Map<string, DirectoryTreeWatcher>();
  let scope: FileIndexWatchScope = {
    roots: [],
    depth: 0,
    includeHidden: false,
    ignore: [],
  };

  function forgetBrokenRootWatcher(root: string) {
    watchers.get(root)?.close();
    watchers.delete(root);
    deps.onRelevantChange();
  }

  function startRootWatcher(root: string, watchedPath: string) {
    try {
      const watcher = deps.watchDirectoryTree(watchedPath, (relativePath) => {
        const prefix = path.relative(root, watchedPath);
        const changed = relativePath ? path.join(prefix, relativePath) : null;
        if (changedPathCanAffectFileIndex(root, changed, scope))
          deps.onRelevantChange();
      });
      watcher.on('error', () => forgetBrokenRootWatcher(watchedPath));
      watchers.set(watchedPath, watcher);
    } catch {
      // Missing or unreadable roots are retried on the next index sync.
    }
  }

  function sync(nextScope: FileIndexWatchScope) {
    scope = nextScope;
    const directories =
      (deps.watchDirectoriesIndividually ??
      requiresIndividualDirectoryWatches(process.platform))
        ? nextScope.indexedDirectories || []
        : [];
    const allDirectories = [...new Set([...nextScope.roots, ...directories])];
    if (allDirectories.length > MAX_INDIVIDUAL_DIRECTORY_WATCHES) {
      logWarn(
        'files.index.watch-limit',
        {
          count: allDirectories.length,
          limit: MAX_INDIVIDUAL_DIRECTORY_WATCHES,
        },
        { source: 'host', scope: 'files' },
      );
    }
    const wanted = new Set(
      allDirectories.slice(0, MAX_INDIVIDUAL_DIRECTORY_WATCHES),
    );
    for (const [root, watcher] of watchers) {
      if (wanted.has(root)) continue;
      watcher.close();
      watchers.delete(root);
    }
    for (const watchedPath of wanted) {
      const root = nextScope.roots.find(
        (candidate) =>
          watchedPath === candidate ||
          watchedPath.startsWith(`${candidate}${path.sep}`),
      );
      if (root && !watchers.has(watchedPath))
        startRootWatcher(root, watchedPath);
    }
  }

  function watchedRoots() {
    return [...watchers.keys()];
  }

  function close() {
    for (const watcher of watchers.values()) watcher.close();
    watchers.clear();
  }

  return { sync, watchedRoots, close };
}
