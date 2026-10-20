import assert from 'node:assert/strict';
import test from 'node:test';
import {
  CLIPBOARD_IMAGE_GRACE_MS,
  CLIPBOARD_IMAGE_SWEEP_MAX_DELETES,
  type ClipboardImageSweeperDeps,
  createClipboardImageSweeper,
} from './clipboard-image-sweeper';

const NOW = 10_000_000_000;
const OLD = NOW - CLIPBOARD_IMAGE_GRACE_MS - 1;
const hashed = (n: number) => `${String(n).padStart(40, '0')}.png`;
const REFERENCED = hashed(1);
const ORPHAN = hashed(2);
const DIR = '/data/clipboard-images';

type Setup = {
  files?: Record<string, { mtimeMs?: number; file?: boolean }>;
  memory?: unknown[];
  persisted?: unknown[] | null;
  trusted?: boolean;
  overrides?: Partial<ClipboardImageSweeperDeps>;
};

function createSweeper(setup: Setup = {}) {
  const files = setup.files ?? {
    [REFERENCED]: { mtimeMs: OLD },
    [ORPHAN]: { mtimeMs: OLD },
  };
  const unlinked: string[] = [];
  const warnings: string[] = [];
  const memory = setup.memory ?? [
    { id: 'image:a', type: 'image', imagePath: `${DIR}/${REFERENCED}` },
  ];
  const sweeper = createClipboardImageSweeper({
    getImagesDir: () => DIR,
    getInMemoryHistory: () => memory,
    readPersistedHistory: async () =>
      setup.persisted === undefined ? [] : setup.persisted,
    isStateTrusted: () => setup.trusted !== false,
    readdir: async () => Object.keys(files),
    lstat: async (filePath) => {
      const entry = files[filePath.split('/').pop() as string];
      return {
        isFile: () => entry?.file !== false,
        mtimeMs: entry?.mtimeMs ?? OLD,
      };
    },
    unlink: async (filePath) => {
      unlinked.push(filePath.split('/').pop() as string);
    },
    now: () => NOW,
    logWarn: (message) => warnings.push(message),
    ...setup.overrides,
  });
  return { sweeper, unlinked, warnings, memory };
}

test('deletes an old unreferenced image and keeps referenced ones', async () => {
  const { sweeper, unlinked } = createSweeper();
  const result = await sweeper.sweep();
  assert.deepEqual(unlinked, [ORPHAN]);
  assert.equal(result.deleted, 1);
});

test('keeps a file shared by an image-file entry via imagePath, not id', async () => {
  const { sweeper, unlinked } = createSweeper({
    memory: [
      {
        id: `image-file:${'f'.repeat(40)}`,
        type: 'image',
        imagePath: `${DIR}/${ORPHAN}`,
      },
    ],
  });
  await sweeper.sweep();
  assert.deepEqual(unlinked, [REFERENCED]);
});

test('keeps files referenced only by persisted history', async () => {
  const { sweeper, unlinked } = createSweeper({
    persisted: [{ type: 'image', imagePath: `${DIR}/${ORPHAN}` }],
  });
  await sweeper.sweep();
  assert.deepEqual(unlinked, []);
});

test('aborts without deleting when persisted history is unreadable', async () => {
  const { sweeper, unlinked } = createSweeper({ persisted: null });
  const result = await sweeper.sweep();
  assert.deepEqual(unlinked, []);
  assert.equal(result.reason, 'persisted-unreadable');
});

test('never deletes when state was not loaded from a valid file', async () => {
  const { sweeper, unlinked } = createSweeper({ trusted: false, memory: [] });
  const result = await sweeper.sweep();
  assert.deepEqual(unlinked, []);
  assert.equal(result.reason, 'untrusted-state');
});

test('keeps files newer than the grace window', async () => {
  const { sweeper, unlinked } = createSweeper({
    files: { [ORPHAN]: { mtimeMs: NOW - 1000 } },
    memory: [],
  });
  await sweeper.sweep();
  assert.deepEqual(unlinked, []);
});

test('ignores non-image names, directories and symlinks', async () => {
  const { sweeper, unlinked } = createSweeper({
    files: {
      'notes.txt': { mtimeMs: OLD },
      [`${'a'.repeat(40)}.jpg`]: { mtimeMs: OLD },
      [hashed(3)]: { mtimeMs: OLD, file: false },
    },
    memory: [],
  });
  await sweeper.sweep();
  assert.deepEqual(unlinked, []);
});

test('keeps a file that gains a reference between listing and unlink', async () => {
  const memory: unknown[] = [];
  const { sweeper, unlinked } = createSweeper({
    memory,
    files: { [ORPHAN]: { mtimeMs: OLD } },
    overrides: {
      lstat: async () => {
        memory.push({ type: 'image', imagePath: `${DIR}/${ORPHAN}` });
        return { isFile: () => true, mtimeMs: OLD };
      },
    },
  });
  await sweeper.sweep();
  assert.deepEqual(unlinked, []);
});

test('keeps a file whose mtime was bumped by a re-copy before unlink', async () => {
  const { sweeper, unlinked } = createSweeper({
    files: { [ORPHAN]: { mtimeMs: OLD } },
    memory: [],
    overrides: {
      lstat: async () => ({ isFile: () => true, mtimeMs: NOW }),
    },
  });
  await sweeper.sweep();
  assert.deepEqual(unlinked, []);
});

test('swallows unlink errors and keeps sweeping', async () => {
  const second = hashed(4);
  const attempted: string[] = [];
  const { sweeper, warnings } = createSweeper({
    files: { [ORPHAN]: { mtimeMs: OLD }, [second]: { mtimeMs: OLD } },
    memory: [],
    overrides: {
      unlink: async (filePath) => {
        attempted.push(filePath);
        if (filePath.endsWith(ORPHAN)) throw new Error('EPERM');
      },
    },
  });
  const result = await sweeper.sweep();
  assert.equal(attempted.length, 2);
  assert.equal(result.deleted, 1);
  assert.deepEqual(warnings, ['clipboard.images.sweep.unlinkFailed']);
});

test('caps deletions per run', async () => {
  const files: Record<string, { mtimeMs: number }> = {};
  for (let n = 0; n < CLIPBOARD_IMAGE_SWEEP_MAX_DELETES + 20; n += 1)
    files[hashed(n + 10)] = { mtimeMs: OLD };
  const { sweeper, unlinked } = createSweeper({ files, memory: [] });
  await sweeper.sweep();
  assert.equal(unlinked.length, CLIPBOARD_IMAGE_SWEEP_MAX_DELETES);
});

test('stops at the deadline', async () => {
  let clock = NOW;
  const { sweeper, unlinked } = createSweeper({
    files: { [ORPHAN]: { mtimeMs: OLD }, [hashed(5)]: { mtimeMs: OLD } },
    memory: [],
    overrides: {
      now: () => clock,
      unlink: async (filePath) => {
        unlinked.push(filePath);
        clock += 60_000;
      },
    },
  });
  await sweeper.sweep();
  assert.equal(unlinked.length, 1);
});

test('shares one in-flight sweep and survives failures', async () => {
  let reads = 0;
  const { sweeper, warnings } = createSweeper({
    overrides: {
      readdir: async () => {
        reads += 1;
        throw new Error('boom');
      },
    },
  });
  const [first, second] = await Promise.all([sweeper.sweep(), sweeper.sweep()]);
  assert.equal(reads, 1);
  assert.equal(first, second);
  assert.equal(first.reason, 'error');
  assert.deepEqual(warnings, ['clipboard.images.sweep.failed']);
});
