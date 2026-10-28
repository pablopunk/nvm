import type {
  WindowApplyFailureReason,
  WindowApplyResult,
} from './window-management-types';

export type FailedResult = Extract<WindowApplyResult, { ok: false }>;
export type Settled<Value> =
  | { ok: true; value: Value }
  | { ok: false; failure: FailedResult };

export const FAILURE_MESSAGES: Record<WindowApplyFailureReason, string> = {
  'no-target': 'No target app was captured. Reopen Nevermind and try again.',
  'focus-not-restored':
    'Could not return focus to the target app. No window was changed.',
  'no-window': 'The target app has no window to move.',
  'target-lost': 'The target window is no longer available.',
  'full-screen': 'Exit full screen before moving this window.',
  minimized: 'Restore the window before moving it.',
  denied:
    'Accessibility access is blocked. Open Nevermind OS Permissions, enable access, then restart Nevermind.',
  unapplied: 'The window did not accept the new frame.',
  'not-resizable': 'This window cannot be resized.',
  unsupported: 'Window management is not supported on this Mac.',
  'invalid-spec': 'The window layout request is invalid.',
  'no-history': 'There is no earlier position to restore for this window.',
  'single-display': 'Only one display is connected.',
};

export const PALETTE_NO_TARGET =
  'No target app was captured. Reopen Nevermind from the app you want to move, then try again.';
export const PALETTE_FOCUS_NOT_RESTORED =
  'Could not return focus to the target app. No window was changed.';
export const PALETTE_TARGET_CHANGED =
  'The target app changed. Reopen Nevermind from the window you want to move.';
export const SHORTCUT_NO_TARGET =
  'No frontmost app was available when the shortcut ran.';
export const SHORTCUT_FOCUS_CHANGED =
  'The frontmost app changed before the window was moved. No window was changed.';
export const TARGET_WINDOW_CHANGED =
  'The focused window now belongs to a different app.';
export const NO_DISPLAY_MESSAGE =
  'No display is available to place the window on.';
export const SELF_TARGET_MESSAGE =
  'Nevermind is in front. Focus the window you want to move, then run the command again.';
export const INVALID_SPEC_MESSAGE = FAILURE_MESSAGES['invalid-spec'];
export const INVALID_OPTIONS_MESSAGE = 'The window layout options are invalid.';
export const CORRECTION_FAILED_REASON =
  'Part of the window stayed outside its display.';

export function failure(
  reason: WindowApplyFailureReason,
  message: string = FAILURE_MESSAGES[reason],
): FailedResult {
  return { ok: false, reason, message };
}
