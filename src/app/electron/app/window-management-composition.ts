import { createWindowManagementService } from '../os/window-management';
import { computeTargetFrame, pickDisplayForFrame } from '../os/window-layouts';
import { macosWindowManager } from '../os/macos-window-manager';
import { hasCapability } from '../os/os';
import type {
  WindowFocusTarget,
  WindowInvocationPorts,
} from '../os/window-management-types';
import {
  type WindowInvocationScope,
  windowInvocationFromScope,
} from '../extension-host/window-invocation-scope';

type WindowManagementCompositionDependencies = {
  paletteWindow: {
    peekFocusReturnTarget(): unknown;
    hidePalette(): Promise<unknown>;
  };
  frontmostTarget(): Promise<WindowFocusTarget | null>;
};

function focusTargetFrom(value: unknown): WindowFocusTarget | null {
  const candidate = value as Partial<WindowFocusTarget> | null;
  return candidate &&
    typeof candidate.bundleId === 'string' &&
    Number.isSafeInteger(candidate.pid)
    ? { bundleId: candidate.bundleId, pid: candidate.pid as number }
    : null;
}

export function createWindowManagementComposition(
  dependencies: WindowManagementCompositionDependencies,
) {
  const { paletteWindow, frontmostTarget } = dependencies;
  const ports: WindowInvocationPorts = {
    paletteFocusTarget: () =>
      focusTargetFrom(paletteWindow.peekFocusReturnTarget()),
    hidePaletteAndRestoreFocus: async () => {
      await paletteWindow.hidePalette();
      return true;
    },
    frontmostTarget,
  };
  const service = createWindowManagementService({
    manager: macosWindowManager,
    ports,
    engine: { computeTargetFrame, pickDisplayForFrame },
    supported: () => hasCapability('window-management'),
  });
  return {
    service,
    resolveInvocation: (scope: WindowInvocationScope | undefined) =>
      windowInvocationFromScope(scope, ports.paletteFocusTarget),
  };
}
