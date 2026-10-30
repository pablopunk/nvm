import { watch } from 'node:fs';

export type DirectoryTreeWatcher = {
  close: () => unknown;
  on: (event: string, listener: () => void) => unknown;
};

export type DirectoryTreeWatchDependency = (
  filename: string,
  options: { recursive: boolean },
  listener: (eventType?: string, filename?: string | Buffer | null) => void,
) => DirectoryTreeWatcher;

export type DirectoryTreeChangeListener = (relativePath: string | null) => void;

export function nativelyWatchesNestedDirectories(platform: NodeJS.Platform) {
  return platform === 'darwin' || platform === 'win32';
}

export function requiresIndividualDirectoryWatches(platform: NodeJS.Platform) {
  return platform === 'linux';
}

export const watchesIndexedDirectoriesIndividually =
  requiresIndividualDirectoryWatches(process.platform);

export function createDirectoryTreeWatch(
  platform: NodeJS.Platform,
  watchDependency: DirectoryTreeWatchDependency = watch as never,
) {
  return function watchDirectoryTree(
    root: string,
    onChange: DirectoryTreeChangeListener,
  ) {
    return watchDependency(
      root,
      { recursive: nativelyWatchesNestedDirectories(platform) },
      (_eventType, filename) => onChange(filename ? String(filename) : null),
    );
  };
}

export const watchDirectoryTree = createDirectoryTreeWatch(process.platform);

export function createShallowDirectoryWatch(
  watchDependency: DirectoryTreeWatchDependency = watch as never,
) {
  return function watchDirectoryShallow(
    directory: string,
    onChange: DirectoryTreeChangeListener,
  ) {
    return watchDependency(
      directory,
      { recursive: false },
      (_eventType, filename) => onChange(filename ? String(filename) : null),
    );
  };
}

export const watchDirectoryShallow = createShallowDirectoryWatch();
