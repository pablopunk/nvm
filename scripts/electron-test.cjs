const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

(async () => {
  const root = process.cwd();
  const userDataDir = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-electron-'));
  const artifactDir = path.join(root, 'test-results', 'electron');
  await fs.rm(artifactDir, { recursive: true, force: true });
  await fs.mkdir(artifactDir, { recursive: true });
  await fs.writeFile(
    path.join(artifactDir, 'manifest.json'),
    `${JSON.stringify({ userDataDir, entry: 'dist/main/main.js', testMode: true }, null, 2)}\n`,
  );

  const env = {
    ...process.env,
    NVM_TEST_MODE: '1',
    NVM_TEST_USER_DATA_DIR: userDataDir,
    NVM_TEST_ARTIFACT_DIR: artifactDir,
  };
  const child = spawn(
    process.execPath,
    [
      'node_modules/@playwright/test/cli.js',
      'test',
      'tests/electron/palette.smoke.spec.ts',
    ],
    { cwd: root, env, stdio: 'inherit' },
  );
  const exitCode = await new Promise((resolve) => {
    child.on('exit', (code, signal) => resolve(code ?? (signal ? 1 : 0)));
  });
  await fs.rm(userDataDir, { recursive: true, force: true });
  const cleaned = !(await fs.stat(userDataDir).catch(() => null));
  await fs.writeFile(
    path.join(artifactDir, 'manifest.json'),
    `${JSON.stringify({ userDataDir, entry: 'dist/main/main.js', testMode: true, cleaned }, null, 2)}\n`,
  );
  if (!cleaned) process.exitCode = 1;
  if (exitCode !== 0) process.exitCode = exitCode;
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
