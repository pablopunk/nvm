import path from 'node:path';
import type {
  DirectoryTreeChangeListener,
  DirectoryTreeWatcher,
} from '../os/directory-tree-watch';
import { ignoredByPattern } from './ignore-patterns';

export const FILE_INDEX_CHANGED_EVENT = 'files.index.changed';

export type FileIndexWatchScope = {
  roots: string[];
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

  function startRootWatcher(root: string) {
    try {
      const watcher = deps.watchDirectoryTree(root, (relativePath) => {
        if (changedPathCanAffectFileIndex(root, relativePath, scope))
          deps.onRelevantChange();
      });
      watcher.on('error', () => forgetBrokenRootWatcher(root));
      watchers.set(root, watcher);
    } catch {
      // Missing or unreadable roots are retried on the next index sync.
    }
  }

  function sync(nextScope: FileIndexWatchScope) {
    scope = nextScope;
    const wanted = new Set(nextScope.roots);
    for (const [root, watcher] of watchers) {
      if (wanted.has(root)) continue;
      watcher.close();
      watchers.delete(root);
    }
    for (const root of wanted) if (!watchers.has(root)) startRootWatcher(root);
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
