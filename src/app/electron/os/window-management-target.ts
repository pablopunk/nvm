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
  WindowFocusTarget,
  WindowInfo,
  WindowInvocationPorts,
} from './window-management-types';

function describeFocusedWindowRejection(
  focused: WindowInfo,
  target: WindowFocusTarget,
  options: WindowResolveOptions,
): FailedResult | null {
  if (focused.bundleId !== target.bundleId) {
    return failure('target-lost', TARGET_WINDOW_CHANGED);
  }
  if (focused.isFullScreen && !options.allowFullScreen) {
    return failure('full-screen');
  }
  if (focused.isMinimized) {
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

export interface WritableWindow {
  focused: WindowInfo;
  workArea: Frame;
  displays: DisplayInfo[];
}

export interface WindowResolveOptions {
  allowFullScreen: boolean;
}

export async function resolveWritableWindow(
  dependencies: WindowManagementServiceDependencies,
  target: WindowFocusTarget,
  options: WindowResolveOptions,
): Promise<Settled<WritableWindow>> {
  const { manager, engine } = dependencies;
  if ((await manager.permissionState()) !== 'allowed') {
    return { ok: false, failure: failure('denied') };
  }
  const focused = unwrapEnvelope(await manager.focusedWindow(target.pid));
  if (focused.ok === false) {
    return focused;
  }
  const rejection = describeFocusedWindowRejection(
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
