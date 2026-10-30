import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import {
  markDebugPerformance,
  measureDebugPerformance,
  measureDebugPerformanceSync,
} from '../diagnostics/debug-performance';
import { watchDirectoryTree } from '../os/directory-tree-watch';
import type { JobRegistry } from '../platform/jobs';
import { error as logError, warn as logWarn } from '../platform/logger';
import { thumbnailUrlForPreviewablePath } from './extension-file';
import { pathIsInsideRoot } from './file-index-commit';
import {
  createFileIndexRefresher,
  type FileIndexRefresherDeps,
  type FileIndexScan,
} from './file-index-refresher';
import { sortFoundFiles } from './file-index-sorting';
import { FILE_INDEX_CHANGED_EVENT } from './file-index-watcher';
import {
  displayUserPath,
  expandUserPath,
  extensionForPath,
  fileUrlForPath,
  isVideoPath,
  partitionRootsByExistence,
} from './file-utils';
import {
  attachDateAdded,
  attachFileStats,
  extensionsForFindOptions,
  fileCandidate,
  MAX_FILE_INDEX_LIMIT,
  normalizeFindRoots,
} from './find-files';
import { ignoredByPattern, normalizedIgnorePatterns } from './ignore-patterns';

const DEFAULT_FILE_INDEX_LIMIT = 5000;

export function defaultFileIndexRoots() {
  return ['Desktop', 'Documents', 'Downloads'].map((name) =>
    path.join(os.homedir(), name),
  );
}

function normalizedIndexRoots(options: any = {}) {
  const roots = normalizeFindRoots(options.roots);
  return (roots.length ? roots : defaultFileIndexRoots())
    .map(expandUserPath)
    .filter((root) => root && path.isAbsolute(root));
}

export async function scanFileIndex(options: any = {}): Promise<FileIndexScan> {
  return measureDebugPerformance(
    'files.scan',
    {
      roots: normalizedIndexRoots(options).map(displayUserPath),
      depth: options.depth ?? 2,
      limit: options.limit || DEFAULT_FILE_INDEX_LIMIT,
      alwaysLog: true,
    },
    async () => {
      const roots = normalizedIndexRoots(options);
      const ignored = normalizedIgnorePatterns(options.ignore);
      const includeHidden = Boolean(options.includeHidden);
      const maxDepth = options.depth ?? 2;
      const limit = Math.max(
        1,
        Math.min(
          Number(options.limit || DEFAULT_FILE_INDEX_LIMIT),
          MAX_FILE_INDEX_LIMIT,
        ),
      );
      const extensions = extensionsForFindOptions(options);
      const found = [];
      const unreadableRoots: string[] = [];

      async function walk(dir, depth) {
        if (found.length >= limit) return;
        let entries = [];
        try {
          entries = await fs.readdir(dir, { withFileTypes: true });
        } catch {
          if (depth === maxDepth) unreadableRoots.push(dir);
          return;
        }

        for (const entry of entries) {
          if (found.length >= limit) return;
          if (!includeHidden && entry.name.startsWith('.')) continue;
          const fullPath = path.join(dir, entry.name);
          if (ignoredByPattern(fullPath, entry.name, ignored)) continue;
          if (entry.isFile()) {
            const ext = extensionForPath(entry.name);
            if (extensions && !extensions.has(ext)) continue;
            found.push(fileCandidate(fullPath, entry.name));
            continue;
          }
          if (entry.isDirectory() && depth > 0) await walk(fullPath, depth - 1);
        }
      }

      const { existing, missing } = await partitionRootsByExistence(roots);
      for (const root of missing)
        logWarn(
          'files.scan.missingRoot',
          { root: displayUserPath(root) },
          { source: 'host', scope: 'files' },
        );

      await Promise.all(existing.map((root) => walk(root, maxDepth)));
      await attachFileStats(found);
      await attachDateAdded(found);
      const sorted = sortFoundFiles(found, {
        sortBy: options.sortBy || options.sort || 'added',
        order: options.order,
      });
      markDebugPerformance('files.scan.result', {
        foundCount: sorted.length,
        limit,
        missingRootCount: missing.length,
        unreadableRootCount: unreadableRoots.length,
      });
      const files = sorted.slice(0, limit).map((file) => ({
        ...file,
        url:
          thumbnailUrlForPreviewablePath(file.path, file.stat) ||
          fileUrlForPath(file.path),
        fileUrl: fileUrlForPath(file.path),
        videoUrl: isVideoPath(file.path) ? fileUrlForPath(file.path) : null,
        thumbnailUrl: thumbnailUrlForPreviewablePath(file.path, file.stat),
      }));
      return {
        files,
        existingRoots: existing.filter(
          (root) => !unreadableRoots.includes(root),
        ),
        missingRoots: missing,
        unreadableRoots,
      };
    },
  );
}

export interface FileIndexServiceDeps {
  notifyRootItemsChanged: () => void;
  jobEvents: Pick<JobRegistry, 'emit'>;
  watchDirectoryTree?: FileIndexRefresherDeps['watchDirectoryTree'];
}

function unreadableRootsError(unreadableRoots: string[]) {
  return new Error(
    `File index kept cached entries for unreadable roots: ${unreadableRoots
      .map(displayUserPath)
      .join(', ')}`,
  );
}

export function createFileIndexService(deps: FileIndexServiceDeps) {
  let fileIndexOptions: any = {};
  const refresher = createFileIndexRefresher({
    scan: scanFileIndex,
    notifyChanged: deps.notifyRootItemsChanged,
    watchDirectoryTree: deps.watchDirectoryTree || watchDirectoryTree,
    onRelevantChange: () => deps.jobEvents.emit(FILE_INDEX_CHANGED_EVENT),
    mark: markDebugPerformance,
  });

  function fileIndexSnapshot(options: any = {}) {
    return measureDebugPerformanceSync(
      'files.index-snapshot',
      {
        queryLength: String(options.query || '').length,
        indexedCount: refresher.current().length,
        limit: options.limit,
      },
      () => {
        const { limit, query } = options;
        const roots = normalizeFindRoots(options.roots)
          .map(expandUserPath)
          .filter(Boolean);
        const extensions = extensionsForFindOptions(options);
        const ignored = normalizedIgnorePatterns(options.ignore);
        let entries = refresher.current();
        if (roots.length)
          entries = entries.filter((entry) =>
            roots.some((root) => pathIsInsideRoot(entry.path, root)),
          );
        if (extensions)
          entries = entries.filter((entry) =>
            extensions.has(entry.extension || extensionForPath(entry.path)),
          );
        if (options.ignore)
          entries = entries.filter(
            (entry) => !ignoredByPattern(entry.path, entry.name, ignored),
          );
        if (query) {
          const needle = String(query).toLowerCase();
          entries = entries.filter((entry) =>
            `${entry.name || ''} ${entry.displayPath || ''}`
              .toLowerCase()
              .includes(needle),
          );
        }
        entries = sortFoundFiles(entries, {
          sortBy: options.sortBy || options.sort || 'added',
          order: options.order,
        });
        const max =
          typeof limit === 'number'
            ? Math.max(0, Math.min(limit, entries.length))
            : entries.length;
        return entries.slice(0, max).map((entry) => ({
          id: entry.id,
          name: entry.name,
          path: entry.path,
          displayPath: entry.displayPath,
          extension: entry.extension,
          kind: entry.kind,
          url: entry.url,
          fileUrl: entry.fileUrl,
          videoUrl: entry.videoUrl,
          thumbnailUrl: entry.thumbnailUrl,
          mtimeMs: entry.mtimeMs,
          birthtimeMs: entry.birthtimeMs,
          dateAddedMs: entry.dateAddedMs,
          size: entry.size,
        }));
      },
    );
  }

  async function reindexFiles(options: any = {}) {
    return measureDebugPerformance(
      'files.reindex',
      {
        roots: normalizedIndexRoots(options).map(displayUserPath),
        alwaysLog: true,
      },
      async () => {
        fileIndexOptions = options;
        await refresher.refresh(options);
        return {
          count: refresher.current().length,
          roots: normalizedIndexRoots(options).map(displayUserPath),
        };
      },
    );
  }

  async function indexFiles(context: { reason?: string } = {}) {
    await measureDebugPerformance(
      'files.index',
      { reason: context.reason, alwaysLog: true },
      async () => {
        try {
          const { scan } = await refresher.refresh(fileIndexOptions);
          if (scan.unreadableRoots.length)
            throw unreadableRootsError(scan.unreadableRoots);
        } catch (error) {
          logError('files.index.failed', error, {
            source: 'host',
            scope: 'files',
          });
          throw error;
        }
      },
    );
  }

  return {
    fileIndexSnapshot,
    reindexFiles,
    indexFiles,
    currentFileIndex: refresher.current,
    stopWatchingFileIndexRoots: refresher.close,
  };
}
