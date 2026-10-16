import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { createDictationRecordings } from './dictation-recordings';

test('persists failed audio segments for retry and removes only the selected recording', async () => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-dictation-'));
  try {
    const recordings = createDictationRecordings(directory);
    const id = `dictation-${randomUUID()}`;
    const segments = [new Uint8Array([1, 2]), new Uint8Array([3, 4])];
    await recordings.save(id, segments);
    assert.equal((await recordings.list())[0]?.segmentCount, 2);
    assert.deepEqual(
      await recordings.load(id),
      segments.map((audio) => Buffer.from(audio)),
    );
    await assert.rejects(
      recordings.remove('../other'),
      /Invalid dictation recording ID/,
    );
    await recordings.remove(id);
    assert.deepEqual(await recordings.list(), []);
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});
