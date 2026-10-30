import assert from 'node:assert/strict';
import path from 'node:path';
import test from 'node:test';
import {
  createFileIndexRefresher,
  type FileIndexScan,
} from './file-index-refresher';

const ROOT = path.join(path.sep, 'home', 'me', 'Documents');
const DESKTOP = path.join(path.sep, 'home', 'me', 'Desktop');

function file(name: string, root = ROOT, mtimeMs = 1) {
  const filePath = path.join(root, name);
  return { id: filePath, name, path: filePath, mtimeMs, dateAddedMs: mtimeMs };
}

function scanOf(files: any[], overrides: Partial<FileIndexScan> = {}) {
  return {
    files,
    existingRoots: [ROOT],
    missingRoots: [],
    unreadableRoots: [],
    ...overrides,
  };
}

function inertWatch() {
  return { close() {}, on() {} };
}

function refresherWithScans(
  scans: Array<FileIndexScan | Promise<FileIndexScan>>,
) {
  const notifications: number[] = [];
  const watchedRoots: string[] = [];
  const refresher = createFileIndexRefresher({
    scan: async () => {
      const next = scans.shift();
      if (!next) throw new Error('unexpected scan');
      return next;
    },
    notifyChanged: () => notifications.push(Date.now()),
    watchDirectoryTree: (root) => {
      watchedRoots.push(root);
      return inertWatch();
    },
    onRelevantChange: () => {},
  });
  return { refresher, notifications, watchedRoots };
}

function paths(files: any[]) {
  return files.map((entry) => entry.path).sort();
}

test('create, rename, move, and delete scans replace the cached index', async () => {
  const { refresher, notifications } = refresherWithScans([
    scanOf([file('a.pdf')]),
    scanOf([file('a.pdf'), file('b.pdf')]),
    scanOf([file('a.pdf'), file('renamed.pdf')]),
    scanOf([file('a.pdf'), file(path.join('archive', 'renamed.pdf'))]),
    scanOf([file(path.join('archive', 'renamed.pdf'))]),
  ]);
  await refresher.refresh();
  await refresher.refresh();
  assert.deepEqual(paths(refresher.current()), [
    path.join(ROOT, 'a.pdf'),
    path.join(ROOT, 'b.pdf'),
  ]);
  await refresher.refresh();
  assert.deepEqual(paths(refresher.current()), [
    path.join(ROOT, 'a.pdf'),
    path.join(ROOT, 'renamed.pdf'),
  ]);
  await refresher.refresh();
  assert.deepEqual(paths(refresher.current()), [
    path.join(ROOT, 'a.pdf'),
    path.join(ROOT, 'archive', 'renamed.pdf'),
  ]);
  await refresher.refresh();
  assert.deepEqual(paths(refresher.current()), [
    path.join(ROOT, 'archive', 'renamed.pdf'),
  ]);
  assert.equal(notifications.length, 5);
});

test('an unchanged rescan does not notify the palette', async () => {
  const { refresher, notifications } = refresherWithScans([
    scanOf([file('a.pdf')]),
    scanOf([file('a.pdf')]),
  ]);
  await refresher.refresh();
  const result = await refresher.refresh();
  assert.equal(result.changed, false);
  assert.equal(notifications.length, 1);
});

test('an older scan finishing last does not overwrite a newer index', async () => {
  let finishOld!: (scan: FileIndexScan) => void;
  const oldScan = new Promise<FileIndexScan>((resolve) => {
    finishOld = resolve;
  });
  const { refresher } = refresherWithScans([
    oldScan,
    scanOf([file('new.pdf')]),
  ]);
  const older = refresher.refresh();
  const newer = await refresher.refresh();
  finishOld(scanOf([file('old.pdf')]));
  const stale = await older;
  assert.equal(newer.committed, true);
  assert.equal(stale.committed, false);
  assert.deepEqual(paths(refresher.current()), [path.join(ROOT, 'new.pdf')]);
});

test('unreadable roots keep cached entries while readable roots refresh', async () => {
  const { refresher } = refresherWithScans([
    scanOf([file('doc.pdf'), file('shot.png', DESKTOP)], {
      existingRoots: [ROOT, DESKTOP],
    }),
    scanOf([file('doc-2.pdf')], {
      existingRoots: [ROOT],
      unreadableRoots: [DESKTOP],
    }),
  ]);
  await refresher.refresh();
  const result = await refresher.refresh();
  assert.deepEqual(result.scan.unreadableRoots, [DESKTOP]);
  assert.deepEqual(paths(refresher.current()), [
    path.join(DESKTOP, 'shot.png'),
    path.join(ROOT, 'doc-2.pdf'),
  ]);
});

test('missing roots drop their entries and are not watched', async () => {
  const { refresher, watchedRoots } = refresherWithScans([
    scanOf([file('doc.pdf'), file('shot.png', DESKTOP)], {
      existingRoots: [ROOT, DESKTOP],
    }),
    scanOf([file('doc.pdf')], { missingRoots: [DESKTOP] }),
  ]);
  await refresher.refresh();
  assert.deepEqual(refresher.watchedRoots().sort(), [DESKTOP, ROOT].sort());
  await refresher.refresh();
  assert.deepEqual(paths(refresher.current()), [path.join(ROOT, 'doc.pdf')]);
  assert.deepEqual(refresher.watchedRoots(), [ROOT]);
  assert.equal(watchedRoots.length, 2);
});

test('a failed scan keeps the cached snapshot', async () => {
  const scans: any[] = [scanOf([file('a.pdf')])];
  const refresher = createFileIndexRefresher({
    scan: async () => {
      const next = scans.shift();
      if (!next) throw new Error('disk unavailable');
      return next;
    },
    notifyChanged: () => {},
    watchDirectoryTree: () => inertWatch(),
    onRelevantChange: () => {},
  });
  await refresher.refresh();
  await assert.rejects(refresher.refresh(), /disk unavailable/);
  assert.deepEqual(paths(refresher.current()), [path.join(ROOT, 'a.pdf')]);
});
