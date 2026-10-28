import { windowAccessFailure } from './window-management-access';
import type { WindowManagementServiceDependencies } from './window-management-dependencies';
import { unwrapEnvelope } from './window-management-envelopes';
import {
  type FailedResult,
  failure,
  SELF_TARGET_MESSAGE,
  type Settled,
} from './window-management-failures';
import { noTargetMessage } from './window-management-focused-actions';
import { raiseWindowKey } from './window-management-handle-actions';
import {
  guardShortcutFocus,
  restorePaletteFocus,
} from './window-management-target';
import type {
  ListedWindow,
  MacosWindowManager,
  NeighborDirection,
  NeighborOptions,
  WindowFocusResult,
  WindowFocusTarget,
  WindowInfo,
  WindowInvocation,
} from './window-management-types';
import {
  type NeighborWindow,
  nearestWindowInDirection,
} from './window-neighbors';

interface NeighborRequest {
  direction: NeighborDirection;
  options: NeighborOptions;
  source: WindowInvocation['source'];
  target: WindowFocusTarget;
}

const FRONT_MOST_Z_INDEX = 0;

function isNeighborCandidate(
  entry: ListedWindow,
  selfPid: number | undefined,
  originKey: string,
) {
  return (
    entry.pid !== selfPid &&
    entry.windowKey !== originKey &&
    !entry.isMinimized &&
    !entry.isFullScreen
  );
}

function neighborWindowOf(window: WindowInfo, zIndex: number): NeighborWindow {
  return {
    id: window.windowKey,
    frame: window.frame,
    zIndex,
    displayId: window.displayId,
    isFullScreen: window.isFullScreen,
  };
}

function originWindowOf(
  focused: WindowInfo,
  listed: ListedWindow[],
): NeighborWindow {
  const match = listed.find((entry) => entry.windowKey === focused.windowKey);
  return match
    ? neighborWindowOf(match, match.zIndex)
    : neighborWindowOf(focused, FRONT_MOST_Z_INDEX);
}

async function readCurrentWindow(
  manager: MacosWindowManager,
  pid: number,
): Promise<Settled<WindowInfo>> {
  const focused = unwrapEnvelope(await manager.focusedWindow(pid));
  if (focused.ok === false && focused.failure.reason === 'no-window') {
    return {
      ok: false,
      failure: failure('no-window', NO_FOCUSED_WINDOW_MESSAGE),
    };
  }
  return focused;
}

function focusFromInvocationGuard(
  dependencies: WindowManagementServiceDependencies,
  request: NeighborRequest,
): Promise<FailedResult | null> {
  if (request.source === 'palette') {
    return restorePaletteFocus(dependencies.ports, request.target);
  }
  return guardShortcutFocus(dependencies.ports, request.target);
}

async function runNeighborFocus(
  dependencies: WindowManagementServiceDependencies,
  request: NeighborRequest,
): Promise<WindowFocusResult> {
  const { manager, selfPid } = dependencies;
  const denied = await windowAccessFailure(manager);
  if (denied) {
    return denied;
  }
  const guardFailure = await focusFromInvocationGuard(dependencies, request);
  if (guardFailure) {
    return guardFailure;
  }
  const focused = await readCurrentWindow(manager, request.target.pid);
  if (focused.ok === false) {
    return focused.failure;
  }
  const listed = unwrapEnvelope(await manager.allWindows());
  if (listed.ok === false) {
    return listed.failure;
  }
  const origin = originWindowOf(focused.value, listed.value);
  const candidates = listed.value.filter((entry) =>
    isNeighborCandidate(entry, selfPid, origin.id),
  );
  const neighborKey = nearestWindowInDirection(
    origin,
    candidates.map((entry) => neighborWindowOf(entry, entry.zIndex)),
    request.direction,
    request.options,
  );
  const neighbor = candidates.find((entry) => entry.windowKey === neighborKey);
  if (!neighbor) {
    return failure('no-neighbor');
  }
  return raiseWindowKey(manager, neighbor.pid, neighbor.windowKey);
}

export interface NeighborActionContext {
  dependencies: WindowManagementServiceDependencies;
  enqueue<Result>(pid: number, task: () => Promise<Result>): Promise<Result>;
}

export const NO_FOCUSED_WINDOW_MESSAGE =
  'The target app has no focused window.';

export function focusNeighborWindow(
  context: NeighborActionContext,
  direction: NeighborDirection,
  options: NeighborOptions,
  invocation: WindowInvocation,
): Promise<WindowFocusResult> {
  const { dependencies, enqueue } = context;
  const { target, source } = invocation;
  if (!dependencies.supported()) {
    return Promise.resolve(failure('unsupported'));
  }
  if (!target) {
    return Promise.resolve(failure('no-target', noTargetMessage(source)));
  }
  if (target.pid === dependencies.selfPid) {
    return Promise.resolve(failure('no-target', SELF_TARGET_MESSAGE));
  }
  return enqueue(target.pid, () =>
    runNeighborFocus(dependencies, { direction, options, source, target }),
  );
}
