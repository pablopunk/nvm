import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createDefaultPortableConfig,
  type PortableConfig,
} from '../state/portable-config-schema';
import {
  type PortableConfigEffects,
  reconcilePortableConfigChange,
} from './portable-config-reconcile';

function recordingEffects(overrides: Partial<PortableConfigEffects> = {}) {
  const calls: string[] = [];
  const effects: PortableConfigEffects = {
    applyPaletteHotkey: (accelerator) => {
      calls.push(`hotkey:${accelerator}`);
      return true;
    },
    setLaunchAtLogin: (enabled) => {
      calls.push(`login:${enabled}`);
      return { ok: true };
    },
    setDiagnosticReporting: (enabled) => calls.push(`diagnostics:${enabled}`),
    settingChanged: (id) => calls.push(`setting:${id}`),
    registerActionShortcuts: () => calls.push('shortcuts:register'),
    invalidateShortcutCaches: () => calls.push('shortcuts:invalidate'),
    patchKeyboardShortcutsView: () => calls.push('shortcuts:view'),
    invalidateExtensionRootItems: () => calls.push('root:invalidate'),
    applyJobEnablement: (overrides) =>
      calls.push(`jobs:${JSON.stringify(overrides)}`),
    ...overrides,
  };
  return { effects, calls };
}

function configWith(patch: Partial<PortableConfig>): PortableConfig {
  return { ...createDefaultPortableConfig(), ...patch };
}

test('an unchanged config triggers no live effects', () => {
  const { effects, calls } = recordingEffects();
  const config = configWith({ settings: { paletteHotkey: 'Command+Alt+K' } });
  reconcilePortableConfigChange(config, config, effects);
  assert.deepEqual(calls, []);
});

test('changed settings, aliases, shortcuts and jobs each apply their live effect', () => {
  const { effects, calls } = recordingEffects();
  const previous = createDefaultPortableConfig();
  const next = configWith({
    settings: { paletteHotkey: 'Command+Alt+J', errorReporting: false },
    aliases: { 'default:calculator': ['math'] },
    shortcuts: { 'extension-action:demo:open': null },
    jobs: { enabled: { 'demo.sync': false } },
  });
  reconcilePortableConfigChange(previous, next, effects);
  assert.ok(calls.includes('hotkey:Command+Alt+J'));
  assert.ok(calls.includes('diagnostics:false'));
  assert.ok(calls.includes('setting:errorReporting'));
  assert.ok(calls.includes('root:invalidate'));
  assert.ok(calls.includes('shortcuts:register'));
  assert.ok(calls.includes('jobs:{"demo.sync":false}'));
});

test('a failing effect is contained and the remaining effects still run', () => {
  const { effects, calls } = recordingEffects({
    applyPaletteHotkey: () => {
      throw new Error('registration exploded');
    },
  });
  const next = configWith({
    settings: { paletteHotkey: 'Command+Alt+J' },
    aliases: { a: ['b'] },
  });
  reconcilePortableConfigChange(createDefaultPortableConfig(), next, effects);
  assert.ok(calls.includes('root:invalidate'));
});

test('a rejected hotkey registration is reported without throwing', () => {
  const { effects, calls } = recordingEffects({
    applyPaletteHotkey: () => false,
  });
  const next = configWith({ settings: { paletteHotkey: 'Command+Alt+J' } });
  reconcilePortableConfigChange(createDefaultPortableConfig(), next, effects);
  assert.ok(calls.includes('setting:paletteHotkey'));
});
