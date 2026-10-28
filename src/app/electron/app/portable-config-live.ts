import { reservedPaletteShortcutName } from '../os/os';
import type { ActivationStatus } from '../platform/activation-status';
import { isSpotlightAccelerator } from '../shortcuts/shortcut-utils';
import type { PortableConfig } from '../state/portable-config-schema';
import type { LiveFailure, PortableLive } from './portable-config-service';
import { type SettingsState, settingValue } from './settings';

export type PortableLiveDeps = {
  registerPaletteHotkey: (accelerator: string) => {
    registered: boolean;
    configured: boolean;
  };
  setLaunchAtLogin: (enabled: boolean) => { ok: boolean; message?: string };
  setDiagnosticReporting: (enabled: boolean) => void;
  patchSettingsView: (settingId: string) => void;
  invalidateExtensionRootItems: () => void;
  invalidateShortcutCaches: () => void;
  patchKeyboardShortcutsView: () => void;
  applyJobEnablement: (overrides: Record<string, boolean>) => void;
  applyShortcutView: (view: PortableConfig) => Map<string, string>;
  activation: ActivationStatus;
};

const SETTING_PREFIX = 'setting:';
const SHORTCUT_PREFIX = 'shortcut:';

function anyKeyWithPrefix(keys: ReadonlySet<string>, prefix: string) {
  return [...keys].some((key) => key.startsWith(prefix));
}

export function createPortableConfigLive(deps: PortableLiveDeps): PortableLive {
  let shortcutIdsWithFailures = new Set<string>();

  function paletteHotkeyFailure(value: unknown) {
    const accelerator = String(value ?? '');
    const status = deps.registerPaletteHotkey(accelerator);
    if (status.registered || !status.configured) return null;
    return isSpotlightAccelerator(accelerator)
      ? `${accelerator} is used by ${reservedPaletteShortcutName()}`
      : `Could not register ${accelerator}`;
  }

  function launchAtLoginFailure(enabled: boolean) {
    const result = deps.setLaunchAtLogin(enabled);
    return result.ok
      ? null
      : (result.message ?? 'Launch at login could not be changed');
  }

  function applyShortcutKeys(view: PortableConfig): LiveFailure[] {
    const failures = deps.applyShortcutView(view);
    const ids = new Set([
      ...Object.keys(view.shortcuts),
      ...failures.keys(),
      ...shortcutIdsWithFailures,
    ]);
    const reported: LiveFailure[] = [];
    for (const id of ids) {
      const message = failures.get(id) ?? null;
      deps.activation.set(`${SHORTCUT_PREFIX}${id}`, message);
      if (message) reported.push({ key: `${SHORTCUT_PREFIX}${id}`, message });
    }
    shortcutIdsWithFailures = new Set(failures.keys());
    deps.invalidateShortcutCaches();
    deps.patchKeyboardShortcutsView();
    return reported;
  }

  function applySettingKeys(
    keys: ReadonlySet<string>,
    after: PortableConfig,
    failures: LiveFailure[],
  ) {
    const settle = (key: string, message: string | null) => {
      deps.activation.set(key, message);
      if (message) failures.push({ key, message });
    };
    if (keys.has(`${SETTING_PREFIX}paletteHotkey`))
      settle(
        `${SETTING_PREFIX}paletteHotkey`,
        paletteHotkeyFailure(after.settings.paletteHotkey),
      );
    if (keys.has(`${SETTING_PREFIX}startAtLogin`))
      settle(
        `${SETTING_PREFIX}startAtLogin`,
        launchAtLoginFailure(
          settingValue(after.settings as SettingsState, 'startAtLogin') ===
            true,
        ),
      );
    for (const key of keys) {
      if (!key.startsWith(SETTING_PREFIX)) continue;
      const settingId = key.slice(SETTING_PREFIX.length);
      if (settingId === 'errorReporting')
        deps.setDiagnosticReporting(
          settingValue(after.settings as SettingsState, 'errorReporting') ===
            true,
        );
      deps.patchSettingsView(settingId);
    }
  }

  function apply(
    _before: PortableConfig,
    after: PortableConfig,
    keys: string[],
  ) {
    const relevant = new Set(keys);
    const failures: LiveFailure[] = [];
    applySettingKeys(relevant, after, failures);
    if (anyKeyWithPrefix(relevant, 'alias:'))
      deps.invalidateExtensionRootItems();
    if (anyKeyWithPrefix(relevant, 'job:'))
      deps.applyJobEnablement(after.jobs.enabled);
    if (anyKeyWithPrefix(relevant, SHORTCUT_PREFIX))
      failures.push(...applyShortcutKeys(after));
    return failures;
  }

  return {
    apply,
    applyShortcuts: (view) => applyShortcutKeys(view),
  };
}
