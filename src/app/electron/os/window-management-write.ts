import { settleWrite } from './window-management-envelopes';
import {
  CORRECTION_FAILED_REASON,
  failure,
} from './window-management-failures';
import {
  clampFrameInside,
  classifyFinalOutcome,
  EXACT_TOLERANCE_POINTS,
  framesMatch,
  isFrameInside,
} from './window-management-frames';
import type {
  Frame,
  HelperEnvelope,
  MacosWindowManager,
  SetFrameData,
  WindowApplyResult,
  WindowInfo,
  WriteOutcome,
} from './window-management-types';

function correctionReason(
  correction: HelperEnvelope<SetFrameData>,
  written: SetFrameData,
) {
  return (
    correction.data?.reason ??
    (correction.ok ? written.reason : CORRECTION_FAILED_REASON)
  );
}

function correctionFrame(
  requested: Frame,
  written: SetFrameData,
  workArea: Frame,
): Frame {
  return clampFrameInside(
    { ...requested, width: written.frame.width, height: written.frame.height },
    workArea,
  );
}

async function correctOverflow(
  manager: MacosWindowManager,
  target: WriteTarget,
  requested: Frame,
  written: SetFrameData,
): Promise<WindowApplyResult> {
  const { pid, focused, workArea } = target;
  const correction = await manager.setFrame({
    pid,
    windowKey: focused.windowKey,
    frame: correctionFrame(requested, written, workArea),
    positionOnly: true,
  });
  const observed = correction.data?.frame ?? written.frame;
  return successResult(
    classifyFinalOutcome(observed, requested, focused.frame),
    !framesMatch(observed, focused.frame, EXACT_TOLERANCE_POINTS),
    observed,
    correctionReason(correction, written),
  );
}

export interface WriteTarget {
  pid: number;
  focused: WindowInfo;
  workArea: Frame;
}

export function successResult(
  outcome: WriteOutcome,
  changed: boolean,
  frame: Frame,
  reason?: string,
): WindowApplyResult {
  if (outcome === 'unapplied') {
    return failure('unapplied');
  }
  return {
    ok: true,
    outcome,
    changed,
    frame,
    ...(reason === undefined ? {} : { reason }),
  };
}

export async function writeWithCorrection(
  manager: MacosWindowManager,
  target: WriteTarget,
  requested: Frame,
): Promise<WindowApplyResult> {
  const { pid, focused, workArea } = target;
  const first = settleWrite(
    await manager.setFrame({
      pid,
      windowKey: focused.windowKey,
      frame: requested,
    }),
  );
  if (!first.ok) {
    return first.failure;
  }
  const written = first.value;
  if (written.outcome !== 'partial' || isFrameInside(written.frame, workArea)) {
    return successResult(
      written.outcome,
      written.changed,
      written.frame,
      written.reason,
    );
  }
  return correctOverflow(manager, target, requested, written);
}
