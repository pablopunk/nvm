export const EXTENSION_WINDOW_CAPABILITIES = [
  'windows.always-on-top',
  'windows.all-spaces',
  'windows.frame-restore',
  'windows.display-recovery',
] as const;

export type ExtensionWindowCapability =
  (typeof EXTENSION_WINDOW_CAPABILITIES)[number];

export type ExtensionWindowSession = 'x11' | 'wayland';

export function hasExtensionWindowCapability(
  capability: ExtensionWindowCapability,
  platform: NodeJS.Platform,
  linuxSession: ExtensionWindowSession,
) {
  if (capability === 'windows.always-on-top') {
    return true;
  }
  if (capability === 'windows.all-spaces') {
    return platform !== 'win32';
  }
  if (
    capability === 'windows.frame-restore' ||
    capability === 'windows.display-recovery'
  ) {
    return platform !== 'linux' || linuxSession !== 'wayland';
  }
  return false;
}
