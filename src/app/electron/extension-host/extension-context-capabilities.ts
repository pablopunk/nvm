export function trustedExtensionApiUnavailable(capability: string) {
  return new Error(`Trusted extension API is unavailable: ${capability}`);
}

// The enable decision is the trust boundary. Manifest capabilities are review
// declarations, not runtime privileges.
export const trustedContextCapabilities = {
  canUseDesktopApps: true,
  canUseDesktopFiles: true,
  canUseClipboard: true,
  canUseSystem: true,
  canUseOcr: true,
  canUseUpdates: true,
  canUseShortcuts: true,
  canUseAi: true,
  canUseDictation: true,
  canWriteSettings: true,
  canManageExtensionOwnership: true,
};

export const denyShortcut = (name: string) => () => {
  throw trustedExtensionApiUnavailable(`shortcuts (${name})`);
};
