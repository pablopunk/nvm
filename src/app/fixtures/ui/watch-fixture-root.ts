import os from 'node:os';
import path from 'node:path';

export const WATCH_FIXTURE_ROOT = path.join(
  os.tmpdir(),
  'nvm-extension-watch-fixture',
);
