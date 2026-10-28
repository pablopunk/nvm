import type { OperationContext } from './window-management-dependencies';
import type { DisplayDirection } from './window-management-engine-port';
import { failure } from './window-management-failures';
import { fitRequestedFrame } from './window-management-frames';
import { rememberAppliedLayout } from './window-management-history';
import type { WritableWindow } from './window-management-target';
import type { WindowApplyResult } from './window-management-types';
import { writeWithCorrection } from './window-management-write';

const MIN_DISPLAYS_TO_MOVE = 2;

function findAdjacentDisplay(
  context: OperationContext,
  writable: WritableWindow,
  direction: DisplayDirection,
) {
  const { engine } = context.dependencies;
  const { displays, focused } = writable;
  if (displays.length < MIN_DISPLAYS_TO_MOVE) {
    return null;
  }
  const from = engine.pickDisplayForFrame(displays, focused.frame);
  const to = engine.adjacentDisplay(
    displays,
    from,
    direction,
    context.request.options.displayOrder,
  );
  return to ? { from, to } : null;
}

export async function applyDisplayOperation(
  context: OperationContext,
  writable: WritableWindow,
  direction: DisplayDirection,
): Promise<WindowApplyResult> {
  const { dependencies, memory, request } = context;
  const { engine, manager } = dependencies;
  const { focused } = writable;
  const route = findAdjacentDisplay(context, writable, direction);
  if (!route) {
    return failure('single-display');
  }
  const computed = engine.moveFrameToDisplay({
    frame: focused.frame,
    from: route.from,
    to: route.to,
    sizing: request.options.displayMoveSizing,
  });
  const result = await writeWithCorrection(
    manager,
    { pid: request.target.pid, focused, workArea: route.to.workArea },
    fitRequestedFrame(computed, focused, route.to.workArea),
  );
  if (result.ok) {
    rememberAppliedLayout(memory, focused, result, { cycleIndex: 0 });
  }
  return result;
}
