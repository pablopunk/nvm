import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test, { mock } from 'node:test';
import { createExtensionDraftStore } from './extension-draft-store';
import { createExtensionJsonStore } from './extension-json-store';

mock.module('../platform/logger', { namedExports: { warn: () => {} } });
mock.module('../platform/test-mode', {
  namedExports: { isNvmTestMode: false },
});
const { createExtensionDraftPersistence } = await import(
  './extension-draft-persistence'
);

test('flush persists a debounced editor draft and retries a failed write', async (t) => {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-draft-flush-'),
  );
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const storage = createExtensionJsonStore();
  let attempts = 0;
  const persistence = createExtensionDraftPersistence({
    extensionDraftStore: createExtensionDraftStore(),
    extensionDraftRefs: new Map([
      [
        'ref',
        {
          ownerExtensionId: 'demo',
          key: 'editor',
          version: 1,
          autosaveAction: null,
        },
      ],
    ]),
    extensionJsonStore: {
      replace: (file, data) => {
        attempts += 1;
        if (attempts === 1)
          return Promise.reject(new Error('disk unavailable'));
        return storage.replace(file, data);
      },
    },
    userDataPath: () => directory,
    executeViewAction: async () => {},
  });
  assert.deepEqual(
    await persistence.saveExtensionDraftForIpc({
      ref: 'ref',
      content: 'keep this draft',
    }),
    { ok: true },
  );
  await assert.rejects(persistence.flushPendingDrafts(), /disk unavailable/);
  await persistence.flushPendingDrafts();
  const saved = JSON.parse(
    await fs.readFile(
      path.join(directory, 'extension-editor-drafts.json'),
      'utf8',
    ),
  );
  assert.equal(saved.drafts[0].content, 'keep this draft');
});
