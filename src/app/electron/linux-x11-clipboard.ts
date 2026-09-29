import { spawn } from 'node:child_process';
import {
  DesktopTextAccessError,
  findDesktopTextCommand,
} from './desktop-text-access';

export function createLinuxX11Clipboard(
  dependencies: {
    environment?: NodeJS.ProcessEnv;
    findCommand?: (name: string) => string | null | Promise<string | null>;
    spawnCommand?: typeof spawn;
  } = {},
) {
  const environment = dependencies.environment ?? process.env;
  const findCommand =
    dependencies.findCommand ??
    ((name) => findDesktopTextCommand(name, environment.PATH ?? ''));
  const spawnCommand = dependencies.spawnCommand ?? spawn;
  let executablePath: Promise<string | null> | null = null;

  function commandPath() {
    executablePath ??= Promise.resolve(findCommand('xclip'));
    return executablePath;
  }

  function enabled() {
    return (
      process.platform === 'linux' &&
      environment.XDG_SESSION_TYPE !== 'wayland' &&
      !environment.WAYLAND_DISPLAY &&
      Boolean(environment.DISPLAY)
    );
  }

  function validateSnapshot(snapshot: {
    text?: string;
    html?: string;
    rtf?: string;
    image?: unknown;
    filePaths?: string[];
    bookmark?: { title?: string; url?: string };
  }) {
    if (!enabled()) return;
    if (
      snapshot.html ||
      snapshot.rtf ||
      snapshot.image ||
      snapshot.filePaths?.length ||
      snapshot.bookmark?.title ||
      snapshot.bookmark?.url
    ) {
      throw new DesktopTextAccessError(
        'unavailable',
        'Cannot preserve a formatted X11 clipboard while reading selected text. Copy plain text first and try again.',
      );
    }
  }

  async function writeText(text: string) {
    const executable = await commandPath();
    if (!executable)
      throw new DesktopTextAccessError(
        'unavailable',
        'Install xclip to preserve selected text on X11, then restart Nevermind.',
      );
    await new Promise<void>((resolve, reject) => {
      const child = spawnCommand(
        executable,
        ['-selection', 'clipboard', '-in'],
        {
          stdio: ['pipe', 'ignore', 'ignore'],
          env: environment,
        },
      );
      child.on('error', () =>
        reject(
          new DesktopTextAccessError(
            'unavailable',
            'Could not start xclip. Check that it is installed and available in the desktop session.',
          ),
        ),
      );
      child.stdin?.on('error', reject);
      child.on('close', (code) =>
        code === 0
          ? resolve()
          : reject(
              new DesktopTextAccessError(
                'unavailable',
                'Could not write the X11 clipboard. Check desktop session access.',
              ),
            ),
      );
      child.stdin?.end(text);
    });
  }

  return { enabled, validateSnapshot, writeText };
}
