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
