import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

export const IMAGE_EXTENSIONS = new Set([
  'png',
  'jpg',
  'jpeg',
  'gif',
  'webp',
  'bmp',
  'tiff',
  'tif',
  'heic',
]);
export const VIDEO_EXTENSIONS = new Set([
  'mp4',
  'mov',
  'avi',
  'mkv',
  'webm',
  'wmv',
  'flv',
  'm4v',
]);
export const LOCAL_FILE_PROTOCOL = 'nvm-file';
export const LOCAL_THUMB_PROTOCOL = 'nvm-thumb';

let localFileUrlSecret: Buffer = crypto.randomBytes(32);

export function configureLocalFileUrlSecret(secret: string | Buffer) {
  const value = Buffer.isBuffer(secret)
    ? secret
    : Buffer.from(String(secret), 'base64url');
  if (value.length >= 32) localFileUrlSecret = value;
}

export function expandUserPath(value: string) {
  if (!value) return value;
  if (value === '~') return os.homedir();
  if (value.startsWith('~/')) return path.join(os.homedir(), value.slice(2));
  return value;
}

function canonicalLocalPath(filePath: string) {
  return path.resolve(expandUserPath(filePath));
}

function localFileToken(kind: 'file' | 'thumb', filePath: string) {
  return crypto
    .createHmac('sha256', localFileUrlSecret)
    .update(`${kind}\0${canonicalLocalPath(filePath)}`)
    .digest('base64url');
}

export function verifyLocalFileToken(
  kind: 'file' | 'thumb',
  filePath: string,
  token: string | null,
) {
  if (!token) return false;
  const expected = localFileToken(kind, filePath);
  const provided = Buffer.from(token);
  const actual = Buffer.from(expected);
  return (
    provided.length === actual.length &&
    crypto.timingSafeEqual(provided, actual)
  );
}

export function fileUrlForPath(filePath: string) {
  const resolved = canonicalLocalPath(filePath);
  const url = new URL(
    `${LOCAL_FILE_PROTOCOL}:${pathToFileURL(resolved).href.slice('file:'.length)}`,
  );
  url.searchParams.set('token', localFileToken('file', resolved));
  return url.href;
}

export function thumbnailUrlForPath(filePath: string) {
  const resolved = canonicalLocalPath(filePath);
  const url = new URL(`${LOCAL_THUMB_PROTOCOL}://thumb`);
  url.searchParams.set('path', resolved);
  url.searchParams.set('token', localFileToken('thumb', resolved));
  return url.href;
}

export function extensionForPath(filePath: string) {
  return path.extname(filePath).toLowerCase().replace(/^\./, '');
}

export function isImagePath(filePath: string) {
  return IMAGE_EXTENSIONS.has(extensionForPath(filePath));
}

export function isVideoPath(filePath: string) {
  return VIDEO_EXTENSIONS.has(extensionForPath(filePath));
}

export async function partitionRootsByExistence(roots: string[]) {
  const existing: string[] = [];
  const missing: string[] = [];
  await Promise.all(
    roots.map(async (root) => {
      try {
        await fs.access(root);
        existing.push(root);
      } catch {
        missing.push(root);
      }
    }),
  );
  return { existing, missing };
}
