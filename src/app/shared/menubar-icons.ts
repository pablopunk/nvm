/**
 * Lucide icons pre-rendered for menu bar and tray status items. Native status
 * areas cannot load SVG, so `scripts/render-menubar-icons.cjs` rasterizes this
 * set into `src/app/resources/menubar-icons`. Keep this list and
 * `ExtensionMenuBarIcon` in the extension API declaration in sync.
 */
export const MENUBAR_ICON_NAMES = [
  'zap',
  'circle',
  'circle-dot',
  'circle-check',
  'circle-alert',
  'circle-x',
  'bell',
  'bell-dot',
  'timer',
  'clock',
  'alarm-clock',
  'calendar',
  'coffee',
  'mic',
  'mic-off',
  'video',
  'music',
  'play',
  'pause',
  'activity',
  'cpu',
  'hard-drive',
  'memory-stick',
  'wifi',
  'battery-medium',
  'cloud',
  'cloud-sun',
  'sun',
  'moon',
  'git-pull-request',
  'git-branch',
  'git-commit-horizontal',
  'terminal',
  'code',
  'bug',
  'rocket',
  'server',
  'globe',
  'mail',
  'message-square',
  'inbox',
  'list-todo',
  'square-check',
  'star',
  'heart',
  'flame',
  'eye',
  'lock',
  'shield',
  'sparkles',
  'bot',
  'clipboard',
  'sticky-note',
  'folder',
  'download',
  'upload',
  'refresh-cw',
  'chart-line',
  'gauge',
  'thermometer',
  'droplet',
  'house',
  'briefcase',
  'dollar-sign',
] as const;

export type MenuBarIconName = (typeof MENUBAR_ICON_NAMES)[number];

export const DEFAULT_MENUBAR_ICON: MenuBarIconName = 'zap';

const MENUBAR_ICON_SET = new Set<string>(MENUBAR_ICON_NAMES);

export function isMenuBarIconName(value: unknown): value is MenuBarIconName {
  return typeof value === 'string' && MENUBAR_ICON_SET.has(value);
}

/** Point size of a status item glyph; assets ship at @1x and @2x. */
export const MENUBAR_ICON_POINT_SIZE = 16;
