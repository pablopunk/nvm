import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { writeTextFile } from './write-text-file';

async function temporaryFolder() {
  return fs.mkdtemp(path.join(os.tmpdir(), 'nvm-write-text-'));
}

test('writes a new text file into an existing folder', async () => {
  const folder = await temporaryFolder();
  const target = path.join(folder, 'Trip Plan.md');

  assert.equal(await writeTextFile(target, '# Porto\n'), target);
  assert.equal(await fs.readFile(target, 'utf8'), '# Porto\n');
});

test('refuses to replace an existing file unless overwrite is requested', async () => {
  const folder = await temporaryFolder();
  const target = path.join(folder, 'notes.txt');
  await writeTextFile(target, 'first');

  await assert.rejects(writeTextFile(target, 'second'), /already exists/);
  assert.equal(await fs.readFile(target, 'utf8'), 'first');
  await writeTextFile(target, 'second', { overwrite: true });
  assert.equal(await fs.readFile(target, 'utf8'), 'second');
});

test('never creates missing folders or writes relative paths', async () => {
  const folder = await temporaryFolder();

  await assert.rejects(
    writeTextFile(path.join(folder, 'missing', 'a.txt'), 'x'),
    /does not exist/,
  );
  await assert.rejects(writeTextFile('relative.txt', 'x'), /absolute path/);
});
