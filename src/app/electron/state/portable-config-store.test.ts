import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import {
  createDefaultPortableConfig,
  type PortableConfig,
} from './portable-config-schema';
import { createPortableConfigStore } from './portable-config-store';

const KINDS = { paletteHotkey: 'shortcut', errorReporting: 'boolean' } as const;

async function withConfigFile(
  callback: (filePath: string, directory: string) => Promise<void>,
) {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-portable-config-'),
  );
  try {
    await callback(path.join(directory, 'config.json'), directory);
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
}

function createTestStore(filePath: string) {
  const blockedReasons: Array<string | null> = [];
  const store = createPortableConfigStore({
    filePath: () => filePath,
    settingKinds: KINDS,
    onBlockedReasonChange: (reason) => blockedReasons.push(reason),
  });
  return { store, blockedReasons };
}

function withHotkey(accelerator: string): PortableConfig {
  return {
    ...createDefaultPortableConfig(),
    settings: { paletteHotkey: accelerator },
  };
}

const VALID_TEXT = JSON.stringify({
  version: 1,
  settings: { paletteHotkey: 'Command+Alt+K' },
  aliases: {},
  shortcuts: {},
  jobs: { enabled: {} },
  ai: { defaultModel: 'fast' },
});

test('first boot without a file is writable and the first change creates it', async () => {
  await withConfigFile(async (filePath) => {
    const { store } = createTestStore(filePath);
    const loaded = await store.load();
    assert.equal(loaded.writable, true);

    const result = await store.persist(withHotkey('Command+Alt+J'));
    assert.equal(result.ok, true);
    const written = JSON.parse(await fs.readFile(filePath, 'utf8'));
    assert.equal(written.version, 1);
    assert.equal(written.settings.paletteHotkey, 'Command+Alt+J');
    assert.equal(written.ai.defaultModel, 'fast');
  });
});

test('merges a persisted change onto the latest disk and keeps unknown fields', async () => {
  await withConfigFile(async (filePath) => {
    await fs.writeFile(
      filePath,
      JSON.stringify({
        ...JSON.parse(VALID_TEXT),
        futureSection: { keep: true },
      }),
    );
    const { store } = createTestStore(filePath);
    await store.load();
    await fs.writeFile(
      filePath,
      JSON.stringify({
        ...JSON.parse(VALID_TEXT),
        futureSection: { keep: true },
        aliases: { 'default:calculator': ['math'] },
      }),
    );

    const desired = { ...withHotkey('Command+Alt+J') };
    const result = await store.persist(desired);
    assert.equal(result.ok, true);
    const written = JSON.parse(await fs.readFile(filePath, 'utf8'));
    assert.equal(written.settings.paletteHotkey, 'Command+Alt+J');
    assert.deepEqual(written.aliases, { 'default:calculator': ['math'] });
    assert.deepEqual(written.futureSection, { keep: true });
  });
});

test('an invalid edit after a valid load blocks writes and keeps the last valid config', async () => {
  await withConfigFile(async (filePath) => {
    await fs.writeFile(filePath, VALID_TEXT);
    const { store, blockedReasons } = createTestStore(filePath);
    await store.load();
    await fs.writeFile(filePath, '{ "version": 1, ');

    const poll = await store.pollForExternalChange();
    assert.equal(poll.adopted, null);
    assert.match(poll.blockedReason ?? '', /invalid/);

    const result = await store.persist(withHotkey('Command+Alt+J'));
    assert.equal(result.ok, false);
    assert.equal(
      !result.ok && result.current.settings.paletteHotkey,
      'Command+Alt+K',
    );
    assert.equal(await fs.readFile(filePath, 'utf8'), '{ "version": 1, ');
    assert.equal(blockedReasons.length, 1);
  });
});

test('deleting the file after a valid load blocks writes instead of recreating it', async () => {
  await withConfigFile(async (filePath) => {
    await fs.writeFile(filePath, VALID_TEXT);
    const { store } = createTestStore(filePath);
    await store.load();
    await fs.rm(filePath);

    const poll = await store.pollForExternalChange();
    assert.match(poll.blockedReason ?? '', /deleted/);
    const result = await store.persist(withHotkey('Command+Alt+J'));
    assert.equal(result.ok, false);
    await assert.rejects(fs.access(filePath));
  });
});

test('a valid external edit after a block is adopted and unblocks writes', async () => {
  await withConfigFile(async (filePath) => {
    await fs.writeFile(filePath, VALID_TEXT);
    const { store } = createTestStore(filePath);
    await store.load();
    await fs.rm(filePath);
    await store.pollForExternalChange();
    await fs.writeFile(
      filePath,
      VALID_TEXT.replace('Command+Alt+K', 'Command+Alt+Z'),
    );

    const poll = await store.pollForExternalChange();
    assert.equal(poll.blockedReason, null);
    assert.equal(poll.adopted?.settings.paletteHotkey, 'Command+Alt+Z');
  });
});

test('restoring the last valid contents after an invalid edit unblocks writes', async () => {
  await withConfigFile(async (filePath) => {
    await fs.writeFile(filePath, VALID_TEXT);
    const { store } = createTestStore(filePath);
    await store.load();
    await fs.writeFile(filePath, '{invalid');
    await store.pollForExternalChange();
    assert.equal(store.snapshot().writable, false);
    await fs.writeFile(filePath, VALID_TEXT);
    await store.pollForExternalChange();
    assert.equal(store.snapshot().writable, true);
    assert.equal((await store.persist(withHotkey('Command+Alt+J'))).ok, true);
  });
});

test('writes through a symlink update the target and keep the link and file mode', async () => {
  await withConfigFile(async (filePath, directory) => {
    const realPath = path.join(directory, 'dotfiles-config.json');
    await fs.writeFile(realPath, VALID_TEXT, { mode: 0o640 });
    await fs.chmod(realPath, 0o640);
    const originalMode = (await fs.stat(realPath)).mode & 0o777;
    await fs.symlink(realPath, filePath);
    const { store } = createTestStore(filePath);
    await store.load();

    const result = await store.persist(withHotkey('Command+Alt+J'));
    assert.equal(result.ok, true);
    assert.equal((await fs.lstat(filePath)).isSymbolicLink(), true);
    const written = JSON.parse(await fs.readFile(realPath, 'utf8'));
    assert.equal(written.settings.paletteHotkey, 'Command+Alt+J');
    assert.equal((await fs.stat(realPath)).mode & 0o777, originalMode);
  });
});

test('a write with no changes does not touch the file', async () => {
  await withConfigFile(async (filePath) => {
    await fs.writeFile(filePath, VALID_TEXT);
    const { store } = createTestStore(filePath);
    await store.load();
    const before = await fs.readFile(filePath, 'utf8');

    const result = await store.persist(withHotkey('Command+Alt+K'));
    assert.equal(result.ok, true);
    assert.equal(await fs.readFile(filePath, 'utf8'), before);
  });
});
