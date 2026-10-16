import { extensionForPath } from './file-utils';

export function mimeTypeForPath(filePath) {
  const extension = extensionForPath(filePath);
  const types = {
    png: 'image/png',
    jpg: 'image/jpeg',
    jpeg: 'image/jpeg',
    gif: 'image/gif',
    webp: 'image/webp',
    bmp: 'image/bmp',
    tif: 'image/tiff',
    tiff: 'image/tiff',
    heic: 'image/heic',
    mp4: 'video/mp4',
    mov: 'video/quicktime',
    webm: 'video/webm',
    m4v: 'video/x-m4v',
    mkv: 'video/x-matroska',
    avi: 'video/x-msvideo',
    txt: 'text/plain',
    md: 'text/markdown',
    json: 'application/json',
    pdf: 'application/pdf',
    html: 'text/html',
    htm: 'text/html',
    csv: 'text/csv',
  };
  return types[extension] || 'application/octet-stream';
}
