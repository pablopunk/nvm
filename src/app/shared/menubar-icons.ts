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

/** Nevermind's own status item styles; host-only glyphs extensions cannot use. */
export const APP_MENU_BAR_ICON_STYLES = [
  { value: 'bolt-filled', label: 'Filled Bolt', tint: 'template' },
  { value: 'bolt-outline', label: 'Bolt Outline', tint: 'template' },
  { value: 'emoji', label: 'Emoji ⚡', tint: 'color' },
  { value: 'app-icon', label: 'App Icon', tint: 'color' },
  { value: 'gold-outline', label: 'Gold Bolt Outline', tint: 'color' },
  { value: 'gold-filled', label: 'Gold Bolt', tint: 'color' },
  { value: 'gold-glossy', label: 'Glossy Gold Bolt', tint: 'color' },
] as const;

export type AppMenuBarIconStyle =
  (typeof APP_MENU_BAR_ICON_STYLES)[number]['value'];
export type AppMenuBarGlyph = `nevermind-${AppMenuBarIconStyle}`;
export type StatusItemGlyph = MenuBarIconName | AppMenuBarGlyph;

export const DEFAULT_APP_MENU_BAR_ICON_STYLE: AppMenuBarIconStyle =
  'bolt-filled';

/** macOS shows the native Apple emoji as the status title instead of an image. */
export const APP_MENU_BAR_EMOJI = '⚡';

function appMenuBarStyle(glyph: unknown) {
  return APP_MENU_BAR_ICON_STYLES.find(
    (style) => `nevermind-${style.value}` === glyph,
  );
}

export function appMenuBarGlyph(style: unknown): AppMenuBarGlyph {
  const known = APP_MENU_BAR_ICON_STYLES.find((item) => item.value === style);
  return `nevermind-${known?.value ?? DEFAULT_APP_MENU_BAR_ICON_STYLE}`;
}

export function isAppMenuBarGlyph(value: unknown): value is AppMenuBarGlyph {
  return Boolean(appMenuBarStyle(value));
}

/** Template glyphs follow light/dark; colored ones ship a single variant. */
export function glyphFollowsTheme(glyph: StatusItemGlyph) {
  return appMenuBarStyle(glyph)?.tint !== 'color';
}

export function glyphIsEmoji(glyph: StatusItemGlyph) {
  return glyph === 'nevermind-emoji';
}
