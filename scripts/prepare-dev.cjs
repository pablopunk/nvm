const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const root = path.resolve(__dirname, '..');
const backendDir = path.join(root, 'src', 'backend');
const isWindows = process.platform === 'win32';
const tsxBin = path.join(
  backendDir,
  'node_modules',
  '.bin',
  isWindows ? 'tsx.cmd' : 'tsx',
);
const migration = spawnSync(tsxBin, ['scripts/migrate.ts'], {
  cwd: backendDir,
  encoding: 'utf8',
  env: process.env,
  shell: isWindows,
});
process.stdout.write(migration.stdout || '');
process.stderr.write(migration.stderr || '');
if (migration.status !== 0) process.exit(migration.status || 1);

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
