import assert from 'node:assert/strict';
import test from 'node:test';
import {
  configureLocalFileUrlSecret,
  fileUrlForPath,
  verifyLocalFileToken,
} from './file-utils';

const LOCAL_FILE_URL_SECRET_BYTES = 32;

test('fileUrlForPath preserves path casing in a fixed-host URL', () => {
  configureLocalFileUrlSecret(Buffer.alloc(LOCAL_FILE_URL_SECRET_BYTES, 1));
  const filePath = '/Users/example/Clipboard Images/preview.png';
  const url = new URL(fileUrlForPath(filePath));

  assert.equal(url.host, 'local');
  assert.equal(decodeURIComponent(url.pathname), filePath);
  assert.equal(
    verifyLocalFileToken('file', filePath, url.searchParams.get('token')),
    true,
  );
});
