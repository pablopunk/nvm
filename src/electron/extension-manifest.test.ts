import assert from 'node:assert/strict';
import test from 'node:test';
import { inspectExtensionManifest } from './extension-manifest';

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
