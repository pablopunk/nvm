import { windowAccessFailure } from './window-management-access';
import type {
  ApplyRequest,
  WindowManagementServiceDependencies,
} from './window-management-dependencies';
import { helperFailure } from './window-management-envelopes';
import {
  failure,
  SELF_TARGET_MESSAGE,
  UNKNOWN_WINDOW_MESSAGE,
} from './window-management-failures';
import type {
  WindowHandleRegistry,
  WindowHandleTarget,
} from './window-management-handles';
import type {
  MacosWindowManager,
  WindowApplyResult,
  WindowFocusResult,
  WindowLayoutSpec,
  WindowManagementOptions,
} from './window-management-types';
import { describeInvalidRequest } from './window-management-validation';

function isWindowLost(result: WindowApplyResult | WindowFocusResult) {
  return result.ok === false && result.reason === 'target-lost';
}

function focusTargetOf(target: WindowHandleTarget) {
  return { bundleId: target.appId, pid: target.pid };
}

function raiseTargetWindow(
  context: HandleActionContext,
  target: WindowHandleTarget,
): Promise<WindowFocusResult> {
  return raiseWindowKey(
    context.dependencies.manager,
    target.pid,
    target.windowKey,
  );
}

export async function raiseWindowKey(
  manager: MacosWindowManager,
  pid: number,
  windowKey: string,
): Promise<WindowFocusResult> {
  const denied = await windowAccessFailure(manager);
  if (denied) {
    return denied;
  }
  const raised = await manager.raiseWindow(pid, windowKey);
  return raised.ok ? { ok: true } : helperFailure(raised);
}

export async function applyWindowByHandle(
  context: HandleActionContext,
  handle: string,
  spec: WindowLayoutSpec,
  options: WindowManagementOptions,
): Promise<WindowApplyResult> {
  const { dependencies, handles, runRequest } = context;
  const target = handles.resolve(handle);
  if (!target) {
    return failure('unknown-window', UNKNOWN_WINDOW_MESSAGE);
  }
  const invalidMessage = describeInvalidRequest(spec, options);
  if (invalidMessage) {
    return failure('invalid-spec', invalidMessage);
  }
  if (!dependencies.supported()) {
    return failure('unsupported');
  }
  if (target.pid === dependencies.selfPid) {
    return failure('no-target', SELF_TARGET_MESSAGE);
  }
  const result = await runRequest({
    spec,
    options,
    selection: { kind: 'handle', windowKey: target.windowKey },
    target: focusTargetOf(target),
  });
  if (isWindowLost(result)) {
    handles.forget(handle);
  }
  return result;
}

export async function focusWindowByHandle(
  context: HandleActionContext,
  handle: string,
): Promise<WindowFocusResult> {
  const { dependencies, handles, enqueue } = context;
  const target = handles.resolve(handle);
  if (!target) {
    return failure('unknown-window', UNKNOWN_WINDOW_MESSAGE);
  }
  if (!dependencies.supported()) {
    return failure('unsupported');
  }
  const result = await enqueue(target.pid, () =>
    raiseTargetWindow(context, target),
  );
  if (isWindowLost(result)) {
    handles.forget(handle);
  }
  return result;
}

export interface HandleActionContext {
  dependencies: WindowManagementServiceDependencies;
  handles: WindowHandleRegistry;
  runRequest(request: ApplyRequest): Promise<WindowApplyResult>;
  enqueue<Result>(pid: number, task: () => Promise<Result>): Promise<Result>;
}
