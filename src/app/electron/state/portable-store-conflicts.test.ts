import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import {
  createPortableConfigFileExclusively,
  replacePortableConfigFile,
} from './portable-config-file';
import {
  createDefaultPortableConfig,
  type PortableConfig,
} from './portable-config-schema';
import { createPortableConfigStore } from './portable-config-store';

const KINDS = { paletteHotkey: 'shortcut', errorReporting: 'boolean' } as const;
const VALID_TEXT = JSON.stringify({
  version: 1,
  settings: { paletteHotkey: 'Command+Alt+K' },
  aliases: {},
  shortcuts: {},
  jobs: { enabled: {} },
  ai: { defaultModel: 'fast' },
});

async function withDirectory(callback: (directory: string) => Promise<void>) {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-portable-conflicts-'),
  );
  try {
    await callback(directory);
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
}

function storeFor(filePath: string) {
  return createPortableConfigStore({
    filePath: () => filePath,
    settingKinds: KINDS,
    onBlockedReasonChange: () => undefined,
  });
}

function withHotkey(accelerator: string): PortableConfig {
  return {
    ...createDefaultPortableConfig(),
    settings: { paletteHotkey: accelerator },
  };
}

test('a missing model field uses its default when checking conflicts', async () => {
  await withDirectory(async (directory) => {
    const filePath = path.join(directory, 'config.json');
    await fs.writeFile(filePath, JSON.stringify({ version: 1 }));
    const store = storeFor(filePath);
    await store.load();
    const result = await store.persist({
      ...createDefaultPortableConfig(),
      ai: { defaultModel: 'smart' },
    });
    assert.equal(result.ok, true);
    assert.equal(
      JSON.parse(await fs.readFile(filePath, 'utf8')).ai.defaultModel,
      'smart',
    );
  });
});

test('a write failure returns a failure instead of throwing', async () => {
  await withDirectory(async (directory) => {
    const parent = path.join(directory, 'config-dir');
    const filePath = path.join(parent, 'config.json');
    const store = storeFor(filePath);
    await store.load();
    await fs.writeFile(parent, 'not a directory');
    const result = await store.persist(withHotkey('Command+Alt+K'));
    assert.equal(result.ok, false);
  });
});

test('a same-key edit that landed on disk after load is reported, not overwritten', async () => {
  await withDirectory(async (directory) => {
    const filePath = path.join(directory, 'config.json');
    await fs.writeFile(filePath, VALID_TEXT);
    const store = storeFor(filePath);
    await store.load();
    await fs.writeFile(
      filePath,
      VALID_TEXT.replace('Command+Alt+K', 'Command+Alt+B'),
    );

    const result = await store.persist(withHotkey('Command+Alt+C'));
    assert.equal(result.ok, false);
    assert.match(
      result.ok ? '' : result.reason,
      /settings\.paletteHotkey changed/,
    );
    assert.equal(
      JSON.parse(await fs.readFile(filePath, 'utf8')).settings.paletteHotkey,
      'Command+Alt+B',
    );
    assert.equal(
      !result.ok && result.current.settings.paletteHotkey,
      'Command+Alt+B',
    );
  });
});

test('a competing edit to an unrelated key is merged rather than rejected', async () => {
  await withDirectory(async (directory) => {
    const filePath = path.join(directory, 'config.json');
    await fs.writeFile(filePath, VALID_TEXT);
    const store = storeFor(filePath);
    await store.load();
    await fs.writeFile(
      filePath,
      JSON.stringify({
        ...JSON.parse(VALID_TEXT),
        aliases: { 'default:calculator': ['math'] },
      }),
    );

    const result = await store.persist(withHotkey('Command+Alt+J'));
    assert.equal(result.ok, true);
    const written = JSON.parse(await fs.readFile(filePath, 'utf8'));
    assert.equal(written.settings.paletteHotkey, 'Command+Alt+J');
    assert.deepEqual(written.aliases, { 'default:calculator': ['math'] });
  });
});

test('first create is exclusive and never replaces a file that appeared meanwhile', async () => {
  await withDirectory(async (directory) => {
    const filePath = path.join(directory, 'config.json');
    await fs.writeFile(
      filePath,
      VALID_TEXT.replace('Command+Alt+K', 'Command+Alt+Z'),
    );
    assert.equal(
      await createPortableConfigFileExclusively(filePath, VALID_TEXT),
      'exists',
    );
    assert.match(await fs.readFile(filePath, 'utf8'), /Command\+Alt\+Z/);
  });
});

test('a replace whose guard sees a changed file leaves the file untouched', async () => {
  await withDirectory(async (directory) => {
    const filePath = path.join(directory, 'config.json');
    await fs.writeFile(filePath, VALID_TEXT);
    const outcome = await replacePortableConfigFile(
      filePath,
      '{"version":1}',
      async () => false,
    );
    assert.equal(outcome, 'changed');
    assert.equal(await fs.readFile(filePath, 'utf8'), VALID_TEXT);
    assert.deepEqual(
      (await fs.readdir(directory)).filter((name) => name.endsWith('.tmp')),
      [],
    );
  });
});

test('creating through a symlink keeps the link and writes the target', async () => {
  await withDirectory(async (directory) => {
    const target = path.join(directory, 'dotfiles.json');
    await fs.writeFile(target, VALID_TEXT, { mode: 0o640 });
    await fs.chmod(target, 0o640);
    const originalMode = (await fs.stat(target)).mode & 0o777;
    const link = path.join(directory, 'config.json');
    await fs.symlink(target, link);
    const store = storeFor(link);
    await store.load();

    const result = await store.persist(withHotkey('Command+Alt+Q'));
    assert.equal(result.ok, true);
    assert.equal((await fs.lstat(link)).isSymbolicLink(), true);
    assert.equal((await fs.stat(target)).mode & 0o777, originalMode);
  });
});
