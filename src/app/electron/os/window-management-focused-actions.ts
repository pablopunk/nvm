import type {
  ApplyRequest,
  WindowManagementServiceDependencies,
} from './window-management-dependencies';
import {
  failure,
  PALETTE_NO_TARGET,
  SELF_TARGET_MESSAGE,
  SHORTCUT_NO_TARGET,
} from './window-management-failures';
import type {
  WindowApplyResult,
  WindowInvocation,
  WindowLayoutSpec,
  WindowManagementOptions,
} from './window-management-types';
import { describeInvalidRequest } from './window-management-validation';

function noTargetMessage(source: WindowInvocation['source']) {
  return source === 'palette' ? PALETTE_NO_TARGET : SHORTCUT_NO_TARGET;
}

export function applyFocusedWindow(
  context: FocusedActionContext,
  spec: WindowLayoutSpec,
  options: WindowManagementOptions,
  invocation: WindowInvocation,
): Promise<WindowApplyResult> {
  const { dependencies, runRequest } = context;
  const invalidMessage = describeInvalidRequest(spec, options);
  if (invalidMessage) {
    return Promise.resolve(failure('invalid-spec', invalidMessage));
  }
  if (!dependencies.supported()) {
    return Promise.resolve(failure('unsupported'));
  }
  const { target, source } = invocation;
  if (!target) {
    return Promise.resolve(failure('no-target', noTargetMessage(source)));
  }
  if (target.pid === dependencies.selfPid) {
    return Promise.resolve(failure('no-target', SELF_TARGET_MESSAGE));
  }
  return runRequest({
    spec,
    options,
    selection: { kind: 'focused', source },
    target,
  });
}

export interface FocusedActionContext {
  dependencies: WindowManagementServiceDependencies;
  runRequest(request: ApplyRequest): Promise<WindowApplyResult>;
}
