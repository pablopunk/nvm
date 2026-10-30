import type { ShortcutPlatform } from '../shared/shortcut-utils';
import { shortcutLabel } from './ui';

export function actionsShortcutHint(platform?: ShortcutPlatform) {
  return shortcutLabel('Command+K', platform);
}
