import path from 'node:path';
import type { Clipboard } from 'electron';
import {
  expandUserPath,
  fileUrlForPath,
  isVideoPath,
  thumbnailUrlForPath,
} from './file-utils';
import { hashValue } from './search-utils';

export type ClipboardHistoryItem =
  | { id: string; type: 'text'; text: string; createdAt: number }
  | {
      id: string;
      type: 'image';
      imagePath?: string;
      imageDataUrl?: string;
      thumbnailUrl?: string;
      createdAt: number;
    }
  | {
      id: string;
      type: 'video';
      filePath: string;
      videoUrl: string;
      thumbnailUrl: string;
      createdAt: number;
    };

export async function normalizeClipboardHistory(
  items: unknown,
  limit: number,
  persistImage: (png: Buffer, hash: string) => Promise<string>,
) {
  const normalized = await Promise.all(
    (Array.isArray(items) ? items : []).map(
      async (item: any): Promise<ClipboardHistoryItem | null> => {
        if (item?.type === 'image' && (item.imagePath || item.imageDataUrl)) {
          let imagePath = item.imagePath;
          if (
            !imagePath &&
            typeof item.imageDataUrl === 'string' &&
            item.imageDataUrl.startsWith('data:')
          ) {
            const base64 = item.imageDataUrl.split(',', 2)[1] || '';
            try {
              const png = Buffer.from(base64, 'base64');
              imagePath = await persistImage(png, hashValue(png));
            } catch {}
          }
          const id =
            item.id ||
            (imagePath
              ? `image:${path.basename(imagePath, '.png')}`
              : `image:${hashValue(item.imageDataUrl)}`);
          return {
            id,
            type: 'image',
            imagePath,
            imageDataUrl: imagePath
              ? fileUrlForPath(imagePath)
              : item.imageDataUrl,
            thumbnailUrl: imagePath
              ? thumbnailUrlForPath(imagePath)
              : item.thumbnailUrl || item.imageDataUrl,
            createdAt: item.createdAt || Date.now(),
          };
        }
        if (item?.type === 'video' && item.filePath) {
          const filePath = expandUserPath(item.filePath);
          if (!isVideoPath(filePath)) return null;
          return {
            id: item.id || `video:${hashValue(filePath)}`,
            type: 'video',
            filePath,
            videoUrl: fileUrlForPath(filePath),
            thumbnailUrl: thumbnailUrlForPath(filePath),
            createdAt: item.createdAt || Date.now(),
          };
        }
        if (item?.text) {
          const text = String(item.text).trim();
          if (!text) return null;
          return {
            id: item.id?.startsWith('text:')
              ? item.id
              : `text:${hashValue(text)}`,
            type: 'text',
            text,
            createdAt: item.createdAt || Date.now(),
          };
        }
        return null;
      },
    ),
  );
  return normalized
    .filter((item): item is ClipboardHistoryItem => Boolean(item))
    .slice(0, limit);
}

export function clipboardItemTitle(item: ClipboardHistoryItem) {
  if (item.type === 'image') return 'Clipboard image';
  if (item.type === 'video')
    return path.basename(item.filePath || 'Clipboard video');
  return item.text.length > 72 ? `${item.text.slice(0, 72)}…` : item.text;
}

export function clipboardItemSubtitle(item: ClipboardHistoryItem) {
  const when = new Date(item.createdAt || Date.now()).toLocaleString(
    undefined,
    {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    },
  );
  if (item.type === 'image') return `Image copied ${when}`;
  if (item.type === 'video') return `Video copied ${when}`;
  return `Copied ${when}`;
}

export function clipboardFilePaths(clipboard: Clipboard) {
  const candidates = [
    clipboard.readBuffer('public.file-url').toString('utf8'),
    clipboard.readText(),
  ];
  const paths: string[] = [];
  for (const candidate of candidates) {
    for (const raw of String(candidate || '')
      .replace(/\0/g, '')
      .trim()
      .split(/\r?\n/)) {
      const value = raw.trim();
      if (!value) continue;
      if (value.startsWith('file://'))
        paths.push(decodeURIComponent(new URL(value).pathname));
      else if (path.isAbsolute(value)) paths.push(value);
    }
  }
  return Array.from(new Set(paths));
}

export function clipboardFilePath(clipboard: Clipboard) {
  return clipboardFilePaths(clipboard)[0] || null;
}
