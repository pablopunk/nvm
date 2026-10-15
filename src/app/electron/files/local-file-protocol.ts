import { createReadStream } from 'node:fs';
import fs from 'node:fs/promises';
import path from 'node:path';
import { Readable } from 'node:stream';
import { error as logError } from '../platform/logger';
import {
  LOCAL_FILE_PROTOCOL,
  LOCAL_THUMB_PROTOCOL,
  localFilePathFromUrl,
  verifyLocalFileToken,
} from './file-utils';
import { mimeTypeForPath } from './mime-type';

interface LocalFileProtocolRegistry {
  handle(
    scheme: string,
    handler: (request: Request) => Response | Promise<Response>,
  ): void;
}

export interface LocalFileProtocolDeps {
  protocol: LocalFileProtocolRegistry;
  thumbnailResponseForPath: (filePath: string) => Promise<Response>;
}

async function localFileResponse(requestPath: string, request: Request) {
  const stat = await fs.stat(requestPath).catch(() => null);
  if (!stat?.isFile()) return new Response('File not found', { status: 404 });

  const headers = new Headers({
    'accept-ranges': 'bytes',
    'cache-control': 'private, max-age=3600',
    'content-type': mimeTypeForPath(requestPath),
  });
  const range = request.headers.get('range');
  let start = 0;
  let end = stat.size - 1;
  let status = 200;

  if (range) {
    const match = range.match(/^bytes=(\d*)-(\d*)$/);
    if (!(match && (match[1] || match[2])))
      return new Response('Invalid range', {
        status: 416,
        headers: { 'content-range': `bytes */${stat.size}` },
      });
    if (match[1]) {
      start = Number(match[1]);
      end = match[2] ? Number(match[2]) : stat.size - 1;
    } else {
      start = Math.max(0, stat.size - Number(match[2]));
      end = stat.size - 1;
    }
    if (
      Number.isNaN(start) ||
      Number.isNaN(end) ||
      start > end ||
      start >= stat.size
    )
      return new Response('Invalid range', {
        status: 416,
        headers: { 'content-range': `bytes */${stat.size}` },
      });
    end = Math.min(end, stat.size - 1);
    status = 206;
    headers.set('content-range', `bytes ${start}-${end}/${stat.size}`);
  }

  headers.set('content-length', String(Math.max(0, end - start + 1)));
  return new Response(
    Readable.toWeb(createReadStream(requestPath, { start, end })) as BodyInit,
    { status, headers },
  );
}

export function registerLocalFileProtocol({
  protocol,
  thumbnailResponseForPath,
}: LocalFileProtocolDeps) {
  protocol.handle(LOCAL_FILE_PROTOCOL, (request) => {
    const url = new URL(request.url);
    const requestPath = localFilePathFromUrl(url);
    if (!path.isAbsolute(requestPath))
      return new Response('Invalid file path', { status: 400 });
    if (
      !verifyLocalFileToken('file', requestPath, url.searchParams.get('token'))
    )
      return new Response('Forbidden', { status: 403 });
    return localFileResponse(requestPath, request);
  });

  protocol.handle(LOCAL_THUMB_PROTOCOL, async (request) => {
    const url = new URL(request.url);
    const requestPath = path.resolve(
      decodeURIComponent(url.searchParams.get('path') || ''),
    );
    if (!path.isAbsolute(requestPath))
      return new Response('Invalid file path', { status: 400 });
    if (
      !verifyLocalFileToken('thumb', requestPath, url.searchParams.get('token'))
    )
      return new Response('Forbidden', { status: 403 });
    try {
      return await thumbnailResponseForPath(requestPath);
    } catch (error) {
      logError(
        'thumbnail.create.failed',
        { requestPath, error },
        { source: 'host', scope: 'thumbnail' },
      );
      return new Response('Thumbnail not found', { status: 404 });
    }
  });
}
