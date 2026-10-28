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
import { createWindowMemory } from './window-management-memory';
import { runWindowOperation } from './window-management-operations';
import { enqueueForPid } from './window-management-queue';
import type {
  WindowApplyResult,
  WindowInvocation,
  WindowLayoutSpec,
  WindowManagementOptions,
  WindowManagementService,
} from './window-management-types';
import { describeInvalidRequest } from './window-management-validation';

function noTargetMessage(source: WindowInvocation['source']) {
  return source === 'palette' ? PALETTE_NO_TARGET : SHORTCUT_NO_TARGET;
}

function isTargetGone(result: WindowApplyResult) {
  return (
    !result.ok &&
    (result.reason === 'target-lost' || result.reason === 'no-window')
  );
}

export function createWindowManagementService(
  dependencies: WindowManagementServiceDependencies,
): WindowManagementService {
  const queues = new Map<number, Promise<void>>();
  const memory = createWindowMemory();

  function runQueuedRequest(request: ApplyRequest) {
    return enqueueForPid(queues, request.target.pid, async () => {
      const result = await runWindowOperation({
        dependencies,
        memory,
        request,
      });
      if (isTargetGone(result)) {
        memory.dropPid(request.target.pid);
      }
      return result;
    });
  }

  function apply(
    spec: WindowLayoutSpec,
    options: WindowManagementOptions,
    invocation: WindowInvocation,
  ): Promise<WindowApplyResult> {
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
    return runQueuedRequest({ spec, options, source, target });
  }

  return {
    available: () => dependencies.supported(),
    apply,
  };
}
