import type { OperationContext } from './window-management-dependencies';
import { applyDisplayOperation } from './window-management-display';
import { failure } from './window-management-failures';
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

function rejectUnknownOperationKind(_spec: never): Promise<WindowApplyResult> {
  return Promise.resolve(failure('invalid-spec'));
}

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
    case 'fraction':
    case 'center':
    case 'maximize-width':
    case 'maximize-height':
    case 'scaled-center':
    case 'edge':
    case 'resize-step':
    case 'exact':
      return applyLayoutOperation(context, writable);
    default:
      return rejectUnknownOperationKind(spec);
  }
}

function windowKeyOf(context: OperationContext) {
  const { selection } = context.request;
  return selection.kind === 'handle' ? selection.windowKey : undefined;
}

function focusSourceOf(context: OperationContext) {
  const { selection } = context.request;
  return selection.kind === 'focused' ? selection.source : null;
}

export async function runWindowOperation(
  context: OperationContext,
): Promise<WindowApplyResult> {
  const { dependencies, request } = context;
  const { ports } = dependencies;
  const { spec, target } = request;
  const focusSource = focusSourceOf(context);
  if (focusSource === 'palette') {
    const focusFailure = await restorePaletteFocus(ports, target);
    if (focusFailure) {
      return focusFailure;
    }
  }
  const writable = await resolveWritableWindow(dependencies, target, {
    allowFullScreen: spec.kind === 'toggle-full-screen',
    windowKey: windowKeyOf(context),
  });
  if (writable.ok === false) {
    return writable.failure;
  }
  if (focusSource === 'shortcut') {
    const focusFailure = await guardShortcutFocus(ports, target);
    if (focusFailure) {
      return focusFailure;
    }
  }
  return dispatchOperation(context, writable.value);
}
