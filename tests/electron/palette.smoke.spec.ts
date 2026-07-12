import fs from 'node:fs/promises';
import { createRequire } from 'node:module';
import path from 'node:path';
import { test, expect } from '@playwright/test';
import {
  _electron as electron,
  type ElectronApplication,
} from 'playwright';

const root = process.cwd();
const require = createRequire(import.meta.url);
const artifactDir = process.env.NVM_TEST_ARTIFACT_DIR!;
const userDataDir = process.env.NVM_TEST_USER_DATA_DIR!;

test('searches and invokes the safe built-in action, then hides and shows', async () => {
  await fs.mkdir(artifactDir, { recursive: true });
  await fs.writeFile(
    path.join(artifactDir, 'manifest.json'),
    `${JSON.stringify({ userDataDir, entry: 'dist/main/main.js', testMode: true })}\n`,
  );
  const logPath = path.join(artifactDir, 'main-process.log');
  const log = await fs.open(logPath, 'w');
  let app: ElectronApplication | undefined;
  try {
    app = await electron.launch({
      executablePath: require('electron') as string,
      args: [
        path.join(root, 'dist/main/main.js'),
        `--user-data-dir=${userDataDir}`,
      ],
      env: { ...process.env, NVM_TEST_MODE: '1' },
      timeout: 20_000,
    });
    const childProcess = app.process();
    childProcess.stdout?.on(
      'data',
      (chunk) => void log.write(chunk).catch(() => {}),
    );
    childProcess.stderr?.on(
      'data',
      (chunk) => void log.write(chunk).catch(() => {}),
    );
    const page = await app.firstWindow();
    await expect(page.locator('input[placeholder]').first()).toBeVisible({
      timeout: 10_000,
    });
    const input = page.locator('input[placeholder]').first();
    await input.fill('Test: Confirm safe action');
    const action = page.getByText('Test: Confirm safe action', { exact: true });
    await expect(action).toBeVisible();
    await page.evaluate(() => window.nvm.testInvoke());
    await expect
      .poll(async () => {
        const events = JSON.parse(
          await fs.readFile(
            path.join(artifactDir, 'window-events.json'),
            'utf8',
          ),
        ) as string[];
        return events.includes('hidden') && events.includes('shown');
      })
      .toBe(true);
    const network = JSON.parse(
      await fs.readFile(path.join(artifactDir, 'network.json'), 'utf8'),
    );
    expect(network).toEqual([]);
    const manifest = JSON.parse(
      await fs.readFile(path.join(artifactDir, 'manifest.json'), 'utf8'),
    );
    expect(manifest.userDataDir).toBe(userDataDir);
    expect(path.resolve(userDataDir)).toContain(
      path.resolve(require('node:os').tmpdir()),
    );
  } finally {
    await log.close();
    if (app) {
      const childProcess = app.process();
      if (!childProcess.killed) childProcess.kill('SIGTERM');
      await new Promise<void>((resolve) => {
        if (childProcess.exitCode !== null) return resolve();
        const timer = setTimeout(resolve, 10_000);
        childProcess.once('exit', () => {
          clearTimeout(timer);
          resolve();
        });
      });
      if (childProcess.exitCode === null) childProcess.kill('SIGKILL');
    }
  }
});
