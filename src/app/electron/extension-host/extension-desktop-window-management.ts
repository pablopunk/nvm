import type {
  Frame,
  WindowApplyResult,
  WindowInvocation,
  WindowLayoutSpec,
  WindowManagementService,
} from '../os/window-management-types';
import {
  currentWindowInvocationScope,
  type WindowInvocationScope,
} from './window-invocation-scope';

interface DesktopWindowManagementDeps {
  service: WindowManagementService;
  resolveInvocation: (
    scope: WindowInvocationScope | undefined,
  ) => WindowInvocation | null;
}

interface ExtensionWindowApplyOptions {
  gap?: number;
}

const MISSING_INVOCATION_MESSAGE =
  'Window commands run from the palette or a keyboard shortcut.';

async function applyWindowLayoutForCurrentScope(
  deps: DesktopWindowManagementDeps,
  spec: WindowLayoutSpec,
  options: ExtensionWindowApplyOptions = {},
) {
  const invocation = deps.resolveInvocation(currentWindowInvocationScope());
  if (!invocation) {
    return missingInvocationResult();
  }
  const result = await deps.service.apply(
    spec,
    { gap: options.gap ?? 0 },
    invocation,
  );
  return toExtensionApplyResult(result);
}

function missingInvocationResult() {
  return {
    ok: false,
    reason: 'no-target',
    message: MISSING_INVOCATION_MESSAGE,
  };
}

function pickFrame(frame: Frame): Frame {
  return {
    x: frame.x,
    y: frame.y,
    width: frame.width,
    height: frame.height,
  };
}

function toExtensionApplyResult(result: WindowApplyResult) {
  if (result.ok === false) {
    return { ok: false, reason: result.reason, message: result.message };
  }
  return {
    ok: true,
    outcome: result.outcome,
    changed: result.changed,
    frame: pickFrame(result.frame),
    ...(result.reason ? { reason: result.reason } : {}),
  };
}

export function createDesktopWindowManagement(
  deps: DesktopWindowManagementDeps | undefined,
) {
  if (!deps?.service.available()) {
    return;
  }
  return {
    apply: (spec: WindowLayoutSpec, options?: ExtensionWindowApplyOptions) =>
      applyWindowLayoutForCurrentScope(deps, spec, options),
  };
}

export type { DesktopWindowManagementDeps, ExtensionWindowApplyOptions };
