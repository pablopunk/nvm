import type {
  ApplyRequest,
  WindowManagementServiceDependencies,
} from './window-management-dependencies';
import type { FailedResult } from './window-management-failures';
import { failure } from './window-management-failures';
import {
  applyFocusedWindow,
  type FocusedActionContext,
} from './window-management-focused-actions';
import {
  applyWindowByHandle,
  focusWindowByHandle,
  type HandleActionContext,
} from './window-management-handle-actions';
import { createWindowHandleRegistry } from './window-management-handles';
import { listDisplaysFrom, listWindowsFrom } from './window-management-listing';
import {
  createWindowMemory,
  type WindowMemory,
} from './window-management-memory';
import {
  focusNeighborWindow,
  type NeighborActionContext,
} from './window-management-neighbor';
import { runWindowOperation } from './window-management-operations';
import { enqueueForPid } from './window-management-queue';
import type {
  WindowApplyResult,
  WindowManagementService,
} from './window-management-types';

interface WindowRuntime {
  dependencies: WindowManagementServiceDependencies;
  memory: WindowMemory;
  queues: Map<number, Promise<void>>;
}

function isTargetGone(result: WindowApplyResult) {
  return (
    !result.ok &&
    (result.reason === 'target-lost' || result.reason === 'no-window')
  );
}

function runQueuedRequest(runtime: WindowRuntime, request: ApplyRequest) {
  return enqueueForPid(runtime.queues, request.target.pid, async () => {
    const result = await runWindowOperation({
      dependencies: runtime.dependencies,
      memory: runtime.memory,
      request,
    });
    if (isTargetGone(result)) {
      runtime.memory.dropPid(request.target.pid);
    }
    return result;
  });
}

function whenSupported<Result>(
  dependencies: WindowManagementServiceDependencies,
  run: () => Promise<Result>,
): Promise<Result | FailedResult> {
  return dependencies.supported()
    ? run()
    : Promise.resolve(failure('unsupported'));
}

export function createWindowManagementService(
  dependencies: WindowManagementServiceDependencies,
): WindowManagementService {
  const runtime: WindowRuntime = {
    dependencies,
    memory: createWindowMemory(),
    queues: new Map(),
  };
  const handles = createWindowHandleRegistry({
    now: dependencies.now,
    createId: dependencies.createHandleId,
  });
  const runRequest = (request: ApplyRequest) =>
    runQueuedRequest(runtime, request);
  const focusedContext: FocusedActionContext = { dependencies, runRequest };
  function enqueue<Result>(pid: number, task: () => Promise<Result>) {
    return enqueueForPid(runtime.queues, pid, task);
  }
  const handleContext: HandleActionContext = {
    dependencies,
    handles,
    runRequest,
    enqueue,
  };
  const neighborContext: NeighborActionContext = { dependencies, enqueue };
  return {
    available: () => dependencies.supported(),
    apply: (spec, options, invocation) =>
      applyFocusedWindow(focusedContext, spec, options, invocation),
    applyToWindow: (handle, spec, options) =>
      applyWindowByHandle(handleContext, handle, spec, options),
    focusWindow: (handle) => focusWindowByHandle(handleContext, handle),
    focusNeighbor: (direction, options, invocation) =>
      focusNeighborWindow(neighborContext, direction, options, invocation),
    listDisplays: () =>
      whenSupported(dependencies, () => listDisplaysFrom(dependencies.manager)),
    listWindows: (query) =>
      whenSupported(dependencies, () =>
        listWindowsFrom(dependencies.manager, handles, query),
      ),
  };
}
