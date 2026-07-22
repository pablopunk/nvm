import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const IMAGE_EXTENSIONS = new Set([
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
const VIDEO_EXTENSIONS = new Set([
  'mp4',
  'mov',
  'avi',
  'mkv',
  'webm',
  'wmv',
  'flv',
  'm4v',
]);
const LOCAL_FILE_PROTOCOL = 'nvm-file';
const LOCAL_THUMB_PROTOCOL = 'nvm-thumb';
const LOCAL_FILE_URL_SECRET_BYTES = 32;
const LEADING_PERIOD_PATTERN = /^\./;

let localFileUrlSecret: Buffer = crypto.randomBytes(
  LOCAL_FILE_URL_SECRET_BYTES,
);

function configureLocalFileUrlSecret(secret: string | Buffer) {
  const value = Buffer.isBuffer(secret)
    ? secret
    : Buffer.from(String(secret), 'base64url');
  if (value.length >= LOCAL_FILE_URL_SECRET_BYTES) {
    localFileUrlSecret = value;
  }
}

function expandUserPath(value: string) {
  if (!value) {
    return value;
  }
  if (value === '~') {
    return os.homedir();
  }
  if (value.startsWith('~/')) {
    return path.join(os.homedir(), value.slice(2));
  }
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

function verifyLocalFileToken(
  kind: 'file' | 'thumb',
  filePath: string,
  token: string | null,
) {
  if (!token) {
    return false;
  }
  const expected = localFileToken(kind, filePath);
  const provided = Buffer.from(token);
  const actual = Buffer.from(expected);
  return (
    provided.length === actual.length &&
    crypto.timingSafeEqual(provided, actual)
  );
}

function fileUrlForPath(filePath: string) {
  const resolved = canonicalLocalPath(filePath);
  const url = new URL(
    `${LOCAL_FILE_PROTOCOL}://local${pathToFileURL(resolved).pathname}`,
  );
  url.searchParams.set('token', localFileToken('file', resolved));
  return url.href;
}

function localFilePathFromUrl(urlInput: string | URL) {
  const url = typeof urlInput === 'string' ? new URL(urlInput) : urlInput;
  if (url.host === 'local') {
    return fileURLToPath(`file://${url.pathname}`);
  }
  return path.resolve(
    decodeURIComponent(url.host ? `/${url.host}${url.pathname}` : url.pathname),
  );
}

function thumbnailUrlForPath(filePath: string) {
  const resolved = canonicalLocalPath(filePath);
  const url = new URL(`${LOCAL_THUMB_PROTOCOL}://thumb`);
  url.searchParams.set('path', resolved);
  url.searchParams.set('token', localFileToken('thumb', resolved));
  return url.href;
}

function extensionForPath(filePath: string) {
  return path
    .extname(filePath)
    .toLowerCase()
    .replace(LEADING_PERIOD_PATTERN, '');
}

function isImagePath(filePath: string) {
  return IMAGE_EXTENSIONS.has(extensionForPath(filePath));
}

function isVideoPath(filePath: string) {
  return VIDEO_EXTENSIONS.has(extensionForPath(filePath));
}

async function checkRootExistence(
  root: string,
  existing: string[],
  missing: string[],
) {
  try {
    await fs.access(root);
    existing.push(root);
  } catch (error) {
    const code = (error as NodeJS.ErrnoException | undefined)?.code;
    if (code === 'ENOENT' || code === 'ENOTDIR') {
      missing.push(root);
    } else {
      existing.push(root);
    }
  }
}

async function partitionRootsByExistence(roots: string[]) {
  const existing: string[] = [];
  const missing: string[] = [];
  await Promise.all(
    roots.map((root) => checkRootExistence(root, existing, missing)),
  );
  return { existing, missing };
}

export {
  configureLocalFileUrlSecret,
  expandUserPath,
  extensionForPath,
  fileUrlForPath,
  IMAGE_EXTENSIONS,
  isImagePath,
  isVideoPath,
  LOCAL_FILE_PROTOCOL,
  LOCAL_THUMB_PROTOCOL,
  localFilePathFromUrl,
  partitionRootsByExistence,
  thumbnailUrlForPath,
  VIDEO_EXTENSIONS,
  verifyLocalFileToken,
};
