import assert from 'node:assert/strict';
import test from 'node:test';
import { diffPortableConfig } from './portable-config-projection';
import { createDefaultPortableConfig } from './portable-config-schema';
import {
  changedKeysOf,
  diskViewOf,
  isPortableActionId,
  localBundleOf,
  mergeDiskView,
} from './portable-keys';

test('only stable extension and default action IDs are portable', () => {
  assert.equal(
    isPortableActionId('extension-action:my-tools:open-notes'),
    true,
  );
  assert.equal(isPortableActionId('extension-root:my-tools:open-notes'), true);
  assert.equal(
    isPortableActionId('extension:nevermind.system:builtin:settings'),
    true,
  );
  assert.equal(
    isPortableActionId('extension:nevermind.apps:app:/Users/example/App'),
    false,
  );
  assert.equal(isPortableActionId('default:calculator'), true);
  assert.equal(isPortableActionId('ai-chat:9f2c1e7a-chat'), false);
  assert.equal(isPortableActionId('clipboard:3f9a0b'), false);
  assert.equal(isPortableActionId('uninstall:/Applications/Demo.app'), false);
  assert.equal(isPortableActionId('extension-action:~/tools:open'), false);
  assert.equal(isPortableActionId('ai-tweak-extension:my-tools.ts'), false);
});

test('merging keeps local entries and takes everything else from disk', () => {
  const local = {
    ...createDefaultPortableConfig(),
    aliases: { 'ai-chat:c1': ['local'] },
    shortcuts: { 'ai-chat:c1': 'Command+Shift+L' },
  };
  const disk = {
    ...createDefaultPortableConfig(),
    aliases: { 'extension-action:a:b': ['disk'], 'ai-chat:c1': ['stale'] },
    shortcuts: { 'extension-action:a:b': null },
  };
  const localIds = new Set(['ai-chat:c1']);
  const merged = mergeDiskView(local, disk, localIds);
  assert.deepEqual(merged.aliases, {
    'extension-action:a:b': ['disk'],
    'ai-chat:c1': ['local'],
  });
  assert.deepEqual(merged.shortcuts, {
    'extension-action:a:b': null,
    'ai-chat:c1': 'Command+Shift+L',
  });
});

test('disk view strips local IDs so they never reach the config file', () => {
  const view = {
    ...createDefaultPortableConfig(),
    aliases: { 'extension-action:a:b': ['x'], 'ai-chat:c1': ['y'] },
  };
  const onDisk = diskViewOf(view, new Set(['ai-chat:c1']));
  assert.deepEqual(onDisk.aliases, { 'extension-action:a:b': ['x'] });
});

test('change keys name the portable section they belong to', () => {
  const base = createDefaultPortableConfig();
  const next = {
    ...base,
    settings: { paletteHotkey: 'Command+Alt+J' },
    shortcuts: { 'extension-action:a:b': null },
    jobs: { enabled: { 'cache.thumbnails': false } },
    ai: { defaultModel: 'smart' as const },
  };
  assert.deepEqual(changedKeysOf(diffPortableConfig(base, next)).sort(), [
    'job:cache.thumbnails',
    'model',
    'setting:paletteHotkey',
    'shortcut:extension-action:a:b',
  ]);
});

test('local bundle keeps only local IDs and their values', () => {
  const bundle = localBundleOf({
    localActionIds: ['ai-chat:c1'],
    aliases: { 'extension-action:a:b': ['x'], 'ai-chat:c1': ['y'] },
    shortcuts: { 'ai-chat:c1': 'Command+Shift+L' },
    removedShortcuts: { 'ai-chat:c1': true },
  });
  assert.deepEqual(bundle.localAliases, { 'ai-chat:c1': ['y'] });
  assert.deepEqual(bundle.localShortcuts, { 'ai-chat:c1': 'Command+Shift+L' });
  assert.deepEqual(bundle.localRemovedShortcuts, { 'ai-chat:c1': true });
});
