import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createSettingsService } from './settings-service';

test('does not change the update channel while an installation is in progress', async () => {
  let commits = 0;
  const service = createSettingsService({
    userStateStore: { get: () => ({ settings: {} }) as never },
    commitPortable: async () => {
      commits += 1;
      return { ok: true };
    },
    activationFailure: () => null,
    hasCapability: () => true,
    getLaunchAtLoginEnabled: () => false,
    setLaunchAtLoginEnabled: () => ({ ok: true }),
    osLabel: () => 'test',
    patchOpenView: () => {},
    buildRecordShortcutAction: () => ({}),
    canChangeUpdateChannel: () => false,
  });
  const result = await service.setSetting('updateChannel', 'nightly');
  assert.equal(result.ok, false);
  assert.match(result.message, /installation/);
  assert.equal(commits, 0);
});

function createMenuBarSettingsService(settings: Record<string, unknown> = {}) {
  return createSettingsService({
    userStateStore: { get: () => ({ settings }) as never },
    commitPortable: async () => ({ ok: true }),
    activationFailure: () => null,
    hasCapability: () => true,
    getLaunchAtLoginEnabled: () => false,
    setLaunchAtLoginEnabled: () => ({ ok: true }),
    osLabel: () => 'test',
    patchOpenView: () => {},
    buildRecordShortcutAction: () => ({}),
  });
}

test('the menu bar icon style defaults to the filled bolt', () => {
  assert.equal(
    createMenuBarSettingsService().getSetting('menuBarIconStyle'),
    'bolt-filled',
  );
});

test('choice settings list every other option in the Cmd+K panel', () => {
  const service = createMenuBarSettingsService();
  const row = service
    .settingsItems()
    .find((item) => item.id === 'setting:menuBarIconStyle');
  const choices = row.actionPanel.sections[1].actions;

  assert.equal(choices.length, 6);
  assert.deepEqual(choices[0], {
    type: 'toggleSetting',
    title: 'Use Bolt Outline',
    settingId: 'menuBarIconStyle',
    value: 'bolt-outline',
  });
});

test('Settings offers the app menu bar icon toggle', () => {
  const row = createMenuBarSettingsService()
    .settingsItems()
    .find((item) => item.id === 'setting:menu-bar:nevermind.menubar');
  assert.equal(row.title, 'Show Menu Bar Icon');
  assert.deepEqual(row.accessories, [{ text: 'On' }]);
});
