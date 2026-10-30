import path from 'node:path';
import type {
  DirectoryTreeChangeListener,
  DirectoryTreeWatcher,
} from '../os/directory-tree-watch';
import { watchesIndexedDirectoriesIndividually } from '../os/directory-tree-watch';
import { ignoredByPattern } from './ignore-patterns';

export const FILE_INDEX_CHANGED_EVENT = 'files.index.changed';
const MAX_INDIVIDUAL_DIRECTORY_WATCHES = 2048;

export type FileIndexWatchScope = {
  roots: string[];
  missingRoots?: string[];
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
  watchDirectoryShallow?: (
    directory: string,
    onChange: DirectoryTreeChangeListener,
  ) => DirectoryTreeWatcher;
  onRelevantChange: () => void;
  onWatchLimit?: (count: number, limit: number) => void;
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

export function changedPathCanCreateMissingRoot(
  watchedDirectory: string,
  missingRoot: string,
  relativePath: string | null,
) {
  if (!relativePath) return true;
  const [nextSegment] = path
    .relative(watchedDirectory, missingRoot)
    .split(path.sep);
  return relativePath.split(/[\\/]+/)[0] === nextSegment;
}

export function createFileIndexWatcher(deps: FileIndexWatcherDeps) {
  const watchers = new Map<string, DirectoryTreeWatcher>();
  const missingRootWatchers = new Map<string, DirectoryTreeWatcher>();
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

  function watchClosestExistingParent(missingRoot: string) {
    const watchShallow = deps.watchDirectoryShallow;
    if (!watchShallow) return;
    let directory = path.dirname(missingRoot);
    for (;;) {
      const watchedDirectory = directory;
      try {
        const watcher = watchShallow(watchedDirectory, (relativePath) => {
          if (
            changedPathCanCreateMissingRoot(
              watchedDirectory,
              missingRoot,
              relativePath,
            )
          )
            deps.onRelevantChange();
        });
        watcher.on('error', () => forgetMissingRootWatcher(missingRoot));
        return watcher;
      } catch {
        const parent = path.dirname(directory);
        if (parent === directory) return;
        directory = parent;
      }
    }
  }

  function forgetMissingRootWatcher(missingRoot: string) {
    missingRootWatchers.get(missingRoot)?.close();
    missingRootWatchers.delete(missingRoot);
    deps.onRelevantChange();
  }

  function syncMissingRootWatchers(missingRoots: string[]) {
    const wanted = new Set(
      missingRoots.slice(0, MAX_INDIVIDUAL_DIRECTORY_WATCHES),
    );
    for (const [missingRoot, watcher] of missingRootWatchers) {
      if (wanted.has(missingRoot)) continue;
      watcher.close();
      missingRootWatchers.delete(missingRoot);
    }
    for (const missingRoot of wanted) {
      if (missingRootWatchers.has(missingRoot)) continue;
      const watcher = watchClosestExistingParent(missingRoot);
      if (watcher) missingRootWatchers.set(missingRoot, watcher);
    }
  }

  function sync(nextScope: FileIndexWatchScope) {
    scope = nextScope;
    syncMissingRootWatchers(nextScope.missingRoots || []);
    const directories =
      (deps.watchDirectoriesIndividually ??
      watchesIndexedDirectoriesIndividually)
        ? nextScope.indexedDirectories || []
        : [];
    const allDirectories = [...new Set([...nextScope.roots, ...directories])];
    if (allDirectories.length > MAX_INDIVIDUAL_DIRECTORY_WATCHES) {
      deps.onWatchLimit?.(
        allDirectories.length,
        MAX_INDIVIDUAL_DIRECTORY_WATCHES,
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

  function watchedMissingRoots() {
    return [...missingRootWatchers.keys()];
  }

  function close() {
    for (const watcher of watchers.values()) watcher.close();
    watchers.clear();
    for (const watcher of missingRootWatchers.values()) watcher.close();
    missingRootWatchers.clear();
  }

  return { sync, watchedRoots, watchedMissingRoots, close };
}
