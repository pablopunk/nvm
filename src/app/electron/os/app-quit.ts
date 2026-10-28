import { execFile } from 'node:child_process';

const QUIT_REQUEST_TIMEOUT_MS = 5000;
const QUIT_SCRIPT_LINES = [
  'on run argv',
  'set targetApp to application ((POSIX file (item 1 of argv)) as text)',
  'if targetApp is running then tell targetApp to quit',
  'end run',
];

function quitScriptArguments(appPath: string) {
  return [...QUIT_SCRIPT_LINES.flatMap((line) => ['-e', line]), appPath];
}

export function requestMacAppQuit(appPath: string) {
  return new Promise<void>((resolve, reject) => {
    if (!(appPath.startsWith('/') && appPath.endsWith('.app'))) {
      reject(new Error('Application cannot be quit by path'));
      return;
    }
    execFile(
      '/usr/bin/osascript',
      quitScriptArguments(appPath),
      { timeout: QUIT_REQUEST_TIMEOUT_MS },
      (error) => (error ? reject(error) : resolve()),
    );
  });
}

export function runningMacAppForUninstall(appPath: string) {
  return new Promise<Set<string>>((resolve, reject) => {
    execFile(
      '/bin/ps',
      ['-axo', 'comm='],
      { timeout: 5000, maxBuffer: 1024 * 1024 },
      (error, stdout) => {
        if (error) {
          reject(
            new Error('Could not check whether the application is running'),
          );
          return;
        }
        const prefix = `${appPath.toLowerCase().replace(/\/+$/, '')}/contents/`;
        const running = stdout
          .split(/\r?\n/)
          .some((line) => line.trim().toLowerCase().startsWith(prefix));
        resolve(running ? new Set([appPath]) : new Set());
      },
    );
  });
}
