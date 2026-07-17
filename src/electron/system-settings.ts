import fs from 'node:fs/promises';

interface SystemSettingsEntry {
  id: string;
  title: string;
  aliases?: string[];
}

const SYSTEM_SETTINGS_URL = 'x-apple.systempreferences:';
const SYSTEM_SETTINGS_SIDEBAR =
  '/System/Applications/System Settings.app/Contents/Resources/Sidebar.plist';

const MAC_SYSTEM_SETTINGS_ENTRIES: SystemSettingsEntry[] = [
  {
    id: 'com.apple.systempreferences.AppleIDSettings',
    title: 'Apple Account',
    aliases: ['Apple ID', 'iCloud'],
  },
  { id: 'com.apple.Family-Settings.extension', title: 'Family' },
  {
    id: 'com.apple.wifi-settings-extension',
    title: 'Wi-Fi',
    aliases: ['WiFi', 'Wireless'],
  },
  { id: 'com.apple.BluetoothSettings', title: 'Bluetooth' },
  { id: 'com.apple.Network-Settings.extension', title: 'Network' },
  {
    id: 'com.apple.Notifications-Settings.extension',
    title: 'Notifications',
  },
  { id: 'com.apple.Sound-Settings.extension', title: 'Sound' },
  { id: 'com.apple.Focus-Settings.extension', title: 'Focus' },
  { id: 'com.apple.Screen-Time-Settings.extension', title: 'Screen Time' },
  { id: 'com.apple.systempreferences.GeneralSettings', title: 'General' },
  { id: 'com.apple.Appearance-Settings.extension', title: 'Appearance' },
  {
    id: 'com.apple.Accessibility-Settings.extension',
    title: 'Accessibility',
  },
  {
    id: 'com.apple.ControlCenter-Settings.extension',
    title: 'Control Center',
  },
  {
    id: 'com.apple.Siri-Settings.extension',
    title: 'Apple Intelligence & Siri',
    aliases: ['Siri'],
  },
  { id: 'com.apple.Spotlight-Settings.extension', title: 'Spotlight' },
  {
    id: 'com.apple.settings.PrivacySecurity.extension',
    title: 'Privacy & Security',
    aliases: ['Privacy', 'Security'],
  },
  {
    id: 'com.apple.Desktop-Settings.extension',
    title: 'Desktop & Dock',
    aliases: ['Desktop', 'Dock'],
  },
  { id: 'com.apple.Displays-Settings.extension', title: 'Displays' },
  { id: 'com.apple.Wallpaper-Settings.extension', title: 'Wallpaper' },
  { id: 'com.apple.Battery-Settings.extension', title: 'Battery' },
  { id: 'com.apple.Lock-Screen-Settings.extension', title: 'Lock Screen' },
  {
    id: 'com.apple.Touch-ID-Settings.extension',
    title: 'Touch ID & Password',
    aliases: ['Touch ID', 'Password'],
  },
  {
    id: 'com.apple.Users-Groups-Settings.extension',
    title: 'Users & Groups',
    aliases: ['Users', 'Groups'],
  },
  { id: 'com.apple.Passwords-Settings.extension', title: 'Passwords' },
  {
    id: 'com.apple.Internet-Accounts-Settings.extension',
    title: 'Internet Accounts',
    aliases: ['Accounts'],
  },
  { id: 'com.apple.Game-Center-Settings.extension', title: 'Game Center' },
  { id: 'com.apple.WalletSettingsExtension', title: 'Wallet & Apple Pay' },
  { id: 'com.apple.Keyboard-Settings.extension', title: 'Keyboard' },
  { id: 'com.apple.Mouse-Settings.extension', title: 'Mouse' },
  { id: 'com.apple.Trackpad-Settings.extension', title: 'Trackpad' },
  {
    id: 'com.apple.Game-Controller-Settings.extension',
    title: 'Game Controllers',
  },
  {
    id: 'com.apple.CD-DVD-Settings.extension',
    title: 'CDs & DVDs',
    aliases: ['CD', 'DVD'],
  },
  {
    id: 'com.apple.Print-Scan-Settings.extension',
    title: 'Printers & Scanners',
    aliases: ['Printers', 'Scanners'],
  },
];

const MAC_SYSTEM_SETTINGS_BY_ID = new Map(
  MAC_SYSTEM_SETTINGS_ENTRIES.map((entry) => [entry.id, entry] as const),
);

let systemSettingsEntriesPromise: Promise<SystemSettingsEntry[]> | null = null;

function systemSettingsEntriesFromSidebar(sidebarPlist: string) {
  const entries: SystemSettingsEntry[] = [];
  const seen = new Set<string>();
  for (const match of sidebarPlist.matchAll(/<string>([^<]+)<\/string>/g)) {
    const id = match[1];
    const entry = MAC_SYSTEM_SETTINGS_BY_ID.get(id);
    if (!(entry && !seen.has(id))) {
      continue;
    }
    entries.push(entry);
    seen.add(id);
  }
  return entries;
}

function systemSettingsPaneUrl(
  paneId?: string,
  platform: NodeJS.Platform = process.platform,
) {
  if (
    !(platform === 'darwin' && paneId && MAC_SYSTEM_SETTINGS_BY_ID.has(paneId))
  ) {
    return null;
  }
  return `${SYSTEM_SETTINGS_URL}${paneId}`;
}

function systemSettingsEntries() {
  if (process.platform !== 'darwin') {
    return Promise.resolve([] as SystemSettingsEntry[]);
  }
  if (!systemSettingsEntriesPromise) {
    systemSettingsEntriesPromise = fs
      .readFile(SYSTEM_SETTINGS_SIDEBAR, 'utf8')
      .then(systemSettingsEntriesFromSidebar)
      .catch(() => []);
  }
  return systemSettingsEntriesPromise;
}

export type { SystemSettingsEntry };
export {
  systemSettingsEntries,
  systemSettingsEntriesFromSidebar,
  systemSettingsPaneUrl,
};
