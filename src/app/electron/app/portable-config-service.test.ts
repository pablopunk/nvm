import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import {
  createPortableConfigService,
  type PortableLive,
} from './portable-config-service';

const KINDS = { paletteHotkey: 'shortcut', startAtLogin: 'boolean' } as const;

async function withConfigFile(callback: (filePath: string) => Promise<void>) {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-portable-service-'),
  );
  try {
    await callback(path.join(directory, 'config.json'));
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
}

function emptyRecord(): Record<string, any> {
  return {
    settings: {},
    aliases: {},
    shortcuts: {},
    removedShortcuts: {},
    shortcutActions: {},
    jobSettings: { enabled: {} },
    aiChatDefaultModel: 'fast',
    localActionIds: [],
  };
}

function recordingLive(failing: Record<string, string> = {}) {
  const calls: Array<{ keys: string[]; afterPaletteHotkey?: unknown }> = [];
  const live: PortableLive = {
    apply: (_before, after, keys) => {
      calls.push({ keys, afterPaletteHotkey: after.settings.paletteHotkey });
      return keys
        .filter((key) => key in failing)
        .map((key) => ({ key, message: failing[key] }));
    },
    applyShortcuts: () => [],
  };
  return { calls, live };
}

function serviceFor(
  filePath: string,
  record: Record<string, any>,
  live: PortableLive,
) {
  let saves = 0;
  const service = createPortableConfigService({
    filePath: () => filePath,
    settingKinds: KINDS,
    userStateStore: { get: () => record },
    live,
    scheduleSaveState: () => {
      saves += 1;
    },
    pollMs: 60_000,
  });
  return { service, saveCount: () => saves };
}

test('shortcut preparation is available during live apply and rolled back on failure', async () => {
  await withConfigFile(async (filePath) => {
    const record = emptyRecord();
    const actionId = 'default:calculator';
    const live: PortableLive = {
      apply: (_before, _after, keys) => {
        if (
          keys.includes(`shortcut:${actionId}`) &&
          record.shortcutActions[actionId]
        )
          return [{ key: `shortcut:${actionId}`, message: 'in use' }];
        return [];
      },
      applyShortcuts: () => [],
    };
    const { service } = serviceFor(filePath, record, live);
    await service.loadPortableConfig();
    const result = await service.commit({
      label: 'shortcut:set',
      actionId,
      mutate: (draft) => {
        draft.shortcuts[actionId] = 'Command+K';
      },
      beforeApply: () => {
        record.shortcutActions[actionId] = { id: actionId };
        return () => {
          delete record.shortcutActions[actionId];
        };
      },
    });
    assert.equal(result.ok, false);
    assert.equal(record.shortcutActions[actionId], undefined);
  });
});

test('a portable commit is persisted before it reports success and updates the record', async () => {
  await withConfigFile(async (filePath) => {
    const record = emptyRecord();
    const { live } = recordingLive();
    const { service, saveCount } = serviceFor(filePath, record, live);
    await service.loadPortableConfig();

    const result = await service.commit({
      label: 'alias:add',
      actionId: 'extension-action:my-tools:open-notes',
      mutate: (draft) => {
        draft.aliases['extension-action:my-tools:open-notes'] = ['notes'];
      },
    });

    assert.deepEqual(result, { ok: true });
    const written = JSON.parse(await fs.readFile(filePath, 'utf8'));
    assert.deepEqual(written.aliases, {
      'extension-action:my-tools:open-notes': ['notes'],
    });
    assert.deepEqual(record.aliases, {
      'extension-action:my-tools:open-notes': ['notes'],
    });
    assert.equal(saveCount(), 1);
  });
});

test('a live failure on a changed key rejects the commit, writes nothing, and rolls back', async () => {
  await withConfigFile(async (filePath) => {
    const record = emptyRecord();
    const { calls, live } = recordingLive({
      'setting:paletteHotkey': 'Could not register Command+Alt+J',
    });
    const { service } = serviceFor(filePath, record, live);
    await service.loadPortableConfig();

    const result = await service.commit({
      label: 'palette-hotkey',
      mutate: (draft) => {
        draft.settings.paletteHotkey = 'Command+Alt+J';
      },
    });

    assert.deepEqual(result, {
      ok: false,
      message: 'Could not register Command+Alt+J',
    });
    await assert.rejects(fs.access(filePath));
    assert.equal(record.settings.paletteHotkey, undefined);
    assert.equal(calls.at(-1)?.afterPaletteHotkey, undefined);
  });
});

test('a local-only action key stays out of config.json and is recorded as local', async () => {
  await withConfigFile(async (filePath) => {
    const record = emptyRecord();
    const { live } = recordingLive();
    const { service, saveCount } = serviceFor(filePath, record, live);
    await service.loadPortableConfig();

    const result = await service.commit({
      label: 'alias:generated',
      actionId: 'extension-root:ai-tools:open',
      actionIsGenerated: true,
      mutate: (draft) => {
        draft.aliases['extension-root:ai-tools:open'] = ['generated'];
      },
    });

    assert.deepEqual(result, { ok: true });
    await assert.rejects(fs.access(filePath));
    assert.deepEqual(record.localActionIds, ['extension-root:ai-tools:open']);
    assert.deepEqual(record.aliases, {
      'extension-root:ai-tools:open': ['generated'],
    });
    assert.equal(saveCount(), 1);
  });
});

test('a same-key edit made on disk after load rejects the commit and adopts the disk value', async () => {
  await withConfigFile(async (filePath) => {
    await fs.writeFile(
      filePath,
      JSON.stringify({
        version: 1,
        settings: { paletteHotkey: 'Command+Alt+K' },
      }),
    );
    const record = emptyRecord();
    const { live } = recordingLive();
    const { service } = serviceFor(filePath, record, live);
    await service.loadPortableConfig();
    await fs.writeFile(
      filePath,
      JSON.stringify({
        version: 1,
        settings: { paletteHotkey: 'Command+Alt+B' },
      }),
    );

    const result = await service.commit({
      label: 'palette-hotkey',
      mutate: (draft) => {
        draft.settings.paletteHotkey = 'Command+Alt+C';
      },
    });

    assert.equal(result.ok, false);
    assert.equal(
      JSON.parse(await fs.readFile(filePath, 'utf8')).settings.paletteHotkey,
      'Command+Alt+B',
    );
    assert.equal(record.settings.paletteHotkey, 'Command+Alt+B');
  });
});

test('unresolved shortcut IDs survive a later portable commit', async () => {
  await withConfigFile(async (filePath) => {
    await fs.writeFile(
      filePath,
      JSON.stringify({
        version: 1,
        shortcuts: { 'extension-action:uninstalled:thing': 'Command+Shift+U' },
      }),
    );
    const record = emptyRecord();
    const { live } = recordingLive();
    const { service } = serviceFor(filePath, record, live);
    await service.loadPortableConfig();

    await service.commit({
      label: 'alias:add',
      actionId: 'extension-action:my-tools:open-notes',
      mutate: (draft) => {
        draft.aliases['extension-action:my-tools:open-notes'] = ['notes'];
      },
    });

    const written = JSON.parse(await fs.readFile(filePath, 'utf8'));
    assert.equal(
      written.shortcuts['extension-action:uninstalled:thing'],
      'Command+Shift+U',
    );
  });
});

test('an explicit startAtLogin in config is applied at boot; an absent one is not', async () => {
  await withConfigFile(async (filePath) => {
    await fs.writeFile(
      filePath,
      JSON.stringify({ version: 1, settings: { startAtLogin: true } }),
    );
    const explicit = recordingLive();
    await serviceFor(
      filePath,
      emptyRecord(),
      explicit.live,
    ).service.loadPortableConfig();
    assert.deepEqual(explicit.calls.at(-1)?.keys, ['setting:startAtLogin']);

    await fs.writeFile(filePath, JSON.stringify({ version: 1 }));
    const absent = recordingLive();
    await serviceFor(
      filePath,
      emptyRecord(),
      absent.live,
    ).service.loadPortableConfig();
    assert.equal(absent.calls.length, 0);
  });
});
