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
import { MAX_LISTED_APPS } from './window-management-listing';
import {
  guardShortcutFocus,
  restorePaletteFocus,
} from './window-management-target';
import type {
  MacosWindowManager,
  NeighborDirection,
  NeighborOptions,
  WindowFocusResult,
  WindowFocusTarget,
  WindowInfo,
  WindowInvocation,
} from './window-management-types';
import {
  type NeighborCandidate,
  nearestWindowInDirection,
} from './window-neighbors';

interface NeighborRequest {
  direction: NeighborDirection;
  options: NeighborOptions;
  source: WindowInvocation['source'];
  target: WindowFocusTarget;
}

interface ListedWindow {
  pid: number;
  window: WindowInfo;
}

function isNeighborEligible(window: WindowInfo, currentKey: string) {
  return (
    window.windowKey !== currentKey &&
    !window.isMinimized &&
    !window.isFullScreen
  );
}

function neighborCandidateOf(entry: ListedWindow): NeighborCandidate {
  return { id: entry.window.windowKey, frame: entry.window.frame };
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

async function listNeighborWindows(
  manager: MacosWindowManager,
  selfPid: number | undefined,
  currentKey: string,
): Promise<Settled<ListedWindow[]>> {
  const apps = unwrapEnvelope(await manager.runningApps());
  if (apps.ok === false) {
    return apps;
  }
  const listed: ListedWindow[] = [];
  for (const app of apps.value.slice(0, MAX_LISTED_APPS)) {
    if (app.pid === selfPid) {
      continue;
    }
    // biome-ignore lint/performance/noAwaitInLoops: apps are read one at a time so a slow app cannot fan out helper processes
    const windows = unwrapEnvelope(await manager.windows(app.pid));
    if (windows.ok === false) {
      continue;
    }
    for (const window of windows.value) {
      if (isNeighborEligible(window, currentKey)) {
        listed.push({ pid: app.pid, window });
      }
    }
  }
  return { ok: true, value: listed };
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
  const current = await readCurrentWindow(manager, request.target.pid);
  if (current.ok === false) {
    return current.failure;
  }
  const listed = await listNeighborWindows(
    manager,
    selfPid,
    current.value.windowKey,
  );
  if (listed.ok === false) {
    return listed.failure;
  }
  const neighborKey = nearestWindowInDirection(
    current.value.frame,
    listed.value.map(neighborCandidateOf),
    request.direction,
    request.options,
  );
  const neighbor = listed.value.find(
    (entry) => entry.window.windowKey === neighborKey,
  );
  if (!neighbor) {
    return failure('no-neighbor');
  }
  return raiseWindowKey(manager, neighbor.pid, neighbor.window.windowKey);
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
