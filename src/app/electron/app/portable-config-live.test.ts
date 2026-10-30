import assert from 'node:assert/strict';
import test, { mock } from 'node:test';
import { createActivationStatus } from '../platform/activation-status';
mock.module('../os/os', {
  namedExports: { reservedPaletteShortcutName: () => 'Spotlight' },
});
const { createPortableConfigLive } = await import('./portable-config-live');

function liveWithPatchedSettings() {
  const patched: string[] = [];
  const live = createPortableConfigLive({
    registerPaletteHotkey: () => ({ registered: true, configured: true }),
    setLaunchAtLogin: () => ({ ok: true }),
    setDiagnosticReporting: () => {},
    patchSettingsView: (settingId) => patched.push(settingId),
    invalidateExtensionRootItems: () => {},
    invalidateShortcutCaches: () => {},
    patchKeyboardShortcutsView: () => {},
    applyJobEnablement: () => {},
    applyShortcutView: () => new Map(),
    activation: createActivationStatus(),
  });
  return { live, patched };
}

test('saving a shortcut setting patches the open Settings view in place', () => {
  const { live, patched } = liveWithPatchedSettings();
  const after = { settings: { paletteHotkey: 'Control+Alt+N' } } as any;
  const failures = live.apply({} as any, after, [
    'setting:paletteHotkey',
    'setting:hyperKey',
  ]);
  assert.deepEqual(failures, []);
  assert.deepEqual(patched, ['paletteHotkey', 'hyperKey']);
});
