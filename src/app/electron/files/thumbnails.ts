import fs from 'node:fs/promises';
import crypto from 'node:crypto';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { nativeImage, net } from 'electron';
import {
  markDebugPerformance,
  measureDebugPerformance,
} from '../diagnostics/debug-performance';
import { hasCapability } from '../os/os';
import { warn as logWarn } from '../platform/logger';
import { execFileText } from './exec-file-text';

const THUMBNAIL_SIZE = 512;

export interface ThumbnailJobPort {
  schedule(jobId: string, reason?: string, delayMs?: number): unknown;
  run(jobId: string, reason?: string): Promise<unknown>;
}

export interface ThumbnailServiceDeps {
  jobRegistry: ThumbnailJobPort;
  getIconCacheDir: () => string;
}

export function createThumbnailService(deps: ThumbnailServiceDeps) {
  const { jobRegistry, getIconCacheDir } = deps;
  const pendingThumbnailPaths = new Map<string, string>();

  async function thumbnailCachePath(filePath) {
    const stat = await fs.stat(filePath);
    const key = crypto
      .createHash('sha1')
      .update(`${filePath}:${stat.mtimeMs}:${stat.size}:${THUMBNAIL_SIZE}`)
      .digest('hex');
    return path.join(getIconCacheDir(), 'thumbs', `${key}.png`);
  }

  async function generateQuickLookThumbnail(filePath, cachedPath) {
    if (!hasCapability('quick-look')) return false;
    const outputDir = await fs.mkdtemp(
      path.join(os.tmpdir(), 'nevermind-thumb-'),
    );
    try {
      await execFileText(
        'qlmanage',
        ['-t', '-s', String(THUMBNAIL_SIZE), '-o', outputDir, filePath],
        { timeout: 10_000 },
      );
      const generatedPath = path.join(
        outputDir,
        `${path.basename(filePath)}.png`,
      );
      await fs.mkdir(path.dirname(cachedPath), { recursive: true });
      await fs.copyFile(generatedPath, cachedPath);
      return true;
    } catch {
      return false;
    } finally {
      await fs.rm(outputDir, { recursive: true, force: true }).catch(() => {});
    }
  }

  async function generateThumbnail(filePath, cachedPath) {
    return measureDebugPerformance(
      'thumbnail.generate',
      { filePath, alwaysLog: true },
      async () => {
        let image = null;
        if (typeof nativeImage.createThumbnailFromPath === 'function') {
          image = await nativeImage.createThumbnailFromPath(filePath, {
            width: THUMBNAIL_SIZE,
            height: THUMBNAIL_SIZE,
          });
        }
        if (!image || image.isEmpty())
          image = nativeImage
            .createFromPath(filePath)
            .resize({ width: THUMBNAIL_SIZE, quality: 'good' });
        if (!image || image.isEmpty())
          return generateQuickLookThumbnail(filePath, cachedPath);
        const png = image.toPNG();
        await fs.mkdir(path.dirname(cachedPath), { recursive: true });
        await fs.writeFile(cachedPath, png).catch(() => {});
        return true;
      },
    );
  }

  async function processPendingThumbnails() {
    const entries = Array.from(pendingThumbnailPaths.entries()).slice(0, 4);
    for (const [filePath] of entries) pendingThumbnailPaths.delete(filePath);
    for (const [filePath, cachedPath] of entries) {
      await generateThumbnail(filePath, cachedPath).catch((error) => {
        logWarn(
          'thumbnail.generate.failed',
          { filePath, error },
          { source: 'host', scope: 'cache' },
        );
      });
    }
    if (pendingThumbnailPaths.size)
      jobRegistry.schedule('cache.thumbnails', 'thumbnail-backlog', 250);
  }

  async function thumbnailResponseForPath(filePath) {
    return measureDebugPerformance(
      'thumbnail.response',
      { filePath, alwaysLog: true },
      async () => {
        const cachedPath = await thumbnailCachePath(filePath);
        const cached = await fs.readFile(cachedPath).catch(() => null);
        if (cached) {
          markDebugPerformance('thumbnail.cache-hit', { filePath });
          return new Response(cached, {
            headers: {
              'content-type': 'image/png',
              'cache-control': 'public, max-age=31536000, immutable',
            },
          });
        }

        pendingThumbnailPaths.set(filePath, cachedPath);
        await jobRegistry
          .run('cache.thumbnails', 'thumbnail-request')
          .catch(() => {});
        const generated = await fs.readFile(cachedPath).catch(() => null);
        if (generated)
          return new Response(generated, {
            headers: {
              'content-type': 'image/png',
              'cache-control': 'public, max-age=31536000, immutable',
            },
          });
        return net.fetch(pathToFileURL(filePath).href);
      },
    );
  }

  return { processPendingThumbnails, thumbnailResponseForPath };
}
