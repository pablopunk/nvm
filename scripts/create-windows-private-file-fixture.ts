import fs from 'node:fs/promises';
import path from 'node:path';
import { writePrivateFile } from '../src/electron/private-file';

const fixtureRoot = process.argv[2];
if (!fixtureRoot) throw new Error('Fixture root is required');
await fs.mkdir(fixtureRoot, { recursive: true });
await Promise.all([
  writePrivateFile(
    path.join(fixtureRoot, 'nevermind-auth-by-origin.json'),
    '{"token":"redacted"}\n',
    { processPlatform: 'win32' },
  ),
  writePrivateFile(
    path.join(fixtureRoot, 'byo-key.json'),
    '{"key":"redacted"}\n',
    {
      processPlatform: 'win32',
    },
  ),
]);
