import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { createDirectoryTreeWatch } from '../os/directory-tree-watch';
import { createFileIndexRefresher } from './file-index-refresher';

async function listTopLevelFiles(root: string) {
  const entries = await fs.readdir(root, { withFileTypes: true });
  return entries
    .filter((entry) => entry.isFile() && !entry.name.startsWith('.'))
    .map((entry) => {
      const filePath = path.join(root, entry.name);
      return { id: filePath, name: entry.name, path: filePath };
    });
}

function liveIndexFixture(root: string) {
  let changeSignals = 0;
  const refresher = createFileIndexRefresher({
    scan: async () => ({
      files: await listTopLevelFiles(root),
      existingRoots: [root],
      missingRoots: [],
      unreadableRoots: [],
    }),
    notifyChanged: () => {},
    watchDirectoryTree: createDirectoryTreeWatch(process.platform),
    onRelevantChange: () => {
      changeSignals += 1;
    },
  });

  async function changeAndRefresh(change: () => Promise<unknown>) {
    const before = changeSignals;
    await change();
    const deadline = Date.now() + 3000;
    while (changeSignals === before && Date.now() < deadline)
      await new Promise((resolve) => setTimeout(resolve, 25));
    assert.ok(changeSignals > before, 'expected a file change signal');
    await refresher.refresh({ roots: [root], depth: 0 });
    return refresher
      .current()
      .map((entry) => path.relative(root, entry.path))
      .sort();
  }

  return { refresher, changeAndRefresh };
}

test('live watcher refreshes the index after create, rename, move, and delete', async (t) => {
  const base = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-file-index-'));
  const root = path.join(base, 'root');
  const outside = path.join(base, 'outside');
  await fs.mkdir(root);
  await fs.mkdir(outside);
  const fixture = liveIndexFixture(root);
  t.after(async () => {
    fixture.refresher.close();
    await fs.rm(base, { recursive: true, force: true });
  });
  await fixture.refresher.refresh({ roots: [root], depth: 0 });
  assert.deepEqual(fixture.refresher.current(), []);

  assert.deepEqual(
    await fixture.changeAndRefresh(() =>
      fs.writeFile(path.join(root, 'a.txt'), 'a'),
    ),
    ['a.txt'],
  );
  assert.deepEqual(
    await fixture.changeAndRefresh(() =>
      fs.rename(path.join(root, 'a.txt'), path.join(root, 'b.txt')),
    ),
    ['b.txt'],
  );
  assert.deepEqual(
    await fixture.changeAndRefresh(() =>
      fs.rename(path.join(root, 'b.txt'), path.join(outside, 'b.txt')),
    ),
    [],
  );
  assert.deepEqual(
    await fixture.changeAndRefresh(() =>
      fs.rename(path.join(outside, 'b.txt'), path.join(root, 'c.txt')),
    ),
    ['c.txt'],
  );
  assert.deepEqual(
    await fixture.changeAndRefresh(() => fs.rm(path.join(root, 'c.txt'))),
    [],
  );
});
