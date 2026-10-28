import {
  type WindowInvocationScope,
  windowInvocationFromScope,
} from '../extension-host/window-invocation-scope';
import { macosWindowManager } from '../os/macos-window-manager';
import { hasCapability } from '../os/os';
import {
  adjacentDisplay,
  cycleSequence,
  moveFrameToDisplay,
} from '../os/window-display-geometry';
import { computeTargetFrame, pickDisplayForFrame } from '../os/window-layouts';
import { createWindowManagementService } from '../os/window-management';
import type {
  WindowFocusTarget,
  WindowInvocationPorts,
} from '../os/window-management-types';

type WindowManagementCompositionDependencies = {
  paletteWindow: {
    peekFocusReturnTarget(): unknown;
    hidePalette(): Promise<unknown>;
    win?: { isVisible(): boolean } | null;
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
    hidePaletteAndRestoreFocus: async (target) => {
      if (
        focusTargetFrom(paletteWindow.peekFocusReturnTarget())?.pid !==
        target.pid
      )
        return false;
      if (paletteWindow.win?.isVisible()) await paletteWindow.hidePalette();
      return true;
    },
    frontmostTarget,
  };
  const service = createWindowManagementService({
    manager: macosWindowManager,
    ports,
    engine: {
      computeTargetFrame,
      pickDisplayForFrame,
      moveFrameToDisplay,
      cycleSequence,
      adjacentDisplay: (displays, from, direction) =>
        adjacentDisplay({ displays, currentDisplayId: from.id, direction }),
    },
    supported: () => hasCapability('window-management'),
    selfPid: process.pid,
  });
  return {
    service,
    resolveInvocation: (scope: WindowInvocationScope | undefined) =>
      windowInvocationFromScope(scope, ports.paletteFocusTarget),
  };
}
