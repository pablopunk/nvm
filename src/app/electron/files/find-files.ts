import fs from 'node:fs/promises';
import path from 'node:path';
import { fileDateAddedMs } from '../os/os';
import { warn as logWarn } from '../platform/logger';
import { fileToExtensionFile } from './extension-file';
import {
  applyDateAdded,
  findFilesNeedsStats,
  includeDimensionsForFindOptions,
  selectFindFiles,
} from './file-index-sorting';
import {
  displayUserPath,
  expandUserPath,
  extensionForPath,
  IMAGE_EXTENSIONS,
  isImagePath,
  isVideoPath,
  partitionRootsByExistence,
  VIDEO_EXTENSIONS,
} from './file-utils';
import { ignoredByPattern, normalizedIgnorePatterns } from './ignore-patterns';

export const MAX_FILE_INDEX_LIMIT = 20_000;

export function normalizeFindRoots(roots) {
  if (Array.isArray(roots)) return roots;
  if (typeof roots === 'string') return [roots];
  return [];
}

export function extensionsForFindOptions(options: any = {}) {
  const kinds = Array.isArray(options.kind)
    ? options.kind
    : options.kind
      ? [options.kind]
      : [];
  const kindExtensions = kinds.flatMap((kind) => {
    if (kind === 'image') return Array.from(IMAGE_EXTENSIONS);
    if (kind === 'video') return Array.from(VIDEO_EXTENSIONS);
    if (kind === 'media') return [...IMAGE_EXTENSIONS, ...VIDEO_EXTENSIONS];
    return [];
  });
  const patterns = Array.isArray(options.patterns)
    ? options.patterns
    : options.pattern
      ? [options.pattern]
      : [];
  const patternExtensions = patterns
    .map((pattern) => String(pattern).match(/\.([a-z0-9]+)$/i)?.[1])
    .filter(Boolean);
  const requestedExtensions = [
    ...(options.extensions || []),
    ...kindExtensions,
    ...patternExtensions,
  ];
  return requestedExtensions.length
    ? new Set(
        requestedExtensions.map((ext) =>
          String(ext).toLowerCase().replace(/^\./, ''),
        ),
      )
    : null;
}

export function fileCandidate(fullPath, name = path.basename(fullPath)) {
  return {
    path: fullPath,
    name,
    displayPath: displayUserPath(fullPath),
    extension: extensionForPath(name),
    kind: isImagePath(fullPath)
      ? 'image'
      : isVideoPath(fullPath)
        ? 'video'
        : 'file',
  };
}

async function eachChunk(items, size, fn) {
  for (let index = 0; index < items.length; index += size)
    await Promise.all(items.slice(index, index + size).map(fn));
}

export async function attachFileStats(files) {
  await eachChunk(files, 100, async (file: any) => {
    const stat = await fs.stat(file.path).catch(() => null);
    file.stat = stat;
    file.mtimeMs = stat?.mtimeMs || 0;
    file.birthtimeMs = stat?.birthtimeMs || 0;
    file.size = stat?.size || 0;
  });
  return files.filter((file) => file.stat);
}

export async function attachDateAdded(files) {
  return applyDateAdded(
    files,
    await fileDateAddedMs(files.map((file) => file.path)),
  );
}

export async function findFiles(roots, options: any = {}) {
  const limit = Math.max(
    1,
    Math.min(Number(options.limit || 100), MAX_FILE_INDEX_LIMIT),
  );
  const scanLimit = Math.max(
    limit,
    Math.min(
      Number(options.scanLimit || MAX_FILE_INDEX_LIMIT),
      MAX_FILE_INDEX_LIMIT,
    ),
  );
  const maxDepth = options.depth ?? 2;
  const extensions = extensionsForFindOptions(options);
  const ignored = normalizedIgnorePatterns(options.ignore);
  const includeHidden = Boolean(options.includeHidden);
  const candidates: any[] = [];

  async function walk(dir, depth) {
    if (candidates.length >= scanLimit) return;
    let entries = [];
    try {
      entries = await fs.readdir(dir, { withFileTypes: true });
    } catch {
      return;
    }

    for (const entry of entries) {
      if (candidates.length >= scanLimit) return;
      if (!includeHidden && entry.name.startsWith('.')) continue;
      const fullPath = path.join(dir, entry.name);
      if (ignoredByPattern(fullPath, entry.name, ignored)) continue;
      if (entry.isFile()) {
        const ext = extensionForPath(entry.name);
        if (!extensions || extensions.has(ext))
          candidates.push(fileCandidate(fullPath, entry.name));
        continue;
      }
      if (entry.isDirectory() && depth > 0) await walk(fullPath, depth - 1);
    }
  }

  const findRoots = normalizeFindRoots(roots)
    .map(expandUserPath)
    .filter((root) => root && path.isAbsolute(root));

  const { existing, missing } = await partitionRootsByExistence(findRoots);
  for (const root of missing)
    logWarn(
      'files.find.missingRoot',
      { root: displayUserPath(root) },
      { source: 'host', scope: 'files' },
    );

  await Promise.all(existing.map((root) => walk(root, maxDepth)));
  const sortBy = options.sortBy || options.sort || null;
  let found = candidates;
  if (findFilesNeedsStats(sortBy)) found = await attachFileStats(found);
  if (sortBy === 'added') found = await attachDateAdded(found);
  const selected = selectFindFiles(found, options, limit);
  return Promise.all(
    selected.map((file) =>
      fileToExtensionFile(file.path, {
        includeDimensions: includeDimensionsForFindOptions(options),
        stat: file.stat,
        dateAddedMs: file.dateAddedMs,
      }),
    ),
  );
}
