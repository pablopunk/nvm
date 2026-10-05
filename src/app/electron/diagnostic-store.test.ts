import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { createDiagnosticStore } from './diagnostic-store';
import { diagnosticId, failureRecord } from './observability';

test('diagnostic index bounds records and does not persist unknown fields', async () => {
  const directory = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-diagnostics-'),
  );
  try {
    const file = path.join(directory, 'diagnostics.json');
    const store = createDiagnosticStore(file);
    await store.load();
    for (let i = 0; i < 105; i++)
      store.put({
        ...failureRecord(),
        reference: diagnosticId(),
        ...{ secret: 'NEVER_PERSIST' },
      });
    await store.flush();
    assert.equal(store.recent().length, 100);
    assert.ok(!(await fs.readFile(file, 'utf8')).includes('NEVER_PERSIST'));
    const restored = createDiagnosticStore(file);
    await restored.load();
    assert.equal(restored.recent().length, 100);
    assert.equal(restored.recent()[0]?.reporting, 'local_only');
  } finally {
    await fs.rm(directory, { recursive: true, force: true });
  }
});
