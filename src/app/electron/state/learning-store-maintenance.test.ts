import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { LocalLearningStore } from './learning-store';

test('flush persists pending learning traces before the debounce timer fires', async (t) => {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-learning-flush-'),
  );
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const tracesPath = path.join(directory, 'traces.json');
  const store = new LocalLearningStore({
    tracesPath,
    learningsPath: path.join(directory, 'learnings.md'),
  });
  await store.load();
  store.appendMessage('conversation', 'user', 'Remember this');
  await store.flushPendingSave();
  const data = JSON.parse(await fs.readFile(tracesPath, 'utf8'));
  assert.equal(data.traces[0].messages[0].content, 'Remember this');
});
