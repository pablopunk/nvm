import path from 'node:path';
import { app } from 'electron';
import {
  DesktopTextAccessError,
  type DesktopTextTarget,
  desktopTextFailure,
  runDesktopTextCommand,
} from './desktop-text-access';

export function createWindowsDesktopText(
  dependencies: {
    run?: typeof runDesktopTextCommand;
    helperPath?: () => string;
  } = {},
) {
  const run = dependencies.run ?? runDesktopTextCommand;
  const helperPath =
    dependencies.helperPath ??
    (() =>
      app.isPackaged
        ? path.join(process.resourcesPath, 'windows-desktop-text.exe')
        : path.join(app.getAppPath(), 'build/native/windows-desktop-text.exe'));

  async function execute(operation: string, target?: DesktopTextTarget) {
    try {
      return await run(
        helperPath(),
        target
          ? [operation, String(target.pid), String(target.windowId)]
          : [operation],
      );
    } catch (error) {
      const failure = error as NodeJS.ErrnoException & {
        code: string | number;
        stderr?: string;
      };
      if (Number(failure.code) === 3 && ['read', 'target'].includes(operation))
        return '';
      if (Number(failure.code) === 2)
        throw new DesktopTextAccessError(
          'denied',
          failure.stderr?.trim() || 'Windows blocked selected-text access.',
        );
      if (Number(failure.code) === 4 || Number(failure.code) === 5)
        throw new DesktopTextAccessError(
          'unavailable',
          failure.stderr?.trim() || 'The source window is unavailable.',
        );
      throw new DesktopTextAccessError(
        'unavailable',
        'The Windows selected-text helper is unavailable. Reinstall Nevermind and try again.',
      );
    }
  }

  async function target(): Promise<DesktopTextTarget | null> {
    const [pidText, windowId] = (await execute('target')).split(/\r?\n/);
    const pid = Number(pidText);
    if (!Number.isSafeInteger(pid) || pid <= 0 || !/^\d+$/.test(windowId ?? ''))
      return null;
    return { bundleId: `windows:${windowId}:${pid}`, windowId, pid };
  }

  async function restore(expected: DesktopTextTarget) {
    if (!expected.windowId || !expected.bundleId.startsWith('windows:'))
      return false;
    await execute('restore', expected);
    return true;
  }

  async function copy(expected: DesktopTextTarget) {
    await execute('copy', expected);
    return true;
  }

  async function read(expected: DesktopTextTarget) {
    return (await execute('read', expected)) || null;
  }

  async function paste(expectedId?: string) {
    const current = await target();
    if (!current || (expectedId && current.bundleId !== expectedId))
      throw new DesktopTextAccessError(
        'unavailable',
        'The source window lost focus. Select the text and try again.',
      );
    await execute('paste', current);
  }

  async function access() {
    try {
      await execute('permissions');
      return {
        state: 'allowed' as const,
        message:
          'Desktop control is available; elevated apps can block access. Press Enter to check again.',
      };
    } catch (error) {
      return desktopTextFailure(error);
    }
  }

  return { target, restore, copy, read, paste, access };
}
