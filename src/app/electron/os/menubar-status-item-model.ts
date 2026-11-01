// Pure status item rules: what each platform shows and how menus are built.
import type { MenuItemConstructorOptions, NativeImage } from 'electron';
import {
  APP_MENU_BAR_EMOJI,
  DEFAULT_MENUBAR_ICON,
  glyphFollowsTheme,
  glyphIsEmoji,
  isAppMenuBarGlyph,
  isMenuBarIconName,
  type StatusItemGlyph,
} from '../../shared/menubar-icons';

export type StatusItemMenuEntry =
  | { type: 'separator' }
  | { type: 'header'; label: string }
  | {
      type: 'item';
      label: string;
      sublabel?: string;
      icon?: StatusItemGlyph;
      checked?: boolean;
      enabled?: boolean;
      accelerator?: string;
      submenu?: StatusItemMenuEntry[];
      onClick?: () => void;
    };

export type StatusItemSpec = {
  icon: StatusItemGlyph;
  title?: string;
  tooltip: string;
  menu: StatusItemMenuEntry[];
  /** Runs on left click instead of opening the menu; the menu moves to right click. */
  onPrimaryClick?: () => void;
};

export type StatusItemPlatform = 'darwin' | 'win32' | 'linux';

export function menuBarStatusItemsPlatform(
  platform: NodeJS.Platform,
): StatusItemPlatform | null {
  return platform === 'darwin' || platform === 'win32' || platform === 'linux'
    ? platform
    : null;
}

function signatureWithoutHandlers(value: unknown) {
  return JSON.stringify(value, (_key, entry) =>
    typeof entry === 'function' ? undefined : entry,
  );
}

export function statusItemAppearanceSignature(spec: StatusItemSpec) {
  return signatureWithoutHandlers([spec.icon, spec.title, spec.tooltip]);
}

export function statusItemMenuSignature(spec: StatusItemSpec) {
  return signatureWithoutHandlers([spec.menu, Boolean(spec.onPrimaryClick)]);
}

/** Linux tray hosts may never report clicks, so the primary action also lives in the menu. */
export function statusItemMenuEntries(
  spec: StatusItemSpec,
  platform: StatusItemPlatform,
  runPrimaryClick: () => void,
): StatusItemMenuEntry[] {
  if (!(platform === 'linux' && spec.onPrimaryClick)) return spec.menu;
  return [
    { type: 'item', label: `Open ${spec.tooltip}`, onClick: runPrimaryClick },
    ...(spec.menu.length ? [{ type: 'separator' as const }] : []),
    ...spec.menu,
  ];
}

/** With a primary click, macOS and Windows show the menu only on right click. */
export function statusItemOwnsContextMenu(
  spec: StatusItemSpec,
  platform: StatusItemPlatform,
) {
  return platform === 'linux' || !spec.onPrimaryClick;
}

export function usesTemplateImages(platform: StatusItemPlatform) {
  return platform === 'darwin';
}

export function supportsStatusTitle(platform: StatusItemPlatform) {
  return platform === 'darwin';
}

function supportsMenuSublabels(platform: StatusItemPlatform) {
  return platform === 'darwin';
}

function supportsMenuAccelerators(platform: StatusItemPlatform) {
  return platform !== 'linux';
}

function supportsMenuItemIcons(platform: StatusItemPlatform) {
  return platform === 'darwin';
}

const LINUX_MENU_REBUILD_COOLDOWN_MS = 5000;

/** Linux trays report no menu open/close, and a rebuild replaces an open menu. */
export function menuRebuildCooldownMs(platform: StatusItemPlatform) {
  return platform === 'linux' ? LINUX_MENU_REBUILD_COOLDOWN_MS : 0;
}

export function opensMenuOnPrimaryClick(platform: StatusItemPlatform) {
  return platform === 'win32';
}

function knownGlyph(icon: StatusItemGlyph): StatusItemGlyph {
  return isMenuBarIconName(icon) || isAppMenuBarGlyph(icon)
    ? icon
    : DEFAULT_MENUBAR_ICON;
}

export function statusItemIconFile(
  icon: StatusItemGlyph,
  platform: StatusItemPlatform,
  darkBackground: boolean,
) {
  const name = knownGlyph(icon);
  const whiteVariant =
    glyphFollowsTheme(name) && !usesTemplateImages(platform) && darkBackground;
  return whiteVariant ? `${name}-white.png` : `${name}.png`;
}

/** The native emoji renders as the macOS status title, so it needs no image. */
export function statusItemUsesEmojiTitle(
  icon: StatusItemGlyph,
  platform: StatusItemPlatform,
) {
  return platform === 'darwin' && glyphIsEmoji(icon);
}

export function statusItemTitle(
  spec: StatusItemSpec,
  platform: StatusItemPlatform,
) {
  const title = spec.title || '';
  if (!statusItemUsesEmojiTitle(spec.icon, platform)) return title;
  return title ? `${APP_MENU_BAR_EMOJI} ${title}` : APP_MENU_BAR_EMOJI;
}

const ALWAYS_DARK_LINUX_PANELS = /gnome|unity|ubuntu|pantheon|budgie/i;

/** Tray glyphs must contrast with the panel, which does not always follow the app theme. */
export function statusAreaIsDark(input: {
  platform: StatusItemPlatform;
  desktopEnvironment?: string;
  shouldUseDarkColors: boolean;
  shouldUseDarkColorsForSystemIntegratedUI?: boolean;
}) {
  if (input.platform === 'win32')
    return (
      input.shouldUseDarkColorsForSystemIntegratedUI ??
      input.shouldUseDarkColors
    );
  if (input.platform === 'linux')
    return (
      ALWAYS_DARK_LINUX_PANELS.test(input.desktopEnvironment || '') ||
      input.shouldUseDarkColors
    );
  return input.shouldUseDarkColors;
}

export function statusItemTooltip(
  spec: StatusItemSpec,
  platform: StatusItemPlatform,
) {
  if (supportsStatusTitle(platform) || !spec.title) return spec.tooltip;
  return `${spec.tooltip} — ${spec.title}`;
}

export function statusItemMenuTemplate(
  entries: StatusItemMenuEntry[],
  platform: StatusItemPlatform,
  menuIcon: (icon: StatusItemGlyph) => NativeImage,
): MenuItemConstructorOptions[] {
  return entries.map((entry): MenuItemConstructorOptions => {
    if (entry.type === 'separator') return { type: 'separator' };
    if (entry.type === 'header') return { label: entry.label, enabled: false };
    const label =
      entry.sublabel && !supportsMenuSublabels(platform)
        ? `${entry.label} — ${entry.sublabel}`
        : entry.label;
    return {
      label,
      ...(entry.sublabel && supportsMenuSublabels(platform)
        ? { sublabel: entry.sublabel }
        : {}),
      ...(entry.checked !== undefined
        ? { type: 'checkbox' as const, checked: entry.checked }
        : {}),
      enabled: entry.enabled !== false,
      ...(entry.accelerator && supportsMenuAccelerators(platform)
        ? { accelerator: entry.accelerator, registerAccelerator: false }
        : {}),
      ...(entry.icon && supportsMenuItemIcons(platform)
        ? { icon: menuIcon(entry.icon) }
        : {}),
      ...(entry.submenu?.length
        ? { submenu: statusItemMenuTemplate(entry.submenu, platform, menuIcon) }
        : {}),
      ...(entry.onClick ? { click: entry.onClick } : {}),
    };
  });
}

export function menuEntryAtPath(
  entries: StatusItemMenuEntry[],
  path: number[],
): StatusItemMenuEntry | undefined {
  let list: StatusItemMenuEntry[] | undefined = entries;
  let entry: StatusItemMenuEntry | undefined;
  for (const index of path) {
    entry = list?.[index];
    list = entry?.type === 'item' ? entry.submenu : undefined;
  }
  return entry;
}
