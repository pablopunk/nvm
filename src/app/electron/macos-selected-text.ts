import { execFile } from 'node:child_process';
import path from 'node:path';
import { app, systemPreferences } from 'electron';
import { debug, warn } from './logger';

type Operation = 'copy' | 'read' | 'replace' | 'permissions';
type Target = { pid: number };
type HelperResult = { exitCode: number; stderr: string; stdout: string };
type PermissionState = 'allowed' | 'denied' | 'unknown';

const ACCESSIBILITY_REQUIRED =
  'Accessibility access is blocked. Open Nevermind OS Permissions, enable access, then restart Nevermind.';

function runHelper(operation: Operation, target?: Target, input?: string) {
  const helperPath = app.isPackaged
    ? path.join(process.resourcesPath, 'macos-selected-text')
    : path.join(app.getAppPath(), 'build', 'native', 'macos-selected-text');
  return new Promise<HelperResult>((resolve) => {
    const child = execFile(
      helperPath,
      target ? [operation, String(target.pid)] : [operation],
      { timeout: 5000 },
      (error, stdout, stderr) =>
        resolve({
          exitCode:
            typeof error?.code === 'number' ? error.code : error ? 1 : 0,
          stderr: String(stderr || error?.message || ''),
          stdout: String(stdout || ''),
        }),
    );
    if (input !== undefined) child.stdin?.end(input);
  });
}

export function createMacosSelectionAccess(dependencies: {
  trusted(): boolean;
  run: typeof runHelper;
}) {
  async function execute(
    operation: Operation,
    target?: Target,
    input?: string,
  ) {
    if (!dependencies.trusted()) throw new Error(ACCESSIBILITY_REQUIRED);
    const result = await dependencies.run(operation, target, input);
    if (result.exitCode === 0) return result;
    if (
      result.exitCode === 3 &&
      (operation === 'read' || operation === 'replace')
    ) {
      debug(
        `selected-text.${operation}.${operation === 'read' ? 'empty' : 'notApplied'}`,
        { pid: target?.pid },
        {
          source: 'host',
          scope: 'selected-text',
        },
      );
      return result;
    }
    warn(
      `selected-text.${operation}.failed`,
      {
        pid: target?.pid,
        exitCode: result.exitCode,
        error: result.stderr.trim(),
      },
      { source: 'host', scope: 'selected-text' },
    );
    if (result.exitCode === 2) throw new Error(ACCESSIBILITY_REQUIRED);
    if (result.exitCode === 4) {
      throw new Error(
        'The source app lost focus. Select the text and try again.',
      );
    }
    throw new Error(
      'The selected-text helper failed. Restart Nevermind and try again.',
    );
  }

  async function permissionState(): Promise<PermissionState> {
    try {
      if (!dependencies.trusted()) return 'denied';
      const result = await dependencies.run('permissions');
      if (result.exitCode === 0) return 'allowed';
      if (result.exitCode === 2) return 'denied';
      warn(
        'selected-text.permissions.failed',
        { exitCode: result.exitCode, error: result.stderr.trim() },
        { source: 'host', scope: 'selected-text' },
      );
    } catch (error) {
      warn('selected-text.permissions.failed', error, {
        source: 'host',
        scope: 'selected-text',
      });
    }
    return 'unknown';
  }

  async function read(target: Target) {
    const result = await execute('read', target);
    return result.exitCode === 0 ? result.stdout || null : null;
  }

  async function copy(target: Target) {
    await execute('copy', target);
    return true;
  }

  async function replace(target: Target, replacement: string) {
    return (await execute('replace', target, replacement)).exitCode === 0;
  }

  return { read, copy, replace, permissionState };
}

export const macosSelectionAccess = createMacosSelectionAccess({
  trusted: () => systemPreferences.isTrustedAccessibilityClient(false),
  run: runHelper,
});
