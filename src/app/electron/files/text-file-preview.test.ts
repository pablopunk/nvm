import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import {
  readTextFilePreview,
  textFilePreviewMarkdown,
} from './text-file-preview';

const KILOBYTE = 1024;
const LARGE_FILE_BYTES = 204_800;
const SPLIT_CHARACTER_LIMIT_BYTES = 4;
const TRUNCATION_NOTE_PATTERN = /Showing the first 0 KB of 200 KB/;
const BINARY_NOTE_PATTERN = /Binary file \(2 KB\)/;

function writeTempFile(name: string, contents: string | Buffer) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'nvm-text-preview-'));
  const filePath = path.join(directory, name);
  fs.writeFileSync(filePath, contents);
  return filePath;
}

test('reads small text files whole', async () => {
  const filePath = writeTempFile('notes.md', '# Notes\n');
  assert.deepEqual(await readTextFilePreview(filePath), {
    kind: 'text',
    text: '# Notes\n',
    truncated: false,
    sizeBytes: 8,
  });
});

test('bounds large files and drops a split trailing character', async () => {
  const filePath = writeTempFile('long.txt', `abc${'é'.repeat(10)}`);
  const preview = await readTextFilePreview(
    filePath,
    SPLIT_CHARACTER_LIMIT_BYTES,
  );
  assert.deepEqual(preview, {
    kind: 'text',
    text: 'abc',
    truncated: true,
    sizeBytes: Buffer.byteLength(`abc${'é'.repeat(10)}`),
  });
});

test('detects binary files and unreadable paths', async () => {
  const binaryPath = writeTempFile('blob.bin', Buffer.from([1, 0, 2]));
  assert.deepEqual(await readTextFilePreview(binaryPath), {
    kind: 'binary',
    sizeBytes: 3,
  });
  const missing = await readTextFilePreview('/nonexistent/nvm-preview.txt');
  assert.equal(missing.kind, 'unreadable');
});

test('fences text with a fence longer than any backtick run', () => {
  const markdown = textFilePreviewMarkdown('/tmp/a.md', {
    kind: 'text',
    text: 'use ```js``` here',
    truncated: false,
    sizeBytes: 17,
  });
  assert.equal(markdown, '````md\nuse ```js``` here\n````');
});

test('notes truncation and binary content in markdown', () => {
  assert.match(
    textFilePreviewMarkdown('/tmp/a.txt', {
      kind: 'text',
      text: 'abc',
      truncated: true,
      sizeBytes: LARGE_FILE_BYTES,
    }),
    TRUNCATION_NOTE_PATTERN,
  );
  assert.match(
    textFilePreviewMarkdown('/tmp/a.bin', {
      kind: 'binary',
      sizeBytes: 2 * KILOBYTE,
    }),
    BINARY_NOTE_PATTERN,
  );
});
