const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

if (process.platform === 'darwin') {
  // The dev app runs as Electron.app (com.github.Electron). If it crashes, macOS
  // can get stuck showing the "reopen windows" crash dialog before our app starts.
  spawnSync(
    'defaults',
    [
      'write',
      'com.github.Electron',
      'ApplePersistenceIgnoreState',
      '-bool',
      'true',
    ],
    {
      stdio: 'ignore',
    },
  );

  fs.rmSync(
    path.join(
      os.homedir(),
      'Library',
      'Saved Application State',
      'com.github.Electron.savedState',
    ),
    {
      recursive: true,
      force: true,
    },
  );
}
