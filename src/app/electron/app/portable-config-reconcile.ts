import { error as logError } from '../platform/logger';
import {
  diffPortableConfig,
  type PortableChange,
} from '../state/portable-config-projection';
import type { PortableConfig } from '../state/portable-config-schema';
import { type SettingId, type SettingsState, settingValue } from './settings';

export type PortableConfigEffects = {
  applyPaletteHotkey: (accelerator: string) => boolean;
  setLaunchAtLogin: (enabled: boolean) => { ok: boolean; message?: string };
  setDiagnosticReporting: (enabled: boolean) => void;
  settingChanged: (settingId: string) => void;
  registerActionShortcuts: () => void;
  invalidateShortcutCaches: () => void;
  patchKeyboardShortcutsView: () => void;
  invalidateExtensionRootItems: () => void;
  applyJobEnablement: (overrides: Record<string, boolean>) => void;
};

function changesUnder(changes: PortableChange[], section: string) {
  return changes.filter((change) => change.path[0] === section);
}

function changedSettingIds(changes: PortableChange[]) {
  return changesUnder(changes, 'settings').map((change) => change.path[1]);
}

function runEffect(name: string, effect: () => void) {
  try {
    effect();
  } catch (error) {
    logError(
      'config.reconcile.failed',
      {
        effect: name,
        error: error instanceof Error ? error.message : String(error),
      },
      { source: 'host', scope: 'state' },
    );
  }
}

function reportFailedAccelerator(settingId: string, accelerator: string) {
  logError(
    'config.reconcile.hotkey.failed',
    new Error(`Could not register ${accelerator} for ${settingId}`),
    { source: 'host', scope: 'state' },
  );
}

function reconcileSetting(
  settingId: string,
  next: PortableConfig,
  effects: PortableConfigEffects,
) {
  const value = settingValue(
    next.settings as SettingsState,
    settingId as SettingId,
  );
  if (settingId === 'paletteHotkey') {
    const registered = effects.applyPaletteHotkey(String(value ?? ''));
    if (!registered) reportFailedAccelerator(settingId, String(value ?? ''));
  }
  if (settingId === 'errorReporting')
    effects.setDiagnosticReporting(value === true);
  if (settingId === 'startAtLogin') {
    const result = effects.setLaunchAtLogin(value === true);
    if (!result.ok)
      logError(
        'config.reconcile.login-item.failed',
        new Error(result.message ?? 'Launch at login could not be changed'),
        { source: 'host', scope: 'state' },
      );
  }
  effects.settingChanged(settingId);
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: Each portable section maps to one live host effect in a single reconcile pass.
export function reconcilePortableConfigChange(
  previous: PortableConfig,
  next: PortableConfig,
  effects: PortableConfigEffects,
) {
  const changes = diffPortableConfig(previous, next);
  if (!changes.length) return;
  for (const settingId of changedSettingIds(changes))
    runEffect(`setting:${settingId}`, () =>
      reconcileSetting(settingId, next, effects),
    );
  if (changesUnder(changes, 'aliases').length)
    runEffect('aliases', () => effects.invalidateExtensionRootItems());
  if (changesUnder(changes, 'shortcuts').length)
    runEffect('shortcuts', () => {
      effects.registerActionShortcuts();
      effects.invalidateShortcutCaches();
      effects.patchKeyboardShortcutsView();
    });
  if (changesUnder(changes, 'jobs').length)
    runEffect('jobs', () => effects.applyJobEnablement(next.jobs.enabled));
}
