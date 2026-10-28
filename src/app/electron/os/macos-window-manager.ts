import { execFile } from 'node:child_process';
import path from 'node:path';
import { app, systemPreferences } from 'electron';
import { warn } from '../platform/logger';
import type {
  DisplayInfo,
  HelperCode,
  HelperEnvelope,
  MacosWindowManager,
  RunningApp,
  SetFrameData,
  SetFrameRequest,
  WindowInfo,
  WindowPermissionState,
} from './window-management-types';

interface WindowHelperRun {
  exitCode: number;
  stdout: string;
  stderr: string;
}

interface WindowHelperDependencies {
  trusted(): boolean;
  run(args: string[]): Promise<WindowHelperRun>;
}

const HELPER_TIMEOUT_MS = 5000;
const HELPER_CODES: readonly HelperCode[] = [
  'ok',
  'denied',
  'no-window',
  'target-lost',
  'unsupported',
  'unapplied',
  'error',
];
const ACCESSIBILITY_DENIED =
  'Accessibility access is blocked. Open Nevermind OS Permissions, enable access, then restart Nevermind.';
const HELPER_UNREADABLE = 'The window helper returned an unreadable response.';
const HELPER_FAILED =
  'The window helper failed. Restart Nevermind and try again.';
const LOG_OPTIONS = { source: 'host', scope: 'window-management' } as const;

function windowHelperPath() {
  return app.isPackaged
    ? path.join(process.resourcesPath, 'macos-selected-text')
    : path.join(app.getAppPath(), 'build', 'native', 'macos-selected-text');
}

function runWindowHelper(args: string[]) {
  return new Promise<WindowHelperRun>((resolve, reject) => {
    execFile(
      windowHelperPath(),
      ['window', ...args],
      { timeout: HELPER_TIMEOUT_MS },
      (error, stdout, stderr) => {
        if (error && typeof error.code !== 'number') {
          reject(error);
          return;
        }
        resolve({
          exitCode: typeof error?.code === 'number' ? error.code : 0,
          stderr: String(stderr || ''),
          stdout: String(stdout || ''),
        });
      },
    );
  });
}

function failure<Data>(
  code: HelperCode,
  message: string,
): HelperEnvelope<Data> {
  return { ok: false, code, message };
}

function isHelperEnvelope(value: unknown): value is HelperEnvelope<unknown> {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const { code } = value as { code?: unknown };
  return typeof code === 'string' && HELPER_CODES.includes(code as HelperCode);
}

function parseHelperEnvelope<Data>(stdout: string): HelperEnvelope<Data> {
  let parsed: unknown;
  try {
    parsed = JSON.parse(stdout.trim());
  } catch {
    return failure('error', HELPER_UNREADABLE);
  }
  if (!isHelperEnvelope(parsed)) {
    return failure('error', HELPER_UNREADABLE);
  }
  const { code, data, message } = parsed as HelperEnvelope<Data>;
  return { ok: code === 'ok', code, data, message };
}

function describeError(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function logHelperRejection(
  args: string[],
  result: WindowHelperRun,
  envelope: HelperEnvelope<unknown>,
) {
  warn(
    'window-management.helper.rejected',
    {
      args: args[0],
      exitCode: result.exitCode,
      code: envelope.code,
      message: envelope.message,
      error: result.stderr.trim(),
    },
    LOG_OPTIONS,
  );
}

async function invokeWindowHelper<Data>(
  dependencies: WindowHelperDependencies,
  args: string[],
): Promise<HelperEnvelope<Data>> {
  try {
    if (!dependencies.trusted()) {
      return failure('denied', ACCESSIBILITY_DENIED);
    }
    const result = await dependencies.run(args);
    const envelope = parseHelperEnvelope<Data>(result.stdout);
    if (!envelope.ok) {
      logHelperRejection(args, result, envelope);
    }
    return envelope;
  } catch (error) {
    warn(
      'window-management.helper.failed',
      { args: args[0], error: describeError(error) },
      LOG_OPTIONS,
    );
    return failure('error', HELPER_FAILED);
  }
}

async function probePermissionState(
  dependencies: WindowHelperDependencies,
): Promise<WindowPermissionState> {
  try {
    if (!dependencies.trusted()) {
      return 'denied';
    }
    const { stdout } = await dependencies.run(['permissions']);
    const { code } = parseHelperEnvelope<unknown>(stdout);
    if (code === 'ok') {
      return 'allowed';
    }
    if (code === 'denied') {
      return 'denied';
    }
  } catch (error) {
    warn(
      'window-management.permissions.failed',
      { error: describeError(error) },
      LOG_OPTIONS,
    );
  }
  return 'unknown';
}

function buildSetFrameArgs(request: SetFrameRequest) {
  const { pid, windowKey, frame, positionOnly } = request;
  const origin = [String(frame.x), String(frame.y)];
  if (positionOnly) {
    return ['set-frame', String(pid), windowKey, ...origin, '--position-only'];
  }
  return [
    'set-frame',
    String(pid),
    windowKey,
    ...origin,
    String(frame.width),
    String(frame.height),
  ];
}

export function createMacosWindowManager(
  dependencies: WindowHelperDependencies,
): MacosWindowManager {
  return {
    permissionState: () => probePermissionState(dependencies),
    displays: () =>
      invokeWindowHelper<DisplayInfo[]>(dependencies, ['displays']),
    focusedWindow: (pid) =>
      invokeWindowHelper<WindowInfo>(dependencies, ['focused', String(pid)]),
    setFrame: (request) =>
      invokeWindowHelper<SetFrameData>(
        dependencies,
        buildSetFrameArgs(request),
      ),
    setFullScreen: (pid, windowKey, fullScreen) =>
      invokeWindowHelper<SetFrameData>(dependencies, [
        'set-fullscreen',
        String(pid),
        windowKey,
        String(fullScreen),
      ]),
    runningApps: () => invokeWindowHelper<RunningApp[]>(dependencies, ['apps']),
    windows: (pid) =>
      invokeWindowHelper<WindowInfo[]>(dependencies, ['list', String(pid)]),
    raiseWindow: (pid, windowKey) =>
      invokeWindowHelper<null>(dependencies, ['raise', String(pid), windowKey]),
  };
}

export const macosWindowManager = createMacosWindowManager({
  trusted: () => systemPreferences.isTrustedAccessibilityClient(false),
  run: runWindowHelper,
});
