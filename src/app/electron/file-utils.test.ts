import assert from 'node:assert/strict';
import path from 'node:path';
import test from 'node:test';
import { pathToFileURL } from 'node:url';
import {
  configureLocalFileUrlSecret,
  fileUrlForPath,
  localFilePathFromUrl,
  thumbnailUrlForPath,
  verifyLocalFileToken,
} from './file-utils';

const LOCAL_FILE_URL_SECRET_BYTES = 32;

test('fileUrlForPath preserves path casing in a fixed-host URL', () => {
  configureLocalFileUrlSecret(Buffer.alloc(LOCAL_FILE_URL_SECRET_BYTES, 1));
  const filePath = path.resolve(
    path.parse(process.cwd()).root,
    'Users',
    'example',
    'Clipboard Images',
    'preview.png',
  );
  const url = new URL(fileUrlForPath(filePath));

  assert.equal(url.host, 'local');
  assert.equal(url.pathname, pathToFileURL(filePath).pathname);
  assert.equal(localFilePathFromUrl(url), filePath);
  assert.equal(
    verifyLocalFileToken('file', filePath, url.searchParams.get('token')),
    true,
  );
});

test('thumbnailUrlForPath changes cache identity without changing authorization', () => {
  configureLocalFileUrlSecret(Buffer.alloc(LOCAL_FILE_URL_SECRET_BYTES, 2));
  const filePath = path.resolve(
    path.parse(process.cwd()).root,
    'Users',
    'example',
    'Desktop',
    'screenshot.png',
  );
  const original = new URL(thumbnailUrlForPath(filePath, '1000:2048'));
  const cached = new URL(thumbnailUrlForPath(filePath));
  const modified = new URL(thumbnailUrlForPath(filePath, '2000:2048'));

  assert.equal(cached.href, original.href);
  assert.notEqual(original.href, modified.href);
  assert.equal(original.searchParams.get('revision'), '1000:2048');
  assert.equal(modified.searchParams.get('revision'), '2000:2048');
  assert.equal(
    verifyLocalFileToken('thumb', filePath, modified.searchParams.get('token')),
    true,
  );
});
