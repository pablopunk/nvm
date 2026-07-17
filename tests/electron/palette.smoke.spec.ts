import fs from 'node:fs/promises';
import { execFile as execFileCallback } from 'node:child_process';
import { createRequire } from 'node:module';
import path from 'node:path';
import { promisify } from 'node:util';
import { test, expect } from '@playwright/test';
import { _electron as electron, type ElectronApplication } from 'playwright';

const root = process.cwd();
const require = createRequire(import.meta.url);
const artifactDir = process.env.NVM_TEST_ARTIFACT_DIR!;
const userDataDir = process.env.NVM_TEST_USER_DATA_DIR!;
const execFile = promisify(execFileCallback);

async function processTree(rootPid: number) {
  if (process.platform === 'win32') return [rootPid];
  const { stdout } = await execFile('ps', ['-axo', 'pid=,ppid=']);
  const children = new Map<number, number[]>();
  for (const line of stdout.split('\n')) {
    const [pidText, parentText] = line.trim().split(/\s+/);
    const pid = Number(pidText);
    const parent = Number(parentText);
    if (!Number.isInteger(pid) || !Number.isInteger(parent)) continue;
    const siblings = children.get(parent) || [];
    siblings.push(pid);
    children.set(parent, siblings);
  }
  const tracked = new Set<number>([rootPid]);
  const pending = [rootPid];
  while (pending.length) {
    const parent = pending.pop()!;
    for (const child of children.get(parent) || []) {
      if (tracked.has(child)) continue;
      tracked.add(child);
      pending.push(child);
    }
  }
  return [...tracked].sort((a, b) => a - b);
}

function isProcessAlive(pid: number) {
  try {
    process.kill(pid, 0);
    return true;
  } catch (error: any) {
    return error?.code === 'EPERM';
  }
}

async function waitForProcessesToExit(pids: number[], timeoutMs: number) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const alive = pids.filter(isProcessAlive);
    if (!alive.length) return [];
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  return pids.filter(isProcessAlive);
}

function terminateTrackedProcesses(pids: number[], signal: NodeJS.Signals) {
  for (const pid of [...pids].sort((a, b) => b - a)) {
    try {
      process.kill(pid, signal);
    } catch (error: any) {
      if (!['ESRCH', 'EPERM'].includes(error?.code)) throw error;
    }
  }
}

async function updateManifest(patch: Record<string, unknown>) {
  const manifestPath = path.join(artifactDir, 'manifest.json');
  let manifest: Record<string, unknown> = {};
  try {
    manifest = JSON.parse(await fs.readFile(manifestPath, 'utf8'));
  } catch {}
  await fs.writeFile(
    manifestPath,
    `${JSON.stringify({ ...manifest, ...patch }, null, 2)}\n`,
  );
}

test('searches and invokes the safe built-in action, then hides and shows', async () => {
  await fs.mkdir(artifactDir, { recursive: true });
  await fs.writeFile(
    path.join(artifactDir, 'manifest.json'),
    `${JSON.stringify({ userDataDir, entry: 'dist/main/main.js', testMode: true })}\n`,
  );
  const logPath = path.join(artifactDir, 'main-process.log');
  const log = await fs.open(logPath, 'w');
  let app: ElectronApplication | undefined;
  let trackedPids: number[] = [];
  let cleanupError: Error | undefined;
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
    trackedPids = await processTree(childProcess.pid);
    await updateManifest({ launchedPid: childProcess.pid, trackedPids });
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
    if (process.platform === 'linux') {
      await input.fill('Open Settings');
      await expect(
        page.getByText('Open Settings', { exact: true }),
      ).toBeVisible();
      await expect(
        page.getByText('Open System Settings', { exact: true }),
      ).toHaveCount(0);
      await expect(
        page.getByText('Start at Login', { exact: true }),
      ).toHaveCount(0);
    }
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
      const closePromise = app.close().catch(() => {});
      const closeCompleted = await Promise.race([
        closePromise.then(() => true),
        new Promise<boolean>((resolve) =>
          setTimeout(() => resolve(false), 5_000),
        ),
      ]);
      const survivorsAfterClose = await waitForProcessesToExit(
        trackedPids,
        1_000,
      );
      let fallbackUsed = false;
      let survivorsAfterFallback = survivorsAfterClose;
      if (survivorsAfterClose.length) {
        fallbackUsed = true;
        terminateTrackedProcesses(survivorsAfterClose, 'SIGTERM');
        survivorsAfterFallback = await waitForProcessesToExit(
          survivorsAfterClose,
          2_000,
        );
        if (survivorsAfterFallback.length) {
          terminateTrackedProcesses(survivorsAfterFallback, 'SIGKILL');
          survivorsAfterFallback = await waitForProcessesToExit(
            survivorsAfterFallback,
            1_000,
          );
        }
      }
      await updateManifest({
        processCleanup: {
          closeCompleted,
          trackedPids,
          survivorsAfterClose,
          fallbackUsed,
          survivorsAfterFallback,
          passed: survivorsAfterFallback.length === 0,
        },
      });
      if (survivorsAfterFallback.length)
        cleanupError = new Error(
          `Tracked Electron processes survived teardown: ${survivorsAfterFallback.join(', ')}`,
        );
    }
  }
  if (cleanupError) throw cleanupError;
});
