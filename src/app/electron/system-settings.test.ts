import assert from 'node:assert/strict';
import test from 'node:test';
import {
  systemSettingsEntriesFromSidebar,
  systemSettingsPaneUrl,
} from './system-settings';

test('system settings entries follow the current macOS sidebar and omit unknown panes', () => {
  const entries = systemSettingsEntriesFromSidebar(`
    <plist><array>
      <string>com.apple.wifi-settings-extension</string>
      <string>com.apple.unknown-settings-extension</string>
      <string>com.apple.settings.PrivacySecurity.extension</string>
      <string>com.apple.wifi-settings-extension</string>
    </array></plist>
  `);

  assert.deepEqual(entries, [
    {
      id: 'com.apple.wifi-settings-extension',
      title: 'Wi-Fi',
      aliases: ['WiFi', 'Wireless'],
    },
    {
      id: 'com.apple.settings.PrivacySecurity.extension',
      title: 'Privacy & Security',
      aliases: ['Privacy', 'Security'],
    },
  ]);
});

test('system settings URLs only accept known pane identifiers', () => {
  assert.equal(
    systemSettingsPaneUrl('com.apple.Keyboard-Settings.extension', 'darwin'),
    'x-apple.systempreferences:com.apple.Keyboard-Settings.extension',
  );
  assert.equal(systemSettingsPaneUrl('javascript:alert(1)', 'darwin'), null);
  assert.equal(
    systemSettingsPaneUrl('com.apple.Keyboard-Settings.extension', 'win32'),
    null,
  );
  assert.equal(systemSettingsPaneUrl(undefined, 'darwin'), null);
});
