import type { ExtensionWindowManagementOptions } from '../resources/nevermind-extension-api';
import type { WindowManagementSettings } from './window-management-settings-model';

type ResolvedOuterGap = NonNullable<
  ExtensionWindowManagementOptions['outerGap']
>;

function resolveOuterGapEdges(
  settings: WindowManagementSettings,
): Required<ResolvedOuterGap> {
  return {
    top: settings.outerGapTop ?? settings.outerGap,
    right: settings.outerGapRight ?? settings.outerGap,
    bottom: settings.outerGapBottom ?? settings.outerGap,
    left: settings.outerGapLeft ?? settings.outerGap,
  };
}

export function windowManagementOptionsFor(
  settings: WindowManagementSettings,
): ExtensionWindowManagementOptions {
  return {
    outerGap: resolveOuterGapEdges(settings),
    innerGap: settings.innerGap,
    applyGapToMaximize: settings.applyGapToMaximize,
    cycle: settings.cycle,
    cycleOrder: settings.cycleOrder,
    displayOrder: settings.displayOrder,
    displayMoveSizing: settings.displayMoveSizing,
  };
}
