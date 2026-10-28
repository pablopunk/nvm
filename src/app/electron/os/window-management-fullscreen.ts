import type { OperationContext } from './window-management-dependencies';
import { settleWrite } from './window-management-envelopes';
import { forgetCycleGroup } from './window-management-history';
import type { WritableWindow } from './window-management-target';
import type { WindowApplyResult } from './window-management-types';
import { successResult } from './window-management-write';

export async function applyFullScreenToggle(
  context: OperationContext,
  writable: WritableWindow,
): Promise<WindowApplyResult> {
  const { dependencies, memory, request } = context;
  const { focused } = writable;
  const settled = settleWrite(
    await dependencies.manager.setFullScreen(
      request.target.pid,
      focused.windowKey,
      !focused.isFullScreen,
    ),
  );
  if (!settled.ok) {
    return settled.failure;
  }
  const { outcome, changed, frame, reason } = settled.value;
  forgetCycleGroup(memory, focused.windowKey);
  return successResult(outcome, changed, frame, reason);
}
