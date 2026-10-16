import fs from 'node:fs/promises';
import path from 'node:path';
import { nativeImage } from 'electron';
import {
  displayUserPath,
  expandUserPath,
  extensionForPath,
  fileUrlForPath,
  isImagePath,
  isVideoPath,
  thumbnailUrlForPath,
} from './file-utils';
import { mimeTypeForPath } from './mime-type';

async function imageDimensionsForPath(filePath) {
  if (!isImagePath(filePath)) return {};
  const image = nativeImage.createFromPath(filePath);
  if (!image || image.isEmpty()) return {};
  const size = image.getSize();
  return size.width && size.height
    ? { width: size.width, height: size.height }
    : {};
}

function thumbnailRevisionForStat(stat) {
  return stat ? `${stat.mtimeMs}:${stat.size}` : undefined;
}

export function thumbnailUrlForPreviewablePath(filePath, knownStat?) {
  const expandedPath = expandUserPath(filePath);
  if (!(isImagePath(expandedPath) || isVideoPath(expandedPath))) return null;
  return thumbnailUrlForPath(expandedPath, thumbnailRevisionForStat(knownStat));
}

export async function fileToExtensionFile(filePath, options: any = {}) {
  const expandedPath = expandUserPath(filePath);
  const stat =
    options.stat === undefined
      ? await fs.stat(expandedPath).catch(() => null)
      : options.stat;
  const dateAddedMs = Number(options.dateAddedMs || 0);
  const dimensions = options.includeDimensions
    ? await imageDimensionsForPath(expandedPath)
    : {};
  return {
    path: expandedPath,
    name: path.basename(expandedPath),
    displayPath: displayUserPath(expandedPath),
    url:
      thumbnailUrlForPreviewablePath(expandedPath, stat) ||
      fileUrlForPath(expandedPath),
    fileUrl: fileUrlForPath(expandedPath),
    videoUrl: isVideoPath(expandedPath) ? fileUrlForPath(expandedPath) : null,
    thumbnailUrl: thumbnailUrlForPreviewablePath(expandedPath, stat),
    kind: isImagePath(expandedPath)
      ? 'image'
      : isVideoPath(expandedPath)
        ? 'video'
        : 'file',
    extension: extensionForPath(expandedPath),
    mimeType: mimeTypeForPath(expandedPath),
    ...dimensions,
    mtime: stat ? new Date(stat.mtimeMs).toISOString() : null,
    mtimeMs: stat?.mtimeMs || 0,
    birthtime: stat ? new Date(stat.birthtimeMs).toISOString() : null,
    birthtimeMs: stat?.birthtimeMs || 0,
    dateAdded: dateAddedMs ? new Date(dateAddedMs).toISOString() : null,
    dateAddedMs,
    size: stat?.size || 0,
  };
}
