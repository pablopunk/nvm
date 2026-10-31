import type { ShortcutPlatform } from '../shared/shortcut-utils';
import { shortcutLabel } from './ui';

export function actionsShortcutHint(
  platform: ShortcutPlatform = globalThis.window?.nvm?.platform ?? 'darwin',
) {
  return shortcutLabel(
    platform === 'darwin' ? 'Command+K' : 'Control+K',
    platform,
  );
}
