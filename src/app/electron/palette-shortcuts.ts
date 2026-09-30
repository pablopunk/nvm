import type { PaletteHotkeyStatus } from '../palette/preload-api';

export const LINUX_PALETTE_RECOVERY_HOTKEYS = [
  'Control+Alt+K',
  'Control+Alt+Shift+K',
  'Control+Shift+Space',
] as const;

type RegisterPaletteHotkeyOptions = {
  accelerator: string;
  platform: NodeJS.Platform;
  register: (accelerator: string, listener: () => void) => boolean;
  listener: () => void;
};

export function registerPaletteHotkey({
  accelerator,
  platform,
  register,
  listener,
}: RegisterPaletteHotkeyOptions): PaletteHotkeyStatus {
  if (!accelerator.trim())
    return { accelerator: '', configured: false, registered: false, platform };

  if (register(accelerator, listener))
    return { accelerator, configured: true, registered: true, platform };

  if (platform === 'linux') {
    for (const recoveryAccelerator of LINUX_PALETTE_RECOVERY_HOTKEYS) {
      if (recoveryAccelerator === accelerator) continue;
      if (register(recoveryAccelerator, listener))
        return {
          accelerator,
          configured: true,
          registered: false,
          recoveryAccelerator,
          platform,
        };
    }
  }

  return { accelerator, configured: true, registered: false, platform };
}

export function isPaletteHotkeyReserved(
  accelerator: string,
  configuredAccelerator: string,
  recoveryAccelerator: string | undefined,
  platform: NodeJS.Platform,
) {
  return (
    accelerator === configuredAccelerator ||
    (platform === 'linux' && accelerator === recoveryAccelerator)
  );
}
