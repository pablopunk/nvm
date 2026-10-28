import type { OperationContext } from './window-management-dependencies';
import { failure } from './window-management-failures';
import { fitFrameInside, fitRequestedFrame } from './window-management-frames';
import type { WritableWindow } from './window-management-target';
import type { WindowApplyResult } from './window-management-types';
import { writeWithCorrection } from './window-management-write';

export async function applyRestoreOperation(
  context: OperationContext,
  writable: WritableWindow,
): Promise<WindowApplyResult> {
  const { dependencies, memory, request } = context;
  const { engine, manager } = dependencies;
  const { focused, displays } = writable;
  const stored = memory.read(focused.windowKey)?.restoreFrame;
  if (!stored) {
    return failure('no-history');
  }
  const display = engine.pickDisplayForFrame(displays, stored);
  const result = await writeWithCorrection(
    manager,
    { pid: request.target.pid, focused, workArea: display.workArea },
    fitRequestedFrame(
      fitFrameInside(stored, display.workArea),
      focused,
      display.workArea,
    ),
  );
  if (result.ok) {
    memory.forget(focused.windowKey);
  }
  return result;
}
