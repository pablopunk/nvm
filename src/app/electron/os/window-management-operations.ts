import type { OperationContext } from './window-management-dependencies';
import { applyDisplayOperation } from './window-management-display';
import { applyFullScreenToggle } from './window-management-fullscreen';
import { applyLayoutOperation } from './window-management-layout';
import { applyRestoreOperation } from './window-management-restore';
import {
  guardShortcutFocus,
  resolveWritableWindow,
  restorePaletteFocus,
  type WritableWindow,
} from './window-management-target';
import type { WindowApplyResult } from './window-management-types';

function dispatchOperation(
  context: OperationContext,
  writable: WritableWindow,
): Promise<WindowApplyResult> {
  const { spec } = context.request;
  switch (spec.kind) {
    case 'restore':
      return applyRestoreOperation(context, writable);
    case 'display':
      return applyDisplayOperation(context, writable, spec.direction);
    case 'toggle-full-screen':
      return applyFullScreenToggle(context, writable);
    default:
      return applyLayoutOperation(context, writable);
  }
}

export async function runWindowOperation(
  context: OperationContext,
): Promise<WindowApplyResult> {
  const { dependencies, request } = context;
  const { ports } = dependencies;
  const { spec, source, target } = request;
  if (source === 'palette') {
    const focusFailure = await restorePaletteFocus(ports, target);
    if (focusFailure) {
      return focusFailure;
    }
  }
  const writable = await resolveWritableWindow(dependencies, target, {
    allowFullScreen: spec.kind === 'toggle-full-screen',
  });
  if (writable.ok === false) {
    return writable.failure;
  }
  if (source === 'shortcut') {
    const focusFailure = await guardShortcutFocus(ports, target);
    if (focusFailure) {
      return focusFailure;
    }
  }
  return dispatchOperation(context, writable.value);
}
