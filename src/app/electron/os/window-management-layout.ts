import { resolveCycleStep } from './window-management-cycling';
import type { OperationContext } from './window-management-dependencies';
import { fitRequestedFrame } from './window-management-frames';
import { rememberAppliedLayout } from './window-management-history';
import { windowGapsFrom } from './window-management-options';
import type { WritableWindow } from './window-management-target';
import type {
  Frame,
  WindowApplyResult,
  WindowLayoutSpec,
} from './window-management-types';
import { writeWithCorrection } from './window-management-write';

function layoutFrameFor(
  context: OperationContext,
  writable: WritableWindow,
  spec: WindowLayoutSpec,
): Frame {
  const { engine } = context.dependencies;
  const { focused, workArea } = writable;
  const computed = engine.computeTargetFrame({
    spec,
    workArea,
    currentFrame: focused.frame,
    gaps: windowGapsFrom(context.request.options),
  });
  return fitRequestedFrame(computed, focused, workArea);
}

export async function applyLayoutOperation(
  context: OperationContext,
  writable: WritableWindow,
): Promise<WindowApplyResult> {
  const { dependencies, memory, request } = context;
  const { focused, workArea } = writable;
  const cycle = resolveCycleStep({
    engine: dependencies.engine,
    lastApplied: memory.read(focused.windowKey)?.lastApplied,
    currentFrame: focused.frame,
    spec: request.spec,
    cycleEnabled: request.options.cycle,
    cycleOrder: request.options.cycleOrder,
  });
  const result = await writeWithCorrection(
    dependencies.manager,
    { pid: request.target.pid, focused, workArea },
    layoutFrameFor(context, writable, cycle.spec),
  );
  if (result.ok) {
    rememberAppliedLayout(memory, focused, result, cycle);
  }
  return result;
}
