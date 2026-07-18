import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { pathToFileURL } from 'node:url';
import {
  createStandaloneExtensionFork,
  inspectExtensionManifest,
} from './extension-manifest';

const GREETING_PATTERN = /const greeting = 'Hello'/;
const FORK_SOURCE_PATTERN = /const __nvmForkSource: any = \{/;
const FORK_ID_PATTERN = /id: __nvmForkExtensionId/;
const FORK_TITLE_PATTERN = /title: "Copy of Demo"/;
const FORK_ACTION_ID_PATTERN = /__nvmForkExtensionId \+ ':' \+ actionId/;
const ORIGINAL_IMPORT_PATTERN = /from '\.\/demo\.ts'/;
const NON_OBJECT_FORK_PATTERN = /Only object-literal extensions can be forked/;

test('inspects literal canonical capabilities without importing source', () => {
  const manifest = inspectExtensionManifest(`
    globalThis.__extensionImported = true;
    export default ({ id: 'demo', title: 'Demo', capabilities: ['system'], actions(ctx) { return [] }, extra: call() } satisfies NevermindExtension);
  `);
  const { idStart: _idStart, idEnd: _idEnd, ...details } = manifest;
  assert.deepEqual(details, {
    id: 'demo',
    title: 'Demo',
    capabilities: ['system'],
    provenance: 'capabilities',
    dynamic: false,
  });
  assert.equal(
    (globalThis as { __extensionImported?: boolean }).__extensionImported,
    undefined,
  );
});

test('uses legacy permissions only when canonical capabilities are absent', () => {
  const manifest = inspectExtensionManifest(
    "export default { id: 'demo', title: 'Demo', capabilities: dynamic, permissions: ['system'] }",
  );
  const { idStart: _idStart, idEnd: _idEnd, ...details } = manifest;
  assert.deepEqual(details, {
    id: 'demo',
    title: 'Demo',
    capabilities: [],
    provenance: 'capabilities',
    dynamic: true,
  });
});

test('materializes a standalone fork without importing the original extension', () => {
  const fork = createStandaloneExtensionFork(
    `
      import type { NevermindExtension } from './nevermind-extension-api';
      const greeting = 'Hello';
      export default {
        id: 'demo',
        title: 'Demo',
        commands: [{ id: 'hello', title: greeting, actionId: 'hello-action', run() {} }],
      } satisfies NevermindExtension;
    `,
    { id: 'demo-copy', title: 'Copy of Demo' },
  );

  assert.match(fork, GREETING_PATTERN);
  assert.match(fork, FORK_SOURCE_PATTERN);
  assert.match(fork, FORK_ID_PATTERN);
  assert.match(fork, FORK_TITLE_PATTERN);
  assert.match(fork, FORK_ACTION_ID_PATTERN);
  assert.doesNotMatch(fork, ORIGINAL_IMPORT_PATTERN);
});

test('loads a standalone fork with its own identity and action ids', async () => {
  const fork = createStandaloneExtensionFork(
    `
      const greeting = 'Hello';
      export default {
        id: 'demo',
        title: 'Demo',
        commands: [{ id: 'hello', title: greeting, actionId: 'hello-action', run() {} }],
      };
    `,
    { id: 'demo-copy', title: 'Copy of Demo' },
  );
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-fork-'));
  const filePath = path.join(directory, 'fork.ts');
  try {
    await fs.writeFile(filePath, fork);
    const module = await import(
      `${pathToFileURL(filePath).href}?fork=${Date.now()}`
    );
    assert.equal(module.default.id, 'demo-copy');
    assert.equal(module.default.title, 'Copy of Demo');
    assert.equal(module.default.commands[0].actionId, 'demo-copy:hello-action');
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});

test('rejects a non-object extension export when forking', () => {
  assert.throws(
    () =>
      createStandaloneExtensionFork('export default createExtension()', {
        id: 'copy',
        title: 'Copy',
      }),
    NON_OBJECT_FORK_PATTERN,
  );
});
