import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { parseUpdateInfo } from 'electron-updater/out/providers/Provider.js';
import { advanceStateCompatibility } from '../state/state-compatibility';
import { isStateSafeForInstall } from './state-update-safety';

test('release metadata and both durable roots gate an older target', async (t) => {
  const root = await fs.mkdtemp(
    path.join(os.tmpdir(), 'nvm-epoch-integration-'),
  );
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const privateRoot = path.join(root, 'private');
  const sharedRoot = path.join(root, 'shared');
  const roots = [privateRoot, sharedRoot];
  await advanceStateCompatibility(privateRoot, 1, sharedRoot);
  for (const directory of roots)
    await fs.writeFile(
      path.join(directory, 'state-compatibility.json'),
      JSON.stringify({ version: 1, highestEpoch: 1, trusted: true }),
    );
  const url = new URL('https://github.com/example/releases/latest.yml');
  const info = parseUpdateInfo(
    'version: 2026.1029.1\nnevermindStateEpoch: 1\n',
    'latest.yml',
    url,
  );
  assert.equal(await isStateSafeForInstall('2026.1030.2', info, roots), true);
  const unknown = parseUpdateInfo('version: 2026.1029.1\n', 'latest.yml', url);
  assert.equal(
    await isStateSafeForInstall('2026.1030.2', unknown, roots),
    false,
  );
  await advanceStateCompatibility(privateRoot, 2, sharedRoot);
  assert.equal(await isStateSafeForInstall('2026.1030.2', info, roots), false);
  assert.equal(
    await isStateSafeForInstall('2026.1028.1', unknown, roots),
    true,
  );
});
