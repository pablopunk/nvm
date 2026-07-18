// biome-ignore-all lint: This end-to-end harness intentionally uses imperative process control, injected environment names, and bounded polling.
import { execFile as execFileCallback } from 'node:child_process';
import fs from 'node:fs/promises';
import { createRequire } from 'node:module';
import path from 'node:path';
import { promisify } from 'node:util';
import { expect, test } from '@playwright/test';
import { type ElectronApplication, _electron as electron } from 'playwright';

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
    if (!(Number.isInteger(pid) && Number.isInteger(parent))) continue;
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

function lifecycleExtensionSource(markerPath: string, version = 'v1') {
  const apiPath = `${markerPath}.apis.json`;
  const directMarkerPath = `${markerPath}.direct.txt`;
  const directScript = `printf '${version}\\n' >> ${JSON.stringify(directMarkerPath)}`;
  return `import { appendFile, writeFile } from 'node:fs/promises';

export default {
  id: 'pab53.lifecycle',
  title: 'PAB-53 Lifecycle',
  capabilities: [],
  commands: [
    {
      id: 'command',
      title: 'PAB-53 Lifecycle ${version}',
      mode: 'background',
      triggers: [{ type: 'startup' }],
      run: async (ctx) => {
        if (ctx.launch) {
          await writeFile(${JSON.stringify(apiPath)}, JSON.stringify({
            ai: typeof ctx.ai === 'function',
            attachments: typeof ctx.ai?.attachments?.file === 'function',
            clipboardHistory: typeof ctx.clipboard?.history?.list === 'function',
            ocr: typeof ctx.ocr?.image === 'function',
            desktopApps: typeof ctx.desktop?.apps?.list === 'function',
            desktopFiles: typeof ctx.desktop?.files?.find === 'function',
            desktopClipboard: typeof ctx.desktop?.clipboard?.readText === 'function',
            desktopShell: typeof ctx.desktop?.shell?.exec === 'function',
            actionShell: typeof ctx.actions?.shellExec === 'function',
            actionSystem: typeof ctx.actions?.system?.lockScreen === 'function',
            actionUpdates: typeof ctx.actions?.updates?.check === 'function',
            settings: typeof ctx.settings?.set === 'function',
            shortcuts: typeof ctx.shortcuts?.list === 'function',
            ownership: typeof ctx.extensions?.ownership?.ownerOf === 'function',
            updates: typeof ctx.updates?.getState === 'function',
          }));
          await appendFile(${JSON.stringify(markerPath)}, '${version}\\n');
        }
        return ctx.ui.list({
          id: 'pab53-lifecycle-${version}',
          title: 'PAB-53 Lifecycle ${version}',
          items: [ctx.ui.item({
            id: 'direct-${version}',
            title: 'Direct action ${version}',
            primaryAction: ctx.actions.shellScript(
              'Run direct ${version}',
              ${JSON.stringify(directScript)},
            ),
          })],
        });
      },
    },
  ],
};
`;
}

function migrationExtensionSource(
  id: string,
  title: string,
  importMarkerPath: string,
) {
  return `import { appendFile } from 'node:fs/promises';

await appendFile(${JSON.stringify(importMarkerPath)}, ${JSON.stringify(`${id}\n`)});

export default {
  id: ${JSON.stringify(id)},
  title: ${JSON.stringify(title)},
  commands: [{ id: 'open', title: ${JSON.stringify(title)}, run: () => undefined }],
};
`;
}

function actionNamed(view: any, title: string) {
  for (const item of view?.items || []) {
    for (const section of item.actionPanel?.sections || []) {
      const action = section.actions?.find(
        (candidate: any) => candidate.title === title,
      );
      if (action) return action;
    }
  }
  throw new Error(`Missing ${title} action in Extensions view`);
}

async function searchTitles(page: any, query: string) {
  return page.evaluate(
    async (value: string) =>
      (await window.nvm.search(value)).map((action) => action.title),
    query,
  );
}

async function openExtensionsView(page: any) {
  return page.evaluate(async () => {
    const actions = await window.nvm.search('Extensions');
    const extensions = actions.find(
      (action) =>
        action.extensionId === 'nevermind.extensions' &&
        action.commandId === 'extensions',
    );
    if (!extensions) throw new Error('Extensions command not found');
    const result = await window.nvm.execute(extensions);
    if (!result?.view) throw new Error('Extensions view did not open');
    return result.view;
  });
}

async function lifecyclePaletteAction(page: any, version: string) {
  return page.evaluate(async (expectedVersion: string) => {
    const title = `PAB-53 Lifecycle ${expectedVersion}`;
    const actions = await window.nvm.search(title);
    const action = actions.find((candidate) => candidate.title === title);
    if (!action) throw new Error(`${title} not found`);
    return action;
  }, version);
}

async function renderLifecycleDirectAction(page: any, version: string) {
  const action = await lifecyclePaletteAction(page, version);
  return page.evaluate(async (paletteAction) => {
    const result = await window.nvm.execute(paletteAction);
    const directAction = result.view?.items?.[0]?.primaryAction;
    if (!directAction) throw new Error('Direct action was not rendered');
    return directAction;
  }, action);
}

async function launchTestApplication(testUserDataDir: string) {
  const app = await electron.launch({
    executablePath: require('electron') as string,
    args: [
      path.join(root, 'dist/main/main.js'),
      `--user-data-dir=${testUserDataDir}`,
    ],
    env: {
      ...process.env,
      NVM_TEST_MODE: '1',
      NVM_TEST_USER_DATA_DIR: testUserDataDir,
    },
    timeout: 20_000,
  });
  const trackedPids = await processTree(app.process().pid);
  const page = await app.firstWindow();
  await expect(page.locator('input[placeholder]').first()).toBeVisible({
    timeout: 10_000,
  });
  return { app, page, trackedPids };
}

async function closeTestApplication(
  app: ElectronApplication,
  trackedPids: number[],
) {
  const closePromise = app.close().catch(() => {});
  await Promise.race([
    closePromise,
    new Promise((resolve) => setTimeout(resolve, 2000)),
  ]);
  const survivors = await waitForProcessesToExit(trackedPids, 500);
  if (survivors.length) terminateTrackedProcesses(survivors, 'SIGKILL');
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
      env: {
        ...process.env,
        NVM_TEST_MODE: '1',
        NVM_TEST_USER_DATA_DIR: userDataDir,
      },
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
      await page.screenshot({
        path: path.join(artifactDir, 'linux-palette.png'),
      });
      await input.fill('Open System Settings');
      await expect(
        page.getByText('Open System Settings', { exact: true }),
      ).toHaveCount(0);
      await input.fill('Start at Login');
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
          setTimeout(() => resolve(false), 5000),
        ),
      ]);
      const survivorsAfterClose = await waitForProcessesToExit(
        trackedPids,
        1000,
      );
      let fallbackUsed = false;
      let survivorsAfterFallback = survivorsAfterClose;
      if (survivorsAfterClose.length) {
        fallbackUsed = true;
        terminateTrackedProcesses(survivorsAfterClose, 'SIGTERM');
        survivorsAfterFallback = await waitForProcessesToExit(
          survivorsAfterClose,
          2000,
        );
        if (survivorsAfterFallback.length) {
          terminateTrackedProcesses(survivorsAfterFallback, 'SIGKILL');
          survivorsAfterFallback = await waitForProcessesToExit(
            survivorsAfterFallback,
            1000,
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

test('dismisses transient alias UI and flushes scheduled state before quit', async () => {
  const stateSafetyUserDataDir = path.join(userDataDir, 'pab4-state-safety');
  const settingsTitle =
    process.platform === 'darwin' ? 'Open System Settings' : 'Open Settings';
  let firstLaunch:
    | Awaited<ReturnType<typeof launchTestApplication>>
    | undefined;
  let firstLaunchExited = false;
  let relaunch: Awaited<ReturnType<typeof launchTestApplication>> | undefined;

  try {
    firstLaunch = await launchTestApplication(stateSafetyUserDataDir);
    const input = firstLaunch.page.locator('input[placeholder]').first();
    await input.fill(settingsTitle);
    const settingsRow = firstLaunch.page
      .getByText(settingsTitle, { exact: true })
      .first()
      .locator('xpath=ancestor::*[@cmdk-item]');
    await expect(settingsRow).toBeVisible();
    await settingsRow.hover();
    await expect(settingsRow).toHaveAttribute('data-selected', 'true');

    await firstLaunch.page.keyboard.press('Control+K');
    const setAlias = firstLaunch.page.getByText('Set alias', { exact: true });
    await expect(setAlias).toBeVisible();
    await setAlias.click();
    const aliasInput = firstLaunch.page.locator(
      'input[placeholder^="Alias for"]',
    );
    await expect(aliasInput).toBeVisible();

    await firstLaunch.page.evaluate(() => window.nvm.hide());
    await expect(aliasInput).toHaveCount(0);
    await expect(input).toBeVisible();

    const scheduledShortcut = await firstLaunch.page.evaluate(async (title) => {
      const actions = await window.nvm.search(title);
      const action = actions.find((candidate) => candidate.title === title);
      if (!action) throw new Error('System Settings action not found');
      const result = await window.nvm.setShortcut(
        action,
        'CommandOrControl+Alt+Shift+8',
      );
      if (!result.ok) throw new Error(result.message);
      const record = (await window.nvm.getShortcuts()).find(
        (candidate) => candidate.actionId === action.id,
      );
      if (!record) throw new Error('Scheduled shortcut not found');
      return { actionId: action.id, accelerator: record.accelerator };
    }, settingsTitle);

    await firstLaunch.page
      .evaluate(() => window.nvm.quitApp())
      .catch(() => undefined);
    const survivors = await waitForProcessesToExit(
      firstLaunch.trackedPids,
      8000,
    );
    expect(survivors).toEqual([]);
    firstLaunchExited = true;

    const persistedState = JSON.parse(
      await fs.readFile(
        path.join(stateSafetyUserDataDir, 'state.json'),
        'utf8',
      ),
    );
    expect(persistedState.shortcuts[scheduledShortcut.actionId]).toBe(
      scheduledShortcut.accelerator,
    );

    relaunch = await launchTestApplication(stateSafetyUserDataDir);
    await expect
      .poll(() => relaunch?.page.evaluate(() => window.nvm.getShortcuts()))
      .toContainEqual(
        expect.objectContaining({
          actionId: scheduledShortcut.actionId,
          accelerator: scheduledShortcut.accelerator,
        }),
      );
  } finally {
    if (firstLaunch && !firstLaunchExited) {
      await closeTestApplication(firstLaunch.app, firstLaunch.trackedPids);
    }
    if (relaunch) {
      await closeTestApplication(relaunch.app, relaunch.trackedPids);
    }
  }
});

test('proposal activation, rollback, disable, and re-enable are transactional', async () => {
  const lifecycleUserDataDir = path.join(userDataDir, 'pab53-lifecycle');
  const extensionsDir = path.join(lifecycleUserDataDir, 'extensions');
  const draftsDir = path.join(lifecycleUserDataDir, 'extension-drafts');
  const filename = 'pab53-lifecycle.ts';
  const draftFile = path.join(draftsDir, filename);
  const markerPath = path.join(lifecycleUserDataDir, 'trigger-runs.txt');
  const directMarkerPath = `${markerPath}.direct.txt`;
  const actionShortcut = 'CommandOrControl+Alt+9';
  const source = lifecycleExtensionSource(markerPath);
  await fs.mkdir(extensionsDir, { recursive: true });
  await fs.mkdir(draftsDir, { recursive: true });
  await fs.writeFile(draftFile, source);
  await fs.writeFile(
    path.join(lifecycleUserDataDir, 'state.json'),
    `${JSON.stringify(
      {
        extensionManager: {
          schemaVersion: 1,
          files: {},
          proposals: {
            [filename]: {
              draftFile,
              provenance: 'ai',
              updatedAt: Date.now(),
            },
          },
        },
      },
      null,
      2,
    )}\n`,
  );

  let app: ElectronApplication | undefined;
  let trackedPids: number[] = [];
  try {
    app = await electron.launch({
      executablePath: require('electron') as string,
      args: [
        path.join(root, 'dist/main/main.js'),
        `--user-data-dir=${lifecycleUserDataDir}`,
      ],
      env: {
        ...process.env,
        NVM_TEST_MODE: '1',
        NVM_TEST_USER_DATA_DIR: lifecycleUserDataDir,
      },
      timeout: 20_000,
    });
    trackedPids = await processTree(app.process().pid);
    const page = await app.firstWindow();
    await expect(page.locator('input[placeholder]').first()).toBeVisible({
      timeout: 10_000,
    });

    expect(await searchTitles(page, 'PAB-53 Lifecycle v1')).not.toContain(
      'PAB-53 Lifecycle v1',
    );
    await expect.poll(() => fs.stat(markerPath).catch(() => null)).toBeNull();

    let extensionsView = await openExtensionsView(page);
    await fs.writeFile(
      path.join(artifactDir, 'extension-lifecycle-view.json'),
      `${JSON.stringify(extensionsView, null, 2)}\n`,
    );
    const enable = actionNamed(extensionsView, 'Enable');
    extensionsView = await page.evaluate(
      async (action) => (await window.nvm.runViewAction(action)).view,
      enable,
    );
    await expect
      .poll(() => searchTitles(page, 'PAB-53 Lifecycle v1'))
      .toContain('PAB-53 Lifecycle v1');
    await expect
      .poll(() => fs.readFile(markerPath, 'utf8').catch(() => ''))
      .toBe('v1\n');
    await expect
      .poll(async () => {
        const value = await fs
          .readFile(`${markerPath}.apis.json`, 'utf8')
          .catch(() => '{}');
        return Object.values(JSON.parse(value)).every(Boolean);
      })
      .toBe(true);

    const v1PaletteAction = await lifecyclePaletteAction(page, 'v1');
    expect(
      await page.evaluate(
        ([action, shortcut]) => window.nvm.setShortcut(action, shortcut),
        [v1PaletteAction, actionShortcut] as const,
      ),
    ).toMatchObject({ ok: true });
    await expect
      .poll(() =>
        page.evaluate(
          (shortcut) => window.nvm.testIsActionShortcutRegistered(shortcut),
          actionShortcut,
        ),
      )
      .toEqual({ registered: true });
    expect(
      (await page.evaluate(() => window.nvm.getShortcuts())).map(
        (record) => record.action.title,
      ),
    ).toContain('PAB-53 Lifecycle v1');
    const v1DirectAction = await renderLifecycleDirectAction(page, 'v1');

    const updateSource = lifecycleExtensionSource(markerPath, 'v2');
    await page.evaluate(
      ([name, proposal]) =>
        window.nvm.testStageExtensionProposal(name, proposal),
      [filename, updateSource] as const,
    );
    await page.evaluate(() =>
      window.nvm.testFailNextExtensionActivation('state-persist'),
    );
    extensionsView = await openExtensionsView(page);
    let apply = actionNamed(extensionsView, 'Apply Update');
    await page.evaluate(
      async (action) => window.nvm.runViewAction(action),
      apply,
    );
    expect(await searchTitles(page, 'PAB-53 Lifecycle v1')).toContain(
      'PAB-53 Lifecycle v1',
    );
    expect(await searchTitles(page, 'PAB-53 Lifecycle v2')).not.toContain(
      'PAB-53 Lifecycle v2',
    );
    expect(await fs.readFile(path.join(extensionsDir, filename), 'utf8')).toBe(
      source,
    );
    let rolledBackState = JSON.parse(
      await fs.readFile(path.join(lifecycleUserDataDir, 'state.json'), 'utf8'),
    );
    expect(rolledBackState.extensionManager.files[filename]).toEqual({
      enabled: true,
    });
    expect(rolledBackState.extensionManager.proposals[filename]).toBeTruthy();
    expect(
      await page.evaluate(() =>
        window.nvm.testRunJob('extension.pab53.lifecycle.command'),
      ),
    ).toEqual({ found: true });
    await expect
      .poll(() => fs.readFile(markerPath, 'utf8').catch(() => ''))
      .toBe('v1\nv1\n');
    expect(
      await page.evaluate(
        (shortcut) => window.nvm.testIsActionShortcutRegistered(shortcut),
        actionShortcut,
      ),
    ).toEqual({ registered: true });
    expect(
      (await page.evaluate(() => window.nvm.getShortcuts())).map(
        (record) => record.action.title,
      ),
    ).toContain('PAB-53 Lifecycle v1');

    await page.evaluate(() =>
      window.nvm.testFailNextExtensionActivation('runtime-commit'),
    );
    extensionsView = await openExtensionsView(page);
    apply = actionNamed(extensionsView, 'Apply Update');
    await page.evaluate(
      async (action) => window.nvm.runViewAction(action),
      apply,
    );
    expect(await searchTitles(page, 'PAB-53 Lifecycle v1')).toContain(
      'PAB-53 Lifecycle v1',
    );
    expect(await searchTitles(page, 'PAB-53 Lifecycle v2')).not.toContain(
      'PAB-53 Lifecycle v2',
    );
    expect(await fs.readFile(path.join(extensionsDir, filename), 'utf8')).toBe(
      source,
    );
    rolledBackState = JSON.parse(
      await fs.readFile(path.join(lifecycleUserDataDir, 'state.json'), 'utf8'),
    );
    expect(rolledBackState.extensionManager.files[filename]).toEqual({
      enabled: true,
    });
    expect(rolledBackState.extensionManager.proposals[filename]).toBeTruthy();
    expect(
      await page.evaluate(() =>
        window.nvm.testRunJob('extension.pab53.lifecycle.command'),
      ),
    ).toEqual({ found: true });
    await expect
      .poll(() => fs.readFile(markerPath, 'utf8').catch(() => ''))
      .toBe('v1\nv1\nv1\n');
    expect(
      await page.evaluate(
        (shortcut) => window.nvm.testIsActionShortcutRegistered(shortcut),
        actionShortcut,
      ),
    ).toEqual({ registered: true });
    expect(
      (await page.evaluate(() => window.nvm.getShortcuts())).map(
        (record) => record.action.title,
      ),
    ).toContain('PAB-53 Lifecycle v1');
    await page.evaluate(
      async (action) => window.nvm.runViewAction(action),
      v1DirectAction,
    );
    await expect
      .poll(() => fs.readFile(directMarkerPath, 'utf8').catch(() => ''))
      .toBe('v1\n');

    extensionsView = await openExtensionsView(page);
    apply = actionNamed(extensionsView, 'Apply Update');
    await page.evaluate(
      async (action) => window.nvm.runViewAction(action),
      apply,
    );
    await expect
      .poll(() => searchTitles(page, 'PAB-53 Lifecycle v2'))
      .toContain('PAB-53 Lifecycle v2');
    expect(await searchTitles(page, 'PAB-53 Lifecycle v1')).not.toContain(
      'PAB-53 Lifecycle v1',
    );
    await expect
      .poll(() => fs.readFile(markerPath, 'utf8').catch(() => ''))
      .toBe('v1\nv1\nv1\nv2\n');
    expect(
      await page.evaluate(
        (shortcut) => window.nvm.testIsActionShortcutRegistered(shortcut),
        actionShortcut,
      ),
    ).toEqual({ registered: true });
    expect(
      (await page.evaluate(() => window.nvm.getShortcuts())).map(
        (record) => record.action.title,
      ),
    ).toContain('PAB-53 Lifecycle v2');
    expect(
      await page.evaluate(
        async (action) => window.nvm.runViewAction(action),
        v1DirectAction,
      ),
    ).toMatchObject({ view: { id: 'action-failed' } });
    expect(await fs.readFile(directMarkerPath, 'utf8')).toBe('v1\n');
    const v2DirectAction = await renderLifecycleDirectAction(page, 'v2');
    await page.evaluate(
      async (action) => window.nvm.runViewAction(action),
      v2DirectAction,
    );
    await expect
      .poll(() => fs.readFile(directMarkerPath, 'utf8').catch(() => ''))
      .toBe('v1\nv2\n');

    extensionsView = await openExtensionsView(page);
    const disable = actionNamed(extensionsView, 'Disable');
    await page.evaluate(
      async (action) => window.nvm.runViewAction(action),
      disable,
    );
    expect(await searchTitles(page, 'PAB-53 Lifecycle v2')).not.toContain(
      'PAB-53 Lifecycle v2',
    );
    expect(
      await page.evaluate(() =>
        window.nvm.testRunJob('extension.pab53.lifecycle.command'),
      ),
    ).toEqual({ found: false });
    expect(
      await page.evaluate(
        (shortcut) => window.nvm.testIsActionShortcutRegistered(shortcut),
        actionShortcut,
      ),
    ).toEqual({ registered: false });
    expect(
      (await page.evaluate(() => window.nvm.getShortcuts())).map(
        (record) => record.action.title,
      ),
    ).not.toContain('PAB-53 Lifecycle v2');
    expect(
      await page.evaluate(
        async (action) => window.nvm.runViewAction(action),
        v2DirectAction,
      ),
    ).toMatchObject({ view: { id: 'action-failed' } });
    expect(await fs.readFile(directMarkerPath, 'utf8')).toBe('v1\nv2\n');

    extensionsView = await openExtensionsView(page);
    const reEnable = actionNamed(extensionsView, 'Enable');
    await page.evaluate(
      async (action) => window.nvm.runViewAction(action),
      reEnable,
    );
    await expect
      .poll(() => searchTitles(page, 'PAB-53 Lifecycle v2'))
      .toContain('PAB-53 Lifecycle v2');
    await expect
      .poll(() => fs.readFile(markerPath, 'utf8').catch(() => ''))
      .toBe('v1\nv1\nv1\nv2\nv2\n');
    expect(
      await page.evaluate(
        (shortcut) => window.nvm.testIsActionShortcutRegistered(shortcut),
        actionShortcut,
      ),
    ).toEqual({ registered: true });
    expect(
      (await page.evaluate(() => window.nvm.getShortcuts())).map(
        (record) => record.action.title,
      ),
    ).toContain('PAB-53 Lifecycle v2');
    const reEnabledDirectAction = await renderLifecycleDirectAction(page, 'v2');
    await page.evaluate(
      async (action) => window.nvm.runViewAction(action),
      reEnabledDirectAction,
    );
    await expect
      .poll(() => fs.readFile(directMarkerPath, 'utf8').catch(() => ''))
      .toBe('v1\nv2\nv2\n');
  } finally {
    if (app) {
      const closePromise = app.close().catch(() => {});
      await Promise.race([
        closePromise,
        new Promise((resolve) => setTimeout(resolve, 2000)),
      ]);
      const survivors = await waitForProcessesToExit(trackedPids, 500);
      if (survivors.length) terminateTrackedProcesses(survivors, 'SIGKILL');
    }
  }
});

test('first upgrade, restart persistence, and later file discovery stay compatible', async () => {
  test.setTimeout(45_000);
  const migrationUserDataDir = path.join(userDataDir, 'pab53-migration');
  const extensionsDir = path.join(migrationUserDataDir, 'extensions');
  const legacyFile = 'legacy-local.ts';
  const legacyImportMarker = path.join(
    migrationUserDataDir,
    'legacy-imports.txt',
  );
  const discoveredFile = 'discovered-later.ts';
  const discoveredImportMarker = path.join(
    migrationUserDataDir,
    'discovered-imports.txt',
  );
  await fs.mkdir(extensionsDir, { recursive: true });
  await fs.writeFile(
    path.join(extensionsDir, legacyFile),
    migrationExtensionSource(
      'pab53.legacy',
      'PAB-53 Legacy Extension',
      legacyImportMarker,
    ),
  );
  await fs.writeFile(
    path.join(migrationUserDataDir, 'state.json'),
    `${JSON.stringify({
      extensionManager: { schemaVersion: 0, files: {}, proposals: {} },
    })}\n`,
  );

  let launched = await launchTestApplication(migrationUserDataDir);
  expect(
    await searchTitles(launched.page, 'PAB-53 Legacy Extension'),
  ).toContain('PAB-53 Legacy Extension');
  await expect
    .poll(() => fs.readFile(legacyImportMarker, 'utf8').catch(() => ''))
    .toBe('pab53.legacy\n');
  let persisted = JSON.parse(
    await fs.readFile(path.join(migrationUserDataDir, 'state.json'), 'utf8'),
  );
  expect(persisted.extensionManager.files[legacyFile]).toEqual({
    enabled: true,
  });
  await closeTestApplication(launched.app, launched.trackedPids);

  launched = await launchTestApplication(migrationUserDataDir);
  expect(
    await searchTitles(launched.page, 'PAB-53 Legacy Extension'),
  ).toContain('PAB-53 Legacy Extension');
  await expect
    .poll(() => fs.readFile(legacyImportMarker, 'utf8').catch(() => ''))
    .toBe('pab53.legacy\npab53.legacy\n');
  await closeTestApplication(launched.app, launched.trackedPids);

  await fs.writeFile(
    path.join(extensionsDir, discoveredFile),
    migrationExtensionSource(
      'pab53.discovered',
      'PAB-53 Discovered Extension',
      discoveredImportMarker,
    ),
  );
  launched = await launchTestApplication(migrationUserDataDir);
  try {
    expect(
      await searchTitles(launched.page, 'PAB-53 Discovered Extension'),
    ).not.toContain('PAB-53 Discovered Extension');
    await expect
      .poll(() => fs.stat(discoveredImportMarker).catch(() => null))
      .toBeNull();
    persisted = JSON.parse(
      await fs.readFile(path.join(migrationUserDataDir, 'state.json'), 'utf8'),
    );
    expect(persisted.extensionManager.files[discoveredFile]).toEqual({
      enabled: false,
    });
    expect(persisted.extensionManager.files[legacyFile]).toEqual({
      enabled: true,
    });
  } finally {
    await closeTestApplication(launched.app, launched.trackedPids);
  }
});
