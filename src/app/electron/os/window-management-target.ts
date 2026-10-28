import { windowAccessFailure } from './window-management-access';
import type { WindowManagementServiceDependencies } from './window-management-dependencies';
import { unwrapEnvelope } from './window-management-envelopes';
import type { Settled } from './window-management-failures';
import {
  type FailedResult,
  failure,
  NO_DISPLAY_MESSAGE,
  PALETTE_FOCUS_NOT_RESTORED,
  PALETTE_TARGET_CHANGED,
  SHORTCUT_FOCUS_CHANGED,
  TARGET_WINDOW_CHANGED,
} from './window-management-failures';
import type {
  DisplayInfo,
  Frame,
  MacosWindowManager,
  WindowFocusTarget,
  WindowInfo,
  WindowInvocationPorts,
} from './window-management-types';

function describeTargetWindowRejection(
  window: WindowInfo,
  target: WindowFocusTarget,
  options: WindowResolveOptions,
): FailedResult | null {
  if (window.bundleId !== target.bundleId) {
    return failure('target-lost', TARGET_WINDOW_CHANGED);
  }
  if (window.isFullScreen && !options.allowFullScreen) {
    return failure('full-screen');
  }
  if (window.isMinimized) {
    return failure('minimized');
  }
  return null;
}

async function isFrontmostPid(
  ports: WindowInvocationPorts,
  pid: number,
): Promise<boolean> {
  return (await ports.frontmostTarget())?.pid === pid;
}

async function readTargetWindow(
  manager: MacosWindowManager,
  target: WindowFocusTarget,
  windowKey: string | undefined,
): Promise<Settled<WindowInfo>> {
  if (windowKey === undefined) {
    return unwrapEnvelope(await manager.focusedWindow(target.pid));
  }
  const listed = unwrapEnvelope(await manager.windows(target.pid));
  if (listed.ok === false) {
    return listed;
  }
  const match = listed.value.find((window) => window.windowKey === windowKey);
  return match
    ? { ok: true, value: match }
    : { ok: false, failure: failure('target-lost') };
}

export interface WritableWindow {
  focused: WindowInfo;
  workArea: Frame;
  displays: DisplayInfo[];
}

export interface WindowResolveOptions {
  allowFullScreen: boolean;
  windowKey?: string;
}

export async function resolveWritableWindow(
  dependencies: WindowManagementServiceDependencies,
  target: WindowFocusTarget,
  options: WindowResolveOptions,
): Promise<Settled<WritableWindow>> {
  const { manager, engine } = dependencies;
  const denied = await windowAccessFailure(manager);
  if (denied) {
    return { ok: false, failure: denied };
  }
  const focused = await readTargetWindow(manager, target, options.windowKey);
  if (focused.ok === false) {
    return focused;
  }
  const rejection = describeTargetWindowRejection(
    focused.value,
    target,
    options,
  );
  if (rejection) {
    return { ok: false, failure: rejection };
  }
  const displays = unwrapEnvelope(await manager.displays());
  if (displays.ok === false) {
    return displays;
  }
  if (displays.value.length === 0) {
    return { ok: false, failure: failure('unsupported', NO_DISPLAY_MESSAGE) };
  }
  const display = engine.pickDisplayForFrame(
    displays.value,
    focused.value.frame,
  );
  return {
    ok: true,
    value: {
      focused: focused.value,
      workArea: display.workArea,
      displays: displays.value,
    },
  };
}

export async function restorePaletteFocus(
  ports: WindowInvocationPorts,
  target: WindowFocusTarget,
): Promise<FailedResult | null> {
  if (ports.paletteFocusTarget()?.pid !== target.pid) {
    return failure('focus-not-restored', PALETTE_TARGET_CHANGED);
  }
  const restored = await ports.hidePaletteAndRestoreFocus(target);
  if (restored && (await isFrontmostPid(ports, target.pid))) {
    return null;
  }
  return failure('focus-not-restored', PALETTE_FOCUS_NOT_RESTORED);
}

export async function guardShortcutFocus(
  ports: WindowInvocationPorts,
  target: WindowFocusTarget,
): Promise<FailedResult | null> {
  if (await isFrontmostPid(ports, target.pid)) {
    return null;
  }
  return failure('focus-not-restored', SHORTCUT_FOCUS_CHANGED);
}
