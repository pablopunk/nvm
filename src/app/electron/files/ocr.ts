import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { captureScreenImage, recognizeTextInImage } from '../os/os';
import { expandUserPath, isImagePath, LOCAL_FILE_PROTOCOL } from './file-utils';

function dataUrlExtension(dataUrl: string) {
  const mime = dataUrl.match(/^data:([^;,]+)/)?.[1] || 'image/png';
  if (mime.includes('jpeg')) return 'jpg';
  if (mime.includes('webp')) return 'webp';
  if (mime.includes('gif')) return 'gif';
  if (mime.includes('tiff')) return 'tiff';
  if (mime.includes('heic')) return 'heic';
  return 'png';
}

async function writeOcrDataUrl(dataUrl: string) {
  const match = dataUrl.match(/^data:([^;,]+)?(;base64)?,(.*)$/);
  if (!match) throw new Error('Invalid OCR data URL');
  const outputPath = path.join(
    os.tmpdir(),
    `nevermind-ocr-input-${crypto.randomUUID()}.${dataUrlExtension(dataUrl)}`,
  );
  const body = decodeURIComponent(match[3] || '');
  await fs.writeFile(
    outputPath,
    match[2] ? Buffer.from(match[3] || '', 'base64') : Buffer.from(body),
  );
  return outputPath;
}

function filePathFromLocalUrl(value: string) {
  if (value.startsWith(`${LOCAL_FILE_PROTOCOL}:`))
    return fileURLToPath(
      `file:${value.slice(`${LOCAL_FILE_PROTOCOL}:`.length)}`,
    );
  if (value.startsWith('file:')) return fileURLToPath(value);
  return null;
}

async function ocrInputPath(input: any) {
  const value =
    typeof input === 'string'
      ? input
      : input?.path || input?.filePath || input?.fileUrl || input?.url;
  if (!value)
    throw new Error(
      'OCR requires an image path, file URL, data URL, or ExtensionFile',
    );
  const text = String(value);
  if (text.startsWith('data:'))
    return { path: await writeOcrDataUrl(text), cleanup: true };
  const urlPath = filePathFromLocalUrl(text);
  return { path: expandUserPath(urlPath || text), cleanup: false };
}

export async function ocrImage(input: any, options: any = {}) {
  const resolved = await ocrInputPath(input);
  try {
    if (!isImagePath(resolved.path))
      throw new Error('OCR currently supports image files only');
    return await recognizeTextInImage(resolved.path, options);
  } finally {
    if (resolved.cleanup)
      await fs.rm(resolved.path, { force: true }).catch(() => {});
  }
}

export async function ocrScreen(options: any = {}) {
  const imagePath = await captureScreenImage(options);
  try {
    return await recognizeTextInImage(imagePath, options);
  } finally {
    await fs.rm(imagePath, { force: true }).catch(() => {});
  }
}
